//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

// Each test uses an isolated schema, never modifying existing application data.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("integration tests require TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 8)
	if _, err = rand.Read(id); err != nil {
		t.Fatal(err)
	}
	schema := "m7_test_" + hex.EncodeToString(id)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{pool: pool}
	t.Cleanup(func() {
		s.Close()
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return s
}

func TestMigrations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if !errors.Is(s.VerifySchema(ctx), ErrSchema) {
		t.Fatal("empty schema was accepted")
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal("reapply:", err)
	}
	if err := s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO group_configs(chat_id) VALUES(42)`); err != nil {
		t.Fatal(err)
	}
	var mode, system string
	if err := s.pool.QueryRow(ctx, `SELECT default_game_mode,ranking_system FROM group_configs WHERE chat_id=42`).Scan(&mode, &system); err != nil {
		t.Fatal(err)
	}
	if mode != "classic" || system != "legacy" {
		t.Fatalf("unexpected defaults %s %s", mode, system)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE schema_migrations SET checksum='tampered'`); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.VerifySchema(ctx), ErrSchema) {
		t.Fatal("modified migration accepted")
	}
	if !errors.Is(s.Migrate(ctx), ErrSchema) {
		t.Fatal("modified migration reapplied")
	}
}

func TestConcurrentMigrations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { done <- s.Migrate(ctx) }()
	}
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if err := s.VerifySchema(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationFailureRollsBack(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	// A conflicting relation forces DDL failure after the ledger was created.
	if _, err := s.pool.Exec(ctx, `CREATE TABLE group_configs (sentinel int)`); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err == nil {
		t.Fatal("conflicting DDL succeeded")
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("migration ledger survived failed transaction")
	}
}
