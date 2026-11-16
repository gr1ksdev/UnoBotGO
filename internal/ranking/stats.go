package ranking

import (
	"context"
	"fmt"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
)

// PlayerStats summarises one player's history. A zero ChatID aggregates the
// player across every group.
type PlayerStats struct {
	System       groups.RankingSystem
	Games        int64
	Wins         int64
	BestPosition int
	AvgPosition  string
	Score        Units
	Rank         int
	Total        int
}

// HasHistory reports whether the player has any scored, eligible participation.
func (p PlayerStats) HasHistory() bool { return p.Games > 0 }

// PlayerStatsRepository reads the accumulated history of a single player.
type PlayerStatsRepository interface {
	ReadPlayerStats(context.Context, int64, int64, time.Time) (PlayerStats, error)
}

// ReadPlayerStats returns the history of a player in a group, or across every
// group when chatID is zero.
func (s *Service) ReadPlayerStats(ctx context.Context, chatID, userID int64) (PlayerStats, error) {
	if s.Repository == nil {
		return PlayerStats{}, fmt.Errorf("ranking: reading is not configured")
	}
	if userID <= 0 {
		return PlayerStats{}, fmt.Errorf("ranking: invalid user")
	}
	return s.Repository.ReadPlayerStats(ctx, chatID, userID, s.now())
}
