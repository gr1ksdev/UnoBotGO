package httpapi

import (
	"context"
	"encoding/json"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type globalRepo struct {
	requests []ranking.GlobalRequest
	page     ranking.GlobalPage
}

func (r *globalRepo) ReadGlobalRanking(_ context.Context, q ranking.GlobalRequest) (ranking.GlobalPage, error) {
	r.requests = append(r.requests, q)
	return r.page, nil
}
func TestAPI(t *testing.T) {
	now := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	repo := &globalRepo{page: ranking.GlobalPage{Rows: []ranking.GlobalRow{{ID: -100123456789, Name: "UNO & 🃏", Score: 9007199254740993, Position: 1, Activity: now}}, More: true}}
	refs, _ := NewReferences(make([]byte, 32))
	a := &API{Rankings: &ranking.GlobalService{Repository: repo, Now: func() time.Time { return now }}, References: refs, Token: "token", MaxAge: time.Hour, Now: func() time.Time { return now }}
	h := a.Handler()
	request := func(path string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if auth {
			r.Header.Set("Authorization", "tma "+signed("token", now, nil))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("/api/v1/rankings/groups", true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "100123456789") {
		t.Fatal("raw ID leak")
	}
	var out response
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Month != "2026-09-01" || out.System != "updated" || out.Items[0].Score != "9007199254740993" || out.Items[0].MaskedID != "ID ••••6789" {
		t.Fatalf("%+v", out)
	}
	if w := request("/api/v1/rankings/groups?cursor="+out.Next, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if repo.requests[1].After == nil {
		t.Fatal("cursor not forwarded")
	}
	for _, path := range []string{"/api/v1/rankings/players?cursor=" + out.Next, "/api/v1/rankings/groups?system=legacy&cursor=" + out.Next, "/api/v1/rankings/groups?limit=101", "/api/v1/rankings/groups?system=bad", "/api/v1/rankings/groups?month=2020-01"} {
		if w := request(path, true); w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
	if w := request("/api/v1/rankings/groups", false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request("/api/v1/rankings/groups/"+out.Items[0].GroupRef, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	now = now.Add(4 * time.Hour)
	if w := request("/api/v1/rankings/groups?cursor="+out.Next, true); w.Code != 400 {
		t.Fatal("expired", w.Code)
	}
}
func TestStatic(t *testing.T) {
	h := Security(Static(fstest.MapFS{"index.html": {Data: []byte("SPA")}, "assets/app.js": {Data: []byte("js")}}))
	for _, tt := range []struct {
		path string
		code int
		body string
	}{{"/groups/opaque", 200, "SPA"}, {"/api/nope", 404, ""}, {"/assets/missing.js", 404, ""}, {"/assets/app.js", 200, "js"}, {"/.keep", 404, ""}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if w.Code != tt.code || (tt.body != "" && w.Body.String() != tt.body) {
			t.Fatal(tt, w.Code, w.Body)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing protection")
		}
	}
}

type mockMedia struct {
	getFn func(kind string, id int64) ([]byte, string, bool)
}

func (m *mockMedia) Get(kind string, id int64) ([]byte, string, bool) {
	if m.getFn != nil {
		return m.getFn(kind, id)
	}
	return nil, "", false
}

func TestAPI_MediaAndErrors(t *testing.T) {
	now := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	repo := &globalRepo{page: ranking.GlobalPage{Rows: []ranking.GlobalRow{{ID: 101, Name: "Player", Score: 100, Position: 1, Activity: now}}}}
	refs, _ := NewReferences(make([]byte, 32))
	media := &mockMedia{}
	a := &API{
		Rankings:   &ranking.GlobalService{Repository: repo, Now: func() time.Time { return now }},
		References: refs,
		Media:      media,
		Token:      "token",
		MaxAge:     time.Hour,
		Now:        func() time.Time { return now },
	}
	h := a.Handler()

	request := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "tma "+signed("token", now, nil))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	// 1. Get player item to obtain sealed avatar ref
	w := request("/api/v1/rankings/players")
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var out response
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	avatarURL := out.Items[0].Avatar

	// 2. Media pending (202)
	media.getFn = func(kind string, id int64) ([]byte, string, bool) {
		return nil, "", true
	}
	w = request(avatarURL)
	if w.Code != 202 || w.Header().Get("Retry-After") != "2" {
		t.Fatalf("expected 202 with Retry-After 2, got %d", w.Code)
	}

	// 3. Media absent (204)
	media.getFn = func(kind string, id int64) ([]byte, string, bool) {
		return nil, "", false
	}
	w = request(avatarURL)
	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// 4. Media success (200)
	media.getFn = func(kind string, id int64) ([]byte, string, bool) {
		return []byte("fake-jpeg"), "image/jpeg", false
	}
	w = request(avatarURL)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || w.Body.String() != "fake-jpeg" {
		t.Fatalf("expected 200 image/jpeg, got %d, mime %s", w.Code, w.Header().Get("Content-Type"))
	}

	// 5. Test 409 ranking_period_changed when month in cursor differs
	oldMonthRef, _ := refs.seal(reference{
		Kind:    "cursor/groups",
		System:  "updated",
		Month:   "2026-08-01",
		Expires: now.Add(time.Hour).Unix(),
		After:   &ranking.PageKey{},
	})
	w = request("/api/v1/rankings/groups?cursor=" + oldMonthRef)
	if w.Code != 409 {
		t.Fatalf("expected 409 ranking_period_changed, got %d", w.Code)
	}
}

func TestAPI_RateLimiting(t *testing.T) {
	now := time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)
	repo := &globalRepo{page: ranking.GlobalPage{}}
	refs, _ := NewReferences(make([]byte, 32))
	a := &API{
		Rankings:   &ranking.GlobalService{Repository: repo, Now: func() time.Time { return now }},
		References: refs,
		Token:      "token",
		MaxAge:     time.Hour,
		Now:        func() time.Time { return now },
	}
	h := a.Handler()

	auth := "tma " + signed("token", now, nil)
	// Send 240 requests (within limit)
	for i := 0; i < 240; i++ {
		r := httptest.NewRequest("GET", "/api/v1/rankings/groups", nil)
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("request %d failed: %d", i, w.Code)
		}
	}
	// 241st request should be rate-limited (429)
	r := httptest.NewRequest("GET", "/api/v1/rankings/groups", nil)
	r.Header.Set("Authorization", auth)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 429 || w.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected 429 with Retry-After: 60, got %d", w.Code)
	}
}
