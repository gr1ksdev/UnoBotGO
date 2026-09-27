// Package ranking contains deterministic scoring and public final-result DTOs.
// It imports neither PostgreSQL nor the UNO engine.
package ranking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
)

type Units int64 // 100 units = one point; never a binary float.
var ErrInvalid = errors.New("ranking: invalid completed result")
var ErrConflict = errors.New("ranking: GameID already contains a different result")
var ErrNeedsProductDecision = errors.New("ranking: scoring policy requires product decision")

// Score uses the specified formula, rounded to hundredths, half up.
func Score(system groups.RankingSystem, n, position int) (Units, error) {
	if !system.Valid() || n < 2 || n > 1000000 || position < 1 || position > n {
		return 0, ErrInvalid
	}
	if system == groups.Legacy {
		if position < n {
			return 100, nil
		}
		return 0, nil
	}
	numerator := int64(1000) * int64(n-position)
	denominator := int64(n - 1)
	q, r := numerator/denominator, numerator%denominator
	if r*2 >= denominator {
		q++
	}
	return Units(q), nil
}

// FormatPoints presents one decimal with comma, rounding stored hundredths half up.
func FormatPoints(u Units) string {
	n := int64(u)
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	tenths := (n + 5) / 10
	return fmt.Sprintf("%s%d,%d", sign, tenths/10, tenths%10)
}

type Player struct {
	LastSeenAt               time.Time
	UserID                   int64
	DisplayName              string
	Username                 string
	Position                 int // zero means no placement; never invent a placement for a leaver.
	FinalStatus              string
	WentOut                  bool
	JoinedAfterStart         bool
	LeaveCount, ReentryCount int
	Score                    Units
}

// Result contains result/audit metadata only, with no cards or private snapshots.
// Finalization callers must Clone at boundaries and treat the value as immutable.
type Result struct {
	GameID                string
	ChatID                int64
	GameMode              groups.Mode
	RankingSystem         groups.RankingSystem
	ConfigRevision        int64
	StartedAt, FinishedAt time.Time
	FinalRevision         uint64
	FinishReason          string
	Players               []Player
	PolicyVersion         string // empty => unscored, awaiting a product decision
}

func (r Result) Clone() Result { r.Players = slices.Clone(r.Players); return r }
func (r Result) Hash() (string, error) {
	r = r.Clone()
	slices.SortFunc(r.Players, func(a, b Player) int {
		if a.UserID < b.UserID {
			return -1
		}
		if a.UserID > b.UserID {
			return 1
		}
		return 0
	})
	// PostgreSQL timestamps have microsecond precision.
	r.StartedAt = r.StartedAt.UTC().Truncate(time.Microsecond)
	r.FinishedAt = r.FinishedAt.UTC().Truncate(time.Microsecond)
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// Validate checks public result consistency and, when evaluated, every award.
// Placements are engine facts: gaps, duplicates or placements on Left players
// are errors, never repaired by inventing/reordering ranking positions.
func (r Result) Validate() error {
	if r.GameID == "" || r.ChatID == 0 || !r.GameMode.Valid() || !r.RankingSystem.Valid() || r.ConfigRevision <= 0 || r.StartedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) || r.FinalRevision == 0 || r.FinalRevision > 1<<63-1 || len(r.Players) == 0 {
		return ErrInvalid
	}
	if r.FinishReason != "completed" && r.FinishReason != "departure" {
		return ErrInvalid
	}
	if r.PolicyVersion != "" && r.PolicyVersion != PlacementPolicyV1 {
		return ErrNeedsProductDecision
	}
	n := r.EligibleCount()
	ids := map[int64]bool{}
	positions := map[int]bool{}
	for _, p := range r.Players {
		if p.UserID <= 0 || ids[p.UserID] || p.Position < 0 || p.Position > n || p.LeaveCount < 0 || p.ReentryCount < 0 {
			return ErrInvalid
		}
		ids[p.UserID] = true
		if p.FinalStatus != "playing" && p.FinalStatus != "went_out" && p.FinalStatus != "left" {
			return ErrInvalid
		}
		if p.FinalStatus == "went_out" && (p.Position == 0 || !p.WentOut) {
			return ErrInvalid
		}
		if p.WentOut && (p.Position == 0 || p.FinalStatus != "went_out") {
			return ErrInvalid
		}
		if p.Position > 0 {
			if !p.Eligible() || positions[p.Position] {
				return ErrInvalid
			}
			if !p.WentOut && p.Position != n {
				return ErrInvalid
			}
			positions[p.Position] = true
		}
		expected := Units(0)
		if r.PolicyVersion != "" && p.Eligible() && n >= 2 {
			var err error
			expected, err = Score(r.RankingSystem, n, p.Position)
			if err != nil {
				return err
			}
		}
		if p.Score != expected {
			return ErrInvalid
		}
	}
	return nil
}

type Commit struct {
	AlreadyPersisted bool
	Scored           bool
}
type Repository interface {
	RecordCompletedGame(context.Context, Result) (Commit, error)
}
type Entry struct {
	UserID         int64
	DisplayName    string
	Score          Units
	CompletedGames int64
	Wins           int64
}
