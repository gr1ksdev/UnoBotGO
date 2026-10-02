//go:build integration

package postgres

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

func TestMonthlyAccumulationAndSeparation(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()

	// September 2026
	sep15 := time.Date(2026, 9, 15, 15, 0, 0, 0, ranking.RankingLocation)
	g1 := eligibleResult(t, groups.Updated, 3, 0, "completed")
	g1.GameID = "game-sep-1"
	g1.StartedAt = sep15.Add(-time.Hour)
	g1.FinishedAt = sep15
	g1.Players[0].UserID = 1
	g1.Players[0].DisplayName = "Player 1"
	g1.Players[1].UserID = 2
	g1.Players[1].DisplayName = "Player 2"
	g1.Players[2].UserID = 3
	g1.Players[2].DisplayName = "Player 3"
	if _, err := s.RecordCompletedGame(ctx, g1); err != nil {
		t.Fatal(err)
	}

	sep20 := time.Date(2026, 9, 20, 18, 0, 0, 0, ranking.RankingLocation)
	g2 := g1.Clone()
	g2.GameID = "game-sep-2"
	g2.StartedAt = sep20.Add(-time.Hour)
	g2.FinishedAt = sep20
	// Player 2 wins this one
	g2.Players[0].UserID, g2.Players[1].UserID = 2, 1
	g2.Players[0].DisplayName, g2.Players[1].DisplayName = "Player 2", "Player 1"
	if _, err := s.RecordCompletedGame(ctx, g2); err != nil {
		t.Fatal(err)
	}

	// October 2026
	oct2 := time.Date(2026, 10, 2, 10, 0, 0, 0, ranking.RankingLocation)
	g3 := g1.Clone()
	g3.GameID = "game-oct-1"
	g3.StartedAt = oct2.Add(-time.Hour)
	g3.FinishedAt = oct2
	// Player 3 wins, Player 1 2nd, Player 2 3rd
	g3.Players[0].UserID = 3
	g3.Players[0].DisplayName = "Player 3"
	g3.Players[1].UserID = 1
	g3.Players[1].DisplayName = "Player 1"
	g3.Players[2].UserID = 2
	g3.Players[2].DisplayName = "Player 2"
	if _, err := s.RecordCompletedGame(ctx, g3); err != nil {
		t.Fatal(err)
	}

	// Check September ranking
	sepRank, err := s.ListGroupRanking(ctx, 42, sep15)
	if err != nil {
		t.Fatal(err)
	}
	if sepRank.MonthName != "Setembro" || sepRank.Total != 3 {
		t.Fatalf("unexpected september rank: %+v", sepRank)
	}
	// Player 2 and Player 1 both have 1500 pts; Player 2 completed Game 2 more recently, so Player 2 is 1st
	if sepRank.Entries[0].UserID != 2 || sepRank.Entries[0].Score != 1500 {
		t.Fatalf("expected Player 2 1st with 1500, got %+v", sepRank.Entries[0])
	}
	if sepRank.Entries[1].UserID != 1 || sepRank.Entries[1].Score != 1500 {
		t.Fatalf("expected Player 1 2nd with 1500, got %+v", sepRank.Entries[1])
	}
	if sepRank.Entries[2].UserID != 3 || sepRank.Entries[2].Score != 0 {
		t.Fatalf("expected Player 3 3rd with 0, got %+v", sepRank.Entries[2])
	}

	// Check October ranking: only g3 counts!
	octRank, err := s.ListGroupRanking(ctx, 42, oct2)
	if err != nil {
		t.Fatal(err)
	}
	if octRank.MonthName != "Outubro" || octRank.Total != 3 {
		t.Fatalf("unexpected october rank: %+v", octRank)
	}
	if octRank.Entries[0].UserID != 3 || octRank.Entries[0].Score != 1000 {
		t.Fatalf("expected Player 3 1st with 1000, got %+v", octRank.Entries[0])
	}
	if octRank.Entries[1].UserID != 1 || octRank.Entries[1].Score != 500 {
		t.Fatalf("expected Player 1 2nd with 500, got %+v", octRank.Entries[1])
	}
	if octRank.Entries[2].UserID != 2 || octRank.Entries[2].Score != 0 {
		t.Fatalf("expected Player 2 3rd with 0, got %+v", octRank.Entries[2])
	}

	// Check November ranking: empty month!
	novDate := time.Date(2026, 11, 1, 0, 0, 0, 0, ranking.RankingLocation)
	novRank, err := s.ListGroupRanking(ctx, 42, novDate)
	if err != nil {
		t.Fatal(err)
	}
	if novRank.MonthName != "Novembro" || novRank.Total != 0 || len(novRank.Entries) != 0 {
		t.Fatalf("expected empty november rank: %+v", novRank)
	}
}

