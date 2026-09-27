//go:build integration

package postgres

import (
	"github.com/malbs/UnoGoBot/internal/groups"
	"testing"
	"time"
)

func TestKnownUsersMutableNamesAndDuplicates(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, id := range []int64{1, 2} {
		if err := s.ObserveGroupUser(ctx, groups.KnownUser{ChatID: 42, UserID: id, DisplayName: "👩🏽‍💻 Ga\u0301briel", Username: "user", LastSeenAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	u := groups.KnownUser{ChatID: 42, UserID: 1, DisplayName: "Novo nome", LastSeenAt: now.Add(time.Minute)}
	if err := s.ObserveGroupUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	u.DisplayName = "Nome antigo"
	u.LastSeenAt = now
	if err := s.ObserveGroupUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	users, err := s.KnownGroupUsers(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].DisplayName != "Novo nome" || users[0].Username != "" || users[1].DisplayName != "👩🏽‍💻 Ga\u0301briel" {
		t.Fatalf("identity/name semantics %+v", users)
	}
}
func TestResultStoresObservedUsersAtomically(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	r := sampleResult("known", groups.Legacy)
	r.Players[0].DisplayName = "🎮 Observado"
	r.Players[0].LastSeenAt = r.FinishedAt
	if _, err := s.RecordCompletedGame(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	users, err := s.KnownGroupUsers(t.Context(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].UserID != 1 || users[0].DisplayName != "🎮 Observado" {
		t.Fatal(users)
	}
}
