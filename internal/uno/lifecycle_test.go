package uno

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

func TestLateJoinAndDeparture(t *testing.T) {
	for _, direction := range []int{1, -1} {
		t.Run(fmt.Sprint(direction), func(t *testing.T) {
			g := scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}, {card(Green, One)}}, card(Red, Five), nil)
			g.state.Direction = direction
			before := g.Snapshot()
			r := apply(t, g, Action{Type: JoinGame, PlayerID: 4})
			s := g.Snapshot()
			if s.CurrentPlayerID != 1 || len(s.Players[3].Hand) != 7 || len(s.DrawPile) != len(before.DrawPile)-7 || !hasEvent(r, PlayerJoined, 4) {
				t.Fatal("late join")
			}
			if s.next(1, -1) != 4 {
				t.Fatal("new seat must be before current in direction of play")
			}
			top := s.DiscardPile[len(s.DiscardPile)-1]
			apply(t, g, Action{Type: LeaveGame, PlayerID: 4})
			s = g.Snapshot()
			if s.DiscardPile[len(s.DiscardPile)-1] != top || len(s.DiscardPile) != 8 || s.Players[3].Status != Left {
				t.Fatal("departed cards not recycled below top")
			}
			rejected(t, g, Action{Type: JoinGame, PlayerID: 4, Revision: s.Revision}, ErrAlreadyJoined)
			expected := s.next(1, 1)
			apply(t, g, Action{Type: LeaveGame, PlayerID: 1})
			if g.Snapshot().CurrentPlayerID != expected || g.Snapshot().Phase != TakingTurn {
				t.Fatal("departure turn")
			}
		})
	}
}

func TestPlacementPolicy(t *testing.T) {
	g := scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Red, Two)}, {card(Red, Three)}}, card(Red, Five), nil)
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
	s := g.Snapshot()
	if s.Phase != TakingTurn || s.CurrentPlayerID != 2 || len(s.Placements) != 1 || s.Players[0].Status != WentOut {
		t.Fatal("did not continue after first placement")
	}
	rejected(t, g, Action{Type: JoinGame, PlayerID: 1, Revision: s.Revision}, ErrAlreadyJoined)
	copy := g.Snapshot()
	copy.Placements[0].PlayerID = 99
	if g.Snapshot().Placements[0].PlayerID != 1 {
		t.Fatal("placement snapshot alias")
	}
	apply(t, g, Action{Type: PlayCard, PlayerID: 2, CardID: s.Players[1].Hand[0]})
	s = g.Snapshot()
	if s.Phase != Finished || len(s.Placements) != 3 || s.Placements[2].WentOut || s.Placements[2].PlayerID != 3 || s.FinishReason != FinishedNormally {
		t.Fatalf("final placements %+v", s.Placements)
	}
}

func TestPlacementLastActionEffects(t *testing.T) {
	for _, rank := range []Rank{Skip, Reverse, DrawTwo, Wild, WildDrawFour} {
		t.Run(fmt.Sprint(rank), func(t *testing.T) {
			color := Red
			if rank >= Wild {
				color = NoColor
			}
			g := scenario(t, BotRules(), [][]Card{{card(color, rank)}, {card(Blue, One)}, {card(Green, One)}}, card(Red, Five), nil)
			apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
			if rank >= Wild {
				apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Blue})
			}
			s := g.Snapshot()
			next := PlayerID(3)
			if rank == Wild {
				next = 2
			}
			if rank == DrawTwo && s.Rules.StackDrawTwo {
				next = 2
			}
			if s.Phase != TakingTurn || s.CurrentPlayerID != next || len(s.Order) != 2 {
				t.Fatalf("placement turn %+v", s)
			}
			if rank == DrawTwo {
				if s.Rules.StackDrawTwo {
					if s.DrawCounter != 2 {
						t.Fatalf("expected DrawCounter 2, got %d", s.DrawCounter)
					}
					apply(t, g, Action{Type: DrawCard, PlayerID: 2})
					s = g.Snapshot()
					if len(s.Players[1].Hand) != 3 || s.CurrentPlayerID != 3 {
						t.Fatalf("expected player 2 to have 3 cards and turn to pass to 3, got hand=%d current=%d", len(s.Players[1].Hand), s.CurrentPlayerID)
					}
				} else if len(s.Players[1].Hand) != 3 {
					t.Fatal("last +2")
				}
			}
			if rank == WildDrawFour && len(s.Players[1].Hand) != 5 {
				t.Fatal("last +4")
			}
		})
	}
}

