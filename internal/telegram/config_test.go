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

func (m *mockGroupRepo) SetRankingPrivate(ctx context.Context, chatID int64, private bool) (groups.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.configs[chatID]
	if !ok {
		c = groups.Defaults(chatID)
	}
	if c.RankingPrivate != private {
		c.RankingPrivate = private
		c.Revision++
	}
	m.configs[chatID] = c
	return c, nil
}

func (m *mockGroupRepo) ToggleRankingPrivate(ctx context.Context, chatID int64) (groups.Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.configs[chatID]
	if !ok {
		c = groups.Defaults(chatID)
	}
	c.RankingPrivate = !c.RankingPrivate
	c.Revision++
	m.configs[chatID] = c
	return c, nil
}

func (m *mockGroupRepo) ObserveGroupTitle(ctx context.Context, chatID int64, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.configs[chatID]
	if !ok {
		c = groups.Defaults(chatID)
	}
	if title != "" && c.Title != title {
		c.Title = title
	}
	m.configs[chatID] = c
	return nil
}

type mockUserRepo struct {
	mu        sync.Mutex
	users     map[string]groups.KnownUser
	groupRepo *mockGroupRepo
}

func newMockUserRepo(groupRepo *mockGroupRepo) *mockUserRepo {
	return &mockUserRepo{
		users:     make(map[string]groups.KnownUser),
		groupRepo: groupRepo,
	}
}

func (m *mockUserRepo) ObserveGroupUser(ctx context.Context, user groups.KnownUser) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.groupRepo != nil {
		m.groupRepo.mu.Lock()
		_, ok := m.groupRepo.configs[user.ChatID]
		m.groupRepo.mu.Unlock()
		if !ok {
			return fmt.Errorf("foreign key violation: chat_id %d not in group_configs", user.ChatID)
		}
	}
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
	bot := newTestBot(api, svc, tokens, renderer, time.Minute, nil)
	groupRepo := newMockGroupRepo()
	userRepo := newMockUserRepo(groupRepo)
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

// 1 & 20: Grupo sem config explícita -> Classic + Updated. Setup nunca bloqueia /novo.
func TestConfig_DefaultsAndNeverBlocked(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1001)

	// User runs /novo without ever opening /config
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/novo@unobot",
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
	if view.GroupConfig.RankingSystem != groups.Updated {
		t.Errorf("expected Updated ranking by default, got %v", view.GroupConfig.RankingSystem)
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
		Text: "/novo@unobot",
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
		Text: "/novo@unobot classico",
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
		Text: "/cancelar@unobot",
	})

	// Next /novo without args returns to Caseiro
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player1"},
		Text: "/novo@unobot",
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
		Text: "/novo@unobot",
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
		Data:    fmt.Sprintf("cfg_rank_legacy_%d", chatID),
	})

	// Check that GroupConfig was updated in storage
	cfg, _ := h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro || cfg.RankingSystem != groups.Legacy {
		t.Fatalf("expected updated config in repo, got %+v", cfg)
	}

	// But the active game still retains Classic rules and Legacy ranking snapshot!
	view, _ := h.svc.PublicView(ctx, summary.GameID)
	if view.Rules.AllowSwapHands {
		t.Error("active game AllowSwapHands was mutated by config change!")
	}
	if view.GroupConfig.RankingSystem != groups.Updated {
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
		Text: "/config@unobot",
	})

	if len(h.api.SentMessages) == 0 {
		t.Fatal("expected config message sent to group")
	}
	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Configuração do Grupo") {
		t.Fatalf("unexpected message text: %s", lastMsg.Text)
	}
	markup, ok := lastMsg.ReplyMarkup.(*telego.InlineKeyboardMarkup)
	if !ok || markup == nil || len(markup.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows of config buttons, got %+v", lastMsg.ReplyMarkup)
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

	// Admin clicks Anon privacy button
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_priv_anon",
		From:    telego.User{ID: adminID, FirstName: "Creator"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_privacy_anon_%d", chatID),
	})

	cfg, _ = h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if !cfg.RankingPrivate {
		t.Errorf("expected RankingPrivate true after clicking anon, got false")
	}

	// Admin clicks Public privacy button
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_priv_pub",
		From:    telego.User{ID: adminID, FirstName: "Creator"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_privacy_public_%d", chatID),
	})

	cfg, _ = h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.RankingPrivate {
		t.Errorf("expected RankingPrivate false after clicking public, got true")
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
		Text: "/config@unobot",
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

	// Regular user tries privacy callback
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_unauth_priv",
		From:    telego.User{ID: regularUserID, FirstName: "Bob"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_privacy_anon_%d", chatID),
	})

	lastAnswer = h.api.AnsweredCallbacks[len(h.api.AnsweredCallbacks)-1]
	if !lastAnswer.ShowAlert || !strings.Contains(lastAnswer.Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("expected alert refusing privacy callback, got: %+v", lastAnswer)
	}

	cfg, _ = h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.RankingPrivate {
		t.Errorf("unauthorized user altered privacy: %+v", cfg)
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
		Text: "/config@unobot",
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
		Text: "/config@unobot",
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
		Text: "/config@unobot",
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
	if cfg.DefaultGameMode != groups.Caseiro || cfg.RankingSystem != groups.Updated {
		t.Fatalf("mode change altered ranking: %+v", cfg)
	}

	// Change ranking to Updated
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb2",
		From:    telego.User{ID: adminID},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_rank_legacy_%d", chatID),
	})
	cfg, _ = h.groupRepo.GetOrCreateGroupConfig(ctx, chatID)
	if cfg.DefaultGameMode != groups.Caseiro || cfg.RankingSystem != groups.Legacy {
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
	newBot := newTestBot(api, newSvc, NewTokenStore(100, 10, time.Now, nil), NewRenderer(nil), time.Minute, nil)
	newBot.SetGroupConfigs(groupRepo)

	// New bot checks /novo
	newBot.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: 10, FirstName: "Player"},
		Text: "/novo@unobot",
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
		Text: "/config@unobot",
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

	legacy := groups.Defaults(chatID)
	legacy.RankingSystem = groups.Legacy
	h.groupRepo.configs[chatID] = legacy

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
	if edited.ReplyMarkup == nil || len(edited.ReplyMarkup.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows of config buttons, got %+v", edited.ReplyMarkup)
	}
}

