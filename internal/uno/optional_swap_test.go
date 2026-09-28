package uno

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func optionalSwapGame(t *testing.T, players int) *Game {
	hands := [][]Card{{card(NoColor, SwapHands), card(Blue, One), card(Red, Two)}, {card(Green, Three)}}
	if players > 2 {
		hands = append(hands, []Card{card(Yellow, Four), card(Red, Five), card(Green, Six)})
	}
	return scenario(t, CaseiroRules(), hands, card(Red, Nine), nil)
}
func TestOptionalSwapResolvesOnlyWithColor(t *testing.T) {
	for _, keep := range []bool{false, true} {
		for _, color := range []Color{Red, Yellow, Green, Blue} {
			for _, players := range []int{2, 3} {
				for _, direction := range []int{1, -1} {
					t.Run(fmt.Sprintf("keep=%v/color=%d/players=%d/dir=%d", keep, color, players, direction), func(t *testing.T) {
						g := optionalSwapGame(t, players)
						g.state.Direction = direction
						before := g.Snapshot()
						id := before.Players[0].Hand[0]
						played := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id})
						pending := g.Snapshot()
						if pending.Revision != before.Revision+1 || pending.Phase != ChoosingPlayer || pending.CurrentPlayerID != 1 || !slices.Equal(pending.Players[0].Hand, before.Players[0].Hand[1:]) {
							t.Fatal("wrong play stage")
						}
						for _, e := range played.Events {
							if e.Type == HandsSwapped || e.Type == HandKept || e.Type == TurnChanged || e.Type == PlayerWon {
								t.Fatal("premature effect", e)
							}
						}
						choice := Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2}
						if keep {
							choice = Action{Type: KeepHand, PlayerID: 1}
						}
						result := apply(t, g, choice)
						selected := g.Snapshot()
						if selected.Revision != before.Revision+2 || selected.Phase != ChoosingColor || selected.CurrentPlayerID != 1 || selected.ActiveColor != before.ActiveColor || !reflect.DeepEqual(selected.Players, pending.Players) || len(result.Events) != 1 || result.Events[0].Type != ColorChoiceRequired {
							t.Fatal("selection applied side effects")
						}
						// Pending state is serializable and restores the exact same resolution.
						data, err := json.Marshal(selected)
						if err != nil {
							t.Fatal(err)
						}
						var decoded State
						if err = json.Unmarshal(data, &decoded); err != nil {
							t.Fatal(err)
						}
						restored, err := Restore(decoded, noShuffle)
						if err != nil {
							t.Fatal(err)
						}
						rejected(t, g, Action{Type: ChooseColor, PlayerID: 2, Color: color, Revision: selected.Revision}, ErrNotYourTurn)
						rejected(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: NoColor, Revision: selected.Revision}, ErrInvalidColor)
						rejected(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: color, Revision: pending.Revision}, ErrStaleRevision)
						for _, kind := range []ActionType{PlayCard, DrawCard, PassTurn, SkipTurn, KeepHand, ChoosePlayer, LeaveGame} {
							rejected(t, g, Action{Type: kind, PlayerID: 1, Revision: selected.Revision}, ErrColorChoiceRequired)
						}
						resolution := apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: color})
						apply(t, restored, Action{Type: ChooseColor, PlayerID: 1, Color: color})
						after := g.Snapshot()
						if !reflect.DeepEqual(after, restored.Snapshot()) {
							t.Fatal("non-deterministic restore")
						}
						if after.Revision != before.Revision+3 || after.Phase != TakingTurn || after.CurrentPlayerID != before.next(1, 1) || after.ActiveColor != color || after.Pending != nil || after.Direction != direction || !slices.Equal(after.Order, before.Order) || !slices.Equal(after.Cards, before.Cards) || !slices.Equal(after.DrawPile, before.DrawPile) {
							t.Fatal("invalid final state")
						}
						if keep {
							if !reflect.DeepEqual(after.Players, pending.Players) || !hasEvent(resolution, HandKept, 1) || hasEvent(resolution, HandsSwapped, 1) {
								t.Fatal("keep moved cards")
							}
						} else {
							if !slices.Equal(after.Players[0].Hand, before.Players[1].Hand) || !slices.Equal(after.Players[1].Hand, before.Players[0].Hand[1:]) || !hasEvent(resolution, HandsSwapped, 1) || hasEvent(resolution, HandKept, 1) {
								t.Fatal("wrong swap")
							}
							if players > 2 && !reflect.DeepEqual(after.Players[2], before.Players[2]) {
								t.Fatal("uninvolved player changed")
							}
						}
						if after.DiscardPile[len(after.DiscardPile)-1] != id {
							t.Fatal("swap card lost identity")
						}
						rejected(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: color, Revision: selected.Revision}, ErrStaleRevision)
					})
				}
			}
		}
	}
}

