package uno

import (
	"errors"
	"testing"
)

func adminGame(t *testing.T) *Game {
	hands := [][]Card{
		{card(Red, One), card(Red, Two)},
		{card(Blue, Three)},
		{card(Green, Four)},
	}
	return scenario(t, BotRules(), hands, card(Red, Nine), nil)
}

func TestForceSkipTurnAdvancesTheCurrentPlayer(t *testing.T) {
	g := adminGame(t)
	before := g.Snapshot()
	if before.CurrentPlayerID != 1 {
		t.Fatal("unexpected first player")
	}
	res := apply(t, g, Action{Type: ForceSkipTurn, PlayerID: 99})
	after := g.Snapshot()
	if after.Revision != before.Revision+1 || after.CurrentPlayerID != 2 {
		t.Fatalf("turn not advanced: %+v", after)
	}
	if after.Players[0].Status != Playing || len(after.Players[0].Hand) != 2 {
		t.Fatal("skipped player lost cards")
	}
	found := false
	for _, e := range res.Events {
		if e.Type == PlayerSkipped && e.PlayerID == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no skip event: %+v", res.Events)
	}
}

func TestForceSkipTurnRejectsInvalidContexts(t *testing.T) {
	g := adminGame(t)
	// A pending colour choice blocks the skip.
	g.state.Pending = &ColorChoice{Actor: 1, Target: 2, PreviousColor: Red}
	g.state.Phase = ChoosingColor
	rejected(t, g, Action{Type: ForceSkipTurn, PlayerID: 99}, ErrColorChoiceRequired)

	g = adminGame(t)
	g.state.Phase = Lobby
	rejected(t, g, Action{Type: ForceSkipTurn, PlayerID: 99}, ErrGameNotStarted)
}

func TestKickPlayerRemovesTheTarget(t *testing.T) {
	g := adminGame(t)
	before := g.Snapshot()
	res := apply(t, g, Action{Type: KickPlayer, PlayerID: 99, TargetID: 2})
	after := g.Snapshot()
	if len(after.Order) != 2 {
		t.Fatalf("target still seated: %+v", after.Order)
	}
	if after.Players[1].Status != Left || len(after.Players[1].Hand) != 0 {
		t.Fatal("target not removed")
	}
	if after.CurrentPlayerID != 1 {
		t.Fatal("turn moved for an unrelated kick")
	}
	// The removed cards stay in the inventory.
	seen := map[CardID]int{}
	for _, id := range after.DrawPile {
		seen[id]++
	}
	for _, id := range after.DiscardPile {
		seen[id]++
	}
	for _, p := range after.Players {
		for _, id := range p.Hand {
			seen[id]++
		}
	}
	if len(seen) != len(after.Cards) {
		t.Fatal("a physical card was lost")
	}
	found := false
	for _, e := range res.Events {
		if e.Type == PlayerLeft && e.PlayerID == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no leave event: %+v", res.Events)
	}
	_ = before
}

func TestKickPlayerRejectsInvalidTargets(t *testing.T) {
	g := adminGame(t)
	rejected(t, g, Action{Type: KickPlayer, PlayerID: 1, TargetID: 1}, ErrInvalidAction)
	rejected(t, g, Action{Type: KickPlayer, PlayerID: 99, TargetID: 77}, ErrUnknownPlayer)
}

func TestKickPlayerEndsTheGameOnTheLastDeparture(t *testing.T) {
	g := scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Blue, Two)}}, card(Red, Nine), nil)
	res := apply(t, g, Action{Type: KickPlayer, PlayerID: 99, TargetID: 2})
	after := g.Snapshot()
	if after.Phase != Finished || after.FinishReason != FinishedByDeparture {
		t.Fatalf("game not finished: %+v", after)
	}
	if len(res.Events) == 0 {
		t.Fatal("no events")
	}
}

func TestAdministrativeActionsDoNotRequireParticipation(t *testing.T) {
	g := adminGame(t)
	// A requester who never joined may still skip the turn.
	apply(t, g, Action{Type: ForceSkipTurn, PlayerID: 4242})
}

func TestAdministrativeActionsRequireARequester(t *testing.T) {
	g := adminGame(t)
	rejected(t, g, Action{Type: ForceSkipTurn, PlayerID: 0}, ErrInvalidAction)
	rejected(t, g, Action{Type: KickPlayer, PlayerID: 0, TargetID: 2}, ErrInvalidAction)
	if !errors.Is(ErrInvalidAction, ErrInvalidAction) {
		t.Fatal("unreachable")
	}
}
