package uno

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
)

func swapScenario(t *testing.T) *Game {
	t.Helper()
	return scenario(t, CaseiroRules(), [][]Card{
		{card(NoColor, SwapHands), card(Blue, One)},
		{card(Green, Two), card(Yellow, Three), card(Red, Four)},
		{card(Red, Five)},
	}, card(Red, Nine), nil)
}

func TestSwapHandsAtomicChoiceAndTurn(t *testing.T) {
	for _, direction := range []int{1, -1} {
		g := swapScenario(t)
		g.state.Direction = direction
		before := g.Snapshot()
		played := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: before.Players[0].Hand[0]})
		pending := g.Snapshot()
		if pending.Phase != ChoosingPlayer || pending.CurrentPlayerID != 1 || pending.ActiveColor != Red || !hasEvent(played, PlayerChoiceRequired, 1) || hasEvent(played, UnoAnnounced, 1) {
			t.Fatalf("wrong pending state/events: %+v, %+v", pending, played)
		}
		// Pending snapshots survive serialization and do not alias hands.
		raw, err := json.Marshal(pending)
		if err != nil {
			t.Fatal(err)
		}
		var decoded State
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		restored, err := Restore(decoded, noShuffle)
		if err != nil {
			t.Fatal(err)
		}
		decoded.Players[0].Hand[0] = "foreign"
		if err := restored.Snapshot().Validate(); err != nil {
			t.Fatal(err)
		}
		r := apply(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2})
		after := g.Snapshot()
		if !slices.Equal(after.Players[0].Hand, before.Players[1].Hand) || !slices.Equal(after.Players[1].Hand, before.Players[0].Hand[1:]) || !slices.Equal(after.Players[2].Hand, before.Players[2].Hand) {
			t.Fatal("hands not swapped exactly")
		}
		if !slices.Equal(after.Cards, before.Cards) || !slices.Equal(after.DrawPile, before.DrawPile) || !slices.Equal(after.Order, before.Order) || after.ActiveColor != Red || after.Direction != direction || after.Phase != TakingTurn {
			t.Fatal("unrelated state changed")
		}
		wantTurn := PlayerID(2)
		if direction < 0 {
			wantTurn = 3
		}
		if after.CurrentPlayerID != wantTurn || !hasEvent(r, HandsSwapped, 1) || !hasEvent(r, UnoAnnounced, 2) || hasEvent(r, UnoAnnounced, 1) {
			t.Fatal(after, r)
		}
		if !slices.Equal(pending.Players[0].Hand, before.Players[0].Hand[1:]) {
			t.Fatal("old snapshot mutated")
		}
	}
}

func TestSwapHandsRejectsInvalidActionsWithoutMutation(t *testing.T) {
	g := swapScenario(t)
	rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2}, ErrInvalidAction)
	rejected(t, g, Action{Type: DrawCard, PlayerID: 1, TargetID: 2}, ErrInvalidAction)
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
	rev := g.state.Revision
	for _, target := range []PlayerID{0, 1, 99} {
		rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: target, Revision: rev}, ErrInvalidSwapTarget)
	}
	rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 2, TargetID: 3, Revision: rev}, ErrNotYourTurn)
	rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2, Revision: rev - 1}, ErrStaleRevision)
	for _, kind := range []ActionType{DrawCard, PassTurn, SkipTurn, CallBluff, LeaveGame} {
		rejected(t, g, Action{Type: kind, PlayerID: 1, Revision: rev}, ErrPlayerChoiceRequired)
	}
	if err := g.CanPlay(1, g.state.Players[0].Hand[0]); !errors.Is(err, ErrPlayerChoiceRequired) {
		t.Fatal(err)
	}
	apply(t, g, Action{Type: LeaveGame, PlayerID: 2})
	rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2, Revision: g.state.Revision}, ErrInvalidSwapTarget)
	apply(t, g, Action{Type: JoinGame, PlayerID: 4})
	apply(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 4})
	rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 4, Revision: rev}, ErrStaleRevision)
}

func TestSwapHandsRestrictions(t *testing.T) {
	for _, test := range []struct {
		name  string
		top   Card
		count int
		last  bool
	}{
		{"last card", card(Red, Nine), 0, true},
		{"on wild", card(NoColor, Wild), 0, false},
		{"on draw four", card(NoColor, WildDrawFour), 0, false},
		{"on swap", card(NoColor, SwapHands), 0, false},
		{"draw two penalty", card(Red, DrawTwo), 2, false},
		{"draw four penalty", card(NoColor, WildDrawFour), 4, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			hand := []Card{card(NoColor, SwapHands)}
			if !test.last {
				hand = append(hand, card(Blue, One))
			}
			g := scenario(t, CaseiroRules(), [][]Card{hand, {card(Red, One)}}, test.top, nil)
			g.state.DrawCounter = test.count
			id := g.state.Players[0].Hand[0]
			if err := g.CanPlay(1, id); !errors.Is(err, ErrCardNotPlayable) {
				t.Fatal(err)
			}
			rejected(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id}, ErrCardNotPlayable)
		})
	}
}

