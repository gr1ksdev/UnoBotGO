package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

var (
	ErrWebhookActive      = errors.New("webhook is active; please delete webhook before running long polling")
	ErrInlineModeDisabled = errors.New("inline mode is disabled for this bot; enable it via @BotFather using /setinline")
	ErrMissingBotUsername = errors.New("bot has no username configured in Telegram")
)

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
}

// SetTurnTimeout configures automatic inactivity skips at the application layer.
func (b *Bot) SetTurnTimeout(timeout time.Duration) { b.turnTimeout = timeout }

func New(
	api BotAPI,
	service *game.Service,
	tokens *TokenStore,
	renderer *Renderer,
	tokenTTL time.Duration,
	logger *slog.Logger,
) *Bot {
	if logger == nil {
		logger = slog.Default()
	}
	if tokens == nil {
		tokens = NewTokenStore(20000, 512, time.Now, nil)
	}
	if renderer == nil {
		renderer = NewRenderer(nil)
	}

	b := &Bot{
		api:      api,
		service:  service,
		tokens:   tokens,
		renderer: renderer,
		logger:   logger,
	}

	// Construct inline handler
	b.inlineHandler = NewInlineHandler(api, service, renderer, tokens, tokenTTL, nil, logger)

	// Construct dispatcher with inline handler callback
	b.dispatcher = NewDispatcher(logger, func(ctx context.Context, query *telego.InlineQuery) {
		b.inlineHandler.HandleInlineQuery(ctx, query)
	})

	// Inject dispatcher back into inline handler
	b.inlineHandler.dispatcher = b.dispatcher

	// Construct command & callback handlers
	b.cmdHandler = NewCommandHandler(api, service, renderer, tokens, "", logger)
	b.cbHandler = NewCallbackHandler(api, service, renderer, tokens, logger)

	return b
}

// Start verifies Telegram bot settings, registers commands, and starts processing updates.
// Run blocks until ctx is canceled, then cleanly drains all queues.
func (b *Bot) Run(ctx context.Context) error {
	// 1. Verify Bot Identity and Inline Mode
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
		b.logger.Warn("inline queries are not enabled for this bot on BotFather", "username", me.Username)
		return ErrInlineModeDisabled
	}

	// 2. Check for active Webhook
	webhookInfo, err := b.api.GetWebhookInfo(ctx)
	if err != nil {
		return fmt.Errorf("check webhook info: %w", err)
	}
	if webhookInfo != nil && webhookInfo.URL != "" {
		b.logger.Error("webhook is active; refusing to start long polling without manual cleanup", "webhook_url", webhookInfo.URL)
		return fmt.Errorf("%w: current URL is %s", ErrWebhookActive, webhookInfo.URL)
	}

	// 3. Register implemented bot commands
	implementedCommands := []telego.BotCommand{
		{Command: "novo", Description: "Criar uma nova partida de UNO"},
		{Command: "entrar", Description: "Entrar na partida de UNO"},
		{Command: "iniciar", Description: "Iniciar a partida (apenas responsável)"},
		{Command: "cancelar", Description: "Cancelar a partida (apenas responsável)"},
		{Command: "sair", Description: "Sair da partida em andamento"},
		{Command: "estado", Description: "Ver estado atual da partida"},
		{Command: "ajuda", Description: "Instruções de como jogar"},
	}

	if err := b.api.SetMyCommands(ctx, &telego.SetMyCommandsParams{Commands: implementedCommands}); err != nil {
		b.logger.Warn("failed to register bot commands with Telegram", "error", err.Error())
	}

	// 4. Start long polling
	updates, err := b.api.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{
		AllowedUpdates: []string{
			"message",
			"inline_query",
			"chosen_inline_result",
			"callback_query",
		},
	})
	if err != nil {
		return fmt.Errorf("start long polling: %w", err)
	}

	b.logger.Info("started long polling updates", "username", b.username)
	if b.turnTimeout > 0 {
		go b.autoSkipLoop(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("shutting down bot, stopping polling and draining queues...")
			b.dispatcher.Stop(10 * time.Second)
			return nil
		case update, ok := <-updates:
			if !ok {
				b.logger.Info("updates channel closed")
				b.dispatcher.Stop(10 * time.Second)
				return nil
			}
			b.processUpdate(ctx, update)
		}
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

func (b *Bot) processUpdate(ctx context.Context, update telego.Update) {
	switch {
	case update.Message != nil:
		chatID := game.ChatID(update.Message.Chat.ID)
		b.dispatcher.EnqueueChat(chatID, func(taskCtx context.Context) {
			b.cmdHandler.HandleMessage(taskCtx, update.Message)
		})
	case update.InlineQuery != nil:
		b.dispatcher.EnqueueInline(update.InlineQuery)
	case update.ChosenInlineResult != nil:
		// ChosenInlineResult handler consumes token and internally schedules work into the chat queue
		b.inlineHandler.HandleChosenInlineResult(ctx, update.ChosenInlineResult)
	case update.CallbackQuery != nil:
		var chatID game.ChatID
		if update.CallbackQuery.Message != nil {
			chatID = game.ChatID(update.CallbackQuery.Message.GetChat().ID)
		}
		b.dispatcher.EnqueueChat(chatID, func(taskCtx context.Context) {
			b.cbHandler.HandleCallback(taskCtx, update.CallbackQuery)
		})
	}
}