// Regressão 1: Primeiro /config em grupo sem group_configs prévio observa usuário com sucesso
// após criar a configuração e exibe o menu para administradores.
func TestConfig_FirstConfigInGroupWithoutExistingConfig_ObservesUser(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1026)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	// Verifica que o grupo não possui config prévia
	h.groupRepo.mu.Lock()
	if _, exists := h.groupRepo.configs[chatID]; exists {
		t.Fatal("expected no existing group config")
	}
	h.groupRepo.mu.Unlock()

	// Admin executa /config pela primeira vez no grupo
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: adminID, FirstName: "AdminNovo", Username: "admin_novo"},
		Text: "/config@unobot",
	})

	// 1. Group config deve existir com os defaults (Classic + Updated)
	h.groupRepo.mu.Lock()
	cfg, exists := h.groupRepo.configs[chatID]
	h.groupRepo.mu.Unlock()
	if !exists {
		t.Fatal("expected group config to be created")
	}
	if cfg.DefaultGameMode != groups.Classic || cfg.RankingSystem != groups.Updated {
		t.Fatalf("expected defaults Classic + Updated, got mode=%v rank=%v", cfg.DefaultGameMode, cfg.RankingSystem)
	}

	// 2. Usuário deve ter sido observado com sucesso (sem erro de foreign key)
	key := fmt.Sprintf("%d:%d", chatID, adminID)
	h.userRepo.mu.Lock()
	user, observed := h.userRepo.users[key]
	h.userRepo.mu.Unlock()
	if !observed {
		t.Fatal("expected admin user to be observed in userRepo")
	}
	if user.DisplayName != "AdminNovo" || user.Username != "admin_novo" {
		t.Fatalf("unexpected user details: %+v", user)
	}

	// 3. Resposta de configuração enviada com botões
	if len(h.api.SentMessages) == 0 {
		t.Fatal("expected config message sent")
	}
	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Configuração do Grupo") {
		t.Fatalf("expected config menu text, got: %s", lastMsg.Text)
	}
}