func TestPendingMembershipChanges(t *testing.T) {
	g := scenario(t, BotRules(), [][]Card{{card(NoColor, WildDrawFour), card(Blue, One)}, {card(Green, One)}, {card(Yellow, One)}}, card(Red, Five), nil)
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
	s := g.Snapshot()
	rejected(t, g, Action{Type: LeaveGame, PlayerID: 1, Revision: s.Revision}, ErrColorChoiceRequired)
	rejected(t, g, Action{Type: ChooseColor, PlayerID: 2, Color: Red, Revision: s.Revision}, ErrNotYourTurn)
	apply(t, g, Action{Type: JoinGame, PlayerID: 4})
	if g.Snapshot().Pending.Target != 2 {
		t.Fatal("late join stole penalty")
	}
	apply(t, g, Action{Type: LeaveGame, PlayerID: 2})
	if g.Snapshot().Pending.Target != 3 {
		t.Fatal("pending target not repaired")
	}
	apply(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Green})
	if len(g.Snapshot().Players[2].Hand) != 5 || g.Snapshot().CurrentPlayerID != 4 {
		t.Fatal("pending penalty after membership change")
	}
}

func TestDepartureAndCancellation(t *testing.T) {
	for _, kind := range []ActionType{LeaveGame, CancelGame} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			g := scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}}, card(Red, Five), nil)
			r := apply(t, g, Action{Type: kind, PlayerID: 1})
			s := g.Snapshot()
			if s.Phase != Finished || hasEvent(r, PlayerWon, 2) {
				t.Fatal("departure/cancellation manufactured played-out win")
			}
			if kind == LeaveGame {
				if s.FinishReason != FinishedByDeparture || len(s.Placements) != 1 || s.Placements[0].PlayerID != 2 || s.Placements[0].WentOut {
					t.Fatal("departure result")
				}
			} else if s.FinishReason != FinishedByCancellation || len(s.Placements) != 0 {
				t.Fatal("cancellation result")
			}
		})
	}
	// Cancelling a pending initial Wild must not require choosing a color.
	g := scenario(t, ClassicRules(), [][]Card{{card(NoColor, Wild)}, {card(Blue, One)}}, card(Red, Five), nil)
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: g.Snapshot().Players[0].Hand[0]})
	apply(t, g, Action{Type: CancelGame, PlayerID: 2})
	if g.Snapshot().Pending != nil {
		t.Fatal("pending cancellation")
	}
}

func TestLobbyAndLimits(t *testing.T) {
	g, err := NewGame("lobby", ClassicRules(), WithShuffler(noShuffle))
	if err != nil {
		t.Fatal(err)
	}
	apply(t, g, Action{Type: JoinGame, PlayerID: 1})
	s := g.Snapshot()
	rejected(t, g, Action{Type: StartGame, PlayerID: 1, DealerID: 1, Revision: s.Revision}, ErrNotEnoughPlayers)
	rejected(t, g, Action{Type: DrawCard, PlayerID: 1, Revision: s.Revision}, ErrGameNotStarted)
	rejected(t, g, Action{Type: JoinGame, PlayerID: 1, Revision: s.Revision}, ErrAlreadyJoined)
	apply(t, g, Action{Type: LeaveGame, PlayerID: 1})
	if g.Snapshot().Phase != Lobby || len(g.Snapshot().Order) != 0 {
		t.Fatal("empty lobby")
	}
	for id := PlayerID(2); id <= 10; id++ {
		apply(t, g, Action{Type: JoinGame, PlayerID: id})
	}
	rejected(t, g, Action{Type: JoinGame, PlayerID: 11, Revision: g.Snapshot().Revision}, ErrPlayerLimit)
	rejected(t, g, Action{Type: StartGame, PlayerID: 2, DealerID: 99, Revision: g.Snapshot().Revision}, ErrUnknownPlayer)
	apply(t, g, Action{Type: StartGame, PlayerID: 2, DealerID: 10})
	rejected(t, g, Action{Type: StartGame, PlayerID: 2, DealerID: 10, Revision: g.Snapshot().Revision}, ErrGameStarted)
	g = scenario(t, ClassicRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}}, card(Red, Five), nil)
	rejected(t, g, Action{Type: JoinGame, PlayerID: 3}, ErrLobbyClosed)
	g.state.Revision = math.MaxUint64
	rejected(t, g, Action{Type: DrawCard, PlayerID: 1, Revision: math.MaxUint64}, ErrInvalidState)
}

