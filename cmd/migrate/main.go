// migrate applies versioned V2 migrations explicitly, without a Telegram token.
package main

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/malbs/UnoGoBot/internal/storage/postgres"
	"log/slog"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	_ = godotenv.Load()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	store, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		return err
	}
	slog.Info("V2 database migrations applied")
	return nil
}
