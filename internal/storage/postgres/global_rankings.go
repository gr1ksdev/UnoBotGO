package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/malbs/UnoGoBot/internal/ranking"
)

// Shared by the Telegram and Mini App reads: only scored, eligible finishes.
const eligibleMonthlyLatest = `
 SELECT DISTINCT ON (p.user_id) p.user_id,p.position,g.finished_at
 FROM completed_games g JOIN completed_game_players p ON p.game_id=g.game_id
 WHERE g.chat_id=$1 AND g.scoring_status='scored'
 AND g.finished_at >= $2::timestamptz AND g.finished_at < $3::timestamptz
 AND p.position IS NOT NULL
 AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out))
 ORDER BY p.user_id,g.finished_at DESC,g.game_id DESC
`

const detailOrder = `score_units DESC,last_placement ASC,last_completed_game_at DESC,id ASC`

func (s *Store) ReadGlobalRanking(ctx context.Context, req ranking.GlobalRequest) (ranking.GlobalPage, error) {
	args := []any{req.GroupID, req.Month, req.Month.AddDate(0, 1, 0), ranking.MonthDateString(req.Month), req.System}
	// Names used for both ordering and presentation have a safe, masked fallback.
	base := `SELECT s.chat_id AS id,COALESCE(NULLIF(btrim(c.title),''),'Grupo ••••' || right(abs(s.chat_id::numeric)::text,4)) COLLATE "C" AS name,
 sum(s.score_units)::bigint AS score_units,max(s.last_finished_at) AS last_completed_game_at,0::integer AS last_placement,
 COALESCE(c.ranking_private, false) AS anonymous
 FROM player_group_monthly_stats s LEFT JOIN group_configs c ON c.chat_id=s.chat_id
 WHERE s.month_start=$4::date AND s.ranking_system=$5 GROUP BY s.chat_id,c.title,c.ranking_private`
	order := `score_units DESC,last_completed_game_at DESC,name COLLATE "C" ASC,id ASC`
	if req.Kind == "players" {
		base = `SELECT s.user_id AS id, COALESCE(NULLIF((array_agg(s.display_name ORDER BY s.last_finished_at DESC,s.chat_id ASC))[1],''),'Jogador ••••' || right(s.user_id::text,4)) COLLATE "C" AS name,
 sum(s.score_units)::bigint AS score_units,max(s.last_finished_at) AS last_completed_game_at,0::integer AS last_placement,
 COALESCE(u.ranking_private, false) AS anonymous
 FROM player_group_monthly_stats s LEFT JOIN user_privacy_settings u ON u.user_id=s.user_id
 WHERE s.month_start=$4::date AND s.ranking_system=$5 GROUP BY s.user_id,u.ranking_private`
	}
	if req.Kind == "detail" {
		base = `SELECT s.user_id AS id,COALESCE(NULLIF(s.display_name,''),'Jogador ••••' || right(s.user_id::text,4)) AS name,
 s.score_units,COALESCE(l.finished_at,s.last_finished_at) AS last_completed_game_at,COALESCE(l.position,2147483647) AS last_placement,
 (COALESCE(c.ranking_private, false) OR COALESCE(u.ranking_private, false)) AS anonymous
 FROM player_group_monthly_stats s
 LEFT JOIN latest l ON l.user_id=s.user_id
 LEFT JOIN group_configs c ON c.chat_id=s.chat_id
 LEFT JOIN user_privacy_settings u ON u.user_id=s.user_id
 WHERE s.chat_id=$1 AND s.month_start=$4::date AND s.ranking_system=$5`
		order = detailOrder
	}
	condition := "true"
	if req.After != nil {
		k := req.After
		args = append(args, int64(k.Score), k.Activity, k.Name, k.ID, k.Placement)
		condition = `(score_units < $6 OR (score_units=$6 AND (last_completed_game_at<$7 OR (last_completed_game_at=$7 AND (name COLLATE "C">$8 OR (name=$8 AND id>$9))))))`
		if req.Kind == "detail" {
			condition = `(score_units<$6 OR (score_units=$6 AND (last_placement>$10 OR (last_placement=$10 AND (last_completed_game_at<$7 OR (last_completed_game_at=$7 AND id>$9)))))) AND $8::text IS NOT NULL`
		} else {
			condition += ` AND $10::integer IS NOT NULL`
		}
	}
	args = append(args, req.Limit+1)
	// A single statement supplies metadata and page in the same PostgreSQL snapshot.
	query := `WITH latest AS (` + eligibleMonthlyLatest + `), base AS (` + base + `), ranked AS (
 SELECT *,row_number() OVER (ORDER BY ` + order + `) AS position FROM base), page AS (
 SELECT * FROM ranked WHERE ` + condition + ` ORDER BY ` + order + fmt.Sprintf(` LIMIT $%d
 ), header AS (
 SELECT COALESCE(NULLIF(btrim(c.title),''),'Grupo ••••' || right(abs(s.chat_id::numeric)::text,4)) AS name,sum(s.score_units)::bigint AS total,
 COALESCE(c.ranking_private, false) AS anonymous
 FROM player_group_monthly_stats s LEFT JOIN group_configs c ON c.chat_id=s.chat_id
 WHERE s.chat_id=$1 AND s.month_start=$4::date AND s.ranking_system=$5 GROUP BY s.chat_id,c.title,c.ranking_private)
 SELECT p.id,p.name,p.score_units,p.last_completed_game_at,p.last_placement,p.position,p.anonymous,h.name,h.total,h.anonymous
 FROM (SELECT 1) anchor LEFT JOIN page p ON true LEFT JOIN header h ON true
 ORDER BY p.position`, len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return ranking.GlobalPage{}, operationError(ctx, "global ranking query")
	}
	defer rows.Close()
	result := ranking.GlobalPage{Rows: []ranking.GlobalRow{}}
	for rows.Next() {
		var id *int64
		var name *string
		var score *int64
		var row ranking.GlobalRow
		var headerName *string
		var headerScore *int64
		// Nullable page (empty standings) still yields the group header.
		var activity *time.Time
		var placement *int
		var position *int64
		var pAnonymous *bool
		var hAnonymous *bool
		if err := rows.Scan(&id, &name, &score, &activity, &placement, &position, &pAnonymous, &headerName, &headerScore, &hAnonymous); err != nil {
			return result, operationError(ctx, "scan global ranking")
		}
		if id != nil {
			row = ranking.GlobalRow{ID: *id, Name: *name, Score: ranking.Units(*score), Activity: *activity, Placement: *placement, Position: *position, Anonymous: pAnonymous != nil && *pAnonymous}
			result.Rows = append(result.Rows, row)
		}
		if headerName != nil {
			result.Group = &ranking.GlobalRow{ID: req.GroupID, Name: *headerName, Score: ranking.Units(*headerScore), Anonymous: hAnonymous != nil && *hAnonymous}
		}
	}
	if rows.Err() != nil {
		return result, operationError(ctx, "read global ranking")
	}
	if req.Kind == "detail" && result.Group == nil {
		return result, ranking.ErrNotFound
	}
	if len(result.Rows) > req.Limit {
		result.More = true
		result.Rows = result.Rows[:req.Limit]
	}
	return result, nil
}