func TestIdenticalCardsHaveIndependentOwnership(t *testing.T) {
	g := scenario(t, ClassicRules(), [][]Card{{card(Red, One), card(Red, One)}, {card(Red, One)}}, card(Red, Five), nil)
	before := g.Snapshot()
	first, second := before.Players[0].Hand[0], before.Players[0].Hand[1]
	apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: second})
	s := g.Snapshot()
	if !reflect.DeepEqual(s.Players[0].Hand, []CardID{first}) || s.DiscardPile[len(s.DiscardPile)-1] != second {
		t.Fatal("removed wrong physical copy")
	}
	rejected(t, g, Action{Type: PlayCard, PlayerID: 2, CardID: first, Revision: s.Revision}, ErrCardNotOwned)
}

func TestDeckRecyclingAndAtomicFailure(t *testing.T) {
	g := scenario(t, ClassicRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}}, card(Red, Five), []Card{card(Green, Two), card(Yellow, Three)})
	// Move all draw cards below the current discard, preserving inventory.
	s := g.Snapshot()
	top := s.DiscardPile[0]
	s.DiscardPile = append(s.DrawPile, s.DiscardPile...)
	s.DrawPile = nil
	g, err := Restore(s, noShuffle)
	if err != nil {
		t.Fatal(err)
	}
	apply(t, g, Action{Type: DrawCard, PlayerID: 1})
	if !reflect.DeepEqual(g.Snapshot().DiscardPile, []CardID{top}) || slices.Contains(g.Snapshot().Players[0].Hand, top) {
		t.Fatal("top recycled")
	}
	// Even partial availability must not commit a partial +2 or color choice.
	for _, rank := range []Rank{DrawTwo, WildDrawFour} {
		color := Red
		if rank == WildDrawFour {
			color = NoColor
		}
		g = scenario(t, ClassicRules(), [][]Card{{card(color, rank), card(Blue, One)}, {card(Green, One)}}, card(Red, Five), []Card{})
		id := g.Snapshot().Players[0].Hand[0]
		if rank == DrawTwo {
			rejected(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id}, ErrDeckEmpty)
		} else {
			apply(t, g, Action{Type: PlayCard, PlayerID: 1, CardID: id})
			rejected(t, g, Action{Type: ChooseColor, PlayerID: 1, Color: Blue, Revision: g.Snapshot().Revision}, ErrDeckEmpty)
		}
	}
	g = scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}}, card(Red, Five), []Card{})
	rejected(t, g, Action{Type: DrawCard, PlayerID: 1}, ErrDeckEmpty)
	rejected(t, g, Action{Type: JoinGame, PlayerID: 3}, ErrDeckEmpty)
}

