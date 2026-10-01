// Package app coordinates migrations, HTTP and Telegram in one process.
package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/malbs/UnoGoBot/internal/config"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/httpapi"
	"github.com/malbs/UnoGoBot/internal/media"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/storage/postgres"
	"github.com/malbs/UnoGoBot/internal/telegram"
	"github.com/malbs/UnoGoBot/web"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type Migrator interface {
	Migrate(context.Context) error
	VerifySchema(context.Context) error
}

// Initialize is the fail-closed startup boundary, independently testable.
func Initialize(ctx context.Context, m Migrator, start func(context.Context) error) error {
	if err := m.Migrate(ctx); err != nil {
		return err
	}
	if err := m.VerifySchema(ctx); err != nil {
		return err
	}
	return start(ctx)
}
func Run(ctx context.Context, cfg *config.Config, dev bool, logger *slog.Logger) error {
	if !dev && !web.Built() {
		return errors.New("frontend missing: run make build")
	}
	refs, err := httpapi.NewReferences(cfg.MiniAppSecret)
	if err != nil {
		return err
	}
	hookPath := ""
	if cfg.TelegramMode == "webhook" {
		u, err := url.Parse(cfg.WebhookURL)
		if err != nil {
			return errors.New("invalid webhook URL")
		}
		hookPath = u.Path
		if hookPath == "" || hookPath == "/" || strings.HasPrefix(hookPath, "/api") || strings.HasPrefix(hookPath, "/assets") || hookPath == "/healthz" || hookPath == "/readyz" {
			return errors.New("WEBHOOK_URL requires a dedicated path, e.g. /telegram")
		}
	}
	connect, cancel := context.WithTimeout(ctx, 10*time.Second)
	store, err := postgres.Open(connect, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer store.Close()
	migration, cancel := context.WithTimeout(ctx, config.MigrationTimeout)
	err = Initialize(migration, store, func(context.Context) error { return nil })
	cancel()
	if err != nil {
		return fmt.Errorf("startup migrations: %w", err)
	}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	svc, err := game.NewService(game.WithHistoryLimit(config.HistoryLimit))
	if err != nil {
		return err
	}
	client, err := telegram.NewBot(cfg.Token, nil, logger)
	if err != nil {
		return err
	}
	photos := media.New(ctx, telegram.AvatarSource{Bot: client})
	api := &httpapi.API{Rankings: &ranking.GlobalService{Repository: store}, References: refs, Media: photos, Token: cfg.Token, MaxAge: config.InitDataMaxAge}
	mux := http.NewServeMux()
	mux.Handle("/api/", api.Handler())
	mux.Handle("/", httpapi.Static(web.Files()))
	var ready atomic.Bool
	var hook atomic.Value
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	})
	if hookPath != "" {
		mux.Handle(hookPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ready.Load() {
				w.WriteHeader(503)
				return
			}
			hook.Load().(http.Handler).ServeHTTP(w, r)
		}))
	}
	server := &http.Server{Handler: httpapi.Security(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	listener, err := net.Listen("tcp", cfg.WebAddr)
	if err != nil {
		return errors.New("cannot start HTTP listener")
	}
	httpDone := make(chan error, 1)
	go func() { httpDone <- server.Serve(listener) }()
	// Constructing the bot starts its dispatcher, only after HTTP and migrations.
	bot := telegram.New(client, svc, telegram.NewTokenStore(config.InlineTokenLimit, config.InlineTokenUserLimit, time.Now, nil), telegram.NewRenderer(nil), config.InlineTokenTTL, logger)
	bot.SetGroupConfigs(store)
	bot.SetKnownUsers(store)
	bot.SetResultRepository(store)
	bot.SetRankingService(&ranking.Service{Repository: store})
	bot.SetTurnTimeout(cfg.TurnTimeout)
	bot.SetTransport(telegram.TransportConfig{Mode: telegram.TransportMode(cfg.TelegramMode), WebhookURL: cfg.WebhookURL, WebhookSecret: cfg.WebhookSecret(), DropPendingUpdates: config.WebhookDropPendingUpdates})
	bot.UseSharedHTTP(func() { ready.Store(true) })
	if hookPath != "" {
		hook.Store(bot.WebhookHandler())
	}
	botDone := make(chan error, 1)
	go func() { botDone <- bot.Run(ctx) }()
	var outcome error
	botExited := false
	select {
	case <-ctx.Done():
	case outcome = <-botDone:
		botExited = true
	case outcome = <-httpDone:
	}
	ready.Store(false)
	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
	}
	if !botExited {
		select {
		case <-botDone:
		case <-shutdown.Done():
			if outcome == nil {
				outcome = errors.New("bot shutdown timeout")
			}
		}
	}
	if errors.Is(outcome, http.ErrServerClosed) || errors.Is(outcome, context.Canceled) {
		return nil
	}
	return outcome
}
