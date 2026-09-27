package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/mymmrac/telego"
)

type mockGroupRepo struct {
	mu                   sync.Mutex
	configs              map[int64]groups.Config
	conflictOnRankChange bool
}

func newMockGroupRepo() *mockGroupRepo {
	return &mockGroupRepo{
		configs: make(map[int64]groups.Config),
	}
}

func (m *mockGroupRepo) GetOrCreateGroupConfig(ctx context.Context, chatID int64) (groups.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.configs[chatID]; ok {
		return c, nil
	}
	c := groups.Defaults(chatID)
	m.configs[chatID] = c
	return c, nil
}

func (m *mockGroupRepo) SetDefaultGameMode(ctx context.Context, chatID int64, mode groups.Mode) (groups.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.configs[chatID]
	if !ok {
		c = groups.Defaults(chatID)
	}
	if c.DefaultGameMode != mode {
		c.DefaultGameMode = mode
		c.Revision++
	}
	m.configs[chatID] = c
	return c, nil
}

func (m *mockGroupRepo) SetRankingSystem(ctx context.Context, chatID int64, system groups.RankingSystem) (groups.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conflictOnRankChange {
		return groups.Config{}, groups.ErrNeedsProductDecision
	}
	c, ok := m.configs[chatID]
	if !ok {
		c = groups.Defaults(chatID)
	}
	if c.RankingSystem != system {
		c.RankingSystem = system
		c.Revision++
	}
	m.configs[chatID] = c
	return c, nil
}

func (m *mockGroupRepo) SetInstalledBy(ctx context.Context, chatID int64, installerID int64) (groups.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.configs[chatID]
	if !ok {
		c = groups.Defaults(chatID)
	}
	c.InstalledByUserID = &installerID
	c.Revision++
	m.configs[chatID] = c
	return c, nil
}

type mockUserRepo struct {
	mu    sync.Mutex
	users map[string]groups.KnownUser
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]groups.KnownUser)}
}

func (m *mockUserRepo) ObserveGroupUser(ctx context.Context, user groups.KnownUser) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d:%d", user.ChatID, user.UserID)
	m.users[key] = user
	return nil
}

func (m *mockUserRepo) KnownGroupUsers(ctx context.Context, chatID int64) ([]groups.KnownUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []groups.KnownUser
	for _, u := range m.users {
		if u.ChatID == chatID {
			result = append(result, u)
		}
	}
	return result, nil
}

type testHarness struct {
	api        *mockBotAPI
	svc        *game.Service
	tokens     *TokenStore
	renderer   *Renderer
	bot        *Bot
	cmdHandler *CommandHandler
	cbHandler  *CallbackHandler
	groupRepo  *mockGroupRepo
	userRepo   *mockUserRepo
}

func newTestHarness() *testHarness {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	tokens := NewTokenStore(100, 10, time.Now, nil)
	renderer := NewRenderer(nil)
	bot := New(api, svc, tokens, renderer, time.Minute, nil)
	groupRepo := newMockGroupRepo()
	userRepo := newMockUserRepo()
	bot.SetGroupConfigs(groupRepo)
	bot.SetKnownUsers(userRepo)
	return &testHarness{
		api:        api,
		svc:        svc,
		tokens:     tokens,
		renderer:   renderer,
		bot:        bot,
		cmdHandler: bot.cmdHandler,
		cbHandler:  bot.cbHandler,
		groupRepo:  groupRepo,
		userRepo:   userRepo,
	}
}


// 1 & 20: Grupo sem config explícita -> Classic + Legacy. Setup nunca bloqueia /novo.
func TestConfig_DefaultsAndNeverBlocked(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1001)

	// User runs /novo without ever opening /config
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/novo",
	})

	summary, err := h.svc.FindChatGame(ctx, game.ChatID(chatID))
	if err != nil {
		t.Fatalf("game should be created with defaults: %v", err)
	}
	view, err := h.svc.PublicView(ctx, summary.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Rules.AllowSwapHands {
		t.Error("expected Classic mode by default (AllowSwapHands = false)")
	}
	if view.GroupConfig.RankingSystem != groups.Legacy {
		t.Errorf("expected Legacy ranking by default, got %v", view.GroupConfig.RankingSystem)
	}
}