func TestMonthlyTimezoneMidnightTransition(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()

	// Match 1: Started in September, finished at 23:59:59 SP on Sept 30 (02:59:59 UTC on Oct 1)
	// Must belong to September
	t1Finish := time.Date(2026, 9, 30, 23, 59, 59, 0, ranking.RankingLocation)
	g1 := eligibleResult(t, groups.Updated, 2, 0, "completed")
	g1.GameID = "edge-sep"
	g1.StartedAt = t1Finish.Add(-30 * time.Minute)
	g1.FinishedAt = t1Finish
	g1.Players[0].UserID = 10
	g1.Players[1].UserID = 20
	if _, err := s.RecordCompletedGame(ctx, g1); err != nil {
		t.Fatal(err)
	}

	// Match 2: Started in September (23:45 SP), finished at 00:00:01 SP on Oct 1 (03:00:01 UTC on Oct 1)
	// Must belong to October
	t2Finish := time.Date(2026, 10, 1, 0, 0, 1, 0, ranking.RankingLocation)
	g2 := eligibleResult(t, groups.Updated, 2, 0, "completed")
	g2.GameID = "edge-oct"
	g2.StartedAt = t2Finish.Add(-30 * time.Minute) // started in September!
	g2.FinishedAt = t2Finish                       // finished in October!
	g2.Players[0].UserID = 30
	g2.Players[1].UserID = 40
	if _, err := s.RecordCompletedGame(ctx, g2); err != nil {
		t.Fatal(err)
	}

	// Query September: must contain User 10 and 20 only
	sepRank, err := s.ListGroupRanking(ctx, 42, t1Finish)
	if err != nil {
		t.Fatal(err)
	}
	if sepRank.Total != 2 || sepRank.MonthName != "Setembro" {
		t.Fatalf("unexpected sep rank: %+v", sepRank)
	}
	if sepRank.Entries[0].UserID != 10 || sepRank.Entries[1].UserID != 20 {
		t.Fatalf("wrong sep users: %+v", sepRank.Entries)
	}

	// Query October: must contain User 30 and 40 only
	octRank, err := s.ListGroupRanking(ctx, 42, t2Finish)
	if err != nil {
		t.Fatal(err)
	}
	if octRank.Total != 2 || octRank.MonthName != "Outubro" {
		t.Fatalf("unexpected oct rank: %+v", octRank)
	}
	if octRank.Entries[0].UserID != 30 || octRank.Entries[1].UserID != 40 {
		t.Fatalf("wrong oct users: %+v", octRank.Entries)
	}
}

