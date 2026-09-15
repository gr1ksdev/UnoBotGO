package game

import (
	"errors"
	"strconv"
	"testing"

	"github.com/malbs/UnoGoBot/internal/uno"
)

// Exact, validated positions; fixtures never become a public recovery API.
func position(t *testing.T, s *Service, rules uno.Rules, wild bool) PublicGameView {
	t.Helper()
	v := create(t, s, 1, 1, rules)
	cards := []uno.Card{{ID: "top", Color: uno.Red, Rank: uno.One}, {ID: "a", Color: uno.Red, Rank: uno.Two}, {ID: "b", Color: uno.Red, Rank: uno.Three}, {ID: "c", Color: uno.Blue, Rank: uno.Four}, {ID: "draw", Color: uno.Red, Rank: uno.Five}}
	if wild {
		cards[1] = uno.Card{ID: "a", Rank: uno.Wild}
	}
	state := uno.State{ID: v.GameID, Revision: 10, Rules: rules, Phase: uno.TakingTurn, Cards: cards, DiscardPile: []uno.CardID{"top"}, DrawPile: []uno.CardID{"draw"}, Players: []uno.Player{{ID: 1, Hand: []uno.CardID{"a"}}, {ID: 2, Hand: []uno.CardID{"b"}}, {ID: 3, Hand: []uno.CardID{"c"}}}, Order: []uno.PlayerID{1, 2, 3}, DealerID: 3, CurrentPlayerID: 1, Direction: 1, ActiveColor: uno.Red}
	engine, err := uno.Restore(state, func([]uno.CardID) {})
	if err != nil {
		t.Fatal(err)
	}
	e, err := s.manager.lockGame(t.Context(), v.GameID)
	if err != nil {
		t.Fatal(err)
	}
	before := e.engine.Snapshot()
	e.engine = engine
	out := s.manager.publish(e, before, state, uno.Result{})
	e.mu.Unlock()
	assertIndexes(t, s)
	return out.View
}

func TestPlacementsAndCompletion(t *testing.T) {
	for _, wild := range []bool{false, true} {
		t.Run(map[bool]string{false: "number", true: "wild"}[wild], func(t *testing.T) {
			s := testService(t)
			v := position(t, s, uno.BotRules(), wild)
			out := act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "a"})
			if wild {
				if out.View.OwnerID != 1 || out.View.ColorChooserID != 1 {
					t.Fatal("pending wild transferred owner")
				}
				games, _ := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
				if len(games) != 1 {
					t.Fatal("chooser lost routing")
				}
				hand, err := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID)
				if err != nil || len(hand.Hand) != 0 {
					t.Fatal("empty chooser hand", err)
				}
				out = act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.ChooseColor, Color: uno.Red})
			}
			if out.View.OwnerID != 2 || len(out.View.Placements) != 1 || out.View.Players[0].Status != uno.WentOut {
				t.Fatal("placement/owner", out.View)
			}
			games, _ := s.FindPlayerGames(t.Context(), Actor{PlayerID: 1})
			if len(games) != 0 {
				t.Fatal("finisher routed")
			}
			if _, err := s.PlayerView(t.Context(), Actor{PlayerID: 1}, v.GameID); !errors.Is(err, ErrNotParticipant) {
				t.Fatal(err)
			}
			assertIndexes(t, s)
			out = act(t, s, v.GameID, Actor{PlayerID: 2}, uno.Action{Type: uno.PlayCard, CardID: "b"})
			if !out.View.Closed || out.View.CloseReason != Completed || len(out.View.Placements) != 3 || out.View.OwnerID != 0 {
				t.Fatal(out.View)
			}
			assertIndexes(t, s)
			if _, err := s.PlayerView(t.Context(), Actor{PlayerID: 3}, v.GameID); !errors.Is(err, ErrGameClosed) {
				t.Fatal(err)
			}
			next := create(t, s, 1, 99, uno.ClassicRules())
			denied(t, s, v.GameID, Actor{PlayerID: 2, ChatID: 1}, uno.Action{Type: uno.CancelGame, PlayerID: 2, Revision: out.View.Revision}, ErrGameClosed)
			got, _ := s.FindChatGame(t.Context(), 1)
			if got.GameID != next.GameID {
				t.Fatal("old action touched new game")
			}
		})
	}
}

