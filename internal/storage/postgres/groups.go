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

func (s *Store) SetRankingSystem(ctx context.Context, chatID int64, system groups.RankingSystem) (groups.Config, error) {
	if chatID == 0 || !system.Valid() {
		return groups.Config{}, groups.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return groups.Config{}, operationError(ctx, "begin set ranking system")
	}
	defer rollback(tx)

	// Ensure group config exists and lock it.
	var currentSystem string
	err = tx.QueryRow(ctx, `INSERT INTO group_configs(chat_id) VALUES($1)
 ON CONFLICT(chat_id) DO UPDATE SET chat_id=EXCLUDED.chat_id RETURNING ranking_system`, chatID).Scan(&currentSystem)
	if err != nil {
		return groups.Config{}, operationError(ctx, "lock group config")
	}

	if currentSystem == string(system) {
		var c groups.Config
		c, err = scanGroup(tx.QueryRow(ctx, `SELECT `+groupColumns+` FROM group_configs WHERE chat_id=$1`, chatID))
		if err != nil {
			return groups.Config{}, operationError(ctx, "read current group config")
		}
		_ = tx.Commit(ctx)
		return c, nil
	}

	// Forbid changing ranking system if existing scores/stats would be incompatible.
	var hasConflict bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM player_group_stats WHERE chat_id=$1 AND ranking_system <> $2
		UNION ALL
		SELECT 1 FROM completed_games WHERE chat_id=$1 AND scoring_status='scored' AND ranking_system <> $2
	)`, chatID, string(system)).Scan(&hasConflict)
	if err != nil {
		return groups.Config{}, operationError(ctx, "check ranking system compatibility")
	}
	if hasConflict {
		return groups.Config{}, groups.ErrNeedsProductDecision
	}

	c, err := scanGroup(tx.QueryRow(ctx, `UPDATE group_configs SET
 ranking_system=$2,
 config_revision=config_revision+1,
 updated_at=now()
 WHERE chat_id=$1 RETURNING `+groupColumns, chatID, string(system)))
	if err != nil {
		return groups.Config{}, operationError(ctx, "update ranking system")
	}

	if err = tx.Commit(ctx); err != nil {
		return groups.Config{}, operationError(ctx, "commit set ranking system")
	}
	return c, nil
}

func (s *Store) SetInstalledBy(ctx context.Context, chatID int64, installerID int64) (groups.Config, error) {
	if chatID == 0 || installerID <= 0 {
		return groups.Config{}, groups.ErrInvalid
	}
	c, err := scanGroup(s.pool.QueryRow(ctx, `INSERT INTO group_configs(chat_id,installed_by_user_id,installed_at) VALUES($1,$2,now())
 ON CONFLICT(chat_id) DO UPDATE SET installed_by_user_id=EXCLUDED.installed_by_user_id,
 installed_at=now(),
 updated_at=now() RETURNING `+groupColumns, chatID, installerID))
	if err != nil {
		return groups.Config{}, operationError(ctx, "set installed by user")
	}
	return c, nil
}