// 2: /novo sem argumento usa DefaultGameMode do grupo (Classic vs Caseiro).
func TestConfig_NovoUsesGroupDefault(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1002)

	// Set group default to Caseiro
	_, err := h.groupRepo.SetDefaultGameMode(ctx, chatID, groups.Caseiro)
	if err != nil {
		t.Fatal(err)
	}

	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/novo",
	})

	summary, err := h.svc.FindChatGame(ctx, game.ChatID(chatID))
	if err != nil {
		t.Fatal(err)
	}
	view, _ := h.svc.PublicView(ctx, summary.GameID)
	if !view.Rules.AllowSwapHands {
		t.Error("expected Caseiro rules for group default Caseiro")
	}
}

// 3 & 4: /novo com override explícito ("classico" / "caseiro") afeta apenas a partida e não altera GroupConfig.
func TestConfig_NovoOverridesPreserveGroupConfig(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1003)

	// Default is Caseiro
	_, _ = h.groupRepo.SetDefaultGameMode(ctx, chatID, groups.Caseiro)

	// /novo classico forces Classic for this match only
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/novo classico",
	})

	summary, _ := h.svc.FindChatGame(ctx, game.ChatID(chatID))
	view, _ := h.svc.PublicView(ctx, summary.GameID)
	if view.Rules.AllowSwapHands {
		t.Error("expected Classic rules on /novo classico override")
	}

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro {
		t.Errorf("group default should stay Caseiro, got %v", cfg.DefaultGameMode)
	}

	// Cancel match
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/cancelar",
	})

	// Next /novo without args returns to Caseiro
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/novo",
	})
	summary2, _ := h.svc.FindChatGame(ctx, game.ChatID(chatID))
	view2, _ := h.svc.PublicView(ctx, summary2.GameID)
	if !view2.Rules.AllowSwapHands {
		t.Error("subsequent /novo should revert to group default (Caseiro)")
	}
}

// 5 & 22: Mudar default durante partida ativa -> partida mantém snapshot.
func TestConfig_ActiveGamePreservesSnapshotOnConfigChange(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1005)

	// Admin creates a Classic game
	h.api.ChatMembers[10] = &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Admin"},
		Text: "/novo",
	})
	summary, _ := h.svc.FindChatGame(ctx, game.ChatID(chatID))

	// While game is active in lobby or running, admin switches default mode to Caseiro and ranking to Updated
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb1",
		From:    telego.User{ID: 10, FirstName: "Admin"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 99},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb2",
		From:    telego.User{ID: 10, FirstName: "Admin"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 99},
		Data:    fmt.Sprintf("cfg_rank_updated_%d", chatID),
	})

	// Check that GroupConfig was updated in storage
	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro || cfg.RankingSystem != groups.Updated {
		t.Fatalf("expected updated config in repo, got %+v", cfg)
	}

	// But the active game still retains Classic rules and Legacy ranking snapshot!
	view, _ := h.svc.PublicView(ctx, summary.GameID)
	if view.Rules.AllowSwapHands {
		t.Error("active game AllowSwapHands was mutated by config change!")
	}
	if view.GroupConfig.RankingSystem != groups.Legacy {
		t.Errorf("active game ranking system snapshot was mutated! got %v", view.GroupConfig.RankingSystem)
	}
}

