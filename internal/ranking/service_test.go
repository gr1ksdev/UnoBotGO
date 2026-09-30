package ranking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
)

type readStub struct {
	chatID int64
	userID int64
	at     time.Time
	err    error
}

func (r *readStub) ListGroupRanking(_ context.Context, id int64, at time.Time) (GroupRanking, error) {
	r.chatID = id
	r.at = at
	return GroupRanking{System: groups.Updated, Total: 1, Entries: []Entry{{UserID: 4, Score: 857}}}, r.err
}

func (r *readStub) ListUserMonthlyRankings(_ context.Context, userID int64, at time.Time) (UserMonthlyRankings, error) {
	r.userID = userID
	r.at = at
	return UserMonthlyRankings{UserID: userID, MonthName: "Outubro"}, r.err
}

func TestRankingServiceScopesReadAndPropagatesFailure(t *testing.T) {
	repo := &readStub{}
	s := &Service{Repository: repo}
	got, err := s.ListGroupRanking(t.Context(), -42)
	if err != nil || repo.chatID != -42 || got.Entries[0].Score != 857 {
		t.Fatal(got, err)
	}
	repo.err = errors.New("read failed")
	if _, err = s.ListGroupRanking(t.Context(), -43); !errors.Is(err, repo.err) {
		t.Fatal(err)
	}
	if _, err = s.ListGroupRanking(t.Context(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	var unconfigured *Service
	if _, err = unconfigured.ListGroupRanking(t.Context(), -42); err == nil {
		t.Fatal("missing repository accepted")
	}

	// User monthly rankings
	repo.err = nil
	userGot, err := s.ListUserMonthlyRankings(t.Context(), 123)
	if err != nil || repo.userID != 123 || userGot.MonthName != "Outubro" {
		t.Fatal(userGot, err)
	}
	repo.err = errors.New("read failed")
	if _, err = s.ListUserMonthlyRankings(t.Context(), 123); !errors.Is(err, repo.err) {
		t.Fatal(err)
	}
	if _, err = s.ListUserMonthlyRankings(t.Context(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = s.ListUserMonthlyRankings(t.Context(), -1); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = unconfigured.ListUserMonthlyRankings(t.Context(), 123); err == nil {
		t.Fatal("missing repository accepted")
	}
}

func TestFormatScoreExactStoredUnits(t *testing.T) {
	for _, tc := range []struct {
		system groups.RankingSystem
		score  Units
		want   string
	}{
		{groups.Legacy, 0, "0 pts"}, {groups.Legacy, 100, "1 pt"}, {groups.Legacy, 200, "2 pts"},
		{groups.Updated, 0, "0,00 pts"}, {groups.Updated, 1000, "10,00 pts"}, {groups.Updated, 857, "8,57 pts"},
		{groups.Updated, 9223372036854775807, "92233720368547758,07 pts"},
	} {
		if got := FormatScore(tc.system, tc.score); got != tc.want {
			t.Fatalf("%s != %s", got, tc.want)
		}
	}
}
