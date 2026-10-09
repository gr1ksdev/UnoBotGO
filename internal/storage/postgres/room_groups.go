package postgres

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/groups"
)

// Candidates only: current platform membership is checked before disclosure/use.
func (s *Store) WebAppGroups(ctx context.Context, user int64) ([]groups.Config, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT c.chat_id,c.title,c.ranking_system,c.config_revision FROM group_configs c LEFT JOIN known_group_users u USING(chat_id) WHERE c.chat_id<0 AND (u.user_id=$1 OR c.installed_by_user_id=$1) ORDER BY c.chat_id LIMIT 50`, user)
	if err != nil {
		return nil, operationError(ctx, "room groups")
	}
	defer rows.Close()
	out := []groups.Config{}
	for rows.Next() {
		var c groups.Config
		if err = rows.Scan(&c.ChatID, &c.Title, &c.RankingSystem, &c.Revision); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
