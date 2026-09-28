package game

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func swapPosition(t *testing.T, s *Service, last bool) PublicGameView {
	t.Helper()
	v := position(t, s, uno.CaseiroRules(), true)
	e := s.manager.byID[v.GameID].entry
	state := e.engine.Snapshot()
	for i := range state.Cards {
		if state.Cards[i].ID == "a" {
			state.Cards[i].Rank = uno.SwapHands
		}
	}
	if last {
		state.Players[0].Hand = []uno.CardID{"a"}
		state.DrawPile = append(state.DrawPile, "extra")
	}
	engine, err := uno.Restore(state, func([]uno.CardID) {})
	if err != nil {
		t.Fatal(err)
	}
	e.engine = engine
	e.startedAt = time.Now().Add(-time.Minute)
	return v
}

func TestKeepHandServiceAuthorizationTimeoutAndConcurrentResolution(t *testing.T) {
	s := testService(t)
	v := swapPosition(t, s, false)
	played := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "a"})
	rev := played.View.Revision
	denied(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.KeepHand, PlayerID: 1, Revision: rev}, ErrForbidden)
	denied(t, s, v.GameID, Actor{PlayerID: 1, ChatID: -200}, uno.Action{Type: uno.KeepHand, PlayerID: 1, Revision: rev}, ErrForbidden)
	denied(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.KeepHand, PlayerID: 2, Revision: rev}, uno.ErrNotYourTurn)
	before := s.manager.byID[v.GameID].entry.engine.Snapshot()
	chosen := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.KeepHand})
	if chosen.Completed != nil || chosen.View.Revision != rev+1 || chosen.View.Phase != uno.ChoosingColor {
		t.Fatal(chosen)
	}
	e := s.manager.byID[v.GameID].entry
	if !reflect.DeepEqual(before.Players, e.engine.Snapshot().Players) {
		t.Fatal("selection changed hands")
	}
	e.turnStarted = time.Now().Add(-time.Hour)
	if len(s.ExpiredTurns(t.Context(), time.Second)) != 0 {
		t.Fatal("pending color timed out")
	}
	if _, applied := s.AutoSkipTurn(t.Context(), ExpiredTurn{GameID: v.GameID, ChatID: v.ChatID, PlayerID: 1, Revision: chosen.View.Revision}, time.Second); applied {
		t.Fatal("pending color skipped")
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, color := range []uno.Color{uno.Red, uno.Blue} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.Apply(t.Context(), Actor{PlayerID: 1}, v.GameID, uno.Action{Type: uno.ChooseColor, PlayerID: 1, Color: color, Revision: chosen.View.Revision})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	accepted, stale := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else if errors.Is(err, uno.ErrStaleRevision) {
			stale++
		} else {
			t.Fatal(err)
		}
	}
	final := e.engine.Snapshot()
	if accepted != 1 || stale != 1 || final.Revision != rev+2 || final.CurrentPlayerID != 2 || !reflect.DeepEqual(before.Players, final.Players) {
		t.Fatal("resolution not atomic", accepted, stale, final)
	}
	assertIndexes(t, s)
}

func TestLastSwapPreservesFinalResult(t *testing.T) {
	s := testService(t)
	v := swapPosition(t, s, true)
	out := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "a"})
	if out.Completed != nil || out.View.Phase != uno.TakingTurn || len(out.View.Placements) != 1 || out.View.Placements[0].PlayerID != 1 {
		t.Fatal(out)
	}
	out = act(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.PlayCard, CardID: "b"})
	if out.Completed == nil {
		t.Fatal("missing normal final result")
	}
	if err := out.Completed.Validate(); err != nil {
		t.Fatal(err)
	}
	for i, p := range out.Completed.Players {
		if p.Position != i+1 || p.WentOut != (i < 2) {
			t.Fatal(p)
		}
	}
	if len(s.PendingResults()) != 1 {
		t.Fatal("result missing or duplicated")
	}
	assertIndexes(t, s)
}
