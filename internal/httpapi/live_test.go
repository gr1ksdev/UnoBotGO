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
		Recovery bool     `json:"recovery"`
		Type     string   `json:"type"`
		Request  string   `json:"request_id"`
		View     liveView `json:"view"`
	}
	if err := wsjson.Read(ctx, conn, &message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "snapshot" || !message.Recovery {
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
	if message.Recovery || len(message.View.Events) == 0 {
		t.Fatal("confirmed journal missing", message)
	}
	for _, event := range message.View.Events {
		if event.ID == "" || event.Revision > message.View.Revision {
			t.Fatal("invalid event identity", event)
		}
		if event.Type != uno.CardPlayed && event.CardID != "" {
			t.Fatal("private card leaked in public journal", event)
		}
	}
	journal, _ := json.Marshal(message.View.Events)
	revision := message.View.Revision
	conn.CloseNow()
	reconnected := dial(signed("token", time.Now(), nil))
	defer reconnected.CloseNow()
	if err = wsjson.Read(ctx, reconnected, &message); err != nil || (message.View.Revision != revision || !message.Recovery) {
		t.Fatal("reconnection did not recover latest state", err)
	}
	recoveredJournal, _ := json.Marshal(message.View.Events)
	if string(journal) != string(recoveredJournal) {
		t.Fatal("recovery changed event IDs")
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

func TestRematchTransportConsensusRecoveryAndPrivacy(t *testing.T) {
	a, id := liveFixture(t)
	initial, _ := a.project(t.Context(), id, 12345)
	if err := a.command(t.Context(), 12345, liveCommand{GameID: id, RequestID: "start-round", Revision: initial.Revision, Type: "start"}); err != nil {
		t.Fatal(err)
	}
	started, _ := a.project(t.Context(), id, 12345)
	if err := a.command(t.Context(), 12345, liveCommand{GameID: id, RequestID: "leave-round", Revision: started.Revision, Type: "leave"}); err != nil {
		t.Fatal(err)
	}
	closed, _ := a.project(t.Context(), id, 12345)
	if err := a.command(t.Context(), 12345, liveCommand{GameID: id, RequestID: "pending-vote", Revision: closed.Revision, Type: "rematch"}); !errors.Is(err, game.ErrResultPending) {
		t.Fatal("pending result bypass", err)
	}
	a.Games.AcknowledgeResult(id)
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	dial := func(user string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/live", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := wsjson.Write(ctx, conn, map[string]any{"init_data": signed("token", time.Now(), map[string]string{"user": user}), "game_id": id}); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	type message struct {
		Type     string   `json:"type"`
		Request  string   `json:"request_id"`
		Recovery bool     `json:"recovery"`
		View     liveView `json:"view"`
	}
	read := func(conn *websocket.Conn, predicate func(message) bool) message {
		t.Helper()
		for {
			var m message
			if err := wsjson.Read(ctx, conn, &m); err != nil {
				t.Fatal(err)
			}
			if predicate(m) {
				return m
			}
		}
	}
	first := dial(`{"id":12345}`)
	defer first.CloseNow()
	second := dial(`{"id":2}`)
	defer second.CloseNow()
	read(first, func(m message) bool { return m.View.Closed })
	read(second, func(m message) bool { return m.View.Closed })
	command := liveCommand{GameID: id, RequestID: "socket-vote-a", Revision: closed.Revision, Type: "rematch"}
	if err := wsjson.Write(ctx, first, command); err != nil {
		t.Fatal(err)
	}
	acknowledged := read(first, func(m message) bool { return m.Request == command.RequestID })
	if acknowledged.Type != "accepted" || acknowledged.View.Rematch.NextGameID != "" {
		t.Fatal("first vote restarted")
	}
	synced := read(second, func(m message) bool { return len(m.View.Rematch.Accepted) == 1 })
	if len(synced.View.Hand) != 0 || len(synced.View.Rematch.Required) != 2 {
		t.Fatal("invalid terminal projection")
	}
	// Explicit withdrawal then an interrupted accept retry must not restore a vote.
	command.RequestID = "socket-withdraw"
	command.Type = "rematch_leave"
	wsjson.Write(ctx, first, command)
	read(first, func(m message) bool { return m.Request == command.RequestID })
	command.RequestID = "socket-vote-a"
	command.Type = "rematch"
	wsjson.Write(ctx, first, command)
	duplicate := read(first, func(m message) bool { return m.Request == command.RequestID })
	if len(duplicate.View.Rematch.Accepted) != 0 {
		t.Fatal("duplicate reaccepted withdrawn vote")
	}
	command.RequestID = "socket-vote-b"
	wsjson.Write(ctx, first, command)
	read(first, func(m message) bool { return m.Request == command.RequestID })
	first.CloseNow()
	recovered := dial(`{"id":12345}`)
	defer recovered.CloseNow()
	snapshot := read(recovered, func(m message) bool { return m.Recovery })
	if len(snapshot.View.Rematch.Accepted) != 1 {
		t.Fatal("reconnect lost vote")
	}
	command.RequestID = "socket-vote-c"
	wsjson.Write(ctx, second, command)
	all := read(second, func(m message) bool { return m.Request == command.RequestID })
	transitioned := read(recovered, func(m message) bool { return m.View.Rematch.NextGameID != "" })
	if all.View.Rematch.NextGameID == "" || transitioned.View.Rematch.NextGameID != all.View.Rematch.NextGameID {
		t.Fatal("clients disagree")
	}
	next := all.View.Rematch.NextGameID
	ownA, err := a.project(ctx, next, 12345)
	if err != nil {
		t.Fatal(err)
	}
	ownB, err := a.project(ctx, next, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownA.Hand) != 7 || len(ownB.Hand) != 7 {
		t.Fatal("round not dealt")
	}
	for _, ca := range ownA.Hand {
		for _, cb := range ownB.Hand {
			if ca.Card.ID == cb.Card.ID {
				t.Fatal("other hand exposed")
			}
		}
	}
}
