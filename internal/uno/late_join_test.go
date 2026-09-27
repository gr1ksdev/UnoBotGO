package uno

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestLateJoinLogicalTail(t *testing.T) {
	for _, rules := range []Rules{BotRules(), CaseiroRules()} {
		for _, count := range []int{2, 4} {
			for _, direction := range []int{1, -1} {
				for current := 1; current <= count; current++ {
					t.Run(fmt.Sprintf("caseiro=%v/players=%d/dir=%d/current=%d", rules.AllowSwapHands, count, direction, current), func(t *testing.T) {
						hands := make([][]Card, count)
						for i := range hands {
							hands[i] = []Card{card(Red, Four), card(Blue, Five)}
						}
						g := scenario(t, rules, hands, card(Red, DrawTwo), nil)
						g.state.CurrentPlayerID = PlayerID(current)
						g.state.Direction = direction
						g.state.DrawCounter = 6
						assertLateJoinTail(t, g, []PlayerID{8, 9})
					})
				}
			}
		}
	}
}

func TestLateJoinAfterReverseAndPlacement(t *testing.T) {
	for _, placement := range []bool{false, true} {
		for _, direction := range []int{1, -1} {
			t.Run(fmt.Sprintf("placement=%v/dir=%d", placement, direction), func(t *testing.T) {
				first := []Card{card(Red, Reverse), card(Red, Four)}
				if placement {
					first = first[:1]
				}
				g := scenario(t, BotRules(), [][]Card{first, {card(Red, One)}, {card(Red, Two)}, {card(Red, Three)}}, card(Red, Nine), nil)
				g.state.Direction = direction
				r := apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.state.Players[0].Hand[0]})
				if !hasEvent(r, DirectionChanged, 0) || g.state.Direction != -direction {
					t.Fatal("reverse did not change direction")
				}
				if placement && len(g.state.Placements) != 1 {
					t.Fatal("missing placement")
				}
				assertLateJoinTail(t, g, []PlayerID{8, 9})
			})
		}
	}
}

// Uses real turn-advance actions after admission, not just the layout of Order.
func assertLateJoinTail(t *testing.T, g *Game, arrivals []PlayerID) {
	t.Helper()
	original := g.Snapshot()
	start := slices.Index(original.Order, original.CurrentPlayerID)
	want := make([]PlayerID, 0, len(original.Order)+len(arrivals))
	for step := range original.Order {
		want = append(want, original.Order[(start+step*original.Direction+len(original.Order))%len(original.Order)])
	}
	for _, id := range arrivals {
		before := g.Snapshot()
		result := apply(t, g, Action{Type: JoinGame, PlayerID: id})
		after := g.Snapshot()
		for _, event := range result.Events {
			if event.Type == TurnChanged {
				t.Fatal("join emitted turn change")
			}
		}
		if before.CurrentPlayerID != after.CurrentPlayerID || before.Direction != after.Direction || before.ActiveColor != after.ActiveColor || before.DrawCounter != after.DrawCounter || before.Phase != after.Phase || before.DrawnCardID != after.DrawnCardID || before.Rules != after.Rules || !reflect.DeepEqual(before.Pending, after.Pending) || !reflect.DeepEqual(before.PendingBluff, after.PendingBluff) || !reflect.DeepEqual(before.Placements, after.Placements) || !reflect.DeepEqual(before.Players, after.Players[:len(before.Players)]) {
			t.Fatal("late join changed existing gameplay state")
		}
		if len(after.player(id).Hand) != 7 {
			t.Fatal("new player must receive seven cards")
		}
		want = append(want, id)
	}
	var got []PlayerID
	for range want {
		id := g.Snapshot().CurrentPlayerID
		got = append(got, id)
		apply(t, g, Action{Type: SkipTurn, PlayerID: id})
	}
	if !slices.Equal(got, want) || g.Snapshot().CurrentPlayerID != original.CurrentPlayerID {
		t.Fatalf("turn cycle = %v, want %v", got, want)
	}
}
