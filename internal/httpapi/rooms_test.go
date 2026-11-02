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
	if inv.Code != 200 || !strings.HasPrefix(invitation.URL, "https://t.me/ConfiguredBot/play?startapp=join_") {
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

func TestFreeRoomInviteDoesNotRequireGroupMembership(t *testing.T) {
	svc, _ := game.NewService()
	refs, _ := NewReferences(make([]byte, 32))
	a := &API{Games: svc, References: refs, Token: "token", MaxAge: time.Hour, BotUsername: func() string { return "ConfiguredBot" }}
	handler := a.Handler()
	request := func(method, path, body string, user int64) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "tma "+signed("token", time.Now(), map[string]string{"user": fmt.Sprintf(`{"id":%d,"first_name":"Jogador"}`, user)}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	created := request("POST", "/api/v1/rooms", `{"room_type":"free","system":"updated","mode":"classic","request_id":"free-room-create"}`, 12345)
	var view liveView
	if err := json.Unmarshal(created.Body.Bytes(), &view); err != nil || created.Code != 200 || view.RoomType != "free" || view.Group != "" || !view.Owner || len(view.Players) != 1 {
		t.Fatal(created.Code, created.Body, err)
	}
	invite := request("GET", "/api/v1/rooms/"+string(view.ID)+"/invite", "", 12345)
	var token struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(invite.Body.Bytes(), &token)
	if invite.Code != 200 || token.Token == "" {
		t.Fatal(invite.Code, invite.Body)
	}
	preview := request("GET", "/api/v1/invites/"+token.Token, "", 2)
	var room struct {
		Type    string `json:"room_type"`
		Label   string `json:"group"`
		CanJoin bool   `json:"can_join"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &room)
	if preview.Code != 200 || room.Type != "free" || room.Label != "Sala livre" || !room.CanJoin {
		t.Fatal(preview.Code, preview.Body)
	}
	joined := request("POST", "/api/v1/invites/"+token.Token+"/join", fmt.Sprintf(`{"request_id":"free-join-0001","expected_revision":%d}`, view.Revision), 2)
	if joined.Code != 200 {
		t.Fatal(joined.Code, joined.Body)
	}
	var joinedView liveView
	if err := json.Unmarshal(joined.Body.Bytes(), &joinedView); err != nil {
		t.Fatal(err)
	}
	reconnected := request("GET", "/api/v1/rooms/"+string(view.ID), "", 2)
	var recovered liveView
	if err := json.Unmarshal(reconnected.Body.Bytes(), &recovered); err != nil || reconnected.Code != 200 || recovered.Revision != joinedView.Revision || recovered.RoomType != "free" {
		t.Fatal("free room snapshot did not recover", reconnected.Code, reconnected.Body, err)
	}
}

func TestFreeRoomCapacityFourAndStartedRejection(t *testing.T) {
	svc, _ := game.NewService()
	refs, _ := NewReferences(make([]byte, 32))
	a := &API{Games: svc, References: refs, Token: "token", MaxAge: time.Hour, BotUsername: func() string { return "ConfiguredBot" }}
	handler := a.Handler()
	request := func(method, path, body string, user int64) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "tma "+signed("token", time.Now(), map[string]string{"user": fmt.Sprintf(`{"id":%d,"first_name":"Jogador"}`, user)}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	created := request("POST", "/api/v1/rooms", `{"room_type":"free","system":"updated","mode":"classic","request_id":"free-create-cap4"}`, 1)
	var view liveView
	_ = json.Unmarshal(created.Body.Bytes(), &view)
	if view.Capacity != 4 {
		t.Fatalf("expected capacity 4, got %d", view.Capacity)
	}
	invite := request("GET", "/api/v1/rooms/"+string(view.ID)+"/invite", "", 1)
	var token struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(invite.Body.Bytes(), &token)

	// Users 2, 3, 4 join (reaching max capacity of 4)
	for u := int64(2); u <= 4; u++ {
		cur, _ := svc.PublicView(t.Context(), view.ID)
		j := request("POST", "/api/v1/invites/"+token.Token+"/join", fmt.Sprintf(`{"request_id":"free-join-%d","expected_revision":%d}`, u, cur.Revision), u)
		if j.Code != 200 {
			t.Fatalf("user %d failed to join: %d", u, j.Code)
		}
	}

	// 5th user tries to join -> rejected (409)
	cur, _ := svc.PublicView(t.Context(), view.ID)
	overflow := request("POST", "/api/v1/invites/"+token.Token+"/join", fmt.Sprintf(`{"request_id":"free-join-5","expected_revision":%d}`, cur.Revision), 5)
	if overflow.Code != 409 {
		t.Fatalf("expected 409 on 5th participant, got %d", overflow.Code)
	}

	// Preview should report can_join: false, capacity: 4, started: false
	preview := request("GET", "/api/v1/invites/"+token.Token, "", 5)
	var previewData struct {
		Capacity int  `json:"capacity"`
		CanJoin  bool `json:"can_join"`
		Started  bool `json:"started"`
	}
	_ = json.Unmarshal(preview.Body.Bytes(), &previewData)
	if previewData.Capacity != 4 || previewData.CanJoin || previewData.Started {
		t.Fatalf("unexpected preview when full: %+v", previewData)
	}

	// Now owner starts the game
	_, err := svc.ApplyWeb(t.Context(), game.Actor{PlayerID: 1}, view.ID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: cur.Revision}, "start-cap4-1234")
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Preview now reports started: true, can_join: false
	previewAfterStart := request("GET", "/api/v1/invites/"+token.Token, "", 6)
	_ = json.Unmarshal(previewAfterStart.Body.Bytes(), &previewData)
	if !previewData.Started || previewData.CanJoin {
		t.Fatalf("expected started true and can_join false, got: %+v", previewData)
	}

	// Joining started game returns 409
	curAfterStart, _ := svc.PublicView(t.Context(), view.ID)
	joinStarted := request("POST", "/api/v1/invites/"+token.Token+"/join", fmt.Sprintf(`{"request_id":"free-join-started","expected_revision":%d}`, curAfterStart.Revision), 6)
	if joinStarted.Code != 409 {
		t.Fatalf("expected 409 on joining started game, got %d", joinStarted.Code)
	}
}
