package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/malbs/UnoGoBot/internal/config"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/telegram"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err.Error())
		os.Exit(1)
	}

	// 2. Setup structured logger
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	logger.Info("starting UnoBotGO V2",
		"history_limit", cfg.HistoryLimit,
		"token_ttl", cfg.InlineTokenTTL,
		"token_limit", cfg.InlineTokenLimit,
		"token_user_limit", cfg.InlineTokenUserLim,
	)

	// 3. Initialize game service
	svc, err := game.NewService(game.WithHistoryLimit(cfg.HistoryLimit))
	if err != nil {
		logger.Error("failed to initialize game service", "error", err.Error())
		os.Exit(1)
	}

	// 4. Initialize token store and renderer
	tokens := telegram.NewTokenStore(cfg.InlineTokenLimit, cfg.InlineTokenUserLim, time.Now, nil)
	renderer := telegram.NewRenderer(nil)

	// 5. Initialize Telegram client
	telegoBot, err := telegram.NewBot(cfg.Token, nil, logger)
	if err != nil {
		logger.Error("failed to initialize telegram bot client", "error", err.Error())
		os.Exit(1)
	}

	// 6. Assemble bot application
	bot := telegram.New(telegoBot, svc, tokens, renderer, cfg.InlineTokenTTL, logger)

	// 7. Setup graceful shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 8. Run bot
	if err := bot.Run(ctx); err != nil {
		logger.Error("bot terminated with error", "error", err.Error())
		os.Exit(1)
	}

	logger.Info("UnoBotGO V2 stopped cleanly")
}
