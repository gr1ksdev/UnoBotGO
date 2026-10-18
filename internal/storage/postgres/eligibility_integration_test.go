//go:build integration

package postgres

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

func eligibleResult(t *testing.T, system groups.RankingSystem, n, abandoned int, reason string) ranking.Result {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	r := ranking.Result{GameID: "eligibility", ChatID: 42, GameMode: groups.Classic, RankingSystem: system, ConfigRevision: 1, StartedAt: now.Add(-time.Minute), FinishedAt: now, FinalRevision: 100, FinishReason: reason}
	for i := 1; i <= n; i++ {
		status := "went_out"
		if i == n {
			status = "playing"
		}
		r.Players = append(r.Players, ranking.Player{UserID: int64(i), DisplayName: fmt.Sprint(i), Position: i, FinalStatus: status, WentOut: i < n})
	}
	for i := 1; i <= abandoned; i++ {
		r.Players = append(r.Players, ranking.Player{UserID: int64(n + i), DisplayName: "abandoned", FinalStatus: "left", LeaveCount: 1})
	}
	result, err := ranking.Prepare(r)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestEligibilityTransaction(t *testing.T) {
	for _, system := range []groups.RankingSystem{groups.Legacy, groups.Updated} {
		for _, tt := range []struct {
			name         string
			n, abandoned int
			reason       string
			expected     []int64
		}{
			{"normal_two", 2, 0, "completed", []int64{1000, 0}},
			{"normal_three", 3, 0, "completed", []int64{1000, 500, 0}},
			{"normal_eight", 8, 0, "completed", []int64{1000, 857, 714, 571, 429, 286, 143, 0}},
			{"one_abandoned", 7, 1, "completed", []int64{1000, 833, 667, 500, 333, 167, 0}},
			{"two_abandoned", 6, 2, "completed", []int64{1000, 800, 600, 400, 200, 0}},
			{"departure", 2, 1, "departure", []int64{1000, 0}},
			{"single_survivor", 1, 1, "departure", []int64{0}},
			{"no_eligible_players", 0, 2, "departure", nil},
		} {
			t.Run(string(system)+"/"+tt.name, func(t *testing.T) {
				s := prepareRanking(t, system)
				r := eligibleResult(t, system, tt.n, tt.abandoned, tt.reason)
				// All arrivals/reentries concluding in a placement use the same transaction.
				if tt.n > 2 {
					r.Players[0].JoinedAfterStart = true
					r.Players[1].JoinedAfterStart = true
					r.Players[1].LeaveCount = 1
					r.Players[1].ReentryCount = 1
				}
				ctx := t.Context()
				done := make(chan error, 2)
				for i := 0; i < 2; i++ {
					go func() { _, err := s.RecordCompletedGame(ctx, r); done <- err }()
				}
				for i := 0; i < 2; i++ {
					if err := <-done; err != nil {
						t.Fatal(err)
					}
				}
				commit, err := s.RecordCompletedGame(ctx, r)
				if err != nil || !commit.AlreadyPersisted || commit.Scored != (tt.n >= 2) {
					t.Fatalf("retry %+v %v", commit, err)
				}
				var status string
				var count int
				var policy *string
				var scoredAt *time.Time
				if err = s.pool.QueryRow(ctx, `SELECT scoring_status,participant_count,policy_version,scored_at FROM completed_games WHERE game_id=$1`, r.GameID).Scan(&status, &count, &policy, &scoredAt); err != nil {
					t.Fatal(err)
				}
				if status != r.ScoringStatus() || count != tt.n+tt.abandoned || policy == nil || *policy != ranking.PlacementPolicyV1 || (scoredAt != nil) != (tt.n >= 2) {
					t.Fatal("audit header inconsistent", status, count, policy, scoredAt)
				}
				for _, p := range r.Players {
					var pos *int
					var score *int64
					if err = s.pool.QueryRow(ctx, `SELECT position,score_units FROM completed_game_players WHERE game_id=$1 AND user_id=$2`, r.GameID, p.UserID).Scan(&pos, &score); err != nil {
						t.Fatal(err)
					}
					want := int64(0)
					if p.Eligible() {
						want = tt.expected[p.Position-1]
						if system == groups.Legacy {
							want = 0
							if tt.n >= 2 && p.Position < tt.n {
								want = 100
							}
						}
					}
					if (pos != nil) != p.Eligible() || score == nil || *score != want {
						t.Fatalf("bad result user=%d pos=%v score=%v want=%d", p.UserID, pos, score, want)
					}
					var scoreTotal, played, wins int64
					err = s.pool.QueryRow(ctx, `SELECT score_units,completed_games,wins FROM player_group_stats WHERE chat_id=42 AND user_id=$1`, p.UserID).Scan(&scoreTotal, &played, &wins)
					if !p.Eligible() || tt.n < 2 {
						if !errors.Is(err, pgx.ErrNoRows) {
							t.Fatal("expected no stats for ineligible/no-award result", p, err)
						}
					} else {
						expectedWins := int64(0)
						if p.Position == 1 {
							expectedWins = 1
						}
						if err != nil || scoreTotal != want || played != 1 || wins != expectedWins {
							t.Fatalf("duplicate/wrong stats %d %d %d: %v", scoreTotal, played, wins, err)
						}
					}
				}
			})
		}
	}
}

func TestAbandonmentDoesNotModifyExistingStats(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	first := eligibleResult(t, groups.Updated, 3, 0, "completed")
	first.GameID = "first"
	if _, err := s.RecordCompletedGame(ctx, first); err != nil {
		t.Fatal(err)
	}
	var before, after [3]int64
	if err := s.pool.QueryRow(ctx, `SELECT score_units,completed_games,wins FROM player_group_stats WHERE user_id=3`).Scan(&before[0], &before[1], &before[2]); err != nil {
		t.Fatal(err)
	}
	second := eligibleResult(t, groups.Updated, 2, 1, "completed")
	if _, err := s.RecordCompletedGame(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := s.pool.QueryRow(ctx, `SELECT score_units,completed_games,wins FROM player_group_stats WHERE user_id=3`).Scan(&after[0], &after[1], &after[2]); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("abandonment changed accumulated stats", before, after)
	}
}

func TestEligibilityRollbackThenRetry(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	r := eligibleResult(t, groups.Updated, 7, 1, "completed")
	if _, err := s.pool.Exec(ctx, `ALTER TABLE completed_game_players ADD CONSTRAINT forced_failure CHECK(user_id<>8)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCompletedGame(ctx, r); err == nil {
		t.Fatal("expected failure after seven player/stats inserts")
	}
	for _, table := range []string{"completed_games", "completed_game_players", "player_group_stats"} {
		var n int
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil || n != 0 {
			t.Fatal("partial persistence", table, n, err)
		}
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE completed_game_players DROP CONSTRAINT forced_failure`); err != nil {
		t.Fatal(err)
	}
	first, err := s.RecordCompletedGame(ctx, r)
	if err != nil || !first.Scored || first.AlreadyPersisted {
		t.Fatalf("retry failed %+v %v", first, err)
	}
	repeat, err := s.RecordCompletedGame(ctx, r)
	if err != nil || !repeat.AlreadyPersisted {
		t.Fatal("committed retry not idempotent", err)
	}
	r.Players[7].DisplayName = "different audit payload"
	if _, err = s.RecordCompletedGame(ctx, r); !errors.Is(err, ranking.ErrConflict) {
		t.Fatal("divergent payload accepted", err)
	}
}

func TestEligibilityMigrationPreservesHistoricalPending(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	list, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.name == "migrations/0005_insufficient_eligible_players.up.sql" {
			break
		}
		if _, err = s.pool.Exec(ctx, m.sql); err != nil {
			t.Fatal(err)
		}
		if _, err = s.pool.Exec(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, m.name, m.checksum); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.GetOrCreateGroupConfig(ctx, 42); err != nil {
		t.Fatal(err)
	}
	raw := eligibleResult(t, groups.Legacy, 2, 1, "completed")
	raw.PolicyVersion = ""
	for i := range raw.Players {
		raw.Players[i].Score = 0
	}
	// Seed a historical pre-0005 record with its original schema. The current
	// repository correctly requires the startup migrations (including origin).
	historicalHash, hashErr := raw.Hash()
	if hashErr != nil {
		t.Fatal(hashErr)
	}
	if _, err = s.pool.Exec(ctx, `INSERT INTO completed_games(game_id,chat_id,game_mode,ranking_system,config_revision,started_at,finished_at,final_revision,finish_reason,payload_hash,participant_count,scoring_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'needs_product_decision')`, raw.GameID, raw.ChatID, raw.GameMode, raw.RankingSystem, raw.ConfigRevision, raw.StartedAt, raw.FinishedAt, raw.FinalRevision, raw.FinishReason, historicalHash, len(raw.Players)); err != nil {
		t.Fatal(err)
	}
	for _, p := range raw.Players {
		var historicalPosition any
		if p.Position > 0 {
			historicalPosition = p.Position
		}
		if _, err = s.pool.Exec(ctx, `INSERT INTO completed_game_players(game_id,user_id,observed_name,final_status,position,went_out,joined_after_start,leave_count,reentry_count) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, raw.GameID, p.UserID, p.DisplayName, p.FinalStatus, historicalPosition, p.WentOut, p.JoinedAfterStart, p.LeaveCount, p.ReentryCount); err != nil {
			t.Fatal(err)
		}
	}
	before, err := raw.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	var hash, status string
	if err = s.pool.QueryRow(ctx, `SELECT payload_hash,scoring_status FROM completed_games WHERE game_id=$1`, raw.GameID).Scan(&hash, &status); err != nil {
		t.Fatal(err)
	}
	if hash != before || status != ranking.StatusNeedsDecision {
		t.Fatal("migration rewrote historical result")
	}
	retry, err := s.RecordCompletedGame(ctx, raw)
	if err != nil || !retry.AlreadyPersisted || retry.Scored {
		t.Fatal("old payload no longer idempotent", err)
	}
	prepared, err := ranking.Prepare(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordCompletedGame(ctx, prepared); !errors.Is(err, ranking.ErrConflict) {
		t.Fatal("historical result implicitly rescored", err)
	}
	if reflect.DeepEqual(raw, prepared) {
		t.Fatal("policy test did not change payload")
	}
}
