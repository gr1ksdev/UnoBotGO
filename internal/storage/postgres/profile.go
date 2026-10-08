package postgres

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

func (s *Store) ReadProfile(ctx context.Context, user int64, before *ranking.HistoryKey) (ranking.Profile, error) {
	out := ranking.Profile{Stats: []ranking.ProfileStats{}, History: []ranking.HistoryEntry{}}
	rows, err := s.pool.Query(ctx, `SELECT ranking_system,sum(score_units)::text,sum(completed_games)::bigint,sum(wins)::bigint FROM player_group_stats WHERE user_id=$1 GROUP BY ranking_system ORDER BY ranking_system`, user)
	if err != nil {
		return out, operationError(ctx, "profile stats")
	}
	for rows.Next() {
		var v ranking.ProfileStats
		if err := rows.Scan(&v.System, &v.Score, &v.Games, &v.Wins); err != nil {
			rows.Close()
			return out, err
		}
		out.Stats = append(out.Stats, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	var finished any
	var gameID string
	if before != nil {
		finished = before.Finished
		gameID = before.ID
	}
	rows, err = s.pool.Query(ctx, `SELECT g.game_id,CASE WHEN COALESCE(c.ranking_private,false) THEN 'Grupo anônimo' ELSE COALESCE(NULLIF(c.title,''),'Grupo') END,g.origin,g.ranking_system,COALESCE(p.position,0),CASE WHEN g.scoring_status='scored' AND p.position IS NOT NULL AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out)) THEN p.score_units::text END,g.scoring_status,g.finished_at FROM completed_games g JOIN completed_game_players p USING(game_id) LEFT JOIN group_configs c USING(chat_id) WHERE p.user_id=$1 AND ($2::timestamptz IS NULL OR (g.finished_at,g.game_id)<($2::timestamptz,$3::text)) ORDER BY g.finished_at DESC,g.game_id DESC LIMIT 21`, user, finished, gameID)
	if err != nil {
		return out, operationError(ctx, "profile history")
	}
	defer rows.Close()
	for rows.Next() {
		var v ranking.HistoryEntry
		if err := rows.Scan(&v.ID, &v.Group, &v.Origin, &v.System, &v.Position, &v.Score, &v.Status, &v.Finished); err != nil {
			return out, err
		}
		out.History = append(out.History, v)
	}
	return out, rows.Err()
}
func (s *Store) ReadResult(ctx context.Context, id string, user int64) ([]ranking.HistoryEntry, error) {
	out := []ranking.HistoryEntry{}
	rows, err := s.pool.Query(ctx, `SELECT g.game_id,g.origin,g.ranking_system,COALESCE(p.position,0),CASE WHEN g.scoring_status='scored' AND p.position IS NOT NULL AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out)) THEN p.score_units::text END,g.scoring_status,g.finished_at FROM completed_games g JOIN completed_game_players p USING(game_id) WHERE g.game_id=$1 AND p.user_id=$2`, id, user)
	if err != nil {
		return nil, operationError(ctx, "result status")
	}
	defer rows.Close()
	for rows.Next() {
		var v ranking.HistoryEntry
		if err := rows.Scan(&v.ID, &v.Origin, &v.System, &v.Position, &v.Score, &v.Status, &v.Finished); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}
	awards, err := s.pool.Query(ctx, `SELECT p.user_id,COALESCE(p.position,0),CASE WHEN g.scoring_status='scored' AND p.position IS NOT NULL AND ((p.final_status='went_out' AND p.went_out) OR (p.final_status='playing' AND NOT p.went_out)) THEN p.score_units::text END FROM completed_game_players p JOIN completed_games g USING(game_id) WHERE g.game_id=$1 ORDER BY p.position NULLS LAST,p.user_id`, id)
	if err != nil {
		return nil, operationError(ctx, "result awards")
	}
	defer awards.Close()
	for awards.Next() {
		var award ranking.ResultAward
		if err := awards.Scan(&award.UserID, &award.Position, &award.Score); err != nil {
			return nil, err
		}
		out[0].Awards = append(out[0].Awards, award)
	}
	return out, awards.Err()
}