// Regressão 2: Primeiro /config em grupo sem group_configs por usuário comum observa o usuário,
// preserva autorização (bloqueia alteração) e cria a configuração com defaults.
func TestConfig_FirstConfigInGroupWithoutExistingConfig_NonAdmin_ObservesUserAndPreservesAuth(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1027)
	regularUserID := int64(25)
	h.api.ChatMembers[regularUserID] = &telego.ChatMemberMember{Status: telego.MemberStatusMember}

	// Verifica que o grupo não possui config prévia
	h.groupRepo.mu.Lock()
	if _, exists := h.groupRepo.configs[chatID]; exists {
		t.Fatal("expected no existing group config")
	}
	h.groupRepo.mu.Unlock()

	// Usuário comum executa /config pela primeira vez no grupo
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: regularUserID, FirstName: "ComumNovo", Username: "comum_novo"},
		Text: "/config@unobot",
	})

	// 1. Group config deve ter sido criada com defaults
	h.groupRepo.mu.Lock()
	cfg, exists := h.groupRepo.configs[chatID]
	h.groupRepo.mu.Unlock()
	if !exists {
		t.Fatal("expected group config to be created")
	}
	if cfg.DefaultGameMode != groups.Classic || cfg.RankingSystem != groups.Updated {
		t.Fatalf("expected defaults Classic + Updated, got mode=%v rank=%v", cfg.DefaultGameMode, cfg.RankingSystem)
	}

	// 2. Usuário comum deve ter sido observado com sucesso
	key := fmt.Sprintf("%d:%d", chatID, regularUserID)
	h.userRepo.mu.Lock()
	user, observed := h.userRepo.users[key]
	h.userRepo.mu.Unlock()
	if !observed {
		t.Fatal("expected regular user to be observed in userRepo")
	}
	if user.DisplayName != "ComumNovo" {
		t.Fatalf("unexpected user details: %+v", user)
	}

	// 3. Autorização preservada: mensagem de acesso negado
	if len(h.api.SentMessages) == 0 {
		t.Fatal("expected permission denied message")
	}
	lastMsg := h.api.SentMessages[len(h.api.SentMessages)-1]
	if !strings.Contains(lastMsg.Text, "Somente administradores ou quem adicionou o bot") {
		t.Fatalf("expected permission denied, got: %s", lastMsg.Text)
	}
}

// Regressão 3: Callback de configuração em grupo recém-criado (sem config prévia)
// garante a criação da config antes de observar o usuário e processa a ação corretamente.
func TestConfig_CallbackInNewlyCreatedGroup_ObservesUser(t *testing.T) {
	h := newTestHarness()
	ctx := t.Context()
	chatID := int64(-1028)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	// Sem config prévia
	h.groupRepo.mu.Lock()
	if _, exists := h.groupRepo.configs[chatID]; exists {
		t.Fatal("expected no existing group config")
	}
	h.groupRepo.mu.Unlock()

	// Callback de alteração de modo acionado
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_new_group",
		From:    telego.User{ID: adminID, FirstName: "AdminCb", Username: "admin_cb"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 200},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})

	// 1. Group config deve existir e ter sido alterada para Caseiro
	h.groupRepo.mu.Lock()
	cfg, exists := h.groupRepo.configs[chatID]
	h.groupRepo.mu.Unlock()
	if !exists {
		t.Fatal("expected group config to be created")
	}
	if cfg.DefaultGameMode != groups.Caseiro {
		t.Fatalf("expected Caseiro mode after callback, got %v", cfg.DefaultGameMode)
	}

	// 2. Usuário do callback observado com sucesso
	key := fmt.Sprintf("%d:%d", chatID, adminID)
	h.userRepo.mu.Lock()
	user, observed := h.userRepo.users[key]
	h.userRepo.mu.Unlock()
	if !observed {
		t.Fatal("expected callback user to be observed in userRepo")
	}
	if user.DisplayName != "AdminCb" {
		t.Fatalf("unexpected user details: %+v", user)
	}
}