func TestMonthlyTieBreakPreviousMonthDataDoesNotInfluence(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	ctx := t.Context()

	sep := time.Date(2026, 9, 20, 12, 0, 0, 0, ranking.RankingLocation)
	// Player 1 wins 5 games in September
	for i := 0; i < 5; i++ {
		r := eligibleResult(t, groups.Legacy, 2, 0, "completed")
		r.GameID = fmt.Sprintf("sep-win-%d", i)
		r.StartedAt = sep.Add(-time.Hour)
		r.FinishedAt = sep.Add(time.Duration(i) * time.Minute)
		r.Players[0].UserID = 1
		r.Players[1].UserID = 99
		if _, err := s.RecordCompletedGame(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	oct := time.Date(2026, 10, 5, 12, 0, 0, 0, ranking.RankingLocation)
	// In October, Player 1 and Player 2 both play in an 8-player game.
	// Player 2 gets 2nd place (1 pt in Legacy).
	// Player 1 gets 3rd place (1 pt in Legacy).
	// Both have 1 pt in October.
	// Player 2 has last placement 2 in October, Player 1 has last placement 3 in October.
	// Player 2 must rank higher in October despite Player 1's 5 wins in September!
	octGame := eligibleResult(t, groups.Legacy, 8, 0, "completed")
	octGame.GameID = "oct-tie"
	octGame.StartedAt = oct.Add(-time.Hour)
	octGame.FinishedAt = oct
	for j := range octGame.Players {
		octGame.Players[j].UserID = int64(100 + j)
	}
	octGame.Players[1].UserID = 2 // position 2
	octGame.Players[2].UserID = 1 // position 3
	if _, err := s.RecordCompletedGame(ctx, octGame); err != nil {
		t.Fatal(err)
	}

	octRank, err := s.ListGroupRanking(ctx, 42, oct)
	if err != nil {
		t.Fatal(err)
	}
	var p1Rank, p2Rank int
	for i, e := range octRank.Entries {
		if e.UserID == 1 {
			p1Rank = i + 1
		}
		if e.UserID == 2 {
			p2Rank = i + 1
		}
	}
	if p2Rank >= p1Rank || p2Rank == 0 || p1Rank == 0 {
		t.Fatalf("expected Player 2 (placement 2) to beat Player 1 (placement 3) in October: p2Rank=%d p1Rank=%d entries=%+v", p2Rank, p1Rank, octRank.Entries)
	}
}

func TestMonthlyIdempotencyAndZeroScore(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()

	ts := time.Date(2026, 9, 10, 10, 0, 0, 0, ranking.RankingLocation)
	r := eligibleResult(t, groups.Updated, 2, 1, "departure") // 2 eligible (pos 1, 2), 1 unranked (pos 0, left)
	r.GameID = "idempotent-game"
	r.StartedAt = ts.Add(-time.Hour)
	r.FinishedAt = ts
	r.Players[0].UserID = 1 // 1st: 1000 pts
	r.Players[1].UserID = 2 // 2nd: 0 pts, eligible
	r.Players[2].UserID = 3 // left: ineligible

	commit, err := s.RecordCompletedGame(ctx, r)
	if err != nil || !commit.Scored || commit.AlreadyPersisted {
		t.Fatalf("first commit failed: %+v %v", commit, err)
	}

	// Retry same game
	retry, err := s.RecordCompletedGame(ctx, r)
	if err != nil || !retry.Scored || !retry.AlreadyPersisted {
		t.Fatalf("retry commit should be already persisted: %+v %v", retry, err)
	}

	rank, err := s.ListGroupRanking(ctx, 42, ts)
	if err != nil {
		t.Fatal(err)
	}
	// Player 1 has 1000 pts (not 2000), Player 2 has 0 pts, Player 3 (abandoner) is absent
	want := []ranking.Entry{
		{UserID: 1, DisplayName: "1", Score: 1000, CompletedGames: 1, Wins: 1},
		{UserID: 2, DisplayName: "2", Score: 0, CompletedGames: 1, Wins: 0},
	}
	if rank.Total != 2 || !reflect.DeepEqual(rank.Entries, want) {
		t.Fatalf("got entries %+v; want %+v", rank.Entries, want)
	}
}

func TestMonthlyMigrationBackfillIdempotency(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()

	// 1. Apply migrations 0001 to 0005 only
	list, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for _, m := range list {
		if m.name == "migrations/0006_monthly_ranking.up.sql" {
			break
		}
		if _, err = s.pool.Exec(ctx, m.sql); err != nil {
			t.Fatal(err)
		}
		if _, err = s.pool.Exec(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, m.name, m.checksum); err != nil {
			t.Fatal(err)
		}
	}

	// 2. Set up group config
	if _, err = s.pool.Exec(ctx, `INSERT INTO group_configs(chat_id, default_game_mode, ranking_system, config_revision) VALUES(42, 'classic', 'updated', 1)`); err != nil {
		t.Fatal(err)
	}

	// 3. Seed historical completed games in September 2026
	sepDate := time.Date(2026, 9, 15, 12, 0, 0, 0, ranking.RankingLocation)
	if _, err = s.pool.Exec(ctx, `
INSERT INTO completed_games(game_id, chat_id, game_mode, ranking_system, config_revision, started_at, finished_at, final_revision, finish_reason, payload_hash, participant_count, scoring_status, policy_version, scored_at)
VALUES('hist-1', 42, 'classic', 'updated', 1, $1, $1, 10, 'completed', 'hash1', 3, 'scored', 'm7_v1', $1)`, sepDate); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `
INSERT INTO completed_game_players(game_id, user_id, observed_name, final_status, position, went_out, score_units, joined_after_start, leave_count, reentry_count)
VALUES('hist-1', 1, 'Player 1', 'went_out', 1, true, 1000, false, 0, 0),
      ('hist-1', 2, 'Player 2', 'playing', 2, false, 500, false, 0, 0),
      ('hist-1', 3, 'Player 3', 'playing', 3, false, 0, false, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `
INSERT INTO player_group_stats(chat_id, user_id, ranking_system, score_units, completed_games, wins, display_name, last_finished_at)
VALUES(42, 1, 'updated', 1000, 1, 1, 'Player 1', $1),
      (42, 2, 'updated', 500, 1, 0, 'Player 2', $1),
      (42, 3, 'updated', 0, 1, 0, 'Player 3', $1)`, sepDate); err != nil {
		t.Fatal(err)
	}

	// 4. Now apply migration 0006 via s.Migrate()
	if err = s.Migrate(ctx); err != nil {
		t.Fatalf("migration 0006 failed: %v", err)
	}
	if err = s.VerifySchema(ctx); err != nil {
		t.Fatalf("schema verification failed: %v", err)
	}

	// 5. Verify player_group_monthly_stats was populated by migration 0006
	rank, err := s.ListGroupRanking(ctx, 42, sepDate)
	if err != nil {
		t.Fatal(err)
	}
	if rank.Total != 3 || rank.MonthName != "Setembro" {
		t.Fatalf("unexpected rank after migration 0006: %+v", rank)
	}
	if rank.Entries[0].UserID != 1 || rank.Entries[0].Score != 1000 || rank.Entries[0].Wins != 1 {
		t.Fatalf("unexpected entry 0: %+v", rank.Entries[0])
	}
	if rank.Entries[1].UserID != 2 || rank.Entries[1].Score != 500 {
		t.Fatalf("unexpected entry 1: %+v", rank.Entries[1])
	}
	if rank.Entries[2].UserID != 3 || rank.Entries[2].Score != 0 {
		t.Fatalf("unexpected entry 2: %+v", rank.Entries[2])
	}

	// 6. Test idempotency of backfill query
	for _, m := range list {
		if m.name == "migrations/0006_monthly_ranking.up.sql" {
			if _, err = s.pool.Exec(ctx, m.sql); err != nil {
				t.Fatalf("re-running migration 0006 failed idempotency: %v", err)
			}
			break
		}
	}

	// 7. Verify rankings remained exactly identical after re-running backfill
	rankAfter, err := s.ListGroupRanking(ctx, 42, sepDate)
	if err != nil || !reflect.DeepEqual(rank, rankAfter) {
		t.Fatalf("re-running backfill altered ranking: before=%+v after=%+v err=%v", rank, rankAfter, err)
	}
}

func TestUserMonthlyRankingsIntegration(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()

	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// 1. Setup groups: 101 (updated, titled), 102 (updated, untitled fallback), 103 (legacy, titled)
	if _, err := s.pool.Exec(ctx, `INSERT INTO group_configs(chat_id, default_game_mode, ranking_system, title)
VALUES(101, 'classic', 'updated', 'UNO da Galera'),
      (102, 'classic', 'updated', ''),
      (103, 'classic', 'legacy', 'Jogatina BR'),
      (104, 'classic', 'updated', 'Antigo Nome')`); err != nil {
		t.Fatal(err)
	}

	sep15 := time.Date(2026, 9, 15, 14, 0, 0, 0, ranking.RankingLocation)

	// Game 1 in Group 101 (Updated)
	g1 := eligibleResult(t, groups.Updated, 3, 0, "completed")
	g1.GameID = "g-user-1"
	g1.ChatID = 101
	g1.StartedAt = sep15.Add(-time.Hour)
	g1.FinishedAt = sep15
	g1.Players[0].UserID = 1
	g1.Players[0].DisplayName = "User 1"
	g1.Players[1].UserID = 2
	g1.Players[1].DisplayName = "User 2"
	g1.Players[2].UserID = 3
	g1.Players[2].DisplayName = "User 3"
	if _, err := s.RecordCompletedGame(ctx, g1); err != nil {
		t.Fatal(err)
	}

	// Game 2 in Group 102 (Updated, untitled)
	sep16 := time.Date(2026, 9, 16, 16, 0, 0, 0, ranking.RankingLocation)
	g2 := eligibleResult(t, groups.Updated, 3, 0, "completed")
	g2.GameID = "g-user-2"
	g2.ChatID = 102
	g2.StartedAt = sep16.Add(-time.Hour)
	g2.FinishedAt = sep16
	// User 2 wins, User 1 2nd
	g2.Players[0].UserID, g2.Players[1].UserID = 2, 1
	g2.Players[0].DisplayName, g2.Players[1].DisplayName = "User 2", "User 1"
	g2.Players[2].UserID = 99
	g2.Players[2].DisplayName = "Other"
	if _, err := s.RecordCompletedGame(ctx, g2); err != nil {
		t.Fatal(err)
	}

	// Game 3 in Group 103 (Legacy)
	sep17 := time.Date(2026, 9, 17, 18, 0, 0, 0, ranking.RankingLocation)
	g3 := eligibleResult(t, groups.Legacy, 3, 0, "completed")
	g3.GameID = "g-user-3"
	g3.ChatID = 103
	g3.RankingSystem = groups.Legacy
	g3.StartedAt = sep17.Add(-time.Hour)
	g3.FinishedAt = sep17
	g3.Players[0].UserID = 1
	g3.Players[0].DisplayName = "User 1"
	g3.Players[1].UserID = 2
	g3.Players[1].DisplayName = "User 2"
	g3.Players[2].UserID = 3
	g3.Players[2].DisplayName = "User 3"
	if _, err := s.RecordCompletedGame(ctx, g3); err != nil {
		t.Fatal(err)
	}

	// Game 4 in Group 101 with Departure/Abandonment: User 4 leaves
	g4 := eligibleResult(t, groups.Updated, 2, 1, "departure")
	g4.GameID = "g-user-4"
	g4.ChatID = 101
	g4.StartedAt = sep17.Add(-30 * time.Minute)
	g4.FinishedAt = sep17.Add(-10 * time.Minute)
	g4.Players[0].UserID = 1
	g4.Players[1].UserID = 2
	g4.Players[2].UserID = 4 // leaver
	if _, err := s.RecordCompletedGame(ctx, g4); err != nil {
		t.Fatal(err)
	}

	// 2. Query User 1 in September 2026
	r1, err := s.ListUserMonthlyRankings(ctx, 1, sep15)
	if err != nil {
		t.Fatalf("ListUserMonthlyRankings failed: %v", err)
	}
	if r1.MonthName != "Setembro" || r1.UserID != 1 {
		t.Fatalf("unexpected metadata: MonthName=%q UserID=%d", r1.MonthName, r1.UserID)
	}
	if r1.Updated == nil || len(r1.Updated.Entries) != 2 {
		t.Fatalf("expected 2 updated entries, got: %+v", r1.Updated)
	}
	// Verify Updated ordering and content (Group 101 has 1000 from g1 + 1000 from g4 = 2000)
	if r1.Updated.Entries[0].ChatID != 101 || r1.Updated.Entries[0].GroupName != "UNO da Galera" || r1.Updated.Entries[0].ScoreUnits != 2000 {
		t.Fatalf("unexpected updated entry 0: %+v", r1.Updated.Entries[0])
	}
	if r1.Updated.Entries[1].ChatID != 102 || r1.Updated.Entries[1].GroupName != "Grupo 102" || r1.Updated.Entries[1].ScoreUnits != 500 {
		t.Fatalf("unexpected updated entry 1 (fallback title): %+v", r1.Updated.Entries[1])
	}
	if r1.Updated.TotalScore != 2500 {
		t.Fatalf("expected updated total 2500, got %d", r1.Updated.TotalScore)
	}

	// Verify Legacy content
	if r1.Legacy == nil || len(r1.Legacy.Entries) != 1 {
		t.Fatalf("expected 1 legacy entry, got: %+v", r1.Legacy)
	}
	if r1.Legacy.Entries[0].ChatID != 103 || r1.Legacy.Entries[0].GroupName != "Jogatina BR" || r1.Legacy.Entries[0].ScoreUnits != 100 {
		t.Fatalf("unexpected legacy entry: %+v", r1.Legacy.Entries[0])
	}
	if r1.Legacy.TotalScore != 100 {
		t.Fatalf("expected legacy total 100, got %d", r1.Legacy.TotalScore)
	}

	// 3. User 3 has score 0 in both systems and MUST appear
	r3, err := s.ListUserMonthlyRankings(ctx, 3, sep15)
	if err != nil {
		t.Fatal(err)
	}
	if r3.Updated == nil || len(r3.Updated.Entries) != 1 || r3.Updated.Entries[0].ScoreUnits != 0 {
		t.Fatalf("user 3 score 0 missing from updated: %+v", r3.Updated)
	}
	if r3.Legacy == nil || len(r3.Legacy.Entries) != 1 || r3.Legacy.Entries[0].ScoreUnits != 0 {
		t.Fatalf("user 3 score 0 missing from legacy: %+v", r3.Legacy)
	}

	// 4. User 4 only had abandonment -> should have no monthly entries
	r4, err := s.ListUserMonthlyRankings(ctx, 4, sep15)
	if err != nil {
		t.Fatal(err)
	}
	if r4.Updated != nil && len(r4.Updated.Entries) > 0 {
		t.Fatalf("abandoned user 4 appeared in ranking: %+v", r4.Updated)
	}

	// 5. Month isolation: October 2026 should be empty for User 1 initially
	octDate := time.Date(2026, 10, 5, 12, 0, 0, 0, ranking.RankingLocation)
	rOct, err := s.ListUserMonthlyRankings(ctx, 1, octDate)
	if err != nil {
		t.Fatal(err)
	}
	if rOct.MonthName != "Outubro" {
		t.Fatalf("expected Outubro, got %q", rOct.MonthName)
	}
	if (rOct.Updated != nil && len(rOct.Updated.Entries) > 0) || (rOct.Legacy != nil && len(rOct.Legacy.Entries) > 0) {
		t.Fatalf("expected empty ranking for October, got: %+v", rOct)
	}

	// 6. Test ObserveGroupTitle with IS DISTINCT FROM and non-empty
	if err = s.ObserveGroupTitle(ctx, 104, "Novo Nome"); err != nil {
		t.Fatal(err)
	}
	var title string
	if err = s.pool.QueryRow(ctx, `SELECT title FROM group_configs WHERE chat_id=104`).Scan(&title); err != nil || title != "Novo Nome" {
		t.Fatalf("expected title 'Novo Nome', got %q (err=%v)", title, err)
	}

	// Empty and whitespace titles must NOT overwrite existing title
	if err = s.ObserveGroupTitle(ctx, 104, ""); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT title FROM group_configs WHERE chat_id=104`).Scan(&title); err != nil || title != "Novo Nome" {
		t.Fatalf("empty title overwrote existing title: %q", title)
	}
	if err = s.ObserveGroupTitle(ctx, 104, "   "); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(ctx, `SELECT title FROM group_configs WHERE chat_id=104`).Scan(&title); err != nil || title != "Novo Nome" {
		t.Fatalf("whitespace title overwrote existing title: %q", title)
	}
}
