package ranking

import (
	"context"
	"errors"
	"testing"

	"github.com/malbs/UnoGoBot/internal/groups"
)

type readStub struct {
	chatID int64
	err    error
}

func (r *readStub) ListGroupRanking(_ context.Context, id int64) (GroupRanking, error) {
	r.chatID = id
	return GroupRanking{System: groups.Updated, Total: 1, Entries: []Entry{{UserID: 4, Score: 857}}}, r.err
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