func TestLastSwapFinishesWithoutChoices(t *testing.T) {
	for _, players := range []int{2, 3} {
		for _, direction := range []int{1, -1} {
			hands := [][]Card{{card(NoColor, SwapHands)}, {card(Blue, Two), card(Red, Three)}}
			if players == 3 {
				hands = append(hands, []Card{card(Green, Four)})
			}
			g := scenario(t, CaseiroRules(), hands, card(Red, Nine), nil)
			g.state.Direction = direction
			before := g.Snapshot()
			r := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: before.Players[0].Hand[0]})
			after := g.Snapshot()
			if after.Revision != before.Revision+1 || after.Players[0].Status != WentOut || len(after.Players[0].Hand) != 0 || after.Placements[0].PlayerID != 1 || !after.Placements[0].WentOut || after.Pending != nil || after.ActiveColor != before.ActiveColor {
				t.Fatal(after)
			}
			if !reflect.DeepEqual(after.Players[1:], before.Players[1:]) {
				t.Fatal("last swap changed another hand/status")
			}
			for _, e := range r.Events {
				if e.Type == PlayerChoiceRequired || e.Type == ColorChoiceRequired || e.Type == HandsSwapped || e.Type == HandKept || (e.Type == PlayerWon && e.PlayerID != 1) {
					t.Fatal("last swap applied optional effect", e)
				}
			}
			if players == 2 {
				if after.Phase != Finished || len(after.Placements) != 2 || after.Placements[1].WentOut || !hasEvent(r, GameFinished, 0) {
					t.Fatal(after)
				}
			} else if after.Phase != TakingTurn || len(after.Placements) != 1 || after.CurrentPlayerID != before.next(1, 1) {
				t.Fatal(after)
			}
		}
	}
}

func TestOptionalSwapTargetDepartureAndValidation(t *testing.T) {
	g := optionalSwapGame(t, 3)
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
	for _, target := range []PlayerID{0, 1, 99} {
		rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: target, Revision: g.state.Revision}, ErrInvalidSwapTarget)
	}
	rejected(t, g, Action{Type: KeepHand, PlayerID: 1, TargetID: 2, Revision: g.state.Revision}, ErrInvalidAction)
	apply(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2})
	selected := g.Snapshot()
	for _, mutate := range []func(*State){
		func(s *State) { s.Pending.SwapHands = false }, func(s *State) { s.Pending.SwapTarget = 1 }, func(s *State) { s.Pending.SwapTarget = 99 }, func(s *State) { s.Pending.DrawCount = 4 }, func(s *State) { s.Pending.Initial = true },
	} {
		s := selected.clone()
		mutate(&s)
		if _, err := Restore(s, noShuffle); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("bad snapshot accepted: %v", err)
		}
	}
	r := apply(t, g, Action{Type: LeaveGame, PlayerID: 2})
	after := g.Snapshot()
	if after.Phase != ChoosingPlayer || after.Pending != nil || after.CurrentPlayerID != 1 || !slices.Equal(after.Players[0].Hand, selected.Players[0].Hand) || !slices.Equal(after.Players[2].Hand, selected.Players[2].Hand) || !hasEvent(r, PlayerChoiceRequired, 1) {
		t.Fatal("invalid selection was not reset safely")
	}
	rejected(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Blue, Revision: selected.Revision}, ErrStaleRevision)
	rejected(t, g, Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2, Revision: after.Revision}, ErrInvalidSwapTarget)
	apply(t, g, Action{Type: KeepHand, PlayerID: 1})
	apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Blue})
	if g.state.ActiveColor != Blue || g.state.CurrentPlayerID != 3 {
		t.Fatal(g.state)
	}
}

func TestOptionalSwapCancelAndRemainingOpponentDeparture(t *testing.T) {
	for _, keep := range []bool{false, true} {
		for _, cancel := range []bool{false, true} {
			g := optionalSwapGame(t, 2)
			apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
			a := Action{Type: ChoosePlayer, PlayerID: 1, TargetID: 2}
			if keep {
				a = Action{Type: KeepHand, PlayerID: 1}
			}
			apply(t, g, a)
			a = Action{Type: LeaveGame, PlayerID: 2}
			if cancel {
				a = Action{Type: CancelGame, PlayerID: 1}
			}
			apply(t, g, a)
			if g.state.Phase != Finished || g.state.Pending != nil {
				t.Fatal("pending decision survived closure")
			}
		}
	}
}

func TestKeepHandAnnouncesUNOOnlyAfterColor(t *testing.T) {
	g := swapScenario(t)
	played := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
	selected := apply(t, g, Action{Type: KeepHand, PlayerID: 1})
	if hasEvent(played, UnoAnnounced, 1) || hasEvent(selected, UnoAnnounced, 1) {
		t.Fatal("premature UNO")
	}
	final := apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Blue})
	if !hasEvent(final, UnoAnnounced, 1) || hasEvent(final, UnoAnnounced, 3) {
		t.Fatal("keep UNO affected wrong player", final)
	}
}
