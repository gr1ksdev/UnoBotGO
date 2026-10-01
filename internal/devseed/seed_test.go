package devseed

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/storage/postgres"
)

func TestValidateEnvironment_Security(t *testing.T) {
	validLocalDB := "postgres://unobot:secret@localhost:5432/unobot_test?sslmode=disable"

	// 1. Neither APP_ENV nor ALLOW_DEV_SEED
	lookupNone := func(k string) (string, bool) { return "", false }
	if err := ValidateEnvironment(lookupNone, validLocalDB); err != ErrProhibitedEnvironment {
		t.Fatalf("expected ErrProhibitedEnvironment, got %v", err)
	}

	// 2. APP_ENV = production
	lookupProd := func(k string) (string, bool) {
		if k == "APP_ENV" {
			return "production", true
		}
		return "", false
	}
	if err := ValidateEnvironment(lookupProd, validLocalDB); err != ErrProhibitedEnvironment {
		t.Fatalf("expected ErrProhibitedEnvironment, got %v", err)
	}

	// 3. APP_ENV = staging
	lookupStaging := func(k string) (string, bool) {
		if k == "APP_ENV" {
			return "staging", true
		}
		return "", false
	}
	if err := ValidateEnvironment(lookupStaging, validLocalDB); err != ErrProhibitedEnvironment {
		t.Fatalf("expected ErrProhibitedEnvironment, got %v", err)
	}

	// 4. Missing DATABASE_URL
	lookupDev := func(k string) (string, bool) {
		if k == "APP_ENV" {
			return "development", true
		}
		return "", false
	}
	if err := ValidateEnvironment(lookupDev, ""); err != ErrMissingDatabaseURL {
		t.Fatalf("expected ErrMissingDatabaseURL, got %v", err)
	}

	// 5. Production host in DATABASE_URL
	prodDB := "postgres://unobot:secret@production-db.internal.prod:5432/unobot?sslmode=require"
	if err := ValidateEnvironment(lookupDev, prodDB); err != ErrProductionHost {
		t.Fatalf("expected ErrProductionHost, got %v", err)
	}

	// 6. Valid with APP_ENV=development
	if err := ValidateEnvironment(lookupDev, validLocalDB); err != nil {
		t.Fatalf("expected nil error for valid dev, got %v", err)
	}

	// 7. Valid with ALLOW_DEV_SEED=1
	lookupAllow := func(k string) (string, bool) {
		if k == "ALLOW_DEV_SEED" {
			return "1", true
		}
		return "", false
	}
	if err := ValidateEnvironment(lookupAllow, validLocalDB); err != nil {
		t.Fatalf("expected nil error for ALLOW_DEV_SEED=1, got %v", err)
	}
}

func TestMaskDatabaseURL(t *testing.T) {
	raw := "postgres://unobot:supersecret@db.local:5432/unobot_dev?sslmode=disable"
	masked := MaskDatabaseURL(raw)
	if masked != "postgres://unobot:***@db.local:5432/unobot_dev" {
		t.Fatalf("unexpected masked url: %s", masked)
	}
}

func TestNamesAndVolumes(t *testing.T) {
	upGroups := updatedGroupNames()
	if len(upGroups) != 80 {
		t.Fatalf("expected 80 updated groups, got %d", len(upGroups))
	}

	upPlayers := updatedPlayerNames()
	if len(upPlayers) != 180 {
		t.Fatalf("expected 180 updated players, got %d", len(upPlayers))
	}

	legGroups := legacyGroupNames()
	if len(legGroups) != 65 {
		t.Fatalf("expected 65 legacy groups, got %d", len(legGroups))
	}

	legPlayers := legacyPlayerNames()
	if len(legPlayers) != 140 {
		t.Fatalf("expected 140 legacy players, got %d", len(legPlayers))
	}
}

func getTestPool(t *testing.T) (*pgxpool.Pool, *postgres.Store) {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to TEST_DATABASE_URL: %v", err)
	}

	store, err := postgres.Open(ctx, dbURL)
	if err != nil {
		pool.Close()
		t.Fatalf("failed to open store: %v", err)
	}

	if err := store.Migrate(ctx); err != nil {
		pool.Close()
		store.Close()
		t.Fatalf("failed to migrate test db: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		store.Close()
	})

	return pool, store
}

