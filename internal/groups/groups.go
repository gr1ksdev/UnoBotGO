// Package groups defines persistent group configuration without a SQL dependency.
package groups

import (
	"context"
	"errors"
	"time"
)

type Mode string
type RankingSystem string

const (
	Classic Mode          = "classic"
	Caseiro Mode          = "caseiro"
	Legacy  RankingSystem = "legacy"
	Updated RankingSystem = "updated"
)

func (m Mode) Valid() bool          { return m == Classic || m == Caseiro }
func (s RankingSystem) Valid() bool { return s == Legacy || s == Updated }

type Config struct {
	ChatID               int64
	DefaultGameMode      Mode
	RankingSystem        RankingSystem
	InstalledByUserID    *int64
	Revision             int64
	Title                string
	CreatedAt, UpdatedAt time.Time
}

func Defaults(chatID int64) Config {
	return Config{ChatID: chatID, DefaultGameMode: Classic, RankingSystem: Updated, Revision: 1}
}

// Snapshot is a value, frozen at creation; mode remains the lobby's own rules.
type Snapshot struct {
	RankingSystem  RankingSystem
	ConfigRevision int64
}

func (c Config) Snapshot() Snapshot { return Snapshot{c.RankingSystem, c.Revision} }

type Repository interface {
	GetOrCreateGroupConfig(context.Context, int64) (Config, error)
	SetDefaultGameMode(context.Context, int64, Mode) (Config, error)
	SetRankingSystem(context.Context, int64, RankingSystem) (Config, error)
	SetInstalledBy(context.Context, int64, int64) (Config, error)
	ObserveGroupTitle(context.Context, int64, string) error
}

var (
	ErrForbidden            = errors.New("groups: configuration permission denied")
	ErrInvalid              = errors.New("groups: invalid configuration")
	ErrNeedsProductDecision = errors.New("groups: cannot switch ranking system with existing accumulated scores")
)

// Membership must be asserted by a trusted platform adapter at action time.
type Membership struct{ Admin, Member bool }

func CanConfigure(config Config, userID int64, role Membership) bool {
	return userID > 0 && (role.Admin || (config.InstalledByUserID != nil && *config.InstalledByUserID == userID && role.Member))
}

// Service validates authorization on every action, independent of buttons/UI.
type Service struct {
	Repository       Repository
	LookupMembership func(context.Context, int64, int64) (Membership, error)
}

func (s Service) CanConfigureUser(ctx context.Context, chatID, userID int64) (Config, bool, error) {
	if chatID == 0 || userID <= 0 {
		return Config{}, false, ErrInvalid
	}
	c, err := s.Repository.GetOrCreateGroupConfig(ctx, chatID)
	if err != nil {
		return Config{}, false, err
	}
	role, err := s.LookupMembership(ctx, chatID, userID)
	if err != nil {
		return Config{}, false, ErrForbidden
	}
	return c, CanConfigure(c, userID, role), nil
}

func (s Service) SetDefaultGameMode(ctx context.Context, chatID, userID int64, mode Mode) (Config, error) {
	if chatID == 0 || userID <= 0 || !mode.Valid() {
		return Config{}, ErrInvalid
	}
	c, err := s.Repository.GetOrCreateGroupConfig(ctx, chatID)
	if err != nil {
		return Config{}, err
	}
	role, err := s.LookupMembership(ctx, chatID, userID)
	if err != nil {
		return Config{}, ErrForbidden
	}
	if !CanConfigure(c, userID, role) {
		return Config{}, ErrForbidden
	}
	return s.Repository.SetDefaultGameMode(ctx, chatID, mode)
}

func (s Service) SetRankingSystem(ctx context.Context, chatID, userID int64, system RankingSystem) (Config, error) {
	if chatID == 0 || userID <= 0 || !system.Valid() {
		return Config{}, ErrInvalid
	}
	c, err := s.Repository.GetOrCreateGroupConfig(ctx, chatID)
	if err != nil {
		return Config{}, err
	}
	role, err := s.LookupMembership(ctx, chatID, userID)
	if err != nil {
		return Config{}, ErrForbidden
	}
	if !CanConfigure(c, userID, role) {
		return Config{}, ErrForbidden
	}
	return s.Repository.SetRankingSystem(ctx, chatID, system)
}

func (s Service) RecordInstallation(ctx context.Context, chatID, installerID int64) (Config, error) {
	if chatID == 0 || installerID <= 0 {
		return Config{}, ErrInvalid
	}
	return s.Repository.SetInstalledBy(ctx, chatID, installerID)
}

func (s Service) ObserveGroupTitle(ctx context.Context, chatID int64, title string) error {
	if chatID == 0 || title == "" || s.Repository == nil {
		return nil
	}
	return s.Repository.ObserveGroupTitle(ctx, chatID, title)
}
