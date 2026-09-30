//go:build integration

package postgres

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

func TestListGroupRankingAccumulationIsolationAndHistory(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	service := &ranking.Service{Repository: s}
	// A real previous game leaves historical users outside games A and B.
	history := eligibleResult(t, groups.Updated, 2, 0, "completed")
	history.GameID = "history"
	history.Players[0].UserID = 99
	history.Players[0].DisplayName = "Ana histórica 🦊"
	history.Players[1].UserID = 100
	if _, err := s.RecordCompletedGame(ctx, history); err != nil {
		t.Fatal(err)
	}
	a := eligibleResult(t, groups.Updated, 3, 1, "completed")
	a.GameID = "A"
	a.Players[0].DisplayName = "Freddy"
	a.Players[1].DisplayName = "Mezi"
	a.Players[2].DisplayName = "João <&>"
	if _, err := s.RecordCompletedGame(ctx, a); err != nil {
		t.Fatal(err)
	}
	b := a.Clone()
	b.GameID = "B"
	b.FinishedAt = a.FinishedAt.Add(time.Second)
	b.Players[0].UserID, b.Players[1].UserID = 2, 1
	b.Players[0].DisplayName, b.Players[1].DisplayName = "Mezi", "Freddy"
	if _, err := s.RecordCompletedGame(ctx, b); err != nil {
		t.Fatal(err)
	}
	if commit, err := s.RecordCompletedGame(ctx, b); err != nil || !commit.AlreadyPersisted {
		t.Fatal(commit, err)
	}
	// Same user IDs in a different group, different ranking system.
	if _, err := s.GetOrCreateGroupConfig(ctx, 43); err != nil {
		t.Fatal(err)
	}
	other := eligibleResult(t, groups.Legacy, 2, 0, "completed")
	other.GameID = "other-chat"
	other.ChatID = 43
	if _, err := s.RecordCompletedGame(ctx, other); err != nil {
		t.Fatal(err)
	}
	got, err := service.ListGroupRanking(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	want := []ranking.Entry{
		{UserID: 2, DisplayName: "Mezi", Score: 1500, CompletedGames: 2, Wins: 1},
		{UserID: 1, DisplayName: "Freddy", Score: 1500, CompletedGames: 2, Wins: 1},
		{UserID: 99, DisplayName: "Ana histórica 🦊", Score: 1000, CompletedGames: 1, Wins: 1},
		{UserID: 100, DisplayName: "2", Score: 0, CompletedGames: 1, Wins: 0},
		{UserID: 3, DisplayName: "João <&>", Score: 0, CompletedGames: 2, Wins: 0},
	}
	if got.System != groups.Updated || got.Total != 5 || !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("ranking=%+v; want %+v", got, want)
	}
	for _, entry := range got.Entries {
		if entry.UserID == 4 {
			t.Fatal("abandoner included")
		}
	}
	isolated, err := service.ListGroupRanking(ctx, 43)
	if err != nil || isolated.Total != 2 || isolated.System != groups.Legacy || isolated.Entries[0].Score != 100 {
		t.Fatal(isolated, err)
	}
	for _, id := range []int64{44, 45} {
		if id == 44 {
			if _, err := s.GetOrCreateGroupConfig(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		empty, err := service.ListGroupRanking(ctx, id)
		if err != nil || empty.Total != 0 || len(empty.Entries) != 0 {
			t.Fatal(empty, err)
		}
	}
}

func TestListGroupRankingBoundAndIncompatibleSystem(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	monthStart := ranking.MonthDateString(time.Now())
	if _, err := s.pool.Exec(ctx, `INSERT INTO player_group_monthly_stats(chat_id,user_id,month_start,ranking_system,score_units,completed_games,wins,display_name,last_finished_at)
 SELECT 42,n,$1::date,'updated',n,1,0,'Pessoa '||n,now() FROM generate_series(1,1000) n`, monthStart); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListGroupRanking(ctx, 42, time.Time{})
	if err != nil || got.Total != 1000 || len(got.Entries) != ranking.MaxRankingEntries || got.Entries[0].Score != 1000 || got.Entries[len(got.Entries)-1].Score != 489 {
		t.Fatal(got, err)
	}
	// Even incompatible rows beyond the returned prefix must fail closed.
	if _, err := s.pool.Exec(ctx, `UPDATE player_group_monthly_stats SET ranking_system='legacy' WHERE chat_id=42 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListGroupRanking(ctx, 42, time.Time{}); !errors.Is(err, ranking.ErrNeedsProductDecision) {
		t.Fatal("mixed systems accepted", err)
	}
}

func TestListGroupRankingAfterCommitFailure(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	// Deferred trigger runs at COMMIT, after all result/stats writes succeeded.
	if _, err := s.pool.Exec(ctx, `CREATE FUNCTION reject_commit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'test commit failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_result_commit AFTER INSERT ON completed_games
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_commit()`); err != nil {
		t.Fatal(err)
	}
	r := eligibleResult(t, groups.Updated, 3, 0, "completed")
	if _, err := s.RecordCompletedGame(ctx, r); err == nil {
		t.Fatal("commit should fail")
	}
	got, err := s.ListGroupRanking(ctx, 42, time.Time{})
	if err != nil || got.Total != 0 {
		t.Fatal("uncommitted stats visible", got, err)
	}
	if _, err := s.pool.Exec(ctx, `DROP TRIGGER reject_result_commit ON completed_games`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCompletedGame(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListGroupRanking(ctx, 42, time.Time{})
	if err != nil || got.Total != 3 || got.Entries[0].Score != 1000 {
		t.Fatal(got, err)
	}
}
