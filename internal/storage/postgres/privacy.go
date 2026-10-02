package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/malbs/UnoGoBot/internal/groups"
)

func (s *Store) GetUserRankingPrivacy(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, groups.ErrInvalid
	}
	var private bool
	err := s.pool.QueryRow(ctx, `SELECT ranking_private FROM user_privacy_settings WHERE user_id=$1`, userID).Scan(&private)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, operationError(ctx, "get user ranking privacy")
	}
	return private, nil
}

func (s *Store) SetUserRankingPrivacy(ctx context.Context, userID int64, private bool) error {
	if userID <= 0 {
		return groups.ErrInvalid
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO user_privacy_settings(user_id, ranking_private)
VALUES($1, $2)
ON CONFLICT(user_id) DO UPDATE SET ranking_private=EXCLUDED.ranking_private`, userID, private)
	if err != nil {
		return operationError(ctx, "set user ranking privacy")
	}
	return nil
}

func (s *Store) ToggleUserRankingPrivacy(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, groups.ErrInvalid
	}
	var newPrivate bool
	err := s.pool.QueryRow(ctx, `INSERT INTO user_privacy_settings(user_id, ranking_private)
VALUES($1, true)
ON CONFLICT(user_id) DO UPDATE SET ranking_private=NOT user_privacy_settings.ranking_private
RETURNING ranking_private`, userID).Scan(&newPrivate)
	if err != nil {
		return false, operationError(ctx, "toggle user ranking privacy")
	}
	return newPrivate, nil
}

func (s *Store) IsEntityAnonymous(ctx context.Context, kind string, id int64) (bool, error) {
	if kind == "group" {
		var private bool
		err := s.pool.QueryRow(ctx, `SELECT ranking_private FROM group_configs WHERE chat_id=$1`, id).Scan(&private)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, operationError(ctx, "check group privacy")
		}
		return private, nil
	}
	if kind == "user" {
		return s.GetUserRankingPrivacy(ctx, id)
	}
	return false, nil
}
