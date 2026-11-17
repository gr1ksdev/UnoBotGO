package telegram

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/malbs/UnoGoBot/internal/config"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/mymmrac/telego"
)

var (
	ErrWebhookActive      = errors.New("webhook is active; please delete webhook before running long polling")
	ErrInlineModeDisabled = errors.New("inline mode is disabled for this bot; enable it via @BotFather using /setinline")
	ErrMissingBotUsername = errors.New("bot has no username configured in Telegram")
)

var allowedUpdates = []string{"message", "inline_query", "chosen_inline_result", "callback_query", "my_chat_member"}

const (
	longPollingRetryTimeout     = 8 * time.Second
	webhookRegistrationAttempts = 3
	webhookRetryBaseDelay       = time.Second
	webhookHealthCheckInterval  = 5 * time.Minute
)

type Bot struct {
	finalizer        *game.Finalizer
	resultRepository ranking.Repository
	groupConfigs     groups.Repository
	knownUsers       groups.UserRepository
	groupsService    *groups.Service
	api              BotAPI
	service          *game.Service
	tokens           *TokenStore
	renderer         *Renderer
	dispatcher       *Dispatcher
	cmdHandler       *CommandHandler
	cbHandler        *CallbackHandler
	inlineHandler    *InlineHandler
	current          *currentGameTracker
	languages        *languageCache
	logger           *slog.Logger
	usernamePublic   atomic.Value
	username         string
	turnTimeout      time.Duration
	transport        TransportConfig
	dedupe           *updateDeduper
	ready            atomic.Bool
	onReady          func()
	retrySleep       func(context.Context, time.Duration) error
}

func (b *Bot) SetGroupConfigs(repository groups.Repository) {
	b.groupConfigs = repository
	b.cmdHandler.groupConfigs = repository
	b.languages.repo = repository
	b.updateGroupsService()
}

func (b *Bot) SetKnownUsers(repository groups.UserRepository) {
	b.knownUsers = repository
	b.cmdHandler.knownUsers = repository
	b.cbHandler.knownUsers = repository
}

// SetReadyCallback registers a hook invoked once the transport is serving
// updates. It never receives payloads and must not block.
func (b *Bot) SetReadyCallback(onReady func()) {
	b.onReady = onReady
}

// markReady flips the readiness probe and notifies the observer.
func (b *Bot) markReady() {
	b.ready.Store(true)
	if b.onReady != nil {
		b.onReady()
	}
}

func (b *Bot) updateGroupsService() {
	if b.groupConfigs == nil {
		return
	}
	svc := &groups.Service{
		Repository:       b.groupConfigs,
		LookupMembership: b.lookupMembership,
	}
	b.groupsService = svc
	b.cmdHandler.SetGroupsService(svc)
	b.cbHandler.SetGroupsService(svc)
}

func (b *Bot) lookupMembership(ctx context.Context, chatID, userID int64) (groups.Membership, error) {
	return lookupMembershipAPI(ctx, b.api, chatID, userID)
}

func (b *Bot) SetTurnTimeout(timeout time.Duration) { b.turnTimeout = timeout }
func (b *Bot) SetTransport(cfg TransportConfig)     { b.transport = cfg.normalized() }
func New(api BotAPI, service *game.Service, tokens *TokenStore, renderer *Renderer, tokenTTL time.Duration, logger *slog.Logger) *Bot {
	if logger == nil {
		logger = slog.Default()
	}
	if tokens == nil {
		tokens = NewTokenStore(config.InlineTokenLimit, config.InlineTokenUserLimit, time.Now, nil)
	}
	if renderer == nil {
		renderer = NewRenderer(nil)
	}
	b := &Bot{api: api, service: service, tokens: tokens, renderer: renderer, logger: logger, transport: TransportConfig{Mode: TransportPolling, ListenAddr: defaultWebhookListenAddr}, dedupe: newUpdateDeduper(time.Now), retrySleep: waitContext, current: newCurrentGameTracker(), languages: newLanguageCache()}
	b.inlineHandler = NewInlineHandler(api, service, renderer, tokens, tokenTTL, nil, logger)
	b.inlineHandler.current = b.current
	b.dispatcher = NewDispatcher(logger, func(ctx context.Context, q *telego.InlineQuery) { b.inlineHandler.HandleInlineQuery(ctx, q) })
	b.inlineHandler.dispatcher = b.dispatcher
	b.cmdHandler = NewCommandHandler(api, service, renderer, tokens, "", logger)
	b.cmdHandler.current = b.current
	b.cmdHandler.languages = b.languages
	b.cbHandler = NewCallbackHandler(api, service, renderer, tokens, logger)
	b.cbHandler.languages = b.languages
	b.cbHandler.applyMenu = b.applyCommandMenu
	return b
}