// 6, 7, 8: Admin pode abrir /config e alterar modo e ranking.
func TestConfig_AdminCanOpenAndModify(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1006)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	// Admin runs /config
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: adminID, FirstName: "Creator"},
		Text: "/config",
	})

	if len(h.api.SentMessages) == 0 {
		t.Fatal("expected config message sent to group")
	}
	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Configuração do Grupo") {
		t.Fatalf("unexpected message text: %s", lastMsg.Text)
	}
	markup, ok := lastMsg.ReplyMarkup.(*telego.InlineKeyboardMarkup)
	if !ok || markup == nil || len(markup.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows of config buttons, got %+v", lastMsg.ReplyMarkup)
	}

	// Admin clicks Caseiro button
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_mode",
		From:    telego.User{ID: adminID, FirstName: "Creator"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro {
		t.Errorf("expected Caseiro mode after click, got %v", cfg.DefaultGameMode)
	}

	// Admin clicks Updated ranking button
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_rank",
		From:    telego.User{ID: adminID, FirstName: "Creator"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_rank_updated_%d", chatID),
	})

	cfg, _ = h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.RankingSystem != groups.Updated {
		t.Errorf("expected Updated ranking after click, got %v", cfg.RankingSystem)
	}
}

// 9: Usuário comum não pode alterar /config nem via comando nem via callback.
func TestConfig_NonAdminCannotModify(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1009)
	regularUserID := int64(20)
	h.api.ChatMembers[regularUserID] = &telego.ChatMemberMember{Status: telego.MemberStatusMember}

	// Regular user sends /config
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: regularUserID, FirstName: "Bob"},
		Text: "/config",
	})

	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("expected permission denied reply, got: %s", lastMsg.Text)
	}

	// Regular user tries callback
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_unauth",
		From:    telego.User{ID: regularUserID, FirstName: "Bob"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})

	lastAnswer := h.api.AnsweredCallbacks[len(h.api.AnsweredCallbacks)-1]
	if !lastAnswer.ShowAlert || !strings.Contains(lastAnswer.Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("expected alert refusing callback, got: %+v", lastAnswer)
	}

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Classic {
		t.Errorf("unauthorized user altered config: %+v", cfg)
	}
}

// 10: Installer conhecido e ainda membro pode alterar.
func TestConfig_InstallerStillMemberCanConfigure(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1010)
	installerID := int64(30)

	// Record installer in repo
	_, _ = h.groupRepo.SetInstalledBy(ctx, chatID, installerID)
	// User is regular member (not admin)
	h.api.ChatMembers[installerID] = &telego.ChatMemberMember{Status: telego.MemberStatusMember}

	// Installer runs /config
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: installerID, FirstName: "Installer"},
		Text: "/config",
	})

	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Configuração do Grupo") {
		t.Fatalf("expected installer to view config, got: %s", lastMsg.Text)
	}

	// Installer changes mode
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_installer",
		From:    telego.User{ID: installerID, FirstName: "Installer"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro {
		t.Errorf("expected installer change to take effect: %+v", cfg)
	}
}

// 11: Installer conhecido que saiu do grupo não pode alterar.
func TestConfig_InstallerLeftGroupCannotConfigure(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1011)
	installerID := int64(30)

	// Record installer
	_, _ = h.groupRepo.SetInstalledBy(ctx, chatID, installerID)
	// User has left the group
	h.api.ChatMembers[installerID] = &telego.ChatMemberLeft{Status: telego.MemberStatusLeft}

	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: installerID, FirstName: "ExMember"},
		Text: "/config",
	})

	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("expected denied for ex-member installer, got: %s", lastMsg.Text)
	}
}

// 12: Installer desconhecido -> admins continuam funcionando normalmente.
func TestConfig_UnknownInstallerAdminsStillWork(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1012)
	adminID := int64(40)
	h.api.ChatMembers[adminID] = &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}

	// No installer recorded
	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.InstalledByUserID != nil {
		t.Fatal("expected nil installer")
	}

	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: adminID, FirstName: "Admin"},
		Text: "/config",
	})

	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Configuração do Grupo") {
		t.Fatalf("admin should configure even if installer is unknown: %s", lastMsg.Text)
	}
}

