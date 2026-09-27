package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/malbs/UnoGoBot/internal/groups"
)

type executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func observeGroupUser(ctx context.Context, db executor, u groups.KnownUser) error {
	if u.ChatID == 0 || u.UserID <= 0 || u.LastSeenAt.IsZero() {
		return groups.ErrInvalid
	}
	var username any
	if u.Username != "" {
		username = u.Username
	}
	_, err := db.Exec(ctx, `INSERT INTO known_group_users(chat_id,user_id,display_name,username,last_seen_at) VALUES($1,$2,$3,$4,$5)
 ON CONFLICT(chat_id,user_id) DO UPDATE SET display_name=EXCLUDED.display_name,username=EXCLUDED.username,last_seen_at=EXCLUDED.last_seen_at
 WHERE EXCLUDED.last_seen_at>=known_group_users.last_seen_at`, u.ChatID, u.UserID, u.DisplayName, username, u.LastSeenAt)
	return err
}
func (s *Store) ObserveGroupUser(ctx context.Context, u groups.KnownUser) error {
	if err := observeGroupUser(ctx, s.pool, u); err != nil {
		return operationError(ctx, "observe group user")
	}
	return nil
}
func (s *Store) KnownGroupUsers(ctx context.Context, chatID int64) ([]groups.KnownUser, error) {
	rows, err := s.pool.Query(ctx, `SELECT chat_id,user_id,display_name,COALESCE(username,''),last_seen_at FROM known_group_users WHERE chat_id=$1 ORDER BY user_id`, chatID)
	if err != nil {
		return nil, operationError(ctx, "list known users")
	}
	defer rows.Close()
	var users []groups.KnownUser
	for rows.Next() {
		var u groups.KnownUser
		if err = rows.Scan(&u.ChatID, &u.UserID, &u.DisplayName, &u.Username, &u.LastSeenAt); err != nil {
			return nil, operationError(ctx, "read known user")
		}
		users = append(users, u)
	}
	if rows.Err() != nil {
		return nil, operationError(ctx, "list known users")
	}
	return users, nil
}