func TestValidateRejectsCorruptSnapshots(t *testing.T) {
	g := scenario(t, ClassicRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}}, card(Red, Five), nil)
	for _, tt := range []struct {
		name    string
		corrupt func(*State)
	}{
		{"duplicate card", func(s *State) { s.DrawPile = append(s.DrawPile, s.Players[0].Hand[0]) }},
		{"missing card", func(s *State) { s.DrawPile = s.DrawPile[1:] }},
		{"unknown card", func(s *State) { s.DrawPile[0] = "missing" }},
		{"duplicate inventory", func(s *State) { s.Cards = append(s.Cards, s.Cards[0]) }},
		{"invalid rank", func(s *State) { s.Cards[0].Rank = 255 }},
		{"wild physical color", func(s *State) { s.Cards[0].Rank = Wild }},
		{"missing turn", func(s *State) { s.CurrentPlayerID = 99 }},
		{"duplicate seat", func(s *State) { s.Order = append(s.Order, 1) }},
		{"unknown seat", func(s *State) { s.Order[0] = 99 }},
		{"inactive hand", func(s *State) { s.Players[0].Status = Left }},
		{"invalid direction", func(s *State) { s.Direction = 0 }},
		{"invalid policy", func(s *State) { s.Rules.EndPolicy = 255 }},
		{"pending mismatch", func(s *State) { s.Pending = &ColorChoice{Actor: 1} }},
		{"drawn card", func(s *State) { s.DrawnCardID = s.Players[1].Hand[0] }},
		{"top color", func(s *State) { s.ActiveColor = Yellow }},
		{"placement", func(s *State) { s.Placements = []Placement{{PlayerID: 99, Position: 1}} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := g.Snapshot()
			tt.corrupt(&s)
			if _, err := Restore(s, noShuffle); !errors.Is(err, ErrInvalidState) {
				t.Fatalf("corruption accepted: %v", err)
			}
		})
	}
}

func TestDeterministicCompleteGames(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		for _, rules := range []Rules{ClassicRules(), BotRules()} {
			t.Run(fmt.Sprintf("%d/%d", seed, rules.EndPolicy), func(t *testing.T) {
				rng := rand.New(rand.NewPCG(seed, seed+1))
				shuffle := func(ids []CardID) { rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] }) }
				g, err := NewGame("simulation", rules, WithShuffler(shuffle))
				if err != nil {
					t.Fatal(err)
				}
				for id := PlayerID(1); id <= 4; id++ {
					apply(t, g, Action{Type: JoinGame, PlayerID: id})
				}
				apply(t, g, Action{Type: StartGame, PlayerID: 1, DealerID: 4})
				for step := 0; step < 3000; step++ {
					s := g.Snapshot()
					if s.Phase == Finished {
						return
					}
					a := Action{PlayerID: s.CurrentPlayerID, Type: DrawCard}
					if s.Phase == ChoosingColor {
						a.Type = ChooseColor
						a.Color = Color(1 + rng.IntN(4))
					} else {
						if s.DrawnCardID != "" {
							a.Type = PassTurn
						}
						for _, id := range s.player(s.CurrentPlayerID).Hand {
							if g.CanPlay(s.CurrentPlayerID, id) == nil {
								a.Type = PlayCard
								a.CardID = id
								break
							}
						}
					}
					apply(t, g, a)
					if len(g.Snapshot().Cards) != 108 {
						t.Fatal("inventory size changed")
					}
				}
				t.Fatal("deterministic game did not finish")
			})
		}
	}
}

func TestEndDuringInitialColorChoice(t *testing.T) {
	for _, action := range []ActionType{LeaveGame, CancelGame} {
		t.Run(fmt.Sprint(action), func(t *testing.T) {
			g := scenario(t, BotRules(), [][]Card{{card(Red, One)}, {card(Blue, One)}}, card(NoColor, Wild), nil)
			s := g.Snapshot()
			s.Phase = ChoosingColor
			s.ActiveColor = NoColor
			s.Pending = &ColorChoice{Actor: 1, Target: 2, Initial: true}
			var err error
			g, err = Restore(s, noShuffle)
			if err != nil {
				t.Fatal(err)
			}
			apply(t, g, Action{Type: action, PlayerID: 2})
			if g.Snapshot().Phase != Finished || g.Snapshot().Pending != nil {
				t.Fatal("initial color choice survived end")
			}
		})
	}
}

func TestInvalidShufflerCannotCorruptCommittedState(t *testing.T) {
	g, err := NewGame("shuffle", ClassicRules(), WithShuffler(func(ids []CardID) { ids[0] = ids[1] }))
	if err != nil {
		t.Fatal(err)
	}
	apply(t, g, Action{Type: JoinGame, PlayerID: 1})
	apply(t, g, Action{Type: JoinGame, PlayerID: 2})
	rejected(t, g, Action{Type: StartGame, PlayerID: 1, DealerID: 2, Revision: 2}, ErrInvalidState)
}
