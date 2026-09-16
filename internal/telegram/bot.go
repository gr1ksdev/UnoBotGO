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
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

var (
	ErrWebhookActive      = errors.New("webhook is active; please delete webhook before running long polling")
	ErrInlineModeDisabled = errors.New("inline mode is disabled for this bot; enable it via @BotFather using /setinline")
	ErrMissingBotUsername = errors.New("bot has no username configured in Telegram")
)

var allowedUpdates = []string{"message", "inline_query", "chosen_inline_result", "callback_query"}

type Bot struct {
	api           BotAPI
	service       *game.Service
	tokens        *TokenStore
	renderer      *Renderer
	dispatcher    *Dispatcher
	cmdHandler    *CommandHandler
	cbHandler     *CallbackHandler
	inlineHandler *InlineHandler
	logger        *slog.Logger
	username      string
	turnTimeout   time.Duration
	transport     TransportConfig
	dedupe        *updateDeduper
}

func (b *Bot) SetTurnTimeout(timeout time.Duration) { b.turnTimeout = timeout }
func (b *Bot) SetTransport(cfg TransportConfig)     { b.transport = cfg.normalized() }
func New(api BotAPI, service *game.Service, tokens *TokenStore, renderer *Renderer, tokenTTL time.Duration, logger *slog.Logger) *Bot {
	if logger == nil {
		logger = slog.Default()
	}
	if tokens == nil {
		tokens = NewTokenStore(20000, 512, time.Now, nil)
	}
	if renderer == nil {
		renderer = NewRenderer(nil)
	}
	b := &Bot{api: api, service: service, tokens: tokens, renderer: renderer, logger: logger, transport: TransportConfig{Mode: TransportPolling, ListenAddr: defaultWebhookListenAddr}, dedupe: newUpdateDeduper(time.Now)}
	b.inlineHandler = NewInlineHandler(api, service, renderer, tokens, tokenTTL, nil, logger)
	b.dispatcher = NewDispatcher(logger, func(ctx context.Context, q *telego.InlineQuery) { b.inlineHandler.HandleInlineQuery(ctx, q) })
	b.inlineHandler.dispatcher = b.dispatcher
	b.cmdHandler = NewCommandHandler(api, service, renderer, tokens, "", logger)
	b.cbHandler = NewCallbackHandler(api, service, renderer, tokens, logger)
	return b
}

func (b *Bot) Run(ctx context.Context) error {
	me, err := b.api.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("verify bot getMe: %w", err)
	}
	if me.Username == "" {
		return ErrMissingBotUsername
	}
	b.username = me.Username
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
	return b.runPolling(ctx)
}
func (b *Bot) registerCommands(ctx context.Context) error {
	commands := []telego.BotCommand{{Command: "novo", Description: "Criar uma nova partida de UNO"}, {Command: "entrar", Description: "Entrar na partida de UNO"}, {Command: "iniciar", Description: "Iniciar a partida (apenas responsável)"}, {Command: "cancelar", Description: "Cancelar a partida (apenas responsável)"}, {Command: "sair", Description: "Sair da partida em andamento"}, {Command: "estado", Description: "Ver estado atual da partida"}, {Command: "ajuda", Description: "Instruções de como jogar"}}
	if err := b.api.SetMyCommands(ctx, &telego.SetMyCommandsParams{Commands: commands}); err != nil {
		b.logger.Warn("failed to register bot commands with Telegram", "error", err.Error())
	}
	return nil
}
func (b *Bot) runPolling(ctx context.Context) error {
	info, err := b.api.GetWebhookInfo(ctx)
	if err != nil {
		return fmt.Errorf("check webhook info: %w", err)
	}
	if info != nil && info.URL != "" {
		if err := b.api.DeleteWebhook(ctx, &telego.DeleteWebhookParams{DropPendingUpdates: false}); err != nil {
			return fmt.Errorf("delete webhook before polling: %w", err)
		}
		b.logger.Info("deleted existing webhook before polling")
	}
	updates, err := b.api.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{AllowedUpdates: allowedUpdates})
	if err != nil {
		return fmt.Errorf("start long polling: %w", err)
	}
	b.logger.Info("started long polling updates", "username", b.username)
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
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle(path, b.webhookHandler(cfg.WebhookSecret))
	server := &http.Server{Addr: cfg.ListenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("start webhook server: %w", err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("webhook server: %w", err)
		}
	case <-ctx.Done():
		_ = listener.Close()
		b.dispatcher.Stop(10 * time.Second)
		return nil
	default:
	}
	if err := b.api.SetWebhook(ctx, &telego.SetWebhookParams{URL: cfg.WebhookURL, SecretToken: cfg.WebhookSecret, AllowedUpdates: allowedUpdates, DropPendingUpdates: cfg.DropPendingUpdates}); err != nil {
		_ = server.Shutdown(context.Background())
		b.dispatcher.Stop(10 * time.Second)
		return fmt.Errorf("set webhook: %w", err)
	}
	b.logger.Info("started webhook server", "addr", cfg.ListenAddr, "path", path)
	b.startScheduler(ctx)
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
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
			for _, outcome := range b.service.AutoSkipExpired(ctx, b.turnTimeout) {
				chatID := outcome.View.ChatID
				b.dispatcher.EnqueueChat(chatID, func(taskCtx context.Context) {
					text := "⏱️ O tempo acabou; o turno foi pulado.\n\n" + b.renderer.RenderPublicState(outcome.View)
					b.cmdHandler.reply(taskCtx, int64(chatID), text, makeGameButtons(outcome.View.GameID))
				})
			}
		}
	}
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
	}
	return true
}
