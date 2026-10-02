package main

import (
	"context"
	"flag"
	"github.com/malbs/UnoGoBot/internal/app"
	"github.com/malbs/UnoGoBot/internal/config"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		slog.Error("UnoBotGO stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	dev := flag.Bool("dev", false, "allow Vite development without embedded frontend; authentication still required")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel}))
	slog.SetDefault(logger)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx, cfg, *dev, logger)
}
