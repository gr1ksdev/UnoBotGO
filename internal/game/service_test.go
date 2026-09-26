package game

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func testService(t *testing.T, options ...Option) *Service {
	t.Helper()
	s, err := NewService(options...)
	if err != nil {
		t.Fatal(err)
	}
	s.manager.newGame = func(id uno.GameID, rules uno.Rules) (*uno.Game, error) {
		return uno.NewGame(id, rules, uno.WithShuffler(func([]uno.CardID) {}))
	}
	return s
}

func create(t *testing.T, s *Service, chat ChatID, owner uno.PlayerID, rules uno.Rules) PublicGameView {
	t.Helper()
	r, err := s.Create(t.Context(), Actor{PlayerID: owner, ChatID: chat}, CreateRequest{ChatName: "group", Rules: rules})
	if err != nil {
		t.Fatal(err)
	}
	return r.View
}

func act(t *testing.T, s *Service, id uno.GameID, actor Actor, a uno.Action) Outcome {
	t.Helper()
	v, err := s.PublicView(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	a.Revision = v.Revision
	a.PlayerID = actor.PlayerID
	r, err := s.Apply(t.Context(), actor, id, a)
	if err != nil {
		t.Fatalf("action %+v: %v", a, err)
	}
	if r.View.Revision != v.Revision+1 {
		t.Fatal("accepted action did not advance one revision")
	}
	return r
}

func join(t *testing.T, s *Service, v PublicGameView, player uno.PlayerID) Outcome {
	t.Helper()
	return act(t, s, v.GameID, Actor{PlayerID: player, ChatID: v.ChatID}, uno.Action{Type: uno.JoinGame})
}

func denied(t *testing.T, s *Service, id uno.GameID, actor Actor, a uno.Action, want error) {
	t.Helper()
	before, err := s.PublicView(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Apply(t.Context(), actor, id, a)
	if !errors.Is(err, want) {
		t.Fatalf("got %v, want %v", err, want)
	}
	if !reflect.DeepEqual(r, Outcome{}) {
		t.Fatalf("failed action returned data %+v", r)
	}
	after, err := s.PublicView(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rejected action changed view")
	}
	assertIndexes(t, s)
}

// Verifies all routing projections against actual runtime state, only at test
// checkpoints with no mutators running. This is not an application API.
func assertIndexes(t *testing.T, s *Service) {
	t.Helper()
	m := s.manager
	wantChat := make(map[ChatID]uno.GameID)
	wantPlayer := make(map[uno.PlayerID]map[uno.GameID]struct{})
	for id, record := range m.byID {
		e := record.entry
		if e.final != nil {
			if e.engine != nil {
				t.Fatal("closed entry retained private runtime")
			}
			continue
		}
		state := e.engine.Snapshot()
		if err := state.Validate(); err != nil {
			t.Fatal(err)
		}
		if record.summary != publicView(e, state).summary() {
			t.Fatal("stale summary")
		}
		if _, exists := wantChat[e.chatID]; exists {
			t.Fatal("duplicate chat")
		}
		wantChat[e.chatID] = id
		for _, p := range state.Players {
			if p.Status == uno.Playing {
				if wantPlayer[p.ID] == nil {
					wantPlayer[p.ID] = make(map[uno.GameID]struct{})
				}
				wantPlayer[p.ID][id] = struct{}{}
			}
		}
	}
	if !reflect.DeepEqual(wantChat, m.byChat) || !reflect.DeepEqual(wantPlayer, m.byPlayer) {
		t.Fatalf("inconsistent routing: chats=%v players=%v", m.byChat, m.byPlayer)
	}
}

func TestCreateEmptyLobbyAndObserverAdministration(t *testing.T) {
	s := testService(t)
	v := create(t, s, -10, 99, uno.BotRules())
	if v.Revision != 0 || len(v.Players) != 0 || v.OwnerID != 99 || v.CreatorID != 99 || v.Closed || len(v.GameID) != 32 {
		t.Fatalf("bad lobby %+v", v)
	}
	games, err := s.FindPlayerGames(t.Context(), Actor{PlayerID: 99})
	if err != nil || len(games) != 0 {
		t.Fatal("observer indexed")
	}
	if _, err = s.PlayerView(t.Context(), Actor{PlayerID: 99}, v.GameID); !errors.Is(err, ErrNotParticipant) {
		t.Fatal(err)
	}
	if _, err = s.Create(t.Context(), Actor{PlayerID: 1, ChatID: -10}, CreateRequest{}); !errors.Is(err, ErrChatOccupied) {
		t.Fatal(err)
	}
	denied(t, s, v.GameID, Actor{PlayerID: 99, ChatID: -10}, uno.Action{Type: uno.StartGame, PlayerID: 99}, uno.ErrNotEnoughPlayers)
	join(t, s, v, 1)
	join(t, s, v, 2)
	denied(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 0}, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2}, ErrForbidden)
	denied(t, s, v.GameID, Actor{PlayerID: 99, ChatID: -10}, uno.Action{Type: uno.StartGame, PlayerID: 99, Revision: 2, DealerID: 99}, uno.ErrUnknownPlayer)
	r := act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -10}, uno.Action{Type: uno.StartGame})
	if r.View.OwnerID != 99 || len(r.View.Players) != 2 {
		t.Fatal("owner enrolled or transferred")
	}
	for _, p := range r.View.Players {
		if p.CardCount != 7 || p.ID == 99 {
			t.Fatal("unexpected participant")
		}
	}
	entry := s.manager.byID[v.GameID].entry
	if entry.engine.Snapshot().DealerID != 1 {
		t.Fatal("default dealer not first active")
	}
	denied(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -10}, uno.Action{Type: uno.CancelGame, PlayerID: 1, Revision: 3}, ErrForbidden)
	denied(t, s, v.GameID, Actor{PlayerID: 99}, uno.Action{Type: uno.DrawCard, PlayerID: 99, Revision: 3}, uno.ErrUnknownPlayer)
	r = act(t, s, v.GameID, Actor{PlayerID: 99, ChatID: -10}, uno.Action{Type: uno.CancelGame})
	if !r.View.Closed || r.View.CloseReason != Cancelled || r.View.OwnerID != 0 || entry.engine != nil || !eventPresent(r.Events, uno.GameFinished) {
		t.Fatal("bad cancellation")
	}
	assertIndexes(t, s)
}