func (b *Bot) Run(ctx context.Context) error {
	defer b.dispatcher.Stop(10 * time.Second)
	me, err := b.api.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("verify bot getMe: %w", err)
	}
	if me.Username == "" {
		return ErrMissingBotUsername
	}
	b.username = me.Username
	b.usernamePublic.Store(me.Username)
	b.renderer.SetBotID(me.ID)
	b.cmdHandler.botUsername = me.Username
	b.logger.Info("connected to telegram bot", "username", me.Username, "id", me.ID)
	if !me.SupportsInlineQueries {
		return ErrInlineModeDisabled
	}
	if err := b.registerCommands(ctx); err != nil {
		return err
	}
	cfg := b.transport.normalized()
	b.transport = cfg
	if cfg.Mode == TransportWebhook {
		return b.runWebhook(ctx, cfg)
	}
	return b.runPolling(ctx, cfg)
}
func (b *Bot) registerCommands(ctx context.Context) error {
	// The global group list is Portuguese. Groups configured for English get a
	// higher-priority per-chat override installed when the admin changes it.
	registrations := []telego.SetMyCommandsParams{
		{
			Commands: []telego.BotCommand{{Command: "start", Description: "Conhecer o UnoBotGO"}},
			Scope:    &telego.BotCommandScopeDefault{Type: telego.ScopeTypeDefault},
		},
		{
			Commands: privateCommandList(groups.CommandsPT),
			Scope:    &telego.BotCommandScopeAllPrivateChats{Type: telego.ScopeTypeAllPrivateChats},
		},
		{
			Commands: commandList(groups.CommandsPT),
			Scope:    &telego.BotCommandScopeAllGroupChats{Type: telego.ScopeTypeAllGroupChats},
		},
	}
	for i := range registrations {
		if err := b.api.SetMyCommands(ctx, &registrations[i]); err != nil {
			b.logger.Warn("failed to register bot commands with Telegram", "scope", registrations[i].Scope.ScopeType(), "error", err.Error())
		}
	}
	return nil
}

// longPollingParams must include a positive Timeout. telego only applies its
// default timeout when params is nil; passing a non-nil struct without Timeout
// results in timeout=0 (short polling) and much higher 429 exposure.
func longPollingParams() *telego.GetUpdatesParams {
	return &telego.GetUpdatesParams{
		Limit:          100,
		Timeout:        longPollingTimeoutSeconds,
		AllowedUpdates: allowedUpdates,
	}
}

func (b *Bot) retry(ctx context.Context, delay time.Duration) error {
	if b.retrySleep == nil {
		return waitContext(ctx, delay)
	}
	return b.retrySleep(ctx, delay)
}

func webhookLogURL(raw string) string {
	if raw == "" {
		return "not-set"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "invalid"
	}
	return u.Scheme + "://" + u.Host + u.Path
}

func (b *Bot) registerWebhook(ctx context.Context, cfg TransportConfig) error {
	params := &telego.SetWebhookParams{
		URL:                cfg.WebhookURL,
		SecretToken:        cfg.WebhookSecret,
		AllowedUpdates:     allowedUpdates,
		DropPendingUpdates: config.WebhookDropPendingUpdates,
	}

	var lastErr error
	for attempt := 1; attempt <= webhookRegistrationAttempts; attempt++ {
		if attempt > 1 {
			if err := b.retry(ctx, time.Duration(attempt-1)*webhookRetryBaseDelay); err != nil {
				return err
			}
		}
		if err := b.api.SetWebhook(ctx, params); err != nil {
			lastErr = err
			b.logger.WarnContext(ctx, "failed to set webhook",
				"attempt", attempt,
				"error", err,
			)
			continue
		}
		b.logger.InfoContext(ctx, "webhook configured", "url", webhookLogURL(cfg.WebhookURL))
		b.logWebhookStatus(ctx)
		return nil
	}

	return fmt.Errorf("set webhook after %d attempts: %w", webhookRegistrationAttempts, lastErr)
}