// 13: Callback refaz autorização a cada clique.
func TestConfig_CallbackRechecksAuthorization(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1013)
	userID := int64(50)

	// Was admin when /config opened
	h.api.ChatMembers[userID] = &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}

	// Demoted to normal member before clicking callback
	h.api.ChatMembers[userID] = &telego.ChatMemberMember{Status: telego.MemberStatusMember}

	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_demoted",
		From:    telego.User{ID: userID, FirstName: "DemotedUser"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})

	lastAnswer := h.api.AnsweredCallbacks[len(h.api.AnsweredCallbacks)-1]
	if !lastAnswer.ShowAlert || !strings.Contains(lastAnswer.Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("demoted user should be denied on callback: %+v", lastAnswer)
	}
}

// 14: Callback com valor inválido é recusado.
func TestConfig_CallbackInvalidValueRejected(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1014)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	invalidPayloads := []string{
		"cfg_invalid",
		"cfg_mode_badmode_-1014",
		"cfg_rank_badrank_-1014",
		fmt.Sprintf("cfg_mode_caseiro_%d", -99999), // wrong chat ID
		"cfg_open",
	}

	for _, payload := range invalidPayloads {
		h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
			ID:      "cb_inv",
			From:    telego.User{ID: adminID},
			Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
			Data:    payload,
		})

		lastAnswer := h.api.AnsweredCallbacks[len(h.api.AnsweredCallbacks)-1]
		if !lastAnswer.ShowAlert || !strings.Contains(lastAnswer.Text, "inválid") {
			t.Fatalf("expected invalid alert for %q, got: %+v", payload, lastAnswer)
		}
	}
}

// 15 & 16: Trocar Classic ↔ Caseiro não altera RankingSystem, e trocar Legacy ↔ Updated não altera DefaultGameMode.
func TestConfig_OrthogonalChanges(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1015)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	// Change mode to Caseiro
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb1",
		From:    telego.User{ID: adminID},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})
	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro || cfg.RankingSystem != groups.Legacy {
		t.Fatalf("mode change altered ranking: %+v", cfg)
	}

	// Change ranking to Updated
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb2",
		From:    telego.User{ID: adminID},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_rank_updated_%d", chatID),
	})
	cfg, _ = h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro || cfg.RankingSystem != groups.Updated {
		t.Fatalf("ranking change altered mode: %+v", cfg)
	}
}

// 17: Update my_chat_member de instalação real registra installer quando há ator e envia boas-vindas com botão [ ⚙️ Configurar ].
func TestConfig_MyChatMember_RealInstallation(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1017)
	installerID := int64(77)

	update := telego.Update{
		UpdateID: 1,
		MyChatMember: &telego.ChatMemberUpdated{
			Chat: telego.Chat{ID: chatID, Type: "supergroup"},
			From: telego.User{ID: installerID, FirstName: "InstallerUser", Username: "installer77"},
			OldChatMember: &telego.ChatMemberLeft{
				Status: telego.MemberStatusLeft,
			},
			NewChatMember: &telego.ChatMemberMember{
				Status: telego.MemberStatusMember,
			},
		},
	}

	accepted := h.bot.processUpdate(ctx, update)
	if !accepted {
		t.Fatal("expected update to be accepted")
	}

	// Drain queue
	h.bot.dispatcher.Stop(2 * time.Second)

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.InstalledByUserID == nil || *cfg.InstalledByUserID != installerID {
		t.Fatalf("expected installer %d recorded, got %+v", installerID, cfg.InstalledByUserID)
	}

	if len(h.api.SentMessages) == 0 {
		t.Fatal("expected welcome message sent")
	}
	welcome := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(welcome.Text, "Obrigado por adicionar o UnoBotGO") {
		t.Fatalf("unexpected welcome text: %s", welcome.Text)
	}
	welcomeMarkup, ok := welcome.ReplyMarkup.(*telego.InlineKeyboardMarkup)
	if !ok || welcomeMarkup == nil || len(welcomeMarkup.InlineKeyboard) == 0 {
		t.Fatal("expected configure button in welcome message")
	}
	btn := welcomeMarkup.InlineKeyboard[0][0]
	if btn.Text != "⚙️ Configurar" || btn.CallbackData != fmt.Sprintf("cfg_open_%d", chatID) {
		t.Fatalf("unexpected button: %+v", btn)
	}

}

