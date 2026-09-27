package postgres

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/groups"
)

const groupColumns = `chat_id,default_game_mode,ranking_system,installed_by_user_id,config_revision,created_at,updated_at`

type scanner interface{ Scan(...any) error }

func scanGroup(row scanner) (groups.Config, error) {
	var c groups.Config
	err := row.Scan(&c.ChatID, &c.DefaultGameMode, &c.RankingSystem, &c.InstalledByUserID, &c.Revision, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

var _ groups.Repository = (*Store)(nil)

// The no-op conflict update also returns the row under concurrent creation.
func (s *Store) GetOrCreateGroupConfig(ctx context.Context, chatID int64) (groups.Config, error) {
	if chatID == 0 {
		return groups.Config{}, groups.ErrInvalid
	}
	c, err := scanGroup(s.pool.QueryRow(ctx, `INSERT INTO group_configs(chat_id) VALUES($1)
 ON CONFLICT(chat_id) DO UPDATE SET chat_id=EXCLUDED.chat_id RETURNING `+groupColumns, chatID))
	if err != nil {
		return groups.Config{}, operationError(ctx, "get group configuration")
	}
	return c, nil
}
func (s *Store) SetDefaultGameMode(ctx context.Context, chatID int64, mode groups.Mode) (groups.Config, error) {
	if chatID == 0 || !mode.Valid() {
		return groups.Config{}, groups.ErrInvalid
	}
	c, err := scanGroup(s.pool.QueryRow(ctx, `INSERT INTO group_configs(chat_id,default_game_mode) VALUES($1,$2)
 ON CONFLICT(chat_id) DO UPDATE SET default_game_mode=EXCLUDED.default_game_mode,
 config_revision=group_configs.config_revision+CASE WHEN group_configs.default_game_mode<>EXCLUDED.default_game_mode THEN 1 ELSE 0 END,
 updated_at=CASE WHEN group_configs.default_game_mode<>EXCLUDED.default_game_mode THEN now() ELSE group_configs.updated_at END RETURNING `+groupColumns, chatID, mode))
	if err != nil {
		return groups.Config{}, operationError(ctx, "set default game mode")
	}
	return c, nil
}