// Tests dynamic updates of summaries in /config message when mode or ranking is changed via callback.
func TestConfig_DynamicCallbackUpdates(t *testing.T) {
	ctx := t.Context()
	h := newTestHarness()
	chatID := int64(-100223344)
	adminID := int64(10)
	h.api.ChatMembers[adminID] = &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}

	h.groupRepo.configs[chatID] = groups.Config{
		ChatID:            chatID,
		DefaultGameMode:   groups.Classic,
		RankingSystem:     groups.Legacy,
		InstalledByUserID: &adminID,
	}

	// 1. Initial /config command
	h.cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup"},
		From: &telego.User{ID: adminID, FirstName: "Admin"},
		Text: "/config@unobot",
	})

	if len(h.api.SentMessages) == 0 {
		t.Fatal("expected config message sent")
	}
	initialMsg := h.api.SentMessages[len(h.api.SentMessages)-1]

	// Verify initial text (Classic + Updated)
	if !strings.Contains(initialMsg.Text, "<b>Modo padrão de partida:</b> Clássico") {
		t.Fatalf("expected Classic mode header in: %s", initialMsg.Text)
	}
	if !strings.Contains(initialMsg.Text, "<blockquote><b>🎮 Clássico</b>\nRegras padrão do bot, sem as combinações extras do modo Caseiro.</blockquote>") {
		t.Fatalf("expected Classic summary in: %s", initialMsg.Text)
	}
	if !strings.Contains(initialMsg.Text, "<blockquote><b>🏆 Legado</b>\nTodos os jogadores elegíveis, exceto o último colocado, recebem +1 ponto.</blockquote>") {
		t.Fatalf("expected Legacy summary in: %s", initialMsg.Text)
	}
	if strings.Contains(initialMsg.Text, "🎮 Caseiro") {
		t.Fatalf("unexpected Caseiro in initial message: %s", initialMsg.Text)
	}
	if strings.Contains(initialMsg.Text, "🏆 Atualizado") {
		t.Fatalf("unexpected Updated in initial message: %s", initialMsg.Text)
	}
	if strings.Count(initialMsg.Text, "<blockquote>") != 3 {
		t.Fatalf("expected 3 blockquotes, got %d", strings.Count(initialMsg.Text, "<blockquote>"))
	}
	if strings.Count(initialMsg.Text, "\n\n────────────\n\n") != 2 {
		t.Fatalf("expected 2 separators in: %s", initialMsg.Text)
	}

	// Verify config was not altered merely by opening /config
	cfgInitial := h.groupRepo.configs[chatID]
	if cfgInitial.DefaultGameMode != groups.Classic || cfgInitial.RankingSystem != groups.Legacy {
		t.Fatalf("opening /config mutated state: %+v", cfgInitial)
	}

	// 2. Change mode to Caseiro via callback
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_mode_caseiro",
		From:    telego.User{ID: adminID, FirstName: "Admin"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_mode_caseiro_%d", chatID),
	})

	if len(h.api.EditedMessages) == 0 {
		t.Fatal("expected edited message after mode callback")
	}
	editedAfterMode := h.api.EditedMessages[len(h.api.EditedMessages)-1]

	// Check ParseMode is HTML
	if editedAfterMode.ParseMode != telego.ModeHTML {
		t.Fatalf("expected ParseMode HTML, got %s", editedAfterMode.ParseMode)
	}

	// Main text and summary updated to Caseiro
	if !strings.Contains(editedAfterMode.Text, "<b>Modo padrão de partida:</b> Caseiro") {
		t.Fatalf("expected Caseiro mode header in: %s", editedAfterMode.Text)
	}
	if !strings.Contains(editedAfterMode.Text, "<blockquote><b>🎮 Caseiro</b>\nPermite combinações extras entre cartas de compra, como +4 sobre +2 e +2 da cor escolhida sobre +4.</blockquote>") {
		t.Fatalf("expected Caseiro summary in: %s", editedAfterMode.Text)
	}
	// Old Classic summary must disappear
	if strings.Contains(editedAfterMode.Text, "🎮 Clássico") {
		t.Fatalf("old Classic summary did not disappear after mode change: %s", editedAfterMode.Text)
	}
	// Ranking still Legacy
	if !strings.Contains(editedAfterMode.Text, "<blockquote><b>🏆 Legado</b>\nTodos os jogadores elegíveis, exceto o último colocado, recebem +1 ponto.</blockquote>") {
		t.Fatalf("expected Legacy summary preserved in: %s", editedAfterMode.Text)
	}
	if strings.Contains(editedAfterMode.Text, "🏆 Atualizado") {
		t.Fatalf("unexpected Updated summary in: %s", editedAfterMode.Text)
	}
	// Exactly 3 blockquotes and 2 separators preserved
	if strings.Count(editedAfterMode.Text, "<blockquote>") != 3 {
		t.Fatalf("expected 3 blockquotes in edited message, got %d", strings.Count(editedAfterMode.Text, "<blockquote>"))
	}
	if strings.Count(editedAfterMode.Text, "\n\n────────────\n\n") != 2 {
		t.Fatalf("expected 2 separators in edited message: %s", editedAfterMode.Text)
	}

	// 3. Change ranking to Updated via callback
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_rank_updated",
		From:    telego.User{ID: adminID, FirstName: "Admin"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_rank_updated_%d", chatID),
	})

	editedAfterRank := h.api.EditedMessages[len(h.api.EditedMessages)-1]

	// Main text and summary updated to Updated
	if !strings.Contains(editedAfterRank.Text, "<b>Sistema de ranking:</b> Atualizado") {
		t.Fatalf("expected Updated rank header in: %s", editedAfterRank.Text)
	}
	if !strings.Contains(editedAfterRank.Text, "<blockquote><b>🏆 Atualizado</b>\nA pontuação varia conforme a colocação: quanto melhor a posição, mais pontos o jogador recebe.</blockquote>") {
		t.Fatalf("expected Updated summary in: %s", editedAfterRank.Text)
	}
	// Old Legacy summary must disappear
	if strings.Contains(editedAfterRank.Text, "🏆 Legado") {
		t.Fatalf("old Legacy summary did not disappear after rank change: %s", editedAfterRank.Text)
	}
	// Mode still Caseiro
	if !strings.Contains(editedAfterRank.Text, "<blockquote><b>🎮 Caseiro</b>\nPermite combinações extras entre cartas de compra, como +4 sobre +2 e +2 da cor escolhida sobre +4.</blockquote>") {
		t.Fatalf("expected Caseiro summary preserved in: %s", editedAfterRank.Text)
	}
	if strings.Contains(editedAfterRank.Text, "🎮 Clássico") {
		t.Fatalf("unexpected Classic summary in: %s", editedAfterRank.Text)
	}
	// Exactly 3 blockquotes and 2 separators preserved
	if strings.Count(editedAfterRank.Text, "<blockquote>") != 3 {
		t.Fatalf("expected 3 blockquotes in edited message, got %d", strings.Count(editedAfterRank.Text, "<blockquote>"))
	}
	if strings.Count(editedAfterRank.Text, "\n\n────────────\n\n") != 2 {
		t.Fatalf("expected 2 separators in edited message: %s", editedAfterRank.Text)
	}

	// Buttons preserved
	if editedAfterRank.ReplyMarkup == nil || len(editedAfterRank.ReplyMarkup.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows of buttons, got %+v", editedAfterRank.ReplyMarkup)
	}

	// Dynamic click 3: switch privacy to anon
	h.cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cb_priv_anon",
		From:    telego.User{ID: adminID, FirstName: "Creator"},
		Message: &telego.Message{Chat: telego.Chat{ID: chatID}, MessageID: 100},
		Data:    fmt.Sprintf("cfg_privacy_anon_%d", chatID),
	})

	if len(h.api.EditedMessages) != 3 {
		t.Fatalf("expected 3 edits, got %d", len(h.api.EditedMessages))
	}
	editedAfterPriv := h.api.EditedMessages[2]
	if !strings.Contains(editedAfterPriv.Text, "🔒 Anônimo") {
		t.Fatalf("expected anonymous privacy blockquote in: %s", editedAfterPriv.Text)
	}
	if strings.Count(editedAfterPriv.Text, "<blockquote>") != 3 {
		t.Fatalf("expected 3 blockquotes after privacy edit, got %d", strings.Count(editedAfterPriv.Text, "<blockquote>"))
	}
	if editedAfterPriv.ReplyMarkup == nil || len(editedAfterPriv.ReplyMarkup.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 rows of buttons after privacy edit, got %+v", editedAfterPriv.ReplyMarkup)
	}
	if editedAfterPriv.ReplyMarkup.InlineKeyboard[2][1].Text != "✅ Anônimo" {
		t.Fatalf("expected ✅ Anônimo button, got: %s", editedAfterPriv.ReplyMarkup.InlineKeyboard[2][1].Text)
	}
}
