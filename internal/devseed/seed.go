package devseed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

const (
	SeedChatIDStart  int64 = -990000000001
	SeedChatIDEnd    int64 = -990000000999
	SeedUserIDStart  int64 = 9900000001
	SeedUserIDEnd    int64 = 9900000999
	SeedGameIDPrefix       = "devseed_game_"
)

var (
	ErrProhibitedEnvironment = errors.New("devseed: execution forbidden: APP_ENV must be 'development' or ALLOW_DEV_SEED must be '1'")
	ErrMissingDatabaseURL   = errors.New("devseed: DATABASE_URL is required")
	ErrProductionHost        = errors.New("devseed: safety guard tripped: target database appears to be a production host")
)

// ValidateEnvironment checks that the environment is strictly development.
func ValidateEnvironment(lookup func(string) (string, bool), dbURL string) error {
	appEnv, _ := lookup("APP_ENV")
	allowSeed, _ := lookup("ALLOW_DEV_SEED")

	if strings.TrimSpace(appEnv) != "development" && strings.TrimSpace(allowSeed) != "1" {
		return ErrProhibitedEnvironment
	}

	if strings.TrimSpace(dbURL) == "" {
		return ErrMissingDatabaseURL
	}

	u, err := url.Parse(dbURL)
	if err != nil {
		return fmt.Errorf("devseed: invalid DATABASE_URL: %w", err)
	}

	hostLower := strings.ToLower(u.Hostname())
	if strings.Contains(hostLower, "production") || strings.Contains(hostLower, "prod.") || strings.HasSuffix(hostLower, ".internal.prod") {
		return ErrProductionHost
	}

	return nil
}

// MaskDatabaseURL returns a sanitized database connection description for logs.
func MaskDatabaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "invalid-url"
	}
	user := u.User.Username()
	host := u.Host
	db := strings.TrimPrefix(u.Path, "/")
	if user != "" {
		return fmt.Sprintf("postgres://%s:***@%s/%s", user, host, db)
	}
	return fmt.Sprintf("postgres://%s/%s", host, db)
}

type Report struct {
	CurrentMonth      string
	MonthStart        string
	Timezone          string
	UpdatedGroups     int
	UpdatedPlayers    int
	UpdatedStatsRows  int
	LegacyGroups      int
	LegacyPlayers     int
	LegacyStatsRows   int
	TotalGames        int
	LargeGroupMembers int
	SmallGroupMembers int
	FallbackGroups    int
	FallbackPlayers   int
	Namespace         string
}

func (r Report) String() string {
	return fmt.Sprintf(`Mini App development seed completed

Current month: %s
Timezone:      %s (Month start: %s)

Updated (Atualizado):
  Groups:            %d
  Players:           %d
  Ranking stats:     %d

Legacy (Legado):
  Groups:            %d
  Players:           %d
  Ranking stats:     %d

Total games recorded: %d
Large group members:  %d
Small group members:  %d
Fallback fixtures:    %d group(s) without title, %d player(s) without name
Avatars:              Local fallbacks only (0 Telegram API calls)
Seed namespace:       %s
`,
		r.CurrentMonth,
		r.Timezone,
		r.MonthStart,
		r.UpdatedGroups,
		r.UpdatedPlayers,
		r.UpdatedStatsRows,
		r.LegacyGroups,
		r.LegacyPlayers,
		r.LegacyStatsRows,
		r.TotalGames,
		r.LargeGroupMembers,
		r.SmallGroupMembers,
		r.FallbackGroups,
		r.FallbackPlayers,
		r.Namespace,
	)
}

type CleanReport struct {
	CompletedGamePlayers int64
	CompletedGames       int64
	MonthlyStats         int64
	CumulativeStats      int64
	KnownUsers           int64
	GroupConfigs         int64
}

func (c CleanReport) String() string {
	return fmt.Sprintf(`Mini App development seed cleanup completed

Rows deleted:
  completed_game_players:     %d
  completed_games:            %d
  player_group_monthly_stats: %d
  player_group_stats:         %d
  known_group_users:          %d
  group_configs:              %d
`,
		c.CompletedGamePlayers,
		c.CompletedGames,
		c.MonthlyStats,
		c.CumulativeStats,
		c.KnownUsers,
		c.GroupConfigs,
	)
}

