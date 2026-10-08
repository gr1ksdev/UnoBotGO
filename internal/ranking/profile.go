package ranking

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/groups"
	"time"
)

type ProfileStats struct {
	System groups.RankingSystem `json:"system"`
	Score  string               `json:"score_units"`
	Games  int64                `json:"games"`
	Wins   int64                `json:"wins"`
}
type ResultAward struct {
	UserID   int64
	Score    *string
	Position int
}
type HistoryEntry struct {
	Awards   []ResultAward        `json:"-"`
	ID       string               `json:"id"`
	Group    string               `json:"group"`
	Origin   string               `json:"origin"`
	System   groups.RankingSystem `json:"system"`
	Position int                  `json:"position"`
	Score    *string              `json:"score_units"`
	Status   string               `json:"status"`
	Finished time.Time            `json:"finished_at"`
}
type Profile struct {
	Stats   []ProfileStats `json:"stats"`
	History []HistoryEntry `json:"history"`
	Next    string         `json:"next_cursor,omitempty"`
}
type HistoryKey struct {
	Finished time.Time
	ID       string
}
type ProfileRepository interface {
	ReadProfile(context.Context, int64, *HistoryKey) (Profile, error)
	ReadResult(context.Context, string, int64) ([]HistoryEntry, error)
}