func TestFirstWinnerAndDeparture(t *testing.T) {
	for _, leave := range []bool{false, true} {
		t.Run(map[bool]string{false: "winner", true: "departure"}[leave], func(t *testing.T) {
			s := testService(t)
			v := position(t, s, uno.ClassicRules(), false)
			var out Outcome
			if leave {
				act(t, s, v.GameID, Actor{PlayerID: 1, ChatID: 1}, uno.Action{Type: uno.LeaveGame})
				out = act(t, s, v.GameID, Actor{PlayerID: 2, ChatID: 1}, uno.Action{Type: uno.LeaveGame})
			} else {
				out = act(t, s, v.GameID, Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, CardID: "a"})
			}
			if !out.View.Closed {
				t.Fatal("not closed")
			}
			if leave && out.View.CloseReason != Departure {
				t.Fatal(out.View)
			}
			for _, p := range out.View.Players {
				if p.Active {
					t.Fatal("closed participant active")
				}
			}
			assertIndexes(t, s)
		})
	}
}

func TestBoundedHistory(t *testing.T) {
	for _, limit := range []int{0, 1, 100} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			var options []Option
			if limit != 100 {
				options = []Option{WithHistoryLimit(limit)}
			}
			s := testService(t, options...)
			open := create(t, s, 2, 9, uno.ClassicRules())
			var ids []uno.GameID
			for i := 0; i < limit+2; i++ {
				v := create(t, s, 1, 9, uno.ClassicRules())
				ids = append(ids, v.GameID)
				act(t, s, v.GameID, Actor{PlayerID: 9, ChatID: 1}, uno.Action{Type: uno.CancelGame})
			}
			for i, id := range ids {
				v, err := s.PublicView(t.Context(), id)
				if i < 2 {
					if !errors.Is(err, ErrGameNotFound) {
						t.Fatal("not evicted", err)
					}
				} else if err != nil || !v.Closed {
					t.Fatal("missing retained summary", err)
				}
			}
			if _, err := s.PublicView(t.Context(), open.GameID); err != nil {
				t.Fatal("open game evicted", err)
			}
			if len(s.manager.history) != limit || len(s.manager.byID) != limit+1 {
				t.Fatal("unbounded history")
			}
			assertIndexes(t, s)
		})
	}
}

func TestRejectedActionsPreserveMembership(t *testing.T) {
	s := testService(t)
	v := position(t, s, uno.BotRules(), false)
	cases := []struct {
		actor  Actor
		action uno.Action
		err    error
	}{
		{Actor{PlayerID: 4, ChatID: 1}, uno.Action{Type: uno.JoinGame, PlayerID: 4, Revision: 10}, uno.ErrDeckEmpty},
		{Actor{PlayerID: 1}, uno.Action{Type: uno.PlayCard, PlayerID: 1, CardID: "b", Revision: 10}, uno.ErrCardNotOwned},
		{Actor{PlayerID: 2}, uno.Action{Type: uno.DrawCard, PlayerID: 2, Revision: 10}, uno.ErrNotYourTurn},
		{Actor{PlayerID: 1}, uno.Action{Type: uno.DrawCard, PlayerID: 1, Revision: 9}, uno.ErrStaleRevision},
	}
	for _, tc := range cases {
		denied(t, s, v.GameID, tc.actor, tc.action, tc.err)
	}
	act(t, s, v.GameID, Actor{PlayerID: 3, ChatID: 1}, uno.Action{Type: uno.LeaveGame})
	if _, err := s.PlayerView(t.Context(), Actor{PlayerID: 3}, v.GameID); !errors.Is(err, ErrNotParticipant) {
		t.Fatal(err)
	}
	assertIndexes(t, s)
}
