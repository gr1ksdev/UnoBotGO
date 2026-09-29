package postgres

import (
	"context"

	"github.com/malbs/UnoGoBot/internal/ranking"
)

var _ ranking.ReadRepository = (*Store)(nil)

// ListGroupRanking reads config, exact total and a bounded ordered prefix in
// one statement/snapshot. Zero-score completed players are valid standings.
// The secondary UserID order does not assign competitive positions.
func (s *Store) ListGroupRanking(ctx context.Context, chatID int64) (ranking.GroupRanking, error) {
	rows, err := s.pool.Query(ctx, `
SELECT COALESCE(c.ranking_system,'legacy'),
 COALESCE(r.user_id,0), COALESCE(r.display_name,''), COALESCE(r.score_units,0),
 COALESCE(r.completed_games,0), COALESCE(r.wins,0), COALESCE(r.total,0), COALESCE(r.compatible,true)
FROM (VALUES ($1::bigint)) AS requested(chat_id)
LEFT JOIN group_configs c ON c.chat_id=requested.chat_id
LEFT JOIN LATERAL (
 SELECT user_id,display_name,score_units,completed_games,wins,
 count(*) OVER () AS total,
 bool_and(ranking_system=c.ranking_system) OVER () AS compatible
 FROM player_group_stats WHERE chat_id=requested.chat_id
 ORDER BY score_units DESC,user_id LIMIT $2
) r ON true
ORDER BY r.score_units DESC,r.user_id`, chatID, ranking.MaxRankingEntries)
	if err != nil {
		return ranking.GroupRanking{}, operationError(ctx, "list group ranking")
	}
	defer rows.Close()
	var result ranking.GroupRanking
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