func TestOwnerTransferAndEmptyLobby(t *testing.T) {
	s := testService(t)
	v := create(t, s, -1, 10, uno.BotRules())
	join(t, s, v, 10)
	r := act(t, s, v.GameID, Actor{PlayerID: 10, ChatID: -1}, uno.Action{Type: uno.LeaveGame})
	if r.View.Closed || r.View.OwnerID != 10 || r.View.Phase != uno.Lobby {
		t.Fatal("empty lobby closed/lost owner")
	}
	if _, err := s.FindChatGame(t.Context(), -1); err != nil {
		t.Fatal(err)
	}
	join(t, s, v, 20)
	join(t, s, v, 30)
	pub, _ := s.PublicView(t.Context(), v.GameID)
	if pub.OwnerID != 10 {
		t.Fatal("observer lost owner role on join")
	}
	act(t, s, v.GameID, Actor{PlayerID: 10, ChatID: -1}, uno.Action{Type: uno.StartGame, DealerID: 30})
	act(t, s, v.GameID, Actor{PlayerID: 10, ChatID: -1}, uno.Action{Type: uno.CancelGame})
	v = create(t, s, -2, 1, uno.BotRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	r = act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -2}, uno.Action{Type: uno.LeaveGame})
	if r.View.OwnerID != 2 || len(r.View.Players) != 2 || r.View.Players[0].Status != uno.Left {
		t.Fatal("owner transfer changed participation")
	}
	denied(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -2}, uno.Action{Type: uno.CancelGame, PlayerID: 1, Revision: 3}, ErrForbidden)
	act(t, s, v.GameID, Actor{PlayerID: 2, ChatID: -2}, uno.Action{Type: uno.CancelGame})
	assertIndexes(t, s)
}