// Clean removes exclusively devseed fixtures in dependency order within a transaction.
func Clean(ctx context.Context, pool *pgxpool.Pool) (CleanReport, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return CleanReport{}, fmt.Errorf("devseed clean: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rep, err := cleanTx(ctx, tx)
	if err != nil {
		return CleanReport{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CleanReport{}, fmt.Errorf("devseed clean: commit tx: %w", err)
	}

	return rep, nil
}

func cleanTx(ctx context.Context, tx pgx.Tx) (CleanReport, error) {
	var rep CleanReport

	// 1. completed_game_players
	t1, err := tx.Exec(ctx, `
		DELETE FROM completed_game_players
		WHERE game_id LIKE $1 || '%'
		   OR user_id BETWEEN $2 AND $3
	`, SeedGameIDPrefix, SeedUserIDStart, SeedUserIDEnd)
	if err != nil {
		return rep, fmt.Errorf("delete completed_game_players: %w", err)
	}
	rep.CompletedGamePlayers = t1.RowsAffected()

	// 2. completed_games
	t2, err := tx.Exec(ctx, `
		DELETE FROM completed_games
		WHERE game_id LIKE $1 || '%'
		   OR chat_id BETWEEN $2 AND $3
	`, SeedGameIDPrefix, SeedChatIDEnd, SeedChatIDStart)
	if err != nil {
		return rep, fmt.Errorf("delete completed_games: %w", err)
	}
	rep.CompletedGames = t2.RowsAffected()

	// 3. player_group_monthly_stats
	t3, err := tx.Exec(ctx, `
		DELETE FROM player_group_monthly_stats
		WHERE chat_id BETWEEN $1 AND $2
		   OR user_id BETWEEN $3 AND $4
	`, SeedChatIDEnd, SeedChatIDStart, SeedUserIDStart, SeedUserIDEnd)
	if err != nil {
		return rep, fmt.Errorf("delete player_group_monthly_stats: %w", err)
	}
	rep.MonthlyStats = t3.RowsAffected()

	// 4. player_group_stats
	t4, err := tx.Exec(ctx, `
		DELETE FROM player_group_stats
		WHERE chat_id BETWEEN $1 AND $2
		   OR user_id BETWEEN $3 AND $4
	`, SeedChatIDEnd, SeedChatIDStart, SeedUserIDStart, SeedUserIDEnd)
	if err != nil {
		return rep, fmt.Errorf("delete player_group_stats: %w", err)
	}
	rep.CumulativeStats = t4.RowsAffected()

	// 5. known_group_users
	t5, err := tx.Exec(ctx, `
		DELETE FROM known_group_users
		WHERE chat_id BETWEEN $1 AND $2
		   OR user_id BETWEEN $3 AND $4
	`, SeedChatIDEnd, SeedChatIDStart, SeedUserIDStart, SeedUserIDEnd)
	if err != nil {
		return rep, fmt.Errorf("delete known_group_users: %w", err)
	}
	rep.KnownUsers = t5.RowsAffected()

	// 6. group_configs
	t6, err := tx.Exec(ctx, `
		DELETE FROM group_configs
		WHERE chat_id BETWEEN $1 AND $2
	`, SeedChatIDEnd, SeedChatIDStart)
	if err != nil {
		return rep, fmt.Errorf("delete group_configs: %w", err)
	}
	rep.GroupConfigs = t6.RowsAffected()

	return rep, nil
}

// Seed populates the database with deterministic fixtures for the current month.
// It is 100% idempotent: running it cleans previous devseed rows and inserts fresh fixtures.
func Seed(ctx context.Context, pool *pgxpool.Pool, now time.Time) (Report, error) {
	if now.IsZero() {
		now = time.Now()
	}
	inLoc := now.In(ranking.RankingLocation)
	monthStart := ranking.MonthStart(inLoc)
	monthDateStr := ranking.MonthDateString(inLoc)
	monthName := fmt.Sprintf("%s/%d", ranking.MonthName(inLoc), inLoc.Year())

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("devseed: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Idempotency: clean any prior devseed data first inside the transaction
	if _, err := cleanTx(ctx, tx); err != nil {
		return Report{}, fmt.Errorf("devseed: clean prior fixtures: %w", err)
	}

	report := Report{
		CurrentMonth:      monthName,
		MonthStart:        monthDateStr,
		Timezone:          ranking.RankingLocation.String(),
		LargeGroupMembers: 60,
		SmallGroupMembers: 2,
		FallbackGroups:    1,
		FallbackPlayers:   1,
		Namespace: fmt.Sprintf("chat_id [%d .. %d], user_id [%d .. %d], game_id %s*",
			SeedChatIDEnd, SeedChatIDStart, SeedUserIDStart, SeedUserIDEnd, SeedGameIDPrefix),
	}

	// 1. Seed Updated (Atualizado) system
	updatedReport, err := seedUpdated(ctx, tx, monthStart, inLoc)
	if err != nil {
		return Report{}, fmt.Errorf("seed updated: %w", err)
	}

	// 2. Seed Legacy (Legado) system
	legacyReport, err := seedLegacy(ctx, tx, monthStart, inLoc)
	if err != nil {
		return Report{}, fmt.Errorf("seed legacy: %w", err)
	}

	report.UpdatedGroups = updatedReport.groups
	report.UpdatedPlayers = updatedReport.players
	report.UpdatedStatsRows = updatedReport.statsRows
	report.LegacyGroups = legacyReport.groups
	report.LegacyPlayers = legacyReport.players
	report.LegacyStatsRows = legacyReport.statsRows
	report.TotalGames = updatedReport.games + legacyReport.games

	if err := tx.Commit(ctx); err != nil {
		return Report{}, fmt.Errorf("devseed: commit tx: %w", err)
	}

	return report, nil
}

type systemStats struct {
	groups    int
	players   int
	statsRows int
	games     int
}

func seedUpdated(ctx context.Context, tx pgx.Tx, monthStart time.Time, now time.Time) (systemStats, error) {
	groupNames := updatedGroupNames()
	playerNames := updatedPlayerNames()

	totalGroups := len(groupNames) // 80 groups
	totalPlayers := len(playerNames) // 180 players

	// Create group_configs
	for i, name := range groupNames {
		chatID := SeedChatIDStart - int64(i)
		_, err := tx.Exec(ctx, `
			INSERT INTO group_configs (chat_id, default_game_mode, ranking_system, title, config_revision)
			VALUES ($1, 'classic', 'updated', $2, 1)
		`, chatID, name)
		if err != nil {
			return systemStats{}, fmt.Errorf("insert group_configs (updated %d): %w", i, err)
		}
	}

	// Group 1: "UNO da Galera" (ChatID: SeedChatIDStart)
	// Key test group matching mockup hero:
	// Freddy (UserID: 9900000001) has played earlier as "Freddy" and later as "Freddy UNO".
	// Detail tie-break:
	// Player 11 (LucasZ) and Player 12 (Mariana) both tied at 100,00 pts (10,000 units):
	// - LucasZ latest placement = 1
	// - Mariana latest placement = 2
	// -> LucasZ wins tie-break by last_placement!
	// Player 13 (Rafa) and Player 14 (Zero) both tied at 50,00 pts (5,000 units), both placement = 2:
	// - Rafa latest finished at now - 1h
	// - Zero latest finished at now - 3h
	// -> Rafa wins tie-break by last_completed_game_at!
	group1ChatID := SeedChatIDStart

	// We define group allocations
	type playerStat struct {
		userID      int64
		displayName string
		scoreUnits  int64
		games       int64
		wins        int64
		lastOffset  time.Duration
		placement   int
	}

	var statsRowsCount int
	var gamesCount int

	insertMember := func(chatID int64, p playerStat) error {
		lastFinished := now.Add(-p.lastOffset)
		if lastFinished.Before(monthStart) {
			lastFinished = monthStart.Add(2 * time.Hour)
		}

		// player_group_monthly_stats
		_, err := tx.Exec(ctx, `
			INSERT INTO player_group_monthly_stats (chat_id, user_id, month_start, ranking_system, score_units, completed_games, wins, display_name, last_finished_at, updated_at)
			VALUES ($1, $2, $3::date, 'updated', $4, $5, $6, $7, $8, now())
		`, chatID, p.userID, monthStart.Format("2006-01-02"), p.scoreUnits, p.games, p.wins, p.displayName, lastFinished)
		if err != nil {
			return fmt.Errorf("insert monthly stats: %w", err)
		}

		// player_group_stats
		_, err = tx.Exec(ctx, `
			INSERT INTO player_group_stats (chat_id, user_id, ranking_system, score_units, completed_games, wins, display_name, last_finished_at, updated_at)
			VALUES ($1, $2, 'updated', $3, $4, $5, $6, $7, now())
		`, chatID, p.userID, p.scoreUnits, p.games, p.wins, p.displayName, lastFinished)
		if err != nil {
			return fmt.Errorf("insert cumulative stats: %w", err)
		}

		// known_group_users
		_, err = tx.Exec(ctx, `
			INSERT INTO known_group_users (chat_id, user_id, display_name, username, last_seen_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (chat_id, user_id) DO NOTHING
		`, chatID, p.userID, p.displayName, fmt.Sprintf("user%d", p.userID%1000), lastFinished)
		if err != nil {
			return fmt.Errorf("insert known user: %w", err)
		}

		// Insert completed_games and completed_game_players to supply 'latest' CTE in detail query
		gameID := fmt.Sprintf("%s%s_%d_%d", SeedGameIDPrefix, "up", chatID, p.userID)
		hash := sha256Hex(gameID)

		_, err = tx.Exec(ctx, `
			INSERT INTO completed_games (game_id, chat_id, game_mode, ranking_system, config_revision, started_at, finished_at, final_revision, finish_reason, payload_hash, participant_count, scoring_status, policy_version, scored_at)
			VALUES ($1, $2, 'classic', 'updated', 1, $3, $4, 10, 'completed', $5, 2, 'scored', 'completed-placements-v1', $4)
			ON CONFLICT (game_id) DO NOTHING
		`, gameID, chatID, lastFinished.Add(-10*time.Minute), lastFinished, hash)
		if err != nil {
			return fmt.Errorf("insert completed game: %w", err)
		}

		status := "went_out"
		wentOut := true
		if p.placement > 1 {
			status = "playing"
			wentOut = false
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO completed_game_players (game_id, user_id, observed_name, username, final_status, position, went_out, score_units, joined_after_start, leave_count, reentry_count)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, 0, 0)
			ON CONFLICT (game_id, user_id) DO NOTHING
		`, gameID, p.userID, p.displayName, fmt.Sprintf("user%d", p.userID%1000), status, p.placement, wentOut, p.scoreUnits)
		if err != nil {
			return fmt.Errorf("insert completed game player: %w", err)
		}

		statsRowsCount++
		gamesCount++
		return nil
	}

	// Group 1 members ("UNO da Galera" total 2.840,00 pts = 284,000 units)
	// Breakdown: Freddy: 150,00 pts; Lívia: 60,00 pts; LucasZ: 10,00 pts; Mariana: 10,00 pts;
	// Rafa: 5,00 pts; Zero: 5,00 pts; others: 44,00 pts -> total = 284,00 pts
	group1Members := []playerStat{
		{SeedUserIDStart + 0, "Freddy UNO", 15000, 15, 12, 1 * time.Hour, 1},
		{SeedUserIDStart + 1, "Lívia", 6000, 10, 6, 2 * time.Hour, 1},
		{SeedUserIDStart + 10, "LucasZ", 1000, 4, 1, 3 * time.Hour, 1},   // Tied score, placement 1
		{SeedUserIDStart + 11, "Mariana", 1000, 5, 0, 4 * time.Hour, 2},  // Tied score, placement 2 (loses tie-break)
		{SeedUserIDStart + 12, "Rafa", 500, 3, 0, 5 * time.Hour, 2},      // Tied score, placement 2, finished 5h ago
		{SeedUserIDStart + 13, "Zero Pontos", 500, 3, 0, 7 * time.Hour, 2},// Tied score, placement 2, finished 7h ago (loses on time)
		{SeedUserIDStart + 14, "José Antônio", 4400, 8, 3, 8 * time.Hour, 1},
	}
	for _, m := range group1Members {
		if err := insertMember(group1ChatID, m); err != nil {
			return systemStats{}, err
		}
	}

	// Earlier historical game in Group 1 for Freddy to test display_name history:
	// earlier game had "Freddy", later game has "Freddy UNO"
	earlierGameID := fmt.Sprintf("%s%s_%d_early", SeedGameIDPrefix, "up", group1ChatID)
	_, _ = tx.Exec(ctx, `
		INSERT INTO completed_games (game_id, chat_id, game_mode, ranking_system, config_revision, started_at, finished_at, final_revision, finish_reason, payload_hash, participant_count, scoring_status, policy_version, scored_at)
		VALUES ($1, $2, 'classic', 'updated', 1, $3, $4, 5, 'completed', $5, 2, 'scored', 'completed-placements-v1', $4)
		ON CONFLICT (game_id) DO NOTHING
	`, earlierGameID, group1ChatID, now.Add(-48*time.Hour), now.Add(-47*time.Hour), sha256Hex(earlierGameID))
	_, _ = tx.Exec(ctx, `
		INSERT INTO completed_game_players (game_id, user_id, observed_name, username, final_status, position, went_out, score_units, joined_after_start, leave_count, reentry_count)
		VALUES ($1, $2, 'Freddy', 'freddy', 'went_out', 1, true, 1000, false, 0, 0)
		ON CONFLICT (game_id, user_id) DO NOTHING
	`, earlierGameID, SeedUserIDStart+0)

	// Group 2: Large Group with 60 members ("Mega Torneio Nacional de UNO 🎴")
	group2ChatID := SeedChatIDStart - 1
	for u := 0; u < 60; u++ {
		score := int64((60 - u) * 250) // Descending scores
		p := playerStat{
			userID:      SeedUserIDStart + int64(u),
			displayName: playerNames[u],
			scoreUnits:  score,
			games:       int64(u%10 + 2),
			wins:        int64((60 - u) / 5),
			lastOffset:  time.Duration(u*40+20) * time.Minute,
			placement:   (u % 4) + 1,
		}
		if err := insertMember(group2ChatID, p); err != nil {
			return systemStats{}, err
		}
	}

	// Group 3: Small Group with exactly 2 members ("Duelo 1v1 Clássico")
	group3ChatID := SeedChatIDStart - 2
	group3Members := []playerStat{
		{SeedUserIDStart + 2, "João Pedro", 8000, 8, 6, 2 * time.Hour, 1},
		{SeedUserIDStart + 3, "Ana Clara 🎮", 2000, 8, 2, 2 * time.Hour, 2},
	}
	for _, m := range group3Members {
		if err := insertMember(group3ChatID, m); err != nil {
			return systemStats{}, err
		}
	}

	// Group 4: Long Name Group
	group4ChatID := SeedChatIDStart - 3
	group4Members := []playerStat{
		{SeedUserIDStart + 4, "Mestre do +4 😈", 15000, 15, 10, 3 * time.Hour, 1},
		{SeedUserIDStart + 5, "Um Nome Extremamente Longo Para Testar Ellipsis e Truncamento Visual", 12000, 12, 8, 4 * time.Hour, 2},
	}
	for _, m := range group4Members {
		if err := insertMember(group4ChatID, m); err != nil {
			return systemStats{}, err
		}
	}

	// Group 5: Fallback group without title ("")
	group5ChatID := SeedChatIDStart - 4
	group5Members := []playerStat{
		{SeedUserIDStart + 16, "", 5000, 5, 3, 5 * time.Hour, 1}, // Player without name fallback
		{SeedUserIDStart + 7, "Beatriz", 3000, 5, 2, 6 * time.Hour, 2},
	}
	for _, m := range group5Members {
		if err := insertMember(group5ChatID, m); err != nil {
			return systemStats{}, err
		}
	}

	// Group 6 & 7: Tie-break groups (Same score 600,00 pts = 60,000 units, different activity)
	group6ChatID := SeedChatIDStart - 5 // "Mesa Empatada Beta"
	group7ChatID := SeedChatIDStart - 6 // "Mesa Empatada Alfa"
	if err := insertMember(group6ChatID, playerStat{SeedUserIDStart + 6, "Player Empate Z", 60000, 10, 6, 1 * time.Hour, 1}); err != nil {
		return systemStats{}, err
	}
	if err := insertMember(group7ChatID, playerStat{SeedUserIDStart + 7, "Player Empate A", 60000, 10, 6, 3 * time.Hour, 1}); err != nil {
		return systemStats{}, err
	}

	// Group 8 & 9: Tie-break groups (Same score 400,00 pts, same activity, sorted by name)
	group8ChatID := SeedChatIDStart - 7 // "Mesa Alfa Mesma Atividade"
	group9ChatID := SeedChatIDStart - 8 // "Mesa Delta Mesma Atividade"
	if err := insertMember(group8ChatID, playerStat{SeedUserIDStart + 8, "Alice Empatada", 40000, 8, 4, 4 * time.Hour, 1}); err != nil {
		return systemStats{}, err
	}
	if err := insertMember(group9ChatID, playerStat{SeedUserIDStart + 9, "Bob Empatado", 40000, 8, 4, 4 * time.Hour, 1}); err != nil {
		return systemStats{}, err
	}

	// Group 10: High Score Group ("Campeonato de Mestres 🏆" score 25.000,00 pts = 2,500,000 units)
	group10ChatID := SeedChatIDStart - 9
	// Freddy plays here too (multi-group test!)
	if err := insertMember(group10ChatID, playerStat{SeedUserIDStart + 0, "Freddy UNO", 2485000, 250, 180, 20 * time.Minute, 1}); err != nil {
		return systemStats{}, err
	}
	if err := insertMember(group10ChatID, playerStat{SeedUserIDStart + 1, "Lívia", 15000, 20, 12, 30 * time.Minute, 2}); err != nil {
		return systemStats{}, err
	}

	// Groups 11 to 80: Populate remaining groups with varied members and scores
	for i := 10; i < totalGroups; i++ {
		chatID := SeedChatIDStart - int64(i)
		memberCount := (i % 6) + 3 // 3 to 8 members per group
		baseScore := int64((totalGroups - i) * 1500)

		for m := 0; m < memberCount; m++ {
			playerIdx := (i*3 + m) % totalPlayers
			score := baseScore / int64(m+1)
			if score < 100 {
				score = int64(m * 100)
			}
			wins := int64(0)
			if m == 0 {
				wins = 2
			}
			p := playerStat{
				userID:      SeedUserIDStart + int64(playerIdx),
				displayName: playerNames[playerIdx],
				scoreUnits:  score,
				games:       int64((i%8) + 2),
				wins:        wins,
				lastOffset:  time.Duration(i*30+m*15) * time.Minute,
				placement:   m + 1,
			}
			if err := insertMember(chatID, p); err != nil {
				return systemStats{}, err
			}
		}
	}

	return systemStats{
		groups:    totalGroups,
		players:   totalPlayers,
		statsRows: statsRowsCount,
		games:     gamesCount,
	}, nil
}

func seedLegacy(ctx context.Context, tx pgx.Tx, monthStart time.Time, now time.Time) (systemStats, error) {
	groupNames := legacyGroupNames()
	playerNames := legacyPlayerNames()

	totalGroups := len(groupNames)   // 65 groups
	totalPlayers := len(playerNames) // 140 players

	// Create group_configs
	for i, name := range groupNames {
		chatID := SeedChatIDStart - 100 - int64(i)
		_, err := tx.Exec(ctx, `
			INSERT INTO group_configs (chat_id, default_game_mode, ranking_system, title, config_revision)
			VALUES ($1, 'classic', 'legacy', $2, 1)
		`, chatID, name)
		if err != nil {
			return systemStats{}, fmt.Errorf("insert group_configs (legacy %d): %w", i, err)
		}
	}

	var statsRowsCount int
	var gamesCount int

	insertLegacyMember := func(chatID int64, userID int64, name string, scoreUnits int64, games int64, wins int64, offset time.Duration, placement int) error {
		lastFinished := now.Add(-offset)
		if lastFinished.Before(monthStart) {
			lastFinished = monthStart.Add(3 * time.Hour)
		}

		// player_group_monthly_stats
		_, err := tx.Exec(ctx, `
			INSERT INTO player_group_monthly_stats (chat_id, user_id, month_start, ranking_system, score_units, completed_games, wins, display_name, last_finished_at, updated_at)
			VALUES ($1, $2, $3::date, 'legacy', $4, $5, $6, $7, $8, now())
		`, chatID, userID, monthStart.Format("2006-01-02"), scoreUnits, games, wins, name, lastFinished)
		if err != nil {
			return fmt.Errorf("insert legacy monthly stats: %w", err)
		}

		// player_group_stats
		_, err = tx.Exec(ctx, `
			INSERT INTO player_group_stats (chat_id, user_id, ranking_system, score_units, completed_games, wins, display_name, last_finished_at, updated_at)
			VALUES ($1, $2, 'legacy', $3, $4, $5, $6, $7, now())
		`, chatID, userID, scoreUnits, games, wins, name, lastFinished)
		if err != nil {
			return fmt.Errorf("insert legacy cumulative stats: %w", err)
		}

		// known_group_users
		_, err = tx.Exec(ctx, `
			INSERT INTO known_group_users (chat_id, user_id, display_name, username, last_seen_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (chat_id, user_id) DO NOTHING
		`, chatID, userID, name, fmt.Sprintf("leguser%d", userID%1000), lastFinished)
		if err != nil {
			return fmt.Errorf("insert legacy known user: %w", err)
		}

		// completed_games and completed_game_players
		gameID := fmt.Sprintf("%s%s_%d_%d", SeedGameIDPrefix, "leg", chatID, userID)
		hash := sha256Hex(gameID)

		_, err = tx.Exec(ctx, `
			INSERT INTO completed_games (game_id, chat_id, game_mode, ranking_system, config_revision, started_at, finished_at, final_revision, finish_reason, payload_hash, participant_count, scoring_status, policy_version, scored_at)
			VALUES ($1, $2, 'classic', 'legacy', 1, $3, $4, 10, 'completed', $5, 2, 'scored', 'completed-placements-v1', $4)
			ON CONFLICT (game_id) DO NOTHING
		`, gameID, chatID, lastFinished.Add(-10*time.Minute), lastFinished, hash)
		if err != nil {
			return fmt.Errorf("insert legacy completed game: %w", err)
		}

		status := "went_out"
		wentOut := true
		if placement > 1 {
			status = "playing"
			wentOut = false
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO completed_game_players (game_id, user_id, observed_name, username, final_status, position, went_out, score_units, joined_after_start, leave_count, reentry_count)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false, 0, 0)
			ON CONFLICT (game_id, user_id) DO NOTHING
		`, gameID, userID, name, fmt.Sprintf("leguser%d", userID%1000), status, placement, wentOut, scoreUnits)
		if err != nil {
			return fmt.Errorf("insert legacy completed game player: %w", err)
		}

		statsRowsCount++
		gamesCount++
		return nil
	}

	// Legacy scores (1 pt = 100 units). Top scores: 2500 pts (250,000 units), 999 pts, 120 pts, etc.
	legacyScores := []int64{
		250000, // 2500 pts
		99900,  // 999 pts
		12000,  // 120 pts
		5000,   // 50 pts
		2000,   // 20 pts
		1000,   // 10 pts
		500,    // 5 pts
		200,    // 2 pts
		100,    // 1 pt
		0,      // 0 pt
	}

	for i := 0; i < totalGroups; i++ {
		chatID := SeedChatIDStart - 100 - int64(i)
		memberCount := (i % 5) + 3 // 3 to 7 members

		for m := 0; m < memberCount; m++ {
			playerIdx := (i*2 + m) % totalPlayers
			userID := SeedUserIDStart + int64(playerIdx) // Overlapping UserIDs with Updated!
			name := playerNames[playerIdx]

			scoreIdx := (i + m) % len(legacyScores)
			score := legacyScores[scoreIdx]

			games := int64((score / 100) + 1)
			if games < 1 {
				games = 1
			}
			wins := int64(score / 200)

			offset := time.Duration((i*45 + m*20)) * time.Minute
			placement := (m % 3) + 1

			if err := insertLegacyMember(chatID, userID, name, score, games, wins, offset, placement); err != nil {
				return systemStats{}, err
			}
		}
	}

	return systemStats{
		groups:    totalGroups,
		players:   totalPlayers,
		statsRows: statsRowsCount,
		games:     gamesCount,
	}, nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func updatedGroupNames() []string {
	names := []string{
		"UNO da Galera",                                                    // 1: Hero detail match
		"Mega Torneio Nacional de UNO 🎴",                                  // 2: 60 members
		"Duelo 1v1 Clássico",                                               // 3: 2 members
		"Grupo com um nome propositalmente muito grande para testar truncamento e layout visual no Mini App", // 4: Long name
		"",                                                                 // 5: Fallback without title
		"Mesa Empatada Beta",                                               // 6: Tie test
		"Mesa Empatada Alfa",                                               // 7: Tie test
		"Mesa Alfa Mesma Atividade",                                        // 8: Tie test
		"Mesa Delta Mesma Atividade",                                       // 9: Tie test
		"Campeonato de Mestres 🏆",                                         // 10: High score (25.000,00 pts)
		"UNO Brasil",
		"Mesa dos Amigos",
		"Noite do +4 😈",
		"Só Vale Reversa 🔄",
		"Família do UNO 🃏",
		"UNO da Firma 💼",
		"Campeonato de Sexta",
		"UNO Universitário 🎓",
		"Mesa 7",
		"Os Sem Cor",
		"Arena UNO 🏟️",
		"UNO da Madrugada 🌙",
		"Baralho Insano 🔥",
		"Última Carta 🃏",
		"Sem +4 Por Favor 🛑",
		"Clube da Meia-Noite",
		"Reversa Infinita",
		"Mesa das Estrelas ⭐",
		"Amigos do Cartão Amarelo",
		"UNO Express ⚡",
		"Toca do Baralho",
		"Só Amigos 🤝",
		"Liga dos Campeões de UNO",
		"UNO & Pizza 🍕",
		"Sextou com UNO 🎉",
		"Mesa Secreta 🤫",
		"UNO Raiz",
		"Curinga de Ouro 👑",
		"Desafio das Cores 🎨",
		"Mestres do Blefe 🎭",
		"Fim de Tarde UNO 🌇",
		"Torneio Relâmpago ⚡",
		"Giro da Mesa 🔄",
		"Resenha do UNO",
		"Baralho Maluco 🤪",
		"Salão dos Cartas",
		"UNO dos Campeões",
		"Esquadrão +4",
		"Mesa Central",
		"Guerra de Cores",
		"UNO Noturno 🌃",
		"Cartas na Mesa",
		"Liga Amadora de UNO",
		"Gente Boa & UNO",
		"Baralho Voador 🛸",
		"Só uma Partida ⏱️",
		"Mesa de Bar 🍻",
		"Última Rodada",
		"Curinga da Vez 🃏",
		"Reversos Anônimos",
		"Mesa 42",
		"Amigos da Madrugada",
		"UNO sem Fim ♾️",
		"Mesa VIP ✨",
		"Clube do Ponto 🎯",
		"Família Reversa",
		"UNO dos Amigos 💙",
		"Mesa Aberta 🔓",
		"Baralho Quente 🔥",
		"Ponto a Ponto 📍",
		"Liga Regional de UNO",
		"Sábado com UNO ☀️",
		"Tarde de Cartas ☕",
		"Mesa dos Vencedores 🥇",
		"Só Mais Uma 🎲",
		"Amigos de Domingo",
		"Curinga Negro",
		"Noite das Cores 🌈",
		"Baralho Mágico 🪄",
		"Mesa 99",
		"Liga Universitária",
		"UNO Premium 💎",
		"Clube dos Quatro",
		"Reversa de Domingo",
		"Cartas Vivas",
		"Arena Central",
		"Copa UNO Brasil",
		"Amigos do +2",
		"Mesa do Café",
		"UNO Total",
	}
	return names[:80]
}

func updatedPlayerNames() []string {
	names := []string{
		"Freddy",
		"Lívia",
		"João Pedro",
		"Ana Clara 🎮",
		"Mestre do +4 😈",
		"Um Nome Extremamente Longo Para Testar Ellipsis e Truncamento Visual",
		"Player Empate Z",
		"Player Empate A",
		"Alice Empatada",
		"Bob Empatado",
		"LucasZ",
		"Mariana",
		"Rafa",
		"Zero Pontos",
		"José Antônio",
		"Kira ✨",
		"", // 17: Fallback player without name
		"Gustavo",
		"Beatriz",
		"Bo",
		"Al",
		"Sofia 🌸",
		"Carlos Eduardo",
		"Matheus ⚡",
		"Gabriel",
		"Camila",
		"Felipe 🚀",
		"Larissa",
		"Bruno 🔥",
		"Juliana",
		"Thiago",
		"Amanda 💖",
		"Diego",
		"Bruna",
		"Leonardo",
		"Fernanda",
		"Rodrigo",
		"Natália",
		"André 🎯",
		"Bianca",
		"Guilherme",
		"Vanessa",
		"Vinícius",
		"Letícia",
		"Alexandre",
		"Isabela",
		"Lucas",
		"Carolina",
		"Eduardo",
		"Luana",
		"Marcelo",
		"Priscila",
		"Renato",
		"Sabrina",
		"Danilo",
		"Tatiane",
		"Caio",
		"Jéssica",
		"Vitor",
		"Aline",
	}

	// Expand to 180 varied names
	for i := len(names); i < 180; i++ {
		names = append(names, fmt.Sprintf("Jogador UNO #%d 🃏", i+1))
	}
	return names
}

func legacyGroupNames() []string {
	names := []string{
		"Legado UNO Clássico",
		"Mesa Raiz dos Amigos",
		"Velha Guarda do UNO",
		"Clássico sem Frescura",
		"Pontuação Pura",
		"Regra Antiga de Sexta",
		"Mesa dos Veteranos",
		"UNO Retrô 📼",
		"Baralho Tradicional",
		"Liga Raiz de UNO",
	}
	for i := len(names); i < 65; i++ {
		names = append(names, fmt.Sprintf("Grupo Legado %d 🏛️", i+1))
	}
	return names
}

func legacyPlayerNames() []string {
	names := []string{
		"Freddy",
		"Lívia",
		"João Pedro",
		"Ana Clara",
		"Carlos Veterano",
		"Mário Clássico",
		"Pedro Antigo",
		"Antônio",
		"Maria Clara",
		"Francisca",
	}
	for i := len(names); i < 140; i++ {
		names = append(names, fmt.Sprintf("Player Legado #%d", i+1))
	}
	return names
}
