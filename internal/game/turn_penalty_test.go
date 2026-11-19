package game

import (
	"errors"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestEffectiveTimeoutDegradesToTheFloor(t *testing.T) {
	p := TurnPolicy{Initial: 120 * time.Second, Min: 15 * time.Second, Penalty: 20 * time.Second}
	want := []time.Duration{120, 100, 80, 60, 40, 20, 15, 15}
	for skips, seconds := range want {
		if got := p.effectiveTimeout(p.Initial, skips); got != seconds*time.Second {
			t.Fatalf("skips=%d: got %s want %ds", skips, got, seconds)
		}
	}
	// A short base keeps its own floor so existing short-timeout callers work.
	short := TurnPolicy{Initial: time.Nanosecond, Min: 15 * time.Second, Penalty: time.Second}
	if got := short.effectiveTimeout(short.Initial, 0); got != time.Nanosecond {
		t.Fatalf("short base floor changed: %s", got)
	}
}

func testTurnService(t *testing.T, policy TurnPolicy) (*Service, PublicGameView) {
	t.Helper()
	s, err := NewService(WithTurnPolicy(policy))
	if err != nil {
		t.Fatal(err)
	}
	v := create(t, s, 77, 99, uno.BotRules())
	join(t, s, v, 1)
	join(t, s, v, 2)
	act(t, s, v.GameID, Actor{PlayerID: 99, ChatID: 77}, uno.Action{Type: uno.StartGame})
	return s, v
}

func TestAutoSkipChargesThePlayerAndACtionClearsIt(t *testing.T) {
	policy := TurnPolicy{Initial: time.Second, Min: 50 * time.Millisecond, Penalty: 200 * time.Millisecond}
	s, v := testTurnService(t, policy)
	e := s.manager.byID[v.GameID].entry
	e.mu.Lock()
	current := e.engine.Snapshot().CurrentPlayerID
	e.turnStarted = time.Now().Add(-time.Second)
	e.mu.Unlock()

	candidates := s.ExpiredTurns(t.Context(), policy.Initial)
	if len(candidates) != 1 || candidates[0].PlayerID != current {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	out, applied := s.AutoSkipTurn(t.Context(), candidates[0], policy.Initial)
	if !applied || out.Skip == nil || out.Skip.Remaining != 800*time.Millisecond {
		t.Fatalf("skip penalty not applied: %+v", out)
	}
	e.mu.Lock()
	charged := e.skipCount[current]
	e.mu.Unlock()
	if charged != 1 {
		t.Fatalf("skip count = %d, want 1", charged)
	}

	// Skip the other player as well so the first one is current again, then
	// a valid action by them clears the penalty accumulated earlier.
	e.mu.Lock()
	e.turnStarted = time.Now().Add(-time.Second)
	e.mu.Unlock()
	candidates = s.ExpiredTurns(t.Context(), policy.Initial)
	if len(candidates) != 1 {
		t.Fatalf("unexpected second candidates: %+v", candidates)
	}
	if _, applied := s.AutoSkipTurn(t.Context(), candidates[0], policy.Initial); !applied {
		t.Fatal("second skip was not applied")
	}
	view, err := s.PublicView(t.Context(), v.GameID)
	if err != nil || view.CurrentTurn != current {
		t.Fatalf("expected the penalised player back in turn: %+v %v", view, err)
	}
	act(t, s, v.GameID, Actor{PlayerID: current, ChatID: 77}, uno.Action{Type: uno.DrawCard})
	e.mu.Lock()
	after := e.skipCount[current]
	e.mu.Unlock()
	if after != 0 {
		t.Fatalf("action did not clear the penalty: %d", after)
	}
}

func TestManualSkipTiming(t *testing.T) {
	policy := TurnPolicy{Initial: time.Second, Min: 50 * time.Millisecond, Penalty: 200 * time.Millisecond}
	s, v := testTurnService(t, policy)
	view, err := s.PublicView(t.Context(), v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	current := view.CurrentTurn
	other := uno.PlayerID(1)
	if current == 1 {
		other = 2
	}

	// A third party must wait for the deadline.
	_, err = s.Apply(t.Context(), Actor{PlayerID: other, ChatID: 77}, v.GameID, uno.Action{Type: uno.PenaltySkip, PlayerID: other, Revision: view.Revision})
	var early *ErrSkipTooEarly
	if !errors.As(err, &early) {
		t.Fatalf("expected ErrSkipTooEarly, got %v", err)
	}

	// A non-participant is refused even when the deadline has passed.
	_, err = s.Apply(t.Context(), Actor{PlayerID: 4242, ChatID: 77}, v.GameID, uno.Action{Type: uno.PenaltySkip, PlayerID: 4242, Revision: view.Revision})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	// The current player may skip themselves immediately.
	out, err := s.Apply(t.Context(), Actor{PlayerID: current, ChatID: 77}, v.GameID, uno.Action{Type: uno.PenaltySkip, PlayerID: current, Revision: view.Revision})
	if err != nil || out.Skip == nil || out.Skip.PlayerID != current {
		t.Fatalf("self skip failed: %+v %v", out, err)
	}
}