func TestMultipleGamesLookupsAndContext(t *testing.T) {
	s := testService(t)
	a := create(t, s, -2, 90, uno.BotRules())
	b := create(t, s, -1, 90, uno.BotRules())
	join(t, s, a, 1)
	join(t, s, b, 1)
	games, err := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1, ChatID: -1})
	if err != nil || len(games) != 2 || games[0].GameID != a.GameID || games[1].GameID != b.GameID {
		t.Fatal(games, err)
	}
	chat, err := s.FindChatGame(t.Context(), a.ChatID)
	if err != nil || chat.GameID != a.GameID || chat.Revision != 1 {
		t.Fatal(chat, err)
	}
	denied(t, s, a.GameID, Actor{PlayerID: 1, ChatID: b.ChatID}, uno.Action{Type: uno.LeaveGame, PlayerID: 1, Revision: 1}, ErrForbidden)
	denied(t, s, a.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.LeaveGame, PlayerID: 1, Revision: 1}, ErrForbidden)
	denied(t, s, a.GameID, Actor{PlayerID: 1, ChatID: a.ChatID}, uno.Action{Type: uno.LeaveGame, PlayerID: 2, Revision: 1}, ErrForbidden)
	act(t, s, a.GameID, Actor{PlayerID: 1, ChatID: a.ChatID}, uno.Action{Type: uno.LeaveGame})
	games, err = s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
	if err != nil || len(games) != 1 || games[0].GameID != b.GameID {
		t.Fatal(games, err)
	}
	assertIndexes(t, s)
}

