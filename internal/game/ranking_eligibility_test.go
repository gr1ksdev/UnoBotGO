package game

import (
	"errors"
	"fmt"
	"testing"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
)

// Deterministic physical cards let lifecycle tests run through real service
// actions, dealing and placements without modifying an engine snapshot.
func rankingGame(t *testing.T, n int) (*Service, PublicGameView) {
	t.Helper()
	s := testService(t)
	s.manager.newGame = func(id uno.GameID, rules uno.Rules) (*uno.Game, error) {
		cards := make([]uno.Card, 108)
		for i := range cards {
			cards[i] = uno.Card{ID: uno.CardID(fmt.Sprint(i)), Color: uno.Red, Rank: uno.One}
		}
		return uno.NewGame(id, rules, uno.WithDeck(cards), uno.WithShuffler(func([]uno.CardID) {}))
	}
	out, err := s.Create(t.Context(), Actor{ChatID: 42, PlayerID: 1}, CreateRequest{Rules: uno.BotRules(), GroupConfig: groups.Snapshot{RankingSystem: groups.Updated, ConfigRevision: 1}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		join(t, s, out.View, uno.PlayerID(i))
	}
	return s, out.View
}
func finishRankingGame(t *testing.T, s *Service, v PublicGameView) ranking.Result {
	t.Helper()
	for i := 0; i < 200; i++ {
		view, err := s.PublicView(t.Context(), v.GameID)
		if err != nil {
			t.Fatal(err)
		}
		actor := Actor{ChatID: 42, PlayerID: view.CurrentTurn}
		hand, err := s.PlayerView(t.Context(), actor, v.GameID)
		if err != nil {
			t.Fatal(err)
		}
		out := act(t, s, v.GameID, actor, uno.Action{Type: uno.PlayCard, CardID: hand.Hand[0].Card.ID})
		if out.Completed != nil {
			r, err := ranking.Prepare(*out.Completed)
			if err != nil {
				t.Fatal(err)
			}
			return r
		}
		if len(s.PendingResults()) != 0 {
			t.Fatal("partial placement produced final result")
		}
	}
	t.Fatal("game did not finish")
	return ranking.Result{}
}
func TestRankingEligibilityThroughLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name         string
		initial      int
		change       func(*testing.T, *Service, PublicGameView)
		wantN, users int
	}{
		{"normal_two", 2, nil, 2, 2}, {"normal_three", 3, nil, 3, 3}, {"normal_eight", 8, nil, 8, 8},
		{"one_abandonment", 8, func(t *testing.T, s *Service, v PublicGameView) {
			act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 8}, uno.Action{Type: uno.LeaveGame})
		}, 7, 8},
		{"two_abandonments", 8, func(t *testing.T, s *Service, v PublicGameView) {
			for _, id := range []uno.PlayerID{7, 8} {
				act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: id}, uno.Action{Type: uno.LeaveGame})
			}
		}, 6, 8},
		{"late_join", 3, func(t *testing.T, s *Service, v PublicGameView) { join(t, s, v, 4) }, 4, 4},
		{"reentry_concludes", 4, func(t *testing.T, s *Service, v PublicGameView) {
			act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 2}, uno.Action{Type: uno.LeaveGame})
			join(t, s, v, 2)
		}, 4, 4},
		{"reentry_abandons", 4, func(t *testing.T, s *Service, v PublicGameView) {
			a := Actor{ChatID: 42, PlayerID: 2}
			act(t, s, v.GameID, a, uno.Action{Type: uno.LeaveGame})
			join(t, s, v, 2)
			act(t, s, v.GameID, a, uno.Action{Type: uno.LeaveGame})
		}, 3, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, v := rankingGame(t, tt.initial)
			act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 1}, uno.Action{Type: uno.StartGame})
			if tt.change != nil {
				tt.change(t, s, v)
			}
			r := finishRankingGame(t, s, v)
			if r.EligibleCount() != tt.wantN || len(r.Players) != tt.users {
				t.Fatalf("wrong counts %+v", r)
			}
			for _, p := range r.Players {
				if !p.Eligible() && (p.Score != 0 || p.Position != 0 || p.FinalStatus != "left") {
					t.Fatal("abandonment affected ranking", p)
				}
			}
			if tt.name == "late_join" && !r.Players[3].JoinedAfterStart {
				t.Fatal("late join audit missing")
			}
			if tt.name == "reentry_abandons" && (r.Players[1].LeaveCount != 2 || r.Players[1].ReentryCount != 1) {
				t.Fatal("reentry audit missing")
			}
		})
	}
}
func TestLobbyOnlyParticipantExcluded(t *testing.T) {
	s, v := rankingGame(t, 4)
	act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 4}, uno.Action{Type: uno.LeaveGame})
	act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 1}, uno.Action{Type: uno.StartGame})
	r := finishRankingGame(t, s, v)
	if r.EligibleCount() != 3 || len(r.Players) != 4 || r.Players[3].Position != 0 || r.Players[3].Score != 0 {
		t.Fatal("lobby member affected N", r)
	}
}
func TestPlacedPlayerCannotDuplicateInRanking(t *testing.T) {
	s, v := rankingGame(t, 3)
	out := act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 1}, uno.Action{Type: uno.StartGame})
	for len(out.View.Placements) == 0 {
		actor := Actor{ChatID: 42, PlayerID: out.View.CurrentTurn}
		hand, _ := s.PlayerView(t.Context(), actor, v.GameID)
		out = act(t, s, v.GameID, actor, uno.Action{Type: uno.PlayCard, CardID: hand.Hand[0].Card.ID})
	}
	id := out.View.Placements[0].PlayerID
	_, err := s.Apply(t.Context(), Actor{ChatID: 42, PlayerID: id}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: id, Revision: out.View.Revision})
	if !errors.Is(err, uno.ErrAlreadyFinished) {
		t.Fatal("placed player reentered", err)
	}
	r := finishRankingGame(t, s, v)
	count := 0
	for _, p := range r.Players {
		if p.UserID == int64(id) {
			count++
			if p.Position != 1 {
				t.Fatal("placement changed")
			}
		}
	}
	if count != 1 || r.EligibleCount() != 3 {
		t.Fatal("placed player duplicated")
	}
}
func TestDepartureUsesExistingPlacementsForRanking(t *testing.T) {
	for _, withWinner := range []bool{false, true} {
		t.Run(fmt.Sprint(withWinner), func(t *testing.T) {
			n := 2
			if withWinner {
				n = 3
			}
			s, v := rankingGame(t, n)
			out := act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: 1}, uno.Action{Type: uno.StartGame})
			if withWinner {
				for len(out.View.Placements) == 0 {
					actor := Actor{ChatID: 42, PlayerID: out.View.CurrentTurn}
					hand, _ := s.PlayerView(t.Context(), actor, v.GameID)
					out = act(t, s, v.GameID, actor, uno.Action{Type: uno.PlayCard, CardID: hand.Hand[0].Card.ID})
				}
			}
			out = act(t, s, v.GameID, Actor{ChatID: 42, PlayerID: out.View.CurrentTurn}, uno.Action{Type: uno.LeaveGame})
			if out.Completed == nil || out.Completed.FinishReason != "departure" {
				t.Fatal("expected departure")
			}
			r, err := ranking.Prepare(*out.Completed)
			if err != nil {
				t.Fatal(err)
			}
			if withWinner {
				if r.EligibleCount() != 2 || r.ScoringStatus() != ranking.StatusScored {
					t.Fatal("departure placements excluded")
				}
			} else {
				if r.EligibleCount() != 1 || r.ScoringStatus() != ranking.StatusInsufficientPlayers {
					t.Fatal("N1 awarded")
				}
			}
		})
	}
}
