package groups

import (
	"context"
	"time"
)

// KnownUser identity is UserID; observed names are mutable metadata, never keys.
type KnownUser struct {
	ChatID, UserID int64
	DisplayName    string
	Username       string
	LastSeenAt     time.Time
}
type UserRepository interface {
	ObserveGroupUser(context.Context, KnownUser) error
	KnownGroupUsers(context.Context, int64) ([]KnownUser, error)
}