func TestValidationAndCancelledContext(t *testing.T) {
	if _, err := NewService(WithHistoryLimit(-1)); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	s := testService(t, nil)
	v := create(t, s, -1, 99, uno.ClassicRules())
	if _, err := s.Create(t.Context(), Actor{}, CreateRequest{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := s.Create(t.Context(), Actor{PlayerID: 1, ChatID: -2}, CreateRequest{Rules: uno.Rules{EndPolicy: 255}}); !errors.Is(err, uno.ErrInvalidRules) {
		t.Fatal(err)
	}
	if _, err := s.PublicView(t.Context(), "missing"); !errors.Is(err, ErrGameNotFound) {
		t.Fatal(err)
	}
	if _, err := s.PublicView(t.Context(), ""); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := s.PublicView(nil, v.GameID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := s.FindChatGame(t.Context(), 0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := s.FindChatGame(t.Context(), -2); !errors.Is(err, ErrNoActiveGame) {
		t.Fatal(err)
	}
	if _, err := s.FindPlayerGames(t.Context(), Actor{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := s.PlayerView(t.Context(), Actor{}, v.GameID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := s.PlayerView(t.Context(), Actor{PlayerID: 99, ChatID: -2}, v.GameID); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	denied(t, s, v.GameID, Actor{PlayerID: 99, ChatID: -1}, uno.Action{Type: 255, PlayerID: 99}, uno.ErrInvalidAction)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls := []func() error{
		func() error { _, e := s.Create(ctx, Actor{PlayerID: 1, ChatID: -2}, CreateRequest{}); return e },
		func() error {
			_, e := s.Apply(ctx, Actor{PlayerID: 99, ChatID: -1}, v.GameID, uno.Action{Type: uno.CancelGame, PlayerID: 99})
			return e
		},
		func() error { _, e := s.PublicView(ctx, v.GameID); return e },
		func() error { _, e := s.PlayerView(ctx, Actor{PlayerID: 1}, v.GameID); return e },
		func() error { _, e := s.FindChatGame(ctx, -1); return e },
		func() error { _, e := s.FindPlayerGames(ctx, Actor{PlayerID: 1}); return e },
	}
	for _, call := range calls {
		if err := call(); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	pub, _ := s.PublicView(t.Context(), v.GameID)
	if !reflect.DeepEqual(pub, v) {
		t.Fatal("invalid calls mutated game")
	}
	assertIndexes(t, s)
}

func TestFactoryFailuresAndCollision(t *testing.T) {
	for _, where := range []string{"id", "factory", "empty ID", "collision", "cancel"} {
		t.Run(where, func(t *testing.T) {
			s := testService(t)
			v := create(t, s, -1, 99, uno.ClassicRules())
			failure := errors.New("injected failure")
			want := failure
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch where {
			case "id":
				s.manager.newID = func() (uno.GameID, error) { return "", failure }
			case "factory":
				s.manager.newGame = func(uno.GameID, uno.Rules) (*uno.Game, error) { return nil, failure }
			case "empty ID":
				s.manager.newID = func() (uno.GameID, error) { return "", nil }
				want = ErrInvalidArgument
			case "collision":
				s.manager.newID = func() (uno.GameID, error) { return v.GameID, nil }
				want = ErrIDConflict
			case "cancel":
				s.manager.newGame = func(id uno.GameID, r uno.Rules) (*uno.Game, error) { cancel(); return uno.NewGame(id, r) }
				want = context.Canceled
			}
			r, err := s.Create(ctx, Actor{PlayerID: 99, ChatID: -2}, CreateRequest{})
			if !errors.Is(err, want) || !reflect.DeepEqual(r, Outcome{}) || len(s.manager.byID) != 1 {
				t.Fatal(r, err)
			}
			assertIndexes(t, s)
		})
	}
}

func eventPresent(events []uno.Event, kind uno.EventType) bool {
	return slices.ContainsFunc(events, func(e uno.Event) bool { return e.Type == kind })
}

func TestDefaultService(t *testing.T) {
	s, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	v := create(t, s, 1, 99, uno.ClassicRules())
	if v.Revision != 0 || len(v.Players) != 0 || v.OwnerID != 99 {
		t.Fatal(v)
	}
	assertIndexes(t, s)
}

func TestService_CallBluffAuthorized(t *testing.T) {
	s, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	outcome, err := s.Create(ctx, Actor{PlayerID: 1, ChatID: 100}, CreateRequest{
		ChatName: "Bluff Chat",
		Rules:    uno.BotRules(),
	})
	if err != nil {
		t.Fatal(err)
	}
	gameID := outcome.View.GameID

	_, _ = s.Apply(ctx, Actor{PlayerID: 1, ChatID: 100}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: outcome.View.Revision})
	v1, _ := s.PublicView(ctx, gameID)
	_, _ = s.Apply(ctx, Actor{PlayerID: 2, ChatID: 100}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: v1.Revision})
	v2, _ := s.PublicView(ctx, gameID)
	_, _ = s.Apply(ctx, Actor{PlayerID: 1, ChatID: 100}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, DealerID: 1, Revision: v2.Revision})

	vStart, _ := s.PublicView(ctx, gameID)

	// An inline actor with ChatID=0 calling CallBluff must not be rejected by authorize with ErrForbidden
	_, applyErr := s.Apply(ctx, Actor{PlayerID: vStart.CurrentTurn, ChatID: 0}, gameID, uno.Action{
		Type:     uno.CallBluff,
		PlayerID: vStart.CurrentTurn,
		Revision: vStart.Revision,
	})
	if errors.Is(applyErr, ErrForbidden) {
		t.Fatalf("expected CallBluff to pass service authorization, got: %v", applyErr)
	}
}

func TestCaseiroStackedDrawFourViewsNotCanCallBluff(t *testing.T) {
	s := testService(t)
	rules := uno.CaseiroRules()
	v := create(t, s, 200, 99, rules)

	state := uno.State{
		ID:              v.GameID,
		Rules:           rules,
		Phase:           uno.TakingTurn,
		Direction:       1,
		DealerID:        3,
		CurrentPlayerID: 1,
		ActiveColor:     uno.Red,
		Order:           []uno.PlayerID{1, 2, 3},
	}
	add := func(c uno.Card) uno.CardID {
		c.ID = uno.CardID(fmt.Sprintf("card_%d", len(state.Cards)+1))
		state.Cards = append(state.Cards, c)
		return c.ID
	}
	p1Hand := []uno.CardID{add(uno.Card{Color: uno.Red, Rank: uno.DrawTwo})}
	p2Hand := []uno.CardID{add(uno.Card{Color: uno.NoColor, Rank: uno.WildDrawFour}), add(uno.Card{Color: uno.Red, Rank: uno.Seven})}
	p3Hand := []uno.CardID{add(uno.Card{Color: uno.Green, Rank: uno.One})}

	state.Players = []uno.Player{
		{ID: 1, Status: uno.Playing, Hand: p1Hand},
		{ID: 2, Status: uno.Playing, Hand: p2Hand},
		{ID: 3, Status: uno.Playing, Hand: p3Hand},
	}
	state.DiscardPile = []uno.CardID{add(uno.Card{Color: uno.Red, Rank: uno.Five})}
	for range 24 {
		state.DrawPile = append(state.DrawPile, add(uno.Card{Color: uno.Blue, Rank: uno.Nine}))
	}

	eng, err := uno.Restore(state, func([]uno.CardID) {})
	if err != nil {
		t.Fatalf("failed to restore test state: %v", err)
	}

	entry := s.manager.byID[v.GameID].entry
	entry.mu.Lock()
	entry.engine = eng
	entry.mu.Unlock()

	// P1 plays Red DrawTwo
	act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 0}, uno.Action{Type: uno.PlayCard, PlayerID: 1, CardID: p1Hand[0]})

	// P2 responds with WildDrawFour on DrawTwo and chooses Blue
	act(t, s, v.GameID, Actor{PlayerID: 2, ChatID: 0}, uno.Action{Type: uno.PlayCard, PlayerID: 2, CardID: p2Hand[0]})
	act(t, s, v.GameID, Actor{PlayerID: 2, ChatID: 0}, uno.Action{Type: uno.ChooseColor, PlayerID: 2, Color: uno.Blue})

	view, err := s.PublicView(t.Context(), v.GameID)
	if err != nil {
		t.Fatal(err)
	}

	// CanCallBluff must be false because +4 was played on +2
	if view.CanCallBluff {
		t.Fatal("CanCallBluff must be false when +4 was played on +2 in Caseiro")
	}

	// Forced CallBluff from P3 must be rejected with ErrInvalidAction
	_, applyErr := s.Apply(t.Context(), Actor{PlayerID: 3, ChatID: 0}, v.GameID, uno.Action{
		Type:     uno.CallBluff,
		PlayerID: 3,
		Revision: view.Revision,
	})
	if !errors.Is(applyErr, uno.ErrInvalidAction) {
		t.Fatalf("expected ErrInvalidAction for forced CallBluff, got: %v", applyErr)
	}
}

func TestResetChatOwnerRemovesActiveHistoryAndIndexes(t *testing.T) {
	s := testService(t)
	first := create(t, s, -500, 10, uno.BotRules())
	join(t, s, first, 1)
	act(t, s, first.GameID, Actor{PlayerID: 10, ChatID: -500}, uno.Action{Type: uno.CancelGame})

	active := create(t, s, -500, 20, uno.CaseiroRules())
	join(t, s, active, 1)
	other := create(t, s, -501, 30, uno.BotRules())

	result, err := s.ResetChat(t.Context(), Actor{PlayerID: 20, ChatID: -500})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RemovedActive || result.RemovedHistory != 1 || len(result.GameIDs) != 2 || !slices.Contains(result.GameIDs, first.GameID) || !slices.Contains(result.GameIDs, active.GameID) {
		t.Fatalf("unexpected reset result: %+v", result)
	}
	if _, err := s.FindChatGame(t.Context(), -500); !errors.Is(err, ErrNoActiveGame) {
		t.Fatalf("reset chat still active: %v", err)
	}
	if _, err := s.PublicView(t.Context(), active.GameID); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("active game retained: %v", err)
	}
	if games, err := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1}); err != nil || len(games) != 0 {
		t.Fatalf("player index retained: games=%+v err=%v", games, err)
	}
	if summary, err := s.FindChatGame(t.Context(), -501); err != nil || summary.GameID != other.GameID {
		t.Fatalf("other chat affected: summary=%+v err=%v", summary, err)
	}
	if next := create(t, s, -500, 40, uno.BotRules()); next.GameID == "" {
		t.Fatal("could not create game after reset")
	}
	assertIndexes(t, s)
}

