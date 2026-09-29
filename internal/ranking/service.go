package ranking

import (
	"context"
	"errors"
	"fmt"

	"github.com/malbs/UnoGoBot/internal/groups"
)

// GroupRanking is a bounded prefix of the group's cumulative standings.
// Entries are ordered by exact Score descending, then UserID for stability only.
type GroupRanking struct {
	System  groups.RankingSystem
	Entries []Entry
	Total   int64
}

// MaxRankingEntries exceeds the number of shortest possible lines that fit in
// one Telegram message. The repository never transfers an unbounded history.
const MaxRankingEntries = 512

type ReadRepository interface {
	ListGroupRanking(context.Context, int64) (GroupRanking, error)
}

type Service struct{ Repository ReadRepository }

func (s *Service) ListGroupRanking(ctx context.Context, chatID int64) (GroupRanking, error) {
	if chatID == 0 {
		return GroupRanking{}, ErrInvalid
	}
	if s == nil || s.Repository == nil {
		return GroupRanking{}, errors.New("ranking: reading is not configured")
	}
	return s.Repository.ListGroupRanking(ctx, chatID)
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
