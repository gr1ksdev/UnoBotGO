package httpapi

import (
	"context"
	"encoding/json"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type profileRepo struct {
	user   int64
	before *ranking.HistoryKey
}

func (p *profileRepo) ReadProfile(_ context.Context, user int64, before *ranking.HistoryKey) (ranking.Profile, error) {
	p.user = user
	p.before = before
	out := ranking.Profile{Stats: []ranking.ProfileStats{}, History: []ranking.HistoryEntry{}}
	if before != nil {
		return out, nil
	}
	for i := 0; i < 21; i++ {
		out.History = append(out.History, ranking.HistoryEntry{ID: "test-game", Finished: time.Unix(int64(100-i), 0)})
	}
	return out, nil
}
func (p *profileRepo) ReadResult(context.Context, string, int64) ([]ranking.HistoryEntry, error) {
	return nil, nil
}
func TestProfileUsesVerifiedIdentityAndBoundHistoryCursor(t *testing.T) {
	refs, _ := NewReferences(make([]byte, 32))
	repo := &profileRepo{}
	global := &globalRepo{}
	a := &API{Profiles: repo, Rankings: &ranking.GlobalService{Repository: global}, References: refs, Token: "token", MaxAge: time.Hour}
	h := a.Handler()
	request := func(path string, user string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "tma "+signed("token", time.Now(), map[string]string{"user": user}))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("/api/v1/me", `{"id":12345,"first_name":"Nome <seguro>","last_name":"Real"}`)
	if w.Code != 200 || repo.user != 12345 {
		t.Fatal(w.Code, w.Body)
	}
	var body struct {
		ranking.Profile
		Identity item `json:"identity"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Identity.Name != "Nome <seguro> Real" || body.Identity.Avatar == "" || len(body.History) != 20 || body.Next == "" {
		t.Fatal(body)
	}
	if strings.Contains(w.Body.String(), "12345") {
		t.Fatal("raw identity leaked")
	}
	w = request("/api/v1/me?cursor="+body.Next, `{"id":12345}`)
	if w.Code != 200 || repo.before == nil || repo.before.ID != "test-game" {
		t.Fatal(w.Code, repo.before)
	}
	w = request("/api/v1/me?cursor="+body.Next, `{"id":999}`)
	if w.Code != 400 {
		t.Fatal("another identity used history cursor", w.Code)
	}
	w = request("/api/v1/me/position?system=updated", `{"id":12345}`)
	if w.Code != 200 || global.requests[len(global.requests)-1].LookupID != 12345 {
		t.Fatal("personal position did not use authenticated identity", w.Code)
	}
}
