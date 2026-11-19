package uno

import "testing"

func TestPenaltySkipDrawsForTheSkippedPlayerAndAdvances(t *testing.T) {
	g := scenario(t, BotRules(), [][]Card{
		{card(Red, One)},
		{card(Blue, Two)},
	}, card(Green, Three), []Card{card(Yellow, Four)})
	r := apply(t, g, Action{Type: PenaltySkip, PlayerID: 2})
	s := g.Snapshot()
	if s.CurrentPlayerID != 2 {
		t.Fatalf("turn did not advance: %+v", s)
	}
	if got := len(s.Player(1).Hand); got != 2 {
		t.Fatalf("skipped player should draw one card, got %d", got)
	}
	if !hasEvent(r, CardsDrawn, 1) || !hasEvent(r, PlayerSkipped, 1) {
		t.Fatalf("missing penalty events: %+v", r.Events)
	}
}

func TestPenaltySkipWithEmptyDeckOnlyAdvances(t *testing.T) {
	g := scenario(t, BotRules(), [][]Card{
		{card(Red, One)},
		{card(Blue, Two)},
	}, card(Green, Three), []Card{})
	apply(t, g, Action{Type: PenaltySkip, PlayerID: 2})
	s := g.Snapshot()
	if s.CurrentPlayerID != 2 || len(s.Player(1).Hand) != 1 {
		t.Fatalf("empty deck must still advance without changing the hand: %+v", s)
	}
}

func TestPenaltySkipRejectedOutsideTakingTurn(t *testing.T) {
	g, err := NewGame("choice", CaseiroRules())
	if err != nil {
		t.Fatal(err)
	}
	g.state.Phase = ChoosingColor
	g.state.Pending = &ColorChoice{Actor: 1}
	rejected(t, g, Action{Type: PenaltySkip, PlayerID: 2}, ErrInvalidAction)
}