func TestDevSeed_IntegrationFlow(t *testing.T) {
	pool, store := getTestPool(t)
	ctx := context.Background()

	now := time.Now().In(ranking.RankingLocation)

	// Clean before test
	cleanRep, err := Clean(ctx, pool)
	if err != nil {
		t.Fatalf("initial clean failed: %v", err)
	}
	t.Logf("Initial clean: %+v", cleanRep)

	// Insert a simulated "real group" to ensure clean never touches non-seed records
	realChatID := int64(-12345678)
	_, err = pool.Exec(ctx, `
		INSERT INTO group_configs (chat_id, default_game_mode, ranking_system, title, config_revision)
		VALUES ($1, 'classic', 'updated', 'Real Untouched Group', 1)
		ON CONFLICT (chat_id) DO UPDATE SET title = EXCLUDED.title
	`, realChatID)
	if err != nil {
		t.Fatalf("failed to insert real group: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM group_configs WHERE chat_id = $1`, realChatID)
	}()

	// 1. First Seed Run
	report1, err := Seed(ctx, pool, now)
	if err != nil {
		t.Fatalf("first seed failed: %v", err)
	}
	if report1.UpdatedGroups != 80 || report1.LegacyGroups != 65 {
		t.Fatalf("unexpected seed report: %+v", report1)
	}

	// 2. Query initial score sums
	var initialUpdatedScoreSum int64
	err = pool.QueryRow(ctx, `
		SELECT coalesce(sum(score_units), 0)
		FROM player_group_monthly_stats
		WHERE ranking_system = 'updated' AND chat_id BETWEEN $1 AND $2
	`, SeedChatIDEnd, SeedChatIDStart).Scan(&initialUpdatedScoreSum)
	if err != nil {
		t.Fatalf("query initial score sum: %v", err)
	}

	// 3. Second Seed Run (Idempotency test)
	report2, err := Seed(ctx, pool, now)
	if err != nil {
		t.Fatalf("second seed failed: %v", err)
	}
	if report2.UpdatedGroups != report1.UpdatedGroups || report2.LegacyGroups != report1.LegacyGroups {
		t.Fatalf("idempotency report divergence: %+v vs %+v", report1, report2)
	}

	var secondUpdatedScoreSum int64
	err = pool.QueryRow(ctx, `
		SELECT coalesce(sum(score_units), 0)
		FROM player_group_monthly_stats
		WHERE ranking_system = 'updated' AND chat_id BETWEEN $1 AND $2
	`, SeedChatIDEnd, SeedChatIDStart).Scan(&secondUpdatedScoreSum)
	if err != nil {
		t.Fatalf("query second score sum: %v", err)
	}
	if initialUpdatedScoreSum != secondUpdatedScoreSum {
		t.Fatalf("idempotency violation: scores doubled! initial: %d, second: %d", initialUpdatedScoreSum, secondUpdatedScoreSum)
	}

	// 4. Validate Global Rankings queries through store.ReadGlobalRanking
	monthStart := ranking.MonthStart(now)

	// 4.1 Updated Groups pagination (>50 rows)
	groupsReq := ranking.GlobalRequest{
		Kind:   "groups",
		System: groups.Updated,
		Month:  monthStart,
		Limit:  50,
	}
	page1, err := store.ReadGlobalRanking(ctx, groupsReq)
	if err != nil {
		t.Fatalf("read updated groups page 1: %v", err)
	}
	if len(page1.Rows) != 50 || !page1.More {
		t.Fatalf("expected 50 rows and More=true, got %d, more=%v", len(page1.Rows), page1.More)
	}
	if page1.Rows[0].Position != 1 {
		t.Fatalf("expected position 1, got %d", page1.Rows[0].Position)
	}

	// Page 2
	cursorKey := page1.Rows[49].Key()
	groupsReq.After = &cursorKey
	page2, err := store.ReadGlobalRanking(ctx, groupsReq)
	if err != nil {
		t.Fatalf("read updated groups page 2: %v", err)
	}
	if len(page2.Rows) != 30 || page2.More {
		t.Fatalf("expected 30 remaining rows and More=false, got %d, more=%v", len(page2.Rows), page2.More)
	}
	if page2.Rows[0].Position != 51 {
		t.Fatalf("expected position 51 on second page, got %d", page2.Rows[0].Position)
	}

	// 4.2 Updated Players ranking and multi-group aggregation
	playersReq := ranking.GlobalRequest{
		Kind:   "players",
		System: groups.Updated,
		Month:  monthStart,
		Limit:  50,
	}
	playersPage1, err := store.ReadGlobalRanking(ctx, playersReq)
	if err != nil {
		t.Fatalf("read updated players page 1: %v", err)
	}
	if len(playersPage1.Rows) != 50 || !playersPage1.More {
		t.Fatalf("expected 50 players and More=true, got %d, more=%v", len(playersPage1.Rows), playersPage1.More)
	}
	// Player 1 (Freddy) must have latest display_name "Freddy UNO" and top score
	topPlayer := playersPage1.Rows[0]
	if topPlayer.ID != SeedUserIDStart+0 {
		t.Fatalf("expected top player to be Freddy (%d), got %d (%s)", SeedUserIDStart+0, topPlayer.ID, topPlayer.Name)
	}
	if topPlayer.Name != "Freddy UNO" {
		t.Fatalf("expected latest display_name 'Freddy UNO', got %q", topPlayer.Name)
	}
	if topPlayer.Score < 2000000 {
		t.Fatalf("expected high aggregate score >= 20.000,00 pts, got %d", topPlayer.Score)
	}

	// 4.3 Detail View for Group 1 ("UNO da Galera")
	detailReq := ranking.GlobalRequest{
		Kind:    "detail",
		System:  groups.Updated,
		GroupID: SeedChatIDStart,
		Month:   monthStart,
		Limit:   50,
	}
	detailPage, err := store.ReadGlobalRanking(ctx, detailReq)
	if err != nil {
		t.Fatalf("read group 1 detail: %v", err)
	}
	if detailPage.Group == nil || detailPage.Group.Name != "UNO da Galera" {
		t.Fatalf("expected group header 'UNO da Galera', got %+v", detailPage.Group)
	}
	if len(detailPage.Rows) != 7 {
		t.Fatalf("expected 7 members in group 1, got %d", len(detailPage.Rows))
	}
	// Verify tie-break in group detail:
	// LucasZ (UserIDStart+10, placement 1) must be ahead of Mariana (UserIDStart+11, placement 2) with same 10,00 pts
	var lucasPos, marianaPos int64
	for _, r := range detailPage.Rows {
		if r.ID == SeedUserIDStart+10 {
			lucasPos = r.Position
		}
		if r.ID == SeedUserIDStart+11 {
			marianaPos = r.Position
		}
	}
	if lucasPos == 0 || marianaPos == 0 || lucasPos >= marianaPos {
		t.Fatalf("detail placement tie-break failed: LucasZ (%d) vs Mariana (%d)", lucasPos, marianaPos)
	}

	// 4.4 Large group detail (60 members)
	largeDetailReq := ranking.GlobalRequest{
		Kind:    "detail",
		System:  groups.Updated,
		GroupID: SeedChatIDStart - 1, // Mega Torneio
		Month:   monthStart,
		Limit:   50,
	}
	largeDetailPage, err := store.ReadGlobalRanking(ctx, largeDetailReq)
	if err != nil {
		t.Fatalf("read large group detail: %v", err)
	}
	if len(largeDetailPage.Rows) != 50 || !largeDetailPage.More {
		t.Fatalf("expected 50 members and More=true on large group, got %d", len(largeDetailPage.Rows))
	}

	// 4.5 Small group detail (2 members)
	smallDetailReq := ranking.GlobalRequest{
		Kind:    "detail",
		System:  groups.Updated,
		GroupID: SeedChatIDStart - 2, // Duelo 1v1
		Month:   monthStart,
		Limit:   50,
	}
	smallDetailPage, err := store.ReadGlobalRanking(ctx, smallDetailReq)
	if err != nil {
		t.Fatalf("read small group detail: %v", err)
	}
	if len(smallDetailPage.Rows) != 2 || smallDetailPage.More {
		t.Fatalf("expected 2 members and More=false on small group, got %d", len(smallDetailPage.Rows))
	}

	// 4.6 Fallback group without title
	fallbackDetailReq := ranking.GlobalRequest{
		Kind:    "detail",
		System:  groups.Updated,
		GroupID: SeedChatIDStart - 4, // Group 5 (empty title)
		Month:   monthStart,
		Limit:   50,
	}
	fallbackDetailPage, err := store.ReadGlobalRanking(ctx, fallbackDetailReq)
	if err != nil {
		t.Fatalf("read fallback group detail: %v", err)
	}
	if fallbackDetailPage.Group == nil || fallbackDetailPage.Group.Name != "Grupo ••••0005" {
		t.Fatalf("expected fallback name 'Grupo ••••0005', got %+v", fallbackDetailPage.Group)
	}

	// 4.7 Legacy System Isolation
	legacyReq := ranking.GlobalRequest{
		Kind:   "groups",
		System: groups.Legacy,
		Month:  monthStart,
		Limit:  50,
	}
	legacyPage, err := store.ReadGlobalRanking(ctx, legacyReq)
	if err != nil {
		t.Fatalf("read legacy groups: %v", err)
	}
	if len(legacyPage.Rows) != 50 || !legacyPage.More {
		t.Fatalf("expected 50 legacy groups and More=true, got %d", len(legacyPage.Rows))
	}
	for _, r := range legacyPage.Rows {
		if r.ID > SeedChatIDStart-100 {
			t.Fatalf("updated group leaked into legacy: ID %d", r.ID)
		}
	}

	// 5. Cleanup Test
	cleanRep2, err := Clean(ctx, pool)
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if cleanRep2.GroupConfigs != 80+65 {
		t.Fatalf("expected 145 group configs deleted, got %d", cleanRep2.GroupConfigs)
	}

	// Verify all devseed records are gone
	var remainingDevseedCount int64
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM group_configs WHERE chat_id BETWEEN $1 AND $2
	`, SeedChatIDEnd, SeedChatIDStart).Scan(&remainingDevseedCount)
	if err != nil {
		t.Fatalf("verify devseed cleanup: %v", err)
	}
	if remainingDevseedCount != 0 {
		t.Fatalf("expected 0 remaining devseed groups, got %d", remainingDevseedCount)
	}

	// Verify real untouched group is still there!
	var realGroupCount int64
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM group_configs WHERE chat_id = $1
	`, realChatID).Scan(&realGroupCount)
	if err != nil {
		t.Fatalf("verify real group preserved: %v", err)
	}
	if realGroupCount != 1 {
		t.Fatalf("real group was deleted by cleanup!")
	}
}