func (b *Bot) logWebhookStatus(ctx context.Context) {
	info, err := b.api.GetWebhookInfo(ctx)
	if err != nil {
		b.logger.WarnContext(ctx, "failed to read webhook status", "error", err)
		return
	}
	if info == nil {
		return
	}
	attrs := []any{
		"url", webhookLogURL(info.URL),
		"pending_update_count", info.PendingUpdateCount,
		"allowed_updates", info.AllowedUpdates,
	}
	if info.LastErrorDate > 0 {
		attrs = append(attrs,
			"last_error_date", info.LastErrorDate,
			"last_error_message", redactToken(info.LastErrorMessage),
		)
	}
	b.logger.InfoContext(ctx, "webhook status", attrs...)
}

func (b *Bot) startWebhookMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(webhookHealthCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := b.api.GetWebhookInfo(ctx)
				if err != nil {
					b.logger.WarnContext(ctx, "failed to read webhook health", "error", err)
					continue
				}
				if info == nil {
					continue
				}
				lastErrorRecent := info.LastErrorDate > 0 && time.Since(time.Unix(info.LastErrorDate, 0)) < 2*webhookHealthCheckInterval
				if info.PendingUpdateCount > 0 || lastErrorRecent {
					b.logger.WarnContext(ctx, "webhook delivery issue",
						"url", webhookLogURL(info.URL),
						"pending_update_count", info.PendingUpdateCount,
						"last_error_date", info.LastErrorDate,
						"last_error_message", redactToken(info.LastErrorMessage),
					)
				}
			}
		}
	}()
}

func (b *Bot) runPolling(ctx context.Context, cfg TransportConfig) error {
	info, err := b.api.GetWebhookInfo(ctx)
	if err != nil {
		return fmt.Errorf("check webhook info: %w", err)
	}
	if info != nil && info.URL != "" {
		if err := b.api.DeleteWebhook(ctx, &telego.DeleteWebhookParams{DropPendingUpdates: config.WebhookDropPendingUpdates}); err != nil {
			return fmt.Errorf("delete webhook before polling: %w", err)
		}
		b.logger.Info("deleted existing webhook before polling")
	}
	// The probe listener is auxiliary: the bot must run even when the
	// address is unavailable.
	health, err := startHealthServer(cfg.ListenAddr, &b.ready, "", nil)
	if err != nil {
		b.logger.Warn("health probes unavailable", "addr", cfg.ListenAddr, "error", err.Error())
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		health.Shutdown(shutdown)
	}()
	updates, err := b.api.UpdatesViaLongPolling(
		ctx,
		longPollingParams(),
		telego.WithLongPollingRetryTimeout(longPollingRetryTimeout),
	)
	if err != nil {
		return fmt.Errorf("start long polling: %w", err)
	}
	b.logger.Info("started long polling updates", "username", b.username)
	b.markReady()
	b.startScheduler(ctx)
	for {
		select {
		case <-ctx.Done():
			b.logger.Info("shutting down bot, stopping polling and draining queues...")
			b.dispatcher.Stop(10 * time.Second)
			return nil
		case u, ok := <-updates:
			if !ok {
				b.dispatcher.Stop(10 * time.Second)
				return nil
			}
			b.submitUpdate(ctx, u)
		}
	}
}
func (b *Bot) runWebhook(ctx context.Context, cfg TransportConfig) error {
	u, err := url.Parse(cfg.WebhookURL)
	if err != nil {
		return fmt.Errorf("parse webhook URL: %w", err)
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	health, err := startHealthServer(cfg.ListenAddr, &b.ready, path, b.webhookHandler(cfg.WebhookSecret))
	if err != nil {
		return fmt.Errorf("start webhook server: %w", err)
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		health.Shutdown(shutdown)
	}()
	if err := b.registerWebhook(ctx, cfg); err != nil {
		b.dispatcher.Stop(10 * time.Second)
		return err
	}
	b.logger.Info("started webhook server", "addr", cfg.ListenAddr, "path", path)
	b.markReady()
	b.startScheduler(ctx)
	b.startWebhookMonitor(ctx)
	<-ctx.Done()
	b.dispatcher.Stop(10 * time.Second)
	return nil
}
func (b *Bot) webhookHandler(secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(telego.WebhookSecretTokenHeader)), []byte(secret)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		ct, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || ct != "application/json" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()
		var update telego.Update
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&update); err != nil {
			http.Error(w, "invalid update", http.StatusBadRequest)
			return
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			http.Error(w, "invalid update", http.StatusBadRequest)
			return
		}
		if !b.submitUpdate(r.Context(), update) {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
func (b *Bot) startScheduler(ctx context.Context) {
	if b.turnTimeout > 0 {
		go b.autoSkipLoop(ctx)
	}
}
func (b *Bot) autoSkipLoop(ctx context.Context) {
	interval := b.turnTimeout / 4
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, candidate := range b.service.ExpiredTurns(ctx, b.turnTimeout) {
				b.enqueueAutoSkip(candidate)
			}
		}
	}
}

