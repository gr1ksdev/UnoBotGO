package telegram

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
)

type mockUserPrivacyRepo struct {
	mu      sync.Mutex
	privacy map[int64]bool
}

func newMockUserPrivacyRepo() *mockUserPrivacyRepo {
	return &mockUserPrivacyRepo{privacy: make(map[int64]bool)}
}

func (m *mockUserPrivacyRepo) GetUserRankingPrivacy(_ context.Context, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.privacy[userID], nil
}

func (m *mockUserPrivacyRepo) SetUserRankingPrivacy(_ context.Context, userID int64, private bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.privacy[userID] = private
	return nil
}

func (m *mockUserPrivacyRepo) ToggleUserRankingPrivacy(_ context.Context, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	curr := m.privacy[userID]
	m.privacy[userID] = !curr
	return !curr, nil
}

func TestPrivatePrivacyCommand(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(nil), nil, "unobot", nil)
	privRepo := newMockUserPrivacyRepo()
	h.userPrivacy = privRepo

	// First toggle: activate privacy
	msg := &telego.Message{
		Chat: telego.Chat{ID: 100, Type: "private"},
		From: &telego.User{ID: 100, FirstName: "Player 100"},
		Text: "/privacidade",
	}
	h.HandleMessage(t.Context(), msg)

	if len(api.SentMessages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(api.SentMessages))
	}
	if !strings.Contains(api.SentMessages[0].Text, "Privacidade ativada") {
		t.Fatalf("expected 'Privacidade ativada', got %s", api.SentMessages[0].Text)
	}
	if !privRepo.privacy[100] {
		t.Fatalf("expected privacy true for user 100")
	}

	// Second toggle: deactivate privacy
	h.HandleMessage(t.Context(), msg)
	if len(api.SentMessages) != 2 {
		t.Fatalf("expected 2 messages sent, got %d", len(api.SentMessages))
	}
	if !strings.Contains(api.SentMessages[1].Text, "Privacidade desativada") {
		t.Fatalf("expected 'Privacidade desativada', got %s", api.SentMessages[1].Text)
	}
	if privRepo.privacy[100] {
		t.Fatalf("expected privacy false for user 100")
	}
}

func TestGroupPrivacyCommandPermissions(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(nil), nil, "unobot", nil)
	groupRepo := newMockGroupRepo()
	privRepo := newMockUserPrivacyRepo()
	h.groupConfigs = groupRepo
	h.userPrivacy = privRepo

	// Set installer for group -100
	installerID := int64(42)
	adminID := int64(88)
	regularID := int64(99)
	leftInstallerID := int64(43)

	_, _ = groupRepo.SetInstalledBy(t.Context(), -100, installerID)
	_, _ = groupRepo.SetInstalledBy(t.Context(), -200, leftInstallerID)

	h.SetGroupsService(&groups.Service{
		Repository: groupRepo,
		LookupMembership: func(_ context.Context, chatID, userID int64) (groups.Membership, error) {
			if userID == adminID {
				return groups.Membership{Admin: true, Member: true}, nil
			}
			if userID == installerID && chatID == -100 {
				return groups.Membership{Member: true}, nil
			}
			if userID == leftInstallerID && chatID == -200 {
				// Installer left the group!
				return groups.Membership{Member: false}, nil
			}
			return groups.Membership{Member: true}, nil
		},
	})

	// 1. Unaddressed command in group should be ignored
	api.SentMessages = nil
	unaddressed := &telego.Message{
		Chat: telego.Chat{ID: -100, Type: "supergroup"},
		From: &telego.User{ID: adminID},
		Text: "/privacidade",
	}
	h.HandleMessage(t.Context(), unaddressed)
	if len(api.SentMessages) != 0 {
		t.Fatalf("unaddressed command should be ignored in group, got %d replies", len(api.SentMessages))
	}

	// 2. Regular member addressed: should be denied
	api.SentMessages = nil
	regMsg := &telego.Message{
		Chat: telego.Chat{ID: -100, Type: "supergroup"},
		From: &telego.User{ID: regularID},
		Text: "/privacidade@unobot",
	}
	h.HandleMessage(t.Context(), regMsg)
	if len(api.SentMessages) != 1 || !strings.Contains(api.SentMessages[0].Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("regular user should be denied, got %v", api.SentMessages)
	}

	// Group should still be public
	cfg, _ := groupRepo.GetOrCreateGroupConfig(t.Context(), -100)
	if cfg.RankingPrivate {
		t.Fatalf("group should not be private")
	}

	// 3. Admin: should succeed in toggling group privacy
	api.SentMessages = nil
	adminMsg := &telego.Message{
		Chat: telego.Chat{ID: -100, Type: "supergroup"},
		From: &telego.User{ID: adminID},
		Text: "/privacidade@unobot",
	}
	h.HandleMessage(t.Context(), adminMsg)
	if len(api.SentMessages) != 1 || !strings.Contains(api.SentMessages[0].Text, "Privacidade do grupo ativada") {
		t.Fatalf("admin toggle failed, got %v", api.SentMessages)
	}
	cfg, _ = groupRepo.GetOrCreateGroupConfig(t.Context(), -100)
	if !cfg.RankingPrivate {
		t.Fatalf("group should now be private")
	}
	// Admin's own user privacy must NOT have changed!
	if privRepo.privacy[adminID] {
		t.Fatalf("group privacy toggle must not affect user privacy")
	}

	// 4. Installer member: should succeed in toggling back to public
	api.SentMessages = nil
	installerMsg := &telego.Message{
		Chat: telego.Chat{ID: -100, Type: "supergroup"},
		From: &telego.User{ID: installerID},
		Text: "/privacidade@unobot",
	}
	h.HandleMessage(t.Context(), installerMsg)
	if len(api.SentMessages) != 1 || !strings.Contains(api.SentMessages[0].Text, "Privacidade do grupo desativada") {
		t.Fatalf("installer toggle failed, got %v", api.SentMessages)
	}
	cfg, _ = groupRepo.GetOrCreateGroupConfig(t.Context(), -100)
	if cfg.RankingPrivate {
		t.Fatalf("group should now be public")
	}

	// 5. Installer who left the group: must be DENIED!
	api.SentMessages = nil
	leftMsg := &telego.Message{
		Chat: telego.Chat{ID: -200, Type: "supergroup"},
		From: &telego.User{ID: leftInstallerID},
		Text: "/privacidade@unobot",
	}
	h.HandleMessage(t.Context(), leftMsg)
	if len(api.SentMessages) != 1 || !strings.Contains(api.SentMessages[0].Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("left installer must be denied, got %v", api.SentMessages)
	}
}
