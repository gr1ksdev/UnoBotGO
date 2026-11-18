//go:build debugcards

package uno

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestGiveCardSourcesAndConservation(t *testing.T) {
	for _, source := range []string{"draw", "discard"} {
		t.Run(source, func(t *testing.T) {
			g := scenario(t, CaseiroRules(), [][]Card{{card(Red, One)}, {card(Blue, Two)}}, card(Yellow, Nine), []Card{card(Green, Three), card(NoColor, SwapHands), card(Red, Four)})
			wanted := g.state.DrawPile[1]
			if source == "discard" {
				g.state.DrawPile = slices.Delete(g.state.DrawPile, 1, 2)
				g.state.DiscardPile = slices.Insert(g.state.DiscardPile, 0, wanted)
			} else {
				// Put a second matching card in discard; the draw pile must win.
				extra := Card{ID: "second", Rank: SwapHands}
				g.state.Cards = append(g.state.Cards, extra)
				g.state.DiscardPile = slices.Insert(g.state.DiscardPile, 0, extra.ID)
			}
			g.state.DrawnCardID = g.state.Players[0].Hand[0]
			before := g.Snapshot()
			result, c, err := g.GiveCard(before.Revision, 2, NoColor, SwapHands)
			if err != nil {
				t.Fatal(err)
			}
			after := g.Snapshot()
			if c.ID != wanted || !slices.Contains(after.Players[1].Hand, wanted) || result.Revision != before.Revision+1 || len(result.Events) != 1 || result.Events[0].CardID != "" {
				t.Fatal(c, result)
			}
			expected := before.clone()
			expected.Revision++
			expected.Players[1].Hand = append(expected.Players[1].Hand, wanted)
			if source == "draw" {
				expected.DrawPile = slices.Delete(expected.DrawPile, 1, 2)
			} else {
				expected.DiscardPile = slices.Delete(expected.DiscardPile, 0, 1)
			}
			if !reflect.DeepEqual(expected, after) {
				t.Fatal("unrelated state changed")
			}
			if err := after.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGiveCardRejectsWithoutMutation(t *testing.T) {
	for _, name := range []string{"top", "hand", "missing", "target", "inactive", "phase", "penalty", "bluff", "color", "rank", "mode", "revision", "overflow"} {
		t.Run(name, func(t *testing.T) {
			g := scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Blue, Two)}}, card(Yellow, Nine), []Card{card(Green, Three)})
			color, rank, target, revision := Green, Three, PlayerID(2), g.state.Revision
			want := ErrInvalidAction
			switch name {
			case "top":
				color, rank, want = Yellow, Nine, ErrCardNotFound
			case "hand":
				color, rank, want = Blue, Two, ErrCardNotFound
			case "missing":
				color, rank, want = Red, Four, ErrCardNotFound
			case "target":
				target, want = 99, ErrUnknownPlayer
			case "inactive":
				g.state.Players[1].Status = Left
				want = ErrUnknownPlayer
			case "phase":
				g.state.Phase = ChoosingPlayer
			case "penalty":
				g.state.DrawCounter = 2
			case "bluff":
				g.state.PendingBluff = &BluffInfo{Actor: 1, Target: 2}
			case "color":
				color = NoColor
			case "rank":
				rank = Rank(255)
			case "mode":
				color, rank, want = NoColor, SwapHands, ErrInvalidRules
			case "revision":
				revision = 1
				want = ErrStaleRevision
			case "overflow":
				g.state.Revision = math.MaxUint64
				revision = math.MaxUint64
				want = ErrInvalidState
			}
			before := g.Snapshot()
			result, c, err := g.GiveCard(revision, target, color, rank)
			if !errors.Is(err, want) || c != (Card{}) || result.Revision != 0 || !reflect.DeepEqual(before, g.Snapshot()) {
				t.Fatal(err, result, c)
			}
		})
	}
}
