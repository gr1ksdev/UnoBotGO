package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
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
WITH latest AS (`+eligibleMonthlyLatest+`)
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
 WHERE s.chat_id=requested.chat_id AND s.month_start=$4::date
 ORDER BY s.score_units DESC,latest.position ASC NULLS LAST,latest.finished_at DESC NULLS LAST,s.user_id ASC LIMIT $5
) r ON true
ORDER BY r.score_units DESC,r.last_placement ASC NULLS LAST,r.last_completed_game_at DESC NULLS LAST,r.user_id ASC`, chatID, ranking.MonthStart(at), ranking.MonthStart(at).AddDate(0, 1, 0), monthStart, ranking.MaxRankingEntries)
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

// ListUserMonthlyRankings returns the user's monthly ranking entries across groups
// for the canonical month of 'at' in America/Sao_Paulo in a single query.
func (s *Store) ListUserMonthlyRankings(ctx context.Context, userID int64, at time.Time) (ranking.UserMonthlyRankings, error) {
	if at.IsZero() {
		at = time.Now()
	}
	monthStart := ranking.MonthDateString(at)
	rows, err := s.pool.Query(ctx, `
SELECT
  s.chat_id,
  COALESCE(c.title, ''),
  s.ranking_system,
  s.score_units,
  s.last_finished_at
FROM player_group_monthly_stats s
LEFT JOIN group_configs c ON c.chat_id=s.chat_id
WHERE s.user_id=$1 AND s.month_start=$2::date
ORDER BY
  CASE WHEN s.ranking_system='updated' THEN 1 ELSE 2 END ASC,
  s.score_units DESC,
  s.last_finished_at DESC,
  COALESCE(NULLIF(c.title, ''), 'Grupo ' || s.chat_id::text) ASC,
  s.chat_id ASC`, userID, monthStart)
	if err != nil {
		return ranking.UserMonthlyRankings{}, operationError(ctx, "list user monthly rankings")
	}
	defer rows.Close()

	result := ranking.UserMonthlyRankings{
		UserID:     userID,
		MonthName:  ranking.MonthName(at),
		MonthStart: ranking.MonthStart(at),
	}

	for rows.Next() {
		var chatID int64
		var title string
		var systemStr string
		var scoreUnits int64
		var lastFinishedAt time.Time

		if err := rows.Scan(&chatID, &title, &systemStr, &scoreUnits, &lastFinishedAt); err != nil {
			return ranking.UserMonthlyRankings{}, operationError(ctx, "read user monthly rankings")
		}

		groupName := strings.TrimSpace(title)
		if groupName == "" {
			groupName = fmt.Sprintf("Grupo %d", chatID)
		}

		system := groups.RankingSystem(systemStr)
		entry := ranking.UserGroupRankingEntry{
			ChatID:         chatID,
			GroupName:      groupName,
			RankingSystem:  system,
			ScoreUnits:     ranking.Units(scoreUnits),
			LastFinishedAt: lastFinishedAt,
		}

		if system == groups.Updated {
			if result.Updated == nil {
				result.Updated = &ranking.UserMonthlyRankingSection{System: groups.Updated}
			}
			result.Updated.Entries = append(result.Updated.Entries, entry)
			result.Updated.TotalScore += entry.ScoreUnits
		} else if system == groups.Legacy {
			if result.Legacy == nil {
				result.Legacy = &ranking.UserMonthlyRankingSection{System: groups.Legacy}
			}
			result.Legacy.Entries = append(result.Legacy.Entries, entry)
			result.Legacy.TotalScore += entry.ScoreUnits
		}
	}
	if rows.Err() != nil {
		return ranking.UserMonthlyRankings{}, operationError(ctx, "read user monthly rankings")
	}
	return result, nil
}
