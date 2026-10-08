package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func liveFixture(t *testing.T) (*API, uno.GameID) {
	t.Helper()
	svc, _ := game.NewService()
	actor := game.Actor{PlayerID: 12345, ChatID: -42}
	out, err := svc.Create(t.Context(), actor, game.CreateRequest{Rules: uno.BotRules(), ChatName: "Grupo real"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uno.PlayerID{12345, 2} {
		out, err = svc.Apply(t.Context(), game.Actor{PlayerID: id, ChatID: -42}, out.View.GameID, uno.Action{Type: uno.JoinGame, PlayerID: id, Revision: out.View.Revision})
		if err != nil {
			t.Fatal(err)
		}
	}
	refs, _ := NewReferences(make([]byte, 32))
	return &API{Games: svc, References: refs, Token: "token", MaxAge: time.Hour, TurnTimeout: time.Minute, Lifecycle: t.Context()}, out.View.GameID
}
func TestLiveAuthorizationAndProjection(t *testing.T) {
	a, id := liveFixture(t)
	v, err := a.project(t.Context(), id, 12345)
	if err != nil || len(v.Players) != 2 {
		t.Fatal(err, v)
	}
	if _, err = a.project(t.Context(), id, 999); !errors.Is(err, game.ErrForbidden) {
		t.Fatal("outsider snapshot", err)
	}
	if _, err = a.Games.ApplyWeb(t.Context(), game.Actor{PlayerID: 12345, ChatID: -999}, id, uno.Action{Type: uno.StartGame, PlayerID: 12345, Revision: v.Revision}, "forged-group"); !errors.Is(err, game.ErrForbidden) {
		t.Fatal("forged group", err)
	}
	if _, err = a.Games.ApplyWeb(t.Context(), game.Actor{PlayerID: 12345}, id, uno.Action{Type: uno.DrawCard, PlayerID: 2, Revision: v.Revision}, "forged-player"); !errors.Is(err, game.ErrForbidden) {
		t.Fatal("forged identity", err)
	}
	start := liveCommand{GameID: id, RequestID: "start-once", Revision: v.Revision, Type: "start"}
	if err = a.command(t.Context(), 12345, start); err != nil {
		t.Fatal(err)
	}
	fresh, _ := a.project(t.Context(), id, 12345)
	if len(fresh.Hand) != 7 || fresh.Deadline == nil {
		t.Fatal("missing own hand or official deadline", fresh)
	}
	serialized, _ := json.Marshal(fresh)
	if strings.Contains(string(serialized), "ChatID") || strings.Contains(string(serialized), "PlayerID") {
		t.Fatal("raw IDs leaked")
	}
	if err = a.command(t.Context(), 12345, start); err != nil {
		t.Fatal("duplicate rejected", err)
	}
	next, _ := a.project(t.Context(), id, 12345)
	if next.Revision != fresh.Revision {
		t.Fatal("duplicate mutated game")
	}
	stale := start
	stale.Type = "draw"
	stale.RequestID = "stale-draw"
	if err = a.command(t.Context(), 12345, stale); !errors.Is(err, uno.ErrStaleRevision) {
		t.Fatal("stale revision", err)
	}
	if err = a.command(t.Context(), 12345, liveCommand{GameID: id, RequestID: "leave-once", Revision: fresh.Revision, Type: "leave"}); err != nil {
		t.Fatal(err)
	}
	closed, err := a.project(t.Context(), id, 12345)
	if err != nil || !closed.Closed || closed.MyTurn || closed.Deadline != nil || len(closed.Hand) != 0 || closed.Result != nil {
		t.Fatal("invalid two-player closure", err, closed)
	}
}
func TestWebSocketReceivesInlineUpdatesAndReconnects(t *testing.T) {
	a, id := liveFixture(t)
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	dial := func(raw string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/live", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = wsjson.Write(ctx, conn, map[string]any{"init_data": raw, "game_id": id}); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	conn := dial(signed("token", time.Now(), nil))
	defer conn.CloseNow()
	var message struct {
		Type    string   `json:"type"`
		Request string   `json:"request_id"`
		View    liveView `json:"view"`
	}
	if err := wsjson.Read(ctx, conn, &message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "snapshot" {
		t.Fatal(message)
	}
	// A mutation through the existing Inline/service path must push to the socket.
	_, err := a.Games.Apply(ctx, game.Actor{PlayerID: 12345, ChatID: -42}, id, uno.Action{Type: uno.StartGame, PlayerID: 12345, Revision: message.View.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if err = wsjson.Read(ctx, conn, &message); err != nil || message.View.Phase == uno.Lobby || len(message.View.Hand) != 7 {
		t.Fatal("Inline event not pushed", err, message)
	}
	revision := message.View.Revision
	conn.CloseNow()
	reconnected := dial(signed("token", time.Now(), nil))
	defer reconnected.CloseNow()
	if err = wsjson.Read(ctx, reconnected, &message); err != nil || message.View.Revision != revision {
		t.Fatal("reconnection did not recover latest state", err)
	}
	// Actor/group fields are not part of the command contract and cannot be forged.
	if err = wsjson.Write(ctx, reconnected, map[string]any{"game_id": id, "request_id": "forged-actor", "expected_revision": revision, "action": "draw", "player_id": 2, "chat_id": -999}); err != nil {
		t.Fatal(err)
	}
	if err = wsjson.Read(ctx, reconnected, &message); err != nil || message.Type != "rejected" || message.View.Revision != revision {
		t.Fatal("forged command changed state", err, message)
	}

	forged := dial(signed("token", time.Now(), map[string]string{"user": `{"id":999}`}))
	defer forged.CloseNow()
	if err = wsjson.Read(ctx, forged, &message); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatal("unauthorized socket", err)
	}
	altered := dial(signed("token", time.Now(), nil) + "x")
	defer altered.CloseNow()
	if err = wsjson.Read(ctx, altered, &message); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatal("forged initData", err)
	}
}
