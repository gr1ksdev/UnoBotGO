package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/uno"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roomGroupMemory struct{}

func (roomGroupMemory) WebAppGroups(context.Context, int64) ([]groups.Config, error) {
	return []groups.Config{groups.Defaults(-42)}, nil
}
func (roomGroupMemory) GetOrCreateGroupConfig(context.Context, int64) (groups.Config, error) {
	return groups.Defaults(-42), nil
}
func (roomGroupMemory) ObserveGroupTitle(context.Context, int64, string) error { return nil }
func TestDirectRoomInviteAdmissionAndIdempotency(t *testing.T) {
	svc, _ := game.NewService()
	refs, _ := NewReferences(make([]byte, 32))
	allowed := true
	a := &API{Games: svc, References: refs, Token: "token", MaxAge: time.Hour, RoomGroups: roomGroupMemory{}, BotUsername: func() string { return "ConfiguredBot" }, VerifyRoomGroup: func(_ context.Context, chat, user int64) (string, error) {
		if chat != -42 || !allowed || user == 999 {
			return "", errors.New("not member")
		}
		return "Grupo verificado", nil
	}}
	handler := a.Handler()
	request := func(method, path, body string, user int64) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "tma "+signed("token", time.Now(), map[string]string{"user": func() string {
			b, _ := json.Marshal(map[string]any{"id": user, "first_name": "Conta"})
			return string(b)
		}()}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	options := request("GET", "/api/v1/room-groups", "", 12345)
	var choices []struct {
		Ref string `json:"ref"`
	}
	_ = json.Unmarshal(options.Body.Bytes(), &choices)
	if options.Code != 200 || len(choices) != 1 {
		t.Fatal(options.Code, options.Body)
	}
	body, _ := json.Marshal(map[string]any{"group_ref": choices[0].Ref, "mode": "classic", "request_id": "create-unique"})
	created := request("POST", "/api/v1/rooms", string(body), 12345)
	var view liveView
	_ = json.Unmarshal(created.Body.Bytes(), &view)
	if created.Code != 200 || len(view.Players) != 1 || !view.Owner || view.Phase != uno.Lobby {
		t.Fatal(created.Code, created.Body)
	}
	again := request("POST", "/api/v1/rooms", string(body), 12345)
	var duplicate liveView
	_ = json.Unmarshal(again.Body.Bytes(), &duplicate)
	if again.Code != 200 || duplicate.ID != view.ID || duplicate.Revision != view.Revision {
		t.Fatal("create duplicate", again.Body)
	}
	forged := request("POST", "/api/v1/rooms", `{"chat_id":-999,"player_id":999,"mode":"classic","request_id":"forged-id"}`, 12345)
	if forged.Code != 400 {
		t.Fatal("forged group/identity accepted", forged.Code)
	}
	inv := request("GET", "/api/v1/rooms/"+string(view.ID)+"/invite", "", 12345)
	var invitation struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	_ = json.Unmarshal(inv.Body.Bytes(), &invitation)
	if inv.Code != 200 || !strings.HasPrefix(invitation.URL, "https://t.me/ConfiguredBot/ranking?startapp=join_") {
		t.Fatal(inv.Body)
	}
	preview := request("GET", "/api/v1/invites/"+invitation.Token, "", 2)
	if preview.Code != 200 {
		t.Fatal(preview.Code, preview.Body)
	}
	if request("GET", "/api/v1/invites/"+invitation.Token, "", 999).Code != 403 {
		t.Fatal("group outsider admitted")
	}
	if request("GET", "/api/v1/invites/"+invitation.Token+"x", "", 2).Code != 403 {
		t.Fatal("forged invite accepted")
	}
	join, _ := json.Marshal(map[string]any{"request_id": "join-unique", "expected_revision": view.Revision})
	path := "/api/v1/invites/" + invitation.Token + "/join"
	joined := request("POST", path, string(join), 2)
	if joined.Code != 200 {
		t.Fatal(joined.Code, joined.Body)
	}
	repeated := request("POST", path, string(join), 2)
	var repeat liveView
	_ = json.Unmarshal(repeated.Body.Bytes(), &repeat)
	if repeated.Code != 200 || len(repeat.Players) != 2 || repeat.Revision != view.Revision+1 {
		t.Fatal("duplicate join", repeated.Body)
	}
	stale := request("POST", path, `{"request_id":"stale-join","expected_revision":0}`, 3)
	if stale.Code != 409 {
		t.Fatal("stale join", stale.Code)
	}
	// Room lock and capacity remain engine/service decisions.
	if _, _, err := svc.SetLocked(t.Context(), game.Actor{PlayerID: 12345, ChatID: -42}, view.ID, true); err != nil {
		t.Fatal(err)
	}
	denied := request("POST", path, `{"request_id":"locked-join","expected_revision":2}`, 3)
	if denied.Code != 409 {
		t.Fatal("locked room admitted", denied.Code)
	}
	if _, _, err := svc.SetLocked(t.Context(), game.Actor{PlayerID: 12345, ChatID: -42}, view.ID, false); err != nil {
		t.Fatal(err)
	}
	for user := int64(3); user <= 10; user++ {
		current, err := svc.PublicView(t.Context(), view.ID)
		if err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf(`{"request_id":"capacity-join-%d","expected_revision":%d}`, user, current.Revision)
		if joined := request("POST", path, body, user); joined.Code != 200 {
			t.Fatal(joined.Code, joined.Body)
		}
	}
	current, _ := svc.PublicView(t.Context(), view.ID)
	full := request("POST", path, fmt.Sprintf(`{"request_id":"capacity-overflow","expected_revision":%d}`, current.Revision), 11)
	if full.Code != 409 {
		t.Fatal("eleventh participant admitted", full.Code, full.Body)
	}
	preview = request("GET", "/api/v1/invites/"+invitation.Token, "", 11)
	var capacity struct {
		CanJoin  bool `json:"can_join"`
		Capacity int  `json:"capacity"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &capacity)
	if capacity.CanJoin || capacity.Capacity != 10 {
		t.Fatal("incorrect full lobby preview", preview.Body)
	}
	allowed = false
	if request("POST", path, `{"request_id":"revoked-member","expected_revision":2}`, 4).Code != 403 {
		t.Fatal("revoked group membership accepted")
	}
	if request("POST", "/api/v1/rooms", string(body), 12345).Code != 403 {
		t.Fatal("cached create bypassed revoked membership")
	}
}
