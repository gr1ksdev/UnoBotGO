package ranking

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
)

func finalFixture(n, abandoned int) Result {
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	r := Result{GameID: "final", ChatID: 42, GameMode: groups.Classic, RankingSystem: groups.Updated, ConfigRevision: 1, StartedAt: now.Add(-time.Minute), FinishedAt: now, FinalRevision: 100, FinishReason: "completed"}
	for i := 1; i <= n; i++ {
		status := "went_out"
		if i == n {
			status = "playing"
		}
		r.Players = append(r.Players, Player{UserID: int64(i), DisplayName: fmt.Sprint(i), Position: i, FinalStatus: status, WentOut: i < n})
	}
	for i := 1; i <= abandoned; i++ {
		r.Players = append(r.Players, Player{UserID: int64(n + i), FinalStatus: "left", LeaveCount: 1})
	}
	return r
}

func TestEligibleScores(t *testing.T) {
	for _, system := range []groups.RankingSystem{groups.Legacy, groups.Updated} {
		for _, tt := range []struct {
			name         string
			n, abandoned int
			updated      []Units
		}{
			{"normal_two", 2, 0, []Units{1000, 0}},
			{"normal_three", 3, 0, []Units{1000, 500, 0}},
			{"normal_eight", 8, 0, []Units{1000, 857, 714, 571, 429, 286, 143, 0}},
			{"eight_one_abandonment", 7, 1, []Units{1000, 833, 667, 500, 333, 167, 0}},
			{"eight_two_abandonments", 6, 2, []Units{1000, 800, 600, 400, 200, 0}},
		} {
			t.Run(string(system)+"/"+tt.name, func(t *testing.T) {
				input := finalFixture(tt.n, tt.abandoned)
				input.RankingSystem = system
				before := input.Clone()
				got, err := Prepare(input)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(input, before) {
					t.Fatal("input was mutated")
				}
				if got.EligibleCount() != tt.n || len(got.Players) != tt.n+tt.abandoned || got.ScoringStatus() != StatusScored {
					t.Fatal("wrong eligible/result size", got)
				}
				for i, p := range got.Players {
					want := Units(0)
					if i < tt.n {
						want = tt.updated[i]
						if system == groups.Legacy {
							want = 0
							if i < tt.n-1 {
								want = 100
							}
						}
					}
					if p.Score != want {
						t.Errorf("user=%d score=%d want=%d", p.UserID, p.Score, want)
					}
					if i >= tt.n && (p.Eligible() || p.Position != 0) {
						t.Fatal("abandonment given placement")
					}
				}
				retry, err := Prepare(got)
				if err != nil || !reflect.DeepEqual(retry, got) {
					t.Fatal("preparation is not idempotent", err)
				}
			})
		}
	}
}

func TestParticipationHistoryDoesNotChangeEligibility(t *testing.T) {
	for _, tt := range []struct {
		name              string
		late              bool
		leaves, reentries int
		placed            bool
	}{
		{"late_join_concludes", true, 0, 0, true},
		{"left_reentered_concludes", true, 1, 1, true},
		{"left_reentered_left", true, 2, 1, false},
		{"lobby_only", false, 1, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := finalFixture(3, 0)
			index := 1
			if !tt.placed {
				r = finalFixture(3, 1)
				index = 3
			}
			p := &r.Players[index]
			p.JoinedAfterStart = tt.late
			p.LeaveCount = tt.leaves
			p.ReentryCount = tt.reentries
			out, err := Prepare(r)
			if err != nil {
				t.Fatal(err)
			}
			got := out.Players[index]
			want := Units(0)
			if tt.placed {
				want = 500
			}
			if got.Eligible() != tt.placed || got.Score != want || out.EligibleCount() != 3 {
				t.Fatal("history changed eligibility", out)
			}
		})
	}
}

func TestDepartureAndInsufficientEligiblePlayers(t *testing.T) {
	for _, system := range []groups.RankingSystem{groups.Legacy, groups.Updated} {
		for n := 0; n <= 3; n++ {
			t.Run(fmt.Sprintf("%s/N%d", system, n), func(t *testing.T) {
				r := finalFixture(n, 2)
				r.RankingSystem = system
				r.FinishReason = "departure"
				got, err := Prepare(r)
				if err != nil {
					t.Fatal(err)
				}
				if n < 2 {
					if got.ScoringStatus() != StatusInsufficientPlayers {
						t.Fatal("insufficient result not explicit")
					}
					for _, p := range got.Players {
						if p.Score != 0 {
							t.Fatal("awarded with N<2")
						}
					}
				} else if got.ScoringStatus() != StatusScored {
					t.Fatal("eligible departure rejected")
				}
			})
		}
	}
}

func TestInvalidFinalResultsCannotBeScored(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Result)
	}{
		{"cancelled", func(r *Result) { r.FinishReason = "cancelled" }},
		{"unfinished", func(r *Result) { r.FinishReason = "" }},
		{"duplicate_user", func(r *Result) { r.Players[1].UserID = r.Players[0].UserID }},
		{"duplicate_placement", func(r *Result) { r.Players[1].Position = 1 }},
		{"placement_gap", func(r *Result) { r.Players[2].Position = 4 }},
		{"left_with_placement", func(r *Result) { r.Players[1].FinalStatus = "left"; r.Players[1].WentOut = false }},
		{"invented_went_out", func(r *Result) { r.Players[3].WentOut = true }},
		{"invalid_survivor", func(r *Result) { r.Players[0].FinalStatus = "playing"; r.Players[0].WentOut = false }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := finalFixture(3, 1)
			tt.change(&r)
			if _, err := Prepare(r); !errors.Is(err, ErrInvalid) {
				t.Fatal("invalid result accepted", err)
			}
		})
	}
	r, _ := Prepare(finalFixture(3, 1))
	r.Players[3].Score = 100
	if _, err := Prepare(r); !errors.Is(err, ErrInvalid) {
		t.Fatal("divergent prepared award silently corrected")
	}
}
