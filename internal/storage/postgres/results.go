package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"slices"
)

var _ ranking.Repository = (*Store)(nil)

// RecordCompletedGame is synchronous. A successful return means COMMIT succeeded.
// Retrying after an indeterminate commit uses the same GameID and payload hash.
func (s *Store) RecordCompletedGame(ctx context.Context, result ranking.Result) (ranking.Commit, error) {
	r := result.Clone()
	if err := r.Validate(); err != nil {
		return ranking.Commit{}, err
	}
	hash, err := r.Hash()
	if err != nil {
		return ranking.Commit{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ranking.Commit{}, operationError(ctx, "begin result")
	}
	defer rollback(tx)
	// Serialize results for one group and forbid incompatible stats silently mixing.
	var system string
	if err = tx.QueryRow(ctx, `SELECT ranking_system FROM group_configs WHERE chat_id=$1 FOR UPDATE`, r.ChatID).Scan(&system); err != nil {
		return ranking.Commit{}, operationError(ctx, "lock result group")
	}
	var storedHash, status string
	err = tx.QueryRow(ctx, `SELECT payload_hash,scoring_status FROM completed_games WHERE game_id=$1`, r.GameID).Scan(&storedHash, &status)
	if err == nil {
		if storedHash != hash {
			return ranking.Commit{}, ranking.ErrConflict
		}
		// No writes are needed for a confirmed identical retry.
		return ranking.Commit{AlreadyPersisted: true, Scored: status == "scored"}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ranking.Commit{}, operationError(ctx, "check result idempotency")
	}
	scored := r.PolicyVersion != ""
	if scored && system != string(r.RankingSystem) {
		return ranking.Commit{}, ranking.ErrNeedsProductDecision
	}
	status = "needs_product_decision"
	var policy any
	var scoredAt any
	if scored {
		status = "scored"
		policy = r.PolicyVersion
		scoredAt = r.FinishedAt
	}
	// ON CONFLICT also covers accidental reuse of a GameID in different chats.
	tag, err := tx.Exec(ctx, `INSERT INTO completed_games(game_id,chat_id,game_mode,ranking_system,config_revision,started_at,finished_at,final_revision,finish_reason,payload_hash,participant_count,scoring_status,policy_version,scored_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(game_id) DO NOTHING`, r.GameID, r.ChatID, r.GameMode, r.RankingSystem, r.ConfigRevision, r.StartedAt, r.FinishedAt, int64(r.FinalRevision), r.FinishReason, hash, len(r.Players), status, policy, scoredAt)
	if err != nil {
		return ranking.Commit{}, operationError(ctx, "insert completed game")
	}
	if tag.RowsAffected() == 0 {
		return ranking.Commit{}, ranking.ErrConflict
	}
	slices.SortFunc(r.Players, func(a, b ranking.Player) int {
		if a.UserID < b.UserID {
			return -1
		}
		if a.UserID > b.UserID {
			return 1
		}
		return 0
	})
	for _, p := range r.Players {
		if !p.LastSeenAt.IsZero() {
			if err = observeGroupUser(ctx, tx, groups.KnownUser{ChatID: r.ChatID, UserID: p.UserID, DisplayName: p.DisplayName, Username: p.Username, LastSeenAt: p.LastSeenAt}); err != nil {
				return ranking.Commit{}, operationError(ctx, "observe result user")
			}
		}
		var position, score, username any
		if p.Position > 0 {
			position = p.Position
		}
		if scored {
			score = int64(p.Score)
		}
		if p.Username != "" {
			username = p.Username
		}
		if _, err = tx.Exec(ctx, `INSERT INTO completed_game_players(game_id,user_id,observed_name,username,final_status,position,went_out,score_units,joined_after_start,leave_count,reentry_count)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, r.GameID, p.UserID, p.DisplayName, username, p.FinalStatus, position, p.WentOut, score, p.JoinedAfterStart, p.LeaveCount, p.ReentryCount); err != nil {
			return ranking.Commit{}, operationError(ctx, "insert result player")
		}
		if !scored {
			continue
		}
		wins := 0
		if p.Position == 1 {
			wins = 1
		}
		tag, err = tx.Exec(ctx, `INSERT INTO player_group_stats(chat_id,user_id,ranking_system,score_units,completed_games,wins,display_name,last_finished_at)
 VALUES($1,$2,$3,$4,1,$5,$6,$7) ON CONFLICT(chat_id,user_id) DO UPDATE SET
 score_units=player_group_stats.score_units+EXCLUDED.score_units,completed_games=player_group_stats.completed_games+1,wins=player_group_stats.wins+EXCLUDED.wins,
 display_name=CASE WHEN EXCLUDED.last_finished_at>=player_group_stats.last_finished_at THEN EXCLUDED.display_name ELSE player_group_stats.display_name END,
 last_finished_at=GREATEST(player_group_stats.last_finished_at,EXCLUDED.last_finished_at),updated_at=now()
 WHERE player_group_stats.ranking_system=EXCLUDED.ranking_system`, r.ChatID, p.UserID, r.RankingSystem, int64(p.Score), wins, p.DisplayName, r.FinishedAt)
		if err != nil {
			return ranking.Commit{}, operationError(ctx, "update ranking")
		}
		if tag.RowsAffected() != 1 {
			return ranking.Commit{}, ranking.ErrNeedsProductDecision
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return ranking.Commit{}, operationError(ctx, "commit result")
	}
	return ranking.Commit{Scored: scored}, nil
}
