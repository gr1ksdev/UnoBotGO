// devseed populates or cleans deterministic development fixtures for the Mini App.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/malbs/UnoGoBot/internal/devseed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("devseed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cmd := "miniapp"
	if len(os.Args) > 1 {
		cmd = strings.ToLower(strings.TrimSpace(os.Args[1]))
	}

	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		printUsage()
		return nil
	}

	dbURL := os.Getenv("DATABASE_URL")
	if err := devseed.ValidateEnvironment(os.LookupEnv, dbURL); err != nil {
		return err
	}

	maskedDB := devseed.MaskDatabaseURL(dbURL)
	envName := os.Getenv("APP_ENV")
	if envName == "" && os.Getenv("ALLOW_DEV_SEED") == "1" {
		envName = "development (ALLOW_DEV_SEED=1)"
	}

	fmt.Println("=================================================================")
	fmt.Println("WARNING: this command modifies development fixtures.")
	fmt.Printf("Database:    %s\n", maskedDB)
	fmt.Printf("Environment: %s\n", envName)
	fmt.Println("=================================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	switch cmd {
	case "miniapp", "seed", "seed-miniapp":
		report, err := devseed.Seed(ctx, pool, time.Now())
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Print(report.String())
		return nil

	case "clean", "clean-miniapp", "clean-miniapp-seed":
		cleanReport, err := devseed.Clean(ctx, pool)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Print(cleanReport.String())
		return nil

	default:
		printUsage()
		return fmt.Errorf("unknown action %q", cmd)
	}
}

func printUsage() {
	fmt.Print(`UnoBotGO Development Seed Tool

Usage:
  go run ./cmd/devseed <action>

Actions:
  miniapp, seed       Populate deterministic development fixtures for Mini App
  clean, clean-miniapp Clean exclusively development fixtures

Environment requirements:
  APP_ENV=development  (or ALLOW_DEV_SEED=1)
  DATABASE_URL          (PostgreSQL connection string)
`)
}
