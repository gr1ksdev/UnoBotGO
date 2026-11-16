package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

var _ ranking.PlayerStatsRepository = (*Store)(nil)

// ReadPlayerStats aggregates the scored, eligible participation of one player.
// A zero chatID aggregates every group.
func (s *Store) ReadPlayerStats(ctx context.Context, chatID, userID int64, at time.Time) (ranking.PlayerStats, error) {
	if userID <= 0 {
		return ranking.PlayerStats{}, groups.ErrInvalid
	}
	if at.IsZero() {
		at = time.Now()
	}
	const query = `
SELECT g.ranking_system,
       count(*)                                    AS games,
       count(*) FILTER (WHERE p.position = 1)      AS wins,
       coalesce(min(p.position), 0)                AS best_position,
       coalesce(round(avg(p.position)::numeric, 2), 0) AS avg_position,
       coalesce(sum(p.score_units), 0)             AS score_units
FROM completed_game_players p JOIN completed_games g USING(game_id)
WHERE p.user_id=$2 AND g.scoring_status='scored'
 AND ($1::bigint = 0 OR g.chat_id=$1)
 AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out))
GROUP BY g.ranking_system
ORDER BY g.ranking_system`

	rows, err := s.pool.Query(ctx, query, chatID, userID)
	if err != nil {
		return ranking.PlayerStats{}, operationError(ctx, "read player stats")
	}
	defer rows.Close()

	var stats ranking.PlayerStats
	found := false
	for rows.Next() {
		var system string
		var avg string
		var score int64
		if err := rows.Scan(&system, &stats.Games, &stats.Wins, &stats.BestPosition, &avg, &score); err != nil {
			return ranking.PlayerStats{}, operationError(ctx, "scan player stats")
		}
		stats.System = groups.RankingSystem(system)
		stats.AvgPosition = avg
		stats.Score = ranking.Units(score)
		found = true
	}
	if err := rows.Err(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ranking.PlayerStats{}, nil
		}
		return ranking.PlayerStats{}, operationError(ctx, "read player stats")
	}
	if !found {
		return ranking.PlayerStats{}, nil
	}
	return stats, nil
}