// 18: Update irrelevante de my_chat_member não reenvia setup.
func TestConfig_MyChatMember_IrrelevantTransitionIgnored(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1018)

	// Bot was already member, promoted to administrator
	update := telego.Update{
		UpdateID: 2,
		MyChatMember: &telego.ChatMemberUpdated{
			Chat: telego.Chat{ID: chatID, Type: "supergroup"},
			From: telego.User{ID: 99, FirstName: "AdminUser"},
			OldChatMember: &telego.ChatMemberMember{
				Status: telego.MemberStatusMember,
			},
			NewChatMember: &telego.ChatMemberAdministrator{
				Status: telego.MemberStatusAdministrator,
			},
		},
	}

	h.bot.processUpdate(ctx, update)
	h.bot.dispatcher.Stop(2 * time.Second)

	if len(h.api.SentMessages) > 0 {
		t.Fatalf("irrelevant promotion should not send messages: %+v", h.api.SentMessages)
	}
}

// 19: Bot removido e re-adicionado substitui installer anterior.
func TestConfig_MyChatMember_ReEntryReplacesInstaller(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1019)

	// First installation by user 1
	_, _ = h.groupRepo.SetInstalledBy(ctx, chatID, 1)

	// Bot re-added by user 2 after being kicked
	update := telego.Update{
		UpdateID: 3,
		MyChatMember: &telego.ChatMemberUpdated{
			Chat: telego.Chat{ID: chatID, Type: "supergroup"},
			From: telego.User{ID: 2, FirstName: "NewInstaller"},
			OldChatMember: &telego.ChatMemberBanned{
				Status: telego.MemberStatusBanned,
			},
			NewChatMember: &telego.ChatMemberMember{
				Status: telego.MemberStatusMember,
			},
		},
	}

	h.bot.processUpdate(ctx, update)
	h.bot.dispatcher.Stop(2 * time.Second)

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.InstalledByUserID == nil || *cfg.InstalledByUserID != 2 {
		t.Fatalf("expected installer replaced with user 2, got: %+v", cfg.InstalledByUserID)
	}
}

// 21: Configuração persistida sobrevive a reinício.
func TestConfig_PersistedSurvivesRestart(t *testing.T) {
	groupRepo := newMockGroupRepo()
	chatID := int64(-1021)
	ctx := context.Background()

	// Initial bot instance sets configuration
	_, _ = groupRepo.SetDefaultGameMode(ctx, chatID, groups.Caseiro)
	_, _ = groupRepo.SetRankingSystem(ctx, chatID, groups.Updated)
	_, _ = groupRepo.SetInstalledBy(ctx, chatID, 88)

	// Simulate restart by creating completely new Bot and Service instances
	api := newMockBotAPI()
	newSvc, _ := game.NewService()
	newBot := New(api, newSvc, NewTokenStore(100, 10, time.Now, nil), NewRenderer(nil), time.Minute, nil)
	newBot.SetGroupConfigs(groupRepo)

	// New bot checks /novo
	newBot.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player"},
		Text: "/novo",
	})

	summary, err := newSvc.FindChatGame(ctx, game.ChatID(chatID))
	if err != nil {
		t.Fatal(err)
	}
	view, _ := newSvc.PublicView(ctx, summary.GameID)
	if !view.Rules.AllowSwapHands || view.GroupConfig.RankingSystem != groups.Updated {
		t.Fatalf("persisted configuration did not survive restart: rules=%+v rank=%v", view.Rules, view.GroupConfig.RankingSystem)
	}
}