func TestResetChatAuthorizationAndIdempotency(t *testing.T) {
	s := testService(t)
	v := create(t, s, -600, 10, uno.BotRules())
	if _, err := s.ResetChat(t.Context(), Actor{PlayerID: 11, ChatID: -600}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ordinary member reset: %v", err)
	}
	if summary, err := s.FindChatGame(t.Context(), -600); err != nil || summary.GameID != v.GameID {
		t.Fatal("denied reset changed state")
	}
	result, err := s.ResetChat(t.Context(), Actor{PlayerID: 11, ChatID: -600, ChatAdmin: true})
	if err != nil || !result.RemovedActive {
		t.Fatalf("admin reset failed: result=%+v err=%v", result, err)
	}
	result, err = s.ResetChat(t.Context(), Actor{PlayerID: 11, ChatID: -600, ChatAdmin: true})
	if err != nil || len(result.GameIDs) != 0 {
		t.Fatalf("idempotent admin reset failed: result=%+v err=%v", result, err)
	}
	if _, err := s.ResetChat(t.Context(), Actor{PlayerID: 12, ChatID: -700}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-admin reset without game: %v", err)
	}
}

func TestResetChatHonorsContextWhileWaitingForGame(t *testing.T) {
	s := testService(t)
	v := create(t, s, -800, 10, uno.BotRules())
	entry := s.manager.byID[v.GameID].entry
	entry.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := s.ResetChat(ctx, Actor{PlayerID: 10, ChatID: -800})
	entry.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
	if summary, err := s.FindChatGame(t.Context(), -800); err != nil || summary.GameID != v.GameID {
		t.Fatal("timed-out reset changed state")
	}
}
