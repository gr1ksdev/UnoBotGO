package ranking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
)

// GroupRanking is a bounded prefix of the group's cumulative standings.
// Entries are ordered by Score DESC, latest eligible placement ASC,
// latest eligible completion DESC, then UserID ASC for technical stability.
type GroupRanking struct {
	System     groups.RankingSystem
	MonthName  string
	MonthStart time.Time
	Entries    []Entry
	Total      int64
}

// UserGroupRankingEntry represents a user's monthly standing in a specific group.
type UserGroupRankingEntry struct {
	ChatID         int64
	GroupName      string
	RankingSystem  groups.RankingSystem
	ScoreUnits     Units
	LastFinishedAt time.Time
}

// UserMonthlyRankingSection contains the group entries and total for a specific system.
type UserMonthlyRankingSection struct {
	System     groups.RankingSystem
	Entries    []UserGroupRankingEntry
	TotalScore Units
}

// UserMonthlyRankings contains the user's monthly rankings across all groups.
type UserMonthlyRankings struct {
	UserID     int64
	MonthName  string
	MonthStart time.Time
	Updated    *UserMonthlyRankingSection
	Legacy     *UserMonthlyRankingSection
}

// MaxRankingEntries exceeds the number of shortest possible lines that fit in
// one Telegram message. The repository never transfers an unbounded history.
const MaxRankingEntries = 512

type ReadRepository interface {
	ListGroupRanking(ctx context.Context, chatID int64, at time.Time) (GroupRanking, error)
	ListUserMonthlyRankings(ctx context.Context, userID int64, at time.Time) (UserMonthlyRankings, error)
}

type Service struct {
	Repository ReadRepository
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) ListGroupRanking(ctx context.Context, chatID int64) (GroupRanking, error) {
	if chatID == 0 {
		return GroupRanking{}, ErrInvalid
	}
	if s == nil || s.Repository == nil {
		return GroupRanking{}, errors.New("ranking: reading is not configured")
	}
	return s.Repository.ListGroupRanking(ctx, chatID, s.now())
}

func (s *Service) ListGroupRankingAt(ctx context.Context, chatID int64, at time.Time) (GroupRanking, error) {
	if chatID == 0 {
		return GroupRanking{}, ErrInvalid
	}
	if s == nil || s.Repository == nil {
		return GroupRanking{}, errors.New("ranking: reading is not configured")
	}
	if at.IsZero() {
		at = s.now()
	}
	return s.Repository.ListGroupRanking(ctx, chatID, at)
}

func (s *Service) ListUserMonthlyRankings(ctx context.Context, userID int64) (UserMonthlyRankings, error) {
	if userID <= 0 {
		return UserMonthlyRankings{}, ErrInvalid
	}
	if s == nil || s.Repository == nil {
		return UserMonthlyRankings{}, errors.New("ranking: reading is not configured")
	}
	return s.Repository.ListUserMonthlyRankings(ctx, userID, s.now())
}

func (s *Service) ListUserMonthlyRankingsAt(ctx context.Context, userID int64, at time.Time) (UserMonthlyRankings, error) {
	if userID <= 0 {
		return UserMonthlyRankings{}, ErrInvalid
	}
	if s == nil || s.Repository == nil {
		return UserMonthlyRankings{}, errors.New("ranking: reading is not configured")
	}
	if at.IsZero() {
		at = s.now()
	}
	return s.Repository.ListUserMonthlyRankings(ctx, userID, at)
}

// FormatScore formats stored nonnegative hundredths without recalculation or floats.
func FormatScore(system groups.RankingSystem, score Units) string {
	if system == groups.Legacy {
		if score == 100 {
			return "1 pt"
		}
		return fmt.Sprintf("%d pts", score/100)
	}
	return fmt.Sprintf("%d,%02d pts", score/100, score%100)
}
