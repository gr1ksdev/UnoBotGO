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

func TestObserveGroupUserRequiresGroupConfigFK(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	ctx := t.Context()
	nonExistentChatID := int64(99999)
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Observar usuário em chat sem group_configs prévio DEVE falhar pela restrição de FK
	err := s.ObserveGroupUser(ctx, groups.KnownUser{
		ChatID:      nonExistentChatID,
		UserID:      1,
		DisplayName: "Usuário Teste",
		Username:    "teste",
		LastSeenAt:  now,
	})
	if err == nil {
		t.Fatal("expected foreign key violation error when observing user without group_configs, got nil")
	}

	// Criar a configuração do grupo via GetOrCreateGroupConfig (ordem correta)
	if _, err := s.GetOrCreateGroupConfig(ctx, nonExistentChatID); err != nil {
		t.Fatalf("failed to create group config: %v", err)
	}

	// Agora a observação DEVE ter sucesso
	if err := s.ObserveGroupUser(ctx, groups.KnownUser{
		ChatID:      nonExistentChatID,
		UserID:      1,
		DisplayName: "Usuário Teste",
		Username:    "teste",
		LastSeenAt:  now,
	}); err != nil {
		t.Fatalf("expected ObserveGroupUser to succeed after group config created, got: %v", err)
	}

	users, err := s.KnownGroupUsers(ctx, nonExistentChatID)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].UserID != 1 || users[0].DisplayName != "Usuário Teste" {
		t.Fatalf("unexpected users: %+v", users)
	}
}
