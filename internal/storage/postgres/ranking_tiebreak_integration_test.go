//go:build integration

package postgres

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

func rankingIDs(t *testing.T, s *Store, selected map[int64]bool) []int64 {
	t.Helper()
	got, err := s.ListGroupRanking(t.Context(), 42)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, p := range got.Entries {
		if selected == nil || selected[p.UserID] {
			ids = append(ids, p.UserID)
		}
	}
	return ids
}

// Legacy awards the same score to all non-last placements, allowing real
// persisted games to isolate every tie-breaker without changing score_units.
func TestGroupRankingLastEligibleTieBreakers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		positions  []int
		minutes    []int
		extraFirst bool
		want       []int64
	}{
		{"score_before_placement", []int{5, 1}, []int{1, 2}, true, []int64{1, 2}},
		{"first_before_second", []int{2, 1}, []int{2, 1}, false, []int64{2, 1}},
		{"second_before_fifth", []int{5, 2}, []int{2, 1}, false, []int64{2, 1}},
		{"three_placements", []int{3, 1, 2}, []int{3, 2, 1}, false, []int64{2, 3, 1}},
		{"latest_completion", []int{1, 1}, []int{1, 2}, false, []int64{2, 1}},
		{"stable_user_id", []int{1, 1}, []int{1, 1}, false, []int64{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := prepareRanking(t, groups.Legacy)
			base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
			selected := map[int64]bool{}
			for i, pos := range tc.positions {
				id := int64(i + 1)
				selected[id] = true
				r := eligibleResult(t, groups.Legacy, 8, 0, "completed")
				r.GameID = fmt.Sprintf("personal-%d", id)
				r.StartedAt = base.Add(-time.Hour)
				r.FinishedAt = base.Add(time.Duration(tc.minutes[i]) * time.Minute)
				for j := range r.Players {
					r.Players[j].UserID = int64(100 + i*10 + j)
				}
				r.Players[pos-1].UserID = id
				if _, err := s.RecordCompletedGame(t.Context(), r); err != nil {
					t.Fatal(err)
				}
				if tc.extraFirst && i == 0 {
					r.GameID = "extra"
					if _, err := s.RecordCompletedGame(t.Context(), r); err != nil {
						t.Fatal(err)
					}
				}
			}
			if got := rankingIDs(t, s, selected); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestUpdatedRankingChangesAfterPersonalGames(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	var last ranking.Result
	for i := 0; i < 4; i++ {
		r := eligibleResult(t, groups.Updated, 3, 0, "completed")
		r.GameID = fmt.Sprintf("shared-%d", i)
		r.StartedAt = base.Add(-time.Hour)
		r.FinishedAt = base.Add(time.Duration(i) * time.Minute)
		r.Players[0].DisplayName = "Freddy"
		r.Players[1].DisplayName = "Mezi"
		r.Players[2].DisplayName = "Jeesttin"
		if i%2 == 1 {
			r.Players[0].UserID, r.Players[1].UserID = 2, 1
			r.Players[0].DisplayName, r.Players[1].DisplayName = "Mezi", "Freddy"
		}
		if _, err := s.RecordCompletedGame(ctx, r); err != nil {
			t.Fatal(err)
		}
		last = r
	}
	got, err := s.ListGroupRanking(ctx, 42)
	if err != nil || !reflect.DeepEqual(rankingIDs(t, s, nil), []int64{2, 1, 3}) || got.Entries[0].Score != 3000 || got.Entries[1].Score != 3000 || got.Entries[2].Score != 0 {
		t.Fatal(got, err)
	}
	if commit, err := s.RecordCompletedGame(ctx, last); err != nil || !commit.AlreadyPersisted {
		t.Fatal(commit, err)
	}
	if after, err := s.ListGroupRanking(ctx, 42); err != nil || !reflect.DeepEqual(got, after) {
		t.Fatal("duplicate changed ranking", after, err)
	}
	// Different personal games: Mezi wins first, then Freddy. Both now have
	// 4000 and last placement 1, so Freddy's newer completion moves him above.
	for i, id := range []int64{2, 1} {
		r := eligibleResult(t, groups.Updated, 2, 0, "completed")
		r.GameID = fmt.Sprintf("personal-%d", id)
		r.StartedAt = base.Add(-time.Hour)
		r.FinishedAt = base.Add(time.Duration(10+i) * time.Minute)
		r.Players[0].UserID = id
		r.Players[1].UserID = 3
		if _, err := s.RecordCompletedGame(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if got := rankingIDs(t, s, nil); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Fatal(got)
	}
	// Freddy abandons a later scored game played by other historical users.
	// His personal reference must remain the previous win.
	r := eligibleResult(t, groups.Updated, 2, 1, "departure")
	r.GameID = "abandonment"
	r.StartedAt = base.Add(-time.Hour)
	r.FinishedAt = base.Add(20 * time.Minute)
	r.Players[0].UserID = 90
	r.Players[1].UserID = 91
	r.Players[2].UserID = 1
	before, err := s.ListGroupRanking(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCompletedGame(ctx, r); err != nil {
		t.Fatal(err)
	}
	if got := rankingIDs(t, s, map[int64]bool{1: true, 2: true, 3: true}); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Fatal("abandonment changed reference", got)
	}
	// Deferred COMMIT failure cannot replace Freddy's last eligible placement.
	if _, err := s.pool.Exec(ctx, `CREATE FUNCTION reject_tie_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test'; END $$;
 CREATE CONSTRAINT TRIGGER reject_tie_commit AFTER INSERT ON completed_games DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_tie_commit()`); err != nil {
		t.Fatal(err)
	}
	r = eligibleResult(t, groups.Updated, 2, 0, "completed")
	r.GameID = "failed"
	r.StartedAt = base.Add(-time.Hour)
	r.FinishedAt = base.Add(30 * time.Minute)
	r.Players[0].UserID = 90
	r.Players[1].UserID = 1 // zero new points, but worse placement if it were committed
	if _, err := s.RecordCompletedGame(ctx, r); err == nil {
		t.Fatal("commit should fail")
	}
	after, err := s.ListGroupRanking(ctx, 42)
	if err != nil || !reflect.DeepEqual(after.Entries[:2], before.Entries[:2]) {
		t.Fatal("failed commit changed tie-break", after, err)
	}
}

func TestUnscoredGamesDoNotReplaceLastEligiblePlacement(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	ctx := t.Context()
	base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	for i, id := range []int64{1, 2} {
		r := eligibleResult(t, groups.Legacy, 2, 0, "completed")
		r.GameID = fmt.Sprintf("scored-%d", id)
		r.StartedAt = base.Add(-time.Hour)
		r.FinishedAt = base.Add(time.Duration(i) * time.Minute)
		r.Players[0].UserID = id
		r.Players[1].UserID = 99
		if _, err := s.RecordCompletedGame(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	want, err := s.ListGroupRanking(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if want.Entries[0].UserID != 2 {
		t.Fatal(want)
	}
	for _, pending := range []bool{false, true} {
		r := eligibleResult(t, groups.Legacy, 1, 1, "departure")
		if pending {
			r = eligibleResult(t, groups.Legacy, 2, 0, "completed")
			r.PolicyVersion = ""
			for i := range r.Players {
				r.Players[i].Score = 0
			}
		}
		r.GameID = fmt.Sprintf("unscored-%t", pending)
		r.StartedAt = base.Add(-time.Hour)
		r.FinishedAt = base.Add(time.Hour)
		if _, err := s.RecordCompletedGame(ctx, r); err != nil {
			t.Fatal(err)
		}
		got, err := s.ListGroupRanking(ctx, 42)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("unscored game changed ranking", got, err)
		}
	}
}
