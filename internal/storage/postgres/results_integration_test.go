//go:build integration

package postgres

import (
	"errors"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"testing"
	"time"
)

func sampleResult(id string, system groups.RankingSystem) ranking.Result {
	now := time.Now().UTC().Truncate(time.Microsecond)
	r := ranking.Result{GameID: id, ChatID: 42, GameMode: groups.Classic, RankingSystem: system, ConfigRevision: 1, StartedAt: now.Add(-time.Minute), FinishedAt: now, FinalRevision: 20, FinishReason: "completed", PolicyVersion: ranking.PlacementPolicyV1}
	for i := 1; i <= 3; i++ {
		score, _ := ranking.Score(system, 3, i)
		status := "went_out"
		if i == 3 {
			status = "playing"
		}
		r.Players = append(r.Players, ranking.Player{UserID: int64(i), DisplayName: "name", Position: i, FinalStatus: status, WentOut: i < 3, Score: score})
	}
	return r
}
func prepareRanking(t *testing.T, system groups.RankingSystem) *Store {
	t.Helper()
	s := testStore(t)
	ctx := t.Context()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetOrCreateGroupConfig(ctx, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE group_configs SET ranking_system=$1 WHERE chat_id=42`, system); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCompletedGameIdempotencyAndAccumulation(t *testing.T) {
	for _, system := range []groups.RankingSystem{groups.Legacy, groups.Updated} {
		t.Run(string(system), func(t *testing.T) {
			s := prepareRanking(t, system)
			r := sampleResult("first", system)
			done := make(chan error, 2)
			for i := 0; i < 2; i++ {
				go func() { _, err := s.RecordCompletedGame(t.Context(), r); done <- err }()
			}
			for i := 0; i < 2; i++ {
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			}
			commit, err := s.RecordCompletedGame(t.Context(), r)
			if err != nil || !commit.AlreadyPersisted {
				t.Fatalf("retry %+v %v", commit, err)
			}
			r.GameID = "second"
			if _, err = s.RecordCompletedGame(t.Context(), r); err != nil {
				t.Fatal(err)
			}
			var score, count int64
			if err = s.pool.QueryRow(t.Context(), `SELECT score_units,completed_games FROM player_group_stats WHERE chat_id=42 AND user_id=1`).Scan(&score, &count); err != nil {
				t.Fatal(err)
			}
			want := int64(200)
			if system == groups.Updated {
				want = 2000
			}
			if score != want || count != 2 {
				t.Fatalf("duplicate scoring %d %d", score, count)
			}
			r.Players[0].DisplayName = "different"
			if _, err = s.RecordCompletedGame(t.Context(), r); !errors.Is(err, ranking.ErrConflict) {
				t.Fatalf("conflicting retry %v", err)
			}
		})
	}
}
func TestResultRollbackAfterPartialWrites(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	ctx := t.Context()
	// Force failure on player 2, after game, player 1 and player 1 stats inserts.
	if _, err := s.pool.Exec(ctx, `ALTER TABLE completed_game_players ADD CONSTRAINT fail_second CHECK(user_id<>2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCompletedGame(ctx, sampleResult("rollback", groups.Legacy)); err == nil {
		t.Fatal("forced failure succeeded")
	}
	for _, table := range []string{"completed_games", "completed_game_players", "player_group_stats"} {
		var n int
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatal("partial persistence", table, n)
		}
	}
}
func TestPendingPolicyDoesNotScore(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	r := sampleResult("pending", groups.Legacy)
	r.PolicyVersion = ""
	for i := range r.Players {
		r.Players[i].Score = 0
	}
	commit, err := s.RecordCompletedGame(t.Context(), r)
	if err != nil || commit.Scored {
		t.Fatalf("pending %+v %v", commit, err)
	}
	var n int
	if err = s.pool.QueryRow(t.Context(), `SELECT count(*) FROM player_group_stats`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("pending scored %d %v", n, err)
	}
	r.GameID = "cancelled"
	r.FinishReason = "cancelled"
	if _, err = s.RecordCompletedGame(t.Context(), r); !errors.Is(err, ranking.ErrInvalid) {
		t.Fatal("cancelled scored", err)
	}
}
func TestIncompatibleRankingRollsBack(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	r := sampleResult("legacy", groups.Legacy)
	if _, err := s.RecordCompletedGame(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(t.Context(), `UPDATE group_configs SET ranking_system='updated' WHERE chat_id=42`); err != nil {
		t.Fatal(err)
	}
	r = sampleResult("updated", groups.Updated)
	if _, err := s.RecordCompletedGame(t.Context(), r); !errors.Is(err, ranking.ErrNeedsProductDecision) {
		t.Fatal("systems mixed", err)
	}
	var n int
	s.pool.QueryRow(t.Context(), `SELECT count(*) FROM completed_games`).Scan(&n)
	if n != 1 {
		t.Fatal("incompatible game partially stored", n)
	}
}