// 23: KnownGroupUser atualizado nos novos pontos de interação (my_chat_member, /config, callbacks).
func TestConfig_KnownUserObservedAtInteractionPoints(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1023)

	// 1. my_chat_member
	h.cmdHandler.HandleMyChatMember(ctx, &telego.ChatMemberUpdated{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: telego.User{ID: 100, FirstName: "InstUser", Username: "inst100"},
		OldChatMember: &telego.ChatMemberLeft{
			Status: telego.MemberStatusLeft,
		},
		NewChatMember: &telego.ChatMemberMember{
			Status: telego.MemberStatusMember,
		},
	})
	key1 := fmt.Sprintf("%d:%d", chatID, 100)
	if u, ok := h.userRepo.users[key1]; !ok || u.DisplayName != "InstUser" {
		t.Fatalf("expected user 100 observed in my_chat_member, got %+v", u)
	}

	// 2. /config
	h.api.ChatMembers[200] = &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 200, FirstName: "AdminConfig", Username: "admin200"},
		Text: "/config",
	})
	key2 := fmt.Sprintf("%d:%d", chatID, 200)
	if u, ok := h.userRepo.users[key2]; !ok || u.DisplayName != "AdminConfig" {
		t.Fatalf("expected user 200 observed in /config, got %+v", u)
	}

	// 3. Callback
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_obs",
		From:    telego.User{ID: 300, FirstName: "CallbackUser", Username: "cb300"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})
	key3 := fmt.Sprintf("%d:%d", chatID, 300)
	if u, ok := h.userRepo.users[key3]; !ok || u.DisplayName != "CallbackUser" {
		t.Fatalf("expected user 300 observed in callback, got %+v", u)
	}
}

// 24 (User Adjustment 1): Conflito ao trocar sistema de ranking retorna ErrNeedsProductDecision,
// emite alerta claro e preserva a configuração anterior sem quebrar a UI.
func TestConfig_RankingConflictReturnsErrNeedsProductDecision(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1024)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	// Simulate that group already has accumulated scores in Legacy ranking
	h.groupRepo.conflictOnRankChange = true

	// Admin tries to switch ranking to Updated
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_conflict",
		From:    telego.User{ID: adminID, FirstName: "Owner"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_rank_updated_%d", chatID),
	})

	if len(h.api.AnsweredCallbacks) == 0 {
		t.Fatal("expected answered callback")
	}
	lastAnswer := h.api.AnsweredCallbacks[len(h.api.AnsweredCallbacks)-1]
	if !lastAnswer.ShowAlert || !strings.Contains(lastAnswer.Text, "pontuações já acumuladas") {
		t.Fatalf("expected conflict alert to user, got: %+v", lastAnswer)
	}

	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.RankingSystem != groups.Legacy {
		t.Errorf("ranking system must be preserved on conflict, got %v", cfg.RankingSystem)
	}
}

// Welcome button [ ⚙️ Configurar ] (cfg_open_<chatID>) opens config inline.
func TestConfig_WelcomeButtonOpensConfig(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1025)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	// User clicks [ ⚙️ Configurar ] from the welcome message
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_open",
		From:    telego.User{ID: adminID, FirstName: "Owner"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 150},
		Data:    fmt.Sprintf("cfg_open_%d", chatID),
	})

	if len(h.api.EditedMessages) == 0 {
		t.Fatal("expected message to be edited with config")
	}
	edited := h.api.EditedMessages[len(h.api.EditedMessages)-1]
	if !strings.Contains(edited.Text, "Configuração do Grupo") {
		t.Fatalf("expected config text in edited message, got: %s", edited.Text)
	}
	if edited.ReplyMarkup == nil || len(edited.ReplyMarkup.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows of config buttons, got %+v", edited.ReplyMarkup)
	}
}