// Mutation and notification share a chat task with all player actions. Discovery
// outside the queue is read-only; saturation or an obsolete candidate is a no-op.
func (b *Bot) enqueueAutoSkip(candidate game.ExpiredTurn) bool {
	return b.dispatcher.EnqueueChat(candidate.ChatID, func(ctx context.Context) {
		outcome, applied := b.service.AutoSkipTurn(ctx, candidate, b.turnTimeout)
		if !applied {
			return
		}
		notify := b.finalizeOutcome(ctx, outcome)
		if notify != nil {
			b.tokens.InvalidateGame(outcome.View.GameID)
			notify()
			return
		}
		b.cmdHandler.reply(ctx, int64(candidate.ChatID), b.renderer.RenderTurn(outcome.View), makeGameButtons(outcome.View))
	})
}
func (b *Bot) submitUpdate(ctx context.Context, update telego.Update) bool {
	if !b.dedupe.reserve(update.UpdateID) {
		return true
	}
	accepted := b.processUpdate(ctx, update)
	if accepted {
		b.dedupe.commit(update.UpdateID)
	} else {
		b.dedupe.release(update.UpdateID)
	}
	return accepted
}
func (b *Bot) processUpdate(ctx context.Context, update telego.Update) bool {
	if update.Message != nil {
		if command, _, ok := parseMessageCommand(update.Message, b.username); ok && command == "reset" {
			chatID := game.ChatID(update.Message.Chat.ID)
			return b.dispatcher.EnqueueRecovery(chatID, func(c context.Context) {
				b.cmdHandler.HandleReset(c, update.Message, func() { b.dispatcher.ResetChat(chatID) })
			})
		}
	}
	switch {
	case update.Message != nil:
		chatID := game.ChatID(update.Message.Chat.ID)
		return b.dispatcher.EnqueueChat(chatID, func(c context.Context) { b.cmdHandler.HandleMessage(c, update.Message) })
	case update.InlineQuery != nil:
		return b.dispatcher.EnqueueInline(update.InlineQuery)
	case update.ChosenInlineResult != nil:
		return b.inlineHandler.HandleChosenInlineResult(ctx, update.ChosenInlineResult)
	case update.CallbackQuery != nil:
		var chatID game.ChatID
		if update.CallbackQuery.Message != nil {
			chatID = game.ChatID(update.CallbackQuery.Message.GetChat().ID)
		}
		return b.dispatcher.EnqueueChat(chatID, func(c context.Context) { b.cbHandler.HandleCallback(c, update.CallbackQuery) })
	case update.MyChatMember != nil:
		chatID := game.ChatID(update.MyChatMember.Chat.ID)
		return b.dispatcher.EnqueueChat(chatID, func(c context.Context) { b.cmdHandler.HandleMyChatMember(c, update.MyChatMember) })
	}

	return true
}
