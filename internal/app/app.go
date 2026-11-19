// Package app coordinates migrations and the Telegram bot in one process.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/malbs/UnoGoBot/internal/config"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/storage/postgres"
	"github.com/malbs/UnoGoBot/internal/telegram"
)

type Migrator interface {
	Migrate(context.Context) error
	VerifySchema(context.Context) error
}

// Initialize is the fail-closed startup boundary, independently testable.
func Initialize(ctx context.Context, m Migrator, start func(context.Context) error) error {
	migration, cancel := context.WithTimeout(ctx, config.MigrationTimeout)
	defer cancel()
	if err := migration.Err(); err != nil {
		return fmt.Errorf("startup migrations: before migration: %w", err)
	}
	if err := m.Migrate(migration); err != nil {
		return fmt.Errorf("startup migrations: migrate: %w", err)
	}
	if err := m.VerifySchema(migration); err != nil {
		return fmt.Errorf("startup migrations: verify schema: %w", err)
	}
	if err := migration.Err(); err != nil {
		return fmt.Errorf("startup migrations: completion: %w", err)
	}
	cancel()
	// Functional components inherit the application lifecycle, not the migration deadline.
	return start(ctx)
}

// Run starts the bot. The Telegram adapter owns the HTTP listener in every
// transport mode: the webhook endpoint and the liveness probes share one
// address, and no other component binds a port.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	webhookSecret := ""
	if cfg.TelegramMode == "webhook" {
		secret, err := config.DeriveWebhookSecret(cfg.SecretKey)
		if err != nil {
			return err
		}
		webhookSecret = secret
	}
	connect, cancel := context.WithTimeout(ctx, 10*time.Second)
	store, err := postgres.Open(connect, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer store.Close()
	logger.InfoContext(ctx, "database connected")
	err = Initialize(ctx, store, func(context.Context) error { return nil })
	if err != nil {
		return err
	}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	svc, err := game.NewService(
		game.WithHistoryLimit(config.HistoryLimit),
		game.WithTurnPolicy(game.TurnPolicy{Initial: cfg.TurnTimeout, Min: cfg.MinTurnTime, Penalty: cfg.SkipTimePenalty}),
	)
	if err != nil {
		return err
	}
	client, err := telegram.NewBot(cfg.Token, nil, logger)
	if err != nil {
		return err
	}
	// Constructing the bot starts its dispatcher, only after migrations.
	bot := telegram.New(client, svc, telegram.NewTokenStore(config.InlineTokenLimit, config.InlineTokenUserLimit, time.Now, nil), telegram.NewRenderer(nil), config.InlineTokenTTL, logger)
	bot.SetGroupConfigs(store)
	bot.SetKnownUsers(store)
	bot.SetResultRepository(store)
	finalizer := &game.Finalizer{Service: svc, Repository: store, Notify: bot.NotifyCommitted, OnFailure: func(id string, err error) {
		logger.Warn("completed result pending; retry scheduled", "game_id", id, "error", err)
	}}
	bot.SetFinalizer(finalizer)
	bot.SetRankingService(&ranking.Service{Repository: store})
	bot.SetTurnTimeout(cfg.TurnTimeout)
	bot.SetTurnMin(cfg.MinTurnTime)
	bot.SetTransport(telegram.TransportConfig{
		Mode:          telegram.TransportMode(cfg.TelegramMode),
		WebhookURL:    cfg.WebhookURL,
		WebhookSecret: webhookSecret,
		ListenAddr:    cfg.WebAddr,
	})
	go finalizer.Run(ctx)
	botDone := make(chan error, 1)
	go func() { botDone <- bot.Run(ctx) }()
	var outcome error
	botExited := false
	select {
	case <-ctx.Done():
	case outcome = <-botDone:
		botExited = true
	}
	stop()
	if !botExited {
		select {
		case err := <-botDone:
			outcome = err
		case <-time.After(config.ShutdownTimeout):
			outcome = errors.New("bot shutdown timeout")
		}
	}
	if errors.Is(outcome, context.Canceled) {
		return nil
	}
	return outcome
}