func TestSwapHandsLifecycleAndUNO(t *testing.T) {
	t.Run("both have UNO", func(t *testing.T) {
		g := swapScenario(t)
		apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
		r := apply(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 3})
		if !hasEvent(r, UnoAnnounced, 1) || !hasEvent(r, UnoAnnounced, 3) {
			t.Fatal(r)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		g := swapScenario(t)
		apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
		apply(t, g, Action{Type: CancelGame, PlayerID: 1})
		if g.state.Phase != Finished {
			t.Fatal(g.state.Phase)
		}
	})
	t.Run("remaining opponent leaves", func(t *testing.T) {
		g := swapScenario(t)
		apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
		apply(t, g, Action{Type: LeaveGame, PlayerID: 2})
		apply(t, g, Action{Type: LeaveGame, PlayerID: 3})
		if g.state.Phase != Finished || g.state.FinishReason != FinishedByDeparture {
			t.Fatal(g.state)
		}
	})
	t.Run("invalid snapshot", func(t *testing.T) {
		g := swapScenario(t)
		s := g.Snapshot()
		s.Phase = ChoosingPlayer
		if _, err := Restore(s, nil); !errors.Is(err, ErrInvalidState) {
			t.Fatal(err)
		}
		s = g.Snapshot()
		s.Rules = BotRules()
		if _, err := Restore(s, nil); !errors.Is(err, ErrInvalidState) {
			t.Fatal(err)
		}
	})
}

func TestSwapHandsDeckAndLobbyModes(t *testing.T) {
	g, err := NewGame("modes", BotRules(), WithShuffler(noShuffle))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []PlayerID{1, 2} {
		apply(t, g, Action{Type: JoinGame, PlayerID: id})
	}
	for _, rules := range []Rules{CaseiroRules(), BotRules(), CaseiroRules()} {
		before := g.Snapshot()
		apply(t, g, Action{Type: SetRules, PlayerID: 1, Rules: rules})
		s := g.Snapshot()
		want := 108
		if rules.AllowSwapHands {
			want = 109
		}
		if len(s.Cards) != want || len(s.DrawPile) != want || !reflect.DeepEqual(s.Players, before.Players) || !slices.Equal(s.Order, before.Order) {
			t.Fatal(s)
		}
	}
	apply(t, g, Action{Type: StartGame, PlayerID: 1, DealerID: 2})
	if c, _ := g.state.card(g.state.DiscardPile[0]); c.Rank > Nine {
		t.Fatal(c)
	}
	rejected(t, g, Action{Type: SetRules, PlayerID: 1, Rules: BotRules(), Revision: g.state.Revision}, ErrGameStarted)

	custom := ClassicDeck()[:30]
	injected, err := NewGame("custom", BotRules(), WithDeck(custom))
	if err != nil {
		t.Fatal(err)
	}
	apply(t, injected, Action{Type: SetRules, PlayerID: 1, Rules: CaseiroRules()})
	if !slices.Equal(injected.state.Cards, custom) || !injected.state.CustomDeck {
		t.Fatal("custom inventory changed")
	}
	raw, _ := json.Marshal(injected.Snapshot())
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(state, nil)
	if err != nil {
		t.Fatal(err)
	}
	apply(t, restored, Action{Type: SetRules, PlayerID: 1, Rules: BotRules()})
	if !slices.Equal(restored.state.Cards, custom) {
		t.Fatal("restored custom inventory changed")
	}
	special := []Card{{ID: "swap", Rank: SwapHands}}
	if _, err := NewGame("invalid", BotRules(), WithDeck(special)); !errors.Is(err, ErrInvalidState) {
		t.Fatal(err)
	}
	injected, err = NewGame("special", CaseiroRules(), WithDeck(special))
	if err != nil {
		t.Fatal(err)
	}
	rejected(t, injected, Action{Type: SetRules, PlayerID: 1, Rules: BotRules()}, ErrInvalidRules)
}

func TestDrawnSwapAndFollowingColorMatch(t *testing.T) {
	g := scenario(t, CaseiroRules(), [][]Card{{card(Blue, One)}, {card(Red, Two), card(Green, Three)}}, card(Red, Nine), []Card{card(NoColor, SwapHands), card(Yellow, One)})
	apply(t, g, Action{Type: DrawCard, PlayerID: 1})
	swap := g.state.DrawnCardID
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: swap})
	if g.state.DrawnCardID != "" {
		t.Fatal("draw marker survived play")
	}
	apply(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2})
	// Player 2 received a blue card, so cannot match the retained red color.
	if err := g.CanPlay(2, g.state.Players[1].Hand[0]); !errors.Is(err, ErrCardNotPlayable) {
		t.Fatal(err)
	}
	apply(t, g, Action{Type: SkipTurn, PlayerID: 2})
	red := g.state.Players[0].Hand[0]
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: red})
	if g.state.ActiveColor != Red {
		t.Fatal("following red card was not accepted")
	}
}
