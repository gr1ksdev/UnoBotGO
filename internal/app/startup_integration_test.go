//go:build integration

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/malbs/UnoGoBot/internal/storage/postgres"
)

// Match the storage suite's isolated-schema approach; never use application data.
func startupStore(t *testing.T) (*postgres.Store, *pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("startup integration requires TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	schema := "startup_test_" + hex.EncodeToString(id)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL for startup tests")
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	store, err := postgres.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store, admin, schema
}
func TestStartupMigratesEmptyDatabaseBeforeComponents(t *testing.T) {
	store, admin, schema := startupStore(t)
	started := false
	err := Initialize(t.Context(), store, func(ctx context.Context) error {
		if err := store.VerifySchema(ctx); err != nil {
			return err
		}
		// HTTP is created only inside the post-migration boundary. No real Telegram.
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
		defer server.Close()
		response, err := server.Client().Get(server.URL + "/readyz")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatal("migrated startup not ready")
		}
		started = true // Mock bot/workers may now start with the same Store.
		return nil
	})
	if err != nil || !started {
		t.Fatal("startup failed", err)
	}
	var count, distinct int
	if err := admin.QueryRow(t.Context(), "SELECT count(*),count(DISTINCT version) FROM "+schema+".schema_migrations").Scan(&count, &distinct); err != nil || count == 0 || count != distinct {
		t.Fatal("startup ledger", count, distinct, err)
	}
	if err := Initialize(t.Context(), store, func(ctx context.Context) error { return store.VerifySchema(ctx) }); err != nil {
		t.Fatal("second startup", err)
	}
}
func TestStartupMigrationFailureNeverStartsComponents(t *testing.T) {
	for _, failure := range []string{"sql", "checksum"} {
		t.Run(failure, func(t *testing.T) {
			store, admin, schema := startupStore(t)
			if failure == "sql" {
				if _, err := admin.Exec(t.Context(), "CREATE TABLE "+schema+".group_configs(sentinel int)"); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := store.Migrate(t.Context()); err != nil {
					t.Fatal(err)
				}
				if _, err := admin.Exec(t.Context(), "UPDATE "+schema+".schema_migrations SET checksum='tampered'"); err != nil {
					t.Fatal(err)
				}
			}
			started := false
			err := Initialize(t.Context(), store, func(context.Context) error { started = true; return nil })
			var migration *postgres.MigrationError
			if err == nil || started || !errors.As(err, &migration) || migration.Migration == "" {
				t.Fatal("partially started after migration failure", err)
			}
			if failure == "checksum" && !errors.Is(err, postgres.ErrSchema) {
				t.Fatal("schema cause lost", err)
			}
		})
	}
}
