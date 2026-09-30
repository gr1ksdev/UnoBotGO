package postgres

import (
	"context"
	"time"

	"github.com/malbs/UnoGoBot/internal/ranking"
)

var _ ranking.ReadRepository = (*Store)(nil)

// ListGroupRanking reads config, exact total and a bounded ordered prefix for the
// month of 'at' (in America/Sao_Paulo) in one statement/snapshot. Zero-score
// completed players are valid standings. Each user's latest scored, eligible
// participation in that month supplies the tie-breakers.
func (s *Store) ListGroupRanking(ctx context.Context, chatID int64, at time.Time) (ranking.GroupRanking, error) {
	if at.IsZero() {
		at = time.Now()
	}
	monthStart := ranking.MonthDateString(at)
	rows, err := s.pool.Query(ctx, `
WITH latest AS (
 SELECT DISTINCT ON (p.user_id) p.user_id,p.position,g.finished_at
 FROM completed_games g
 JOIN completed_game_players p ON p.game_id=g.game_id
 WHERE g.chat_id=$1 AND g.scoring_status='scored'
 AND (date_trunc('month', (g.finished_at AT TIME ZONE 'America/Sao_Paulo')))::date = $2::date
 AND p.position IS NOT NULL
 AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out))
 ORDER BY p.user_id,g.finished_at DESC,g.game_id DESC
)
SELECT COALESCE(c.ranking_system,'legacy'),
 COALESCE(r.user_id,0), COALESCE(r.display_name,''), COALESCE(r.score_units,0),
 COALESCE(r.completed_games,0), COALESCE(r.wins,0), COALESCE(r.total,0), COALESCE(r.compatible,true)
FROM (VALUES ($1::bigint)) AS requested(chat_id)
LEFT JOIN group_configs c ON c.chat_id=requested.chat_id
LEFT JOIN LATERAL (
 SELECT s.user_id,s.display_name,s.score_units,s.completed_games,s.wins,
 latest.position AS last_placement,latest.finished_at AS last_completed_game_at,
 count(*) OVER () AS total,
 bool_and(s.ranking_system=COALESCE(c.ranking_system,'legacy')) OVER () AS compatible
 FROM player_group_monthly_stats s LEFT JOIN latest ON latest.user_id=s.user_id
 WHERE s.chat_id=requested.chat_id AND s.month_start=$2::date
 ORDER BY s.score_units DESC,latest.position ASC NULLS LAST,latest.finished_at DESC NULLS LAST,s.user_id ASC LIMIT $3
) r ON true
ORDER BY r.score_units DESC,r.last_placement ASC NULLS LAST,r.last_completed_game_at DESC NULLS LAST,r.user_id ASC`, chatID, monthStart, ranking.MaxRankingEntries)
	if err != nil {
		return ranking.GroupRanking{}, operationError(ctx, "list group ranking")
	}
	defer rows.Close()
	result := ranking.GroupRanking{
		MonthName:  ranking.MonthName(at),
		MonthStart: ranking.MonthStart(at),
	}
	for rows.Next() {
		var entry ranking.Entry
		var compatible bool
		if err := rows.Scan(&result.System, &entry.UserID, &entry.DisplayName, &entry.Score, &entry.CompletedGames, &entry.Wins, &result.Total, &compatible); err != nil {
			return ranking.GroupRanking{}, operationError(ctx, "read group ranking")
		}
		if !compatible {
			return ranking.GroupRanking{}, ranking.ErrNeedsProductDecision
		}
		if entry.UserID != 0 {
			result.Entries = append(result.Entries, entry)
		}
	}
	if rows.Err() != nil {
		return ranking.GroupRanking{}, operationError(ctx, "read group ranking")
	}
	return result, nil
}
