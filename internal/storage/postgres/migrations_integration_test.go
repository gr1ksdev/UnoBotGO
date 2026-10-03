//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"reflect"
	"testing"
	"testing/fstest"
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
	if mode != "caseiro" || system != "updated" {
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

type ledgerEntry struct {
	Checksum  string
	AppliedAt time.Time
}

func ledgerSnapshot(t *testing.T, s *Store) map[string]ledgerEntry {
	t.Helper()
	rows, err := s.pool.Query(t.Context(), `SELECT version,checksum,applied_at FROM schema_migrations`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	snapshot := map[string]ledgerEntry{}
	for rows.Next() {
		var name string
		var entry ledgerEntry
		if err := rows.Scan(&name, &entry.Checksum, &entry.AppliedAt); err != nil {
			t.Fatal(err)
		}
		if _, exists := snapshot[name]; exists {
			t.Fatal("duplicate migration", name)
		}
		snapshot[name] = entry
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return snapshot
}
func TestPartialMigrationAndUnchangedLedgerOnReexecution(t *testing.T) {
	s := testStore(t)
	list, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	prefix := fstest.MapFS{}
	for _, m := range list[:2] {
		prefix[m.name] = &fstest.MapFile{Data: []byte(m.sql)}
	}
	if err := s.migrate(t.Context(), prefix); err != nil {
		t.Fatal(err)
	}
	before := ledgerSnapshot(t, s)
	if len(before) != 2 {
		t.Fatal(before)
	}
	if !errors.Is(s.VerifySchema(t.Context()), ErrSchema) {
		t.Fatal("partial schema accepted")
	}
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	full := ledgerSnapshot(t, s)
	if len(full) != len(list) {
		t.Fatal("ledger count", len(full))
	}
	for name, entry := range before {
		if full[name] != entry {
			t.Fatal("applied migration rewritten", name)
		}
	}
	for _, m := range list {
		if full[m.name].Checksum != m.checksum {
			t.Fatal("wrong ledger checksum", m.name)
		}
	}
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full, ledgerSnapshot(t, s)) {
		t.Fatal("up-to-date run modified ledger")
	}
}
func TestInvalidSQLRollsBackWholeBatchAndPreservesCause(t *testing.T) {
	s := testStore(t)
	files := fstest.MapFS{
		"migrations/0001_probe.up.sql":   {Data: []byte("CREATE TABLE migration_probe(id int);")},
		"migrations/0002_invalid.up.sql": {Data: []byte("THIS IS NOT SQL;")},
	}
	err := s.migrate(t.Context(), files)
	var failure *MigrationError
	var cause *pgconn.PgError
	if !errors.As(err, &failure) || failure.Migration != "migrations/0002_invalid.up.sql" || failure.Stage != "execute SQL" || !errors.As(err, &cause) || cause.Code != "42601" {
		t.Fatal("missing migration/SQL cause", err)
	}
	for _, table := range []string{"migration_probe", "schema_migrations"} {
		var exists bool
		if err := s.pool.QueryRow(t.Context(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil || exists {
			t.Fatal("batch rollback incomplete", table, err)
		}
	}
}
func TestLedgerRejectedBeforeAnyPendingSQL(t *testing.T) {
	for _, reason := range []string{"checksum", "unknown", "gap"} {
		t.Run(reason, func(t *testing.T) {
			s := testStore(t)
			files := fstest.MapFS{
				"migrations/0001_probe.up.sql":  {Data: []byte("SELECT nextval('migration_execution_probe');")},
				"migrations/0002_second.up.sql": {Data: []byte("SELECT 2;")},
			}
			list, err := readMigrations(files)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.pool.Exec(t.Context(), `CREATE SEQUENCE migration_execution_probe; CREATE TABLE schema_migrations(version text PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
				t.Fatal(err)
			}
			name, checksum := list[1].name, list[1].checksum
			if reason == "checksum" {
				checksum = "tampered"
			}
			if reason == "unknown" {
				name = "migrations/9999_unknown.up.sql"
			}
			if _, err := s.pool.Exec(t.Context(), `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, name, checksum); err != nil {
				t.Fatal(err)
			}
			before := ledgerSnapshot(t, s)
			if err := s.migrate(t.Context(), files); !errors.Is(err, ErrSchema) {
				t.Fatal("bad ledger accepted", err)
			}
			// Sequence increments survive rollback: this proves SQL was never attempted.
			var called bool
			if err := s.pool.QueryRow(t.Context(), `SELECT is_called FROM migration_execution_probe`).Scan(&called); err != nil || called {
				t.Fatal("pending SQL executed before full validation", err)
			}
			if !reflect.DeepEqual(before, ledgerSnapshot(t, s)) {
				t.Fatal("invalid ledger repaired/changed")
			}
		})
	}
}
func TestMigrationLockTimeoutAndRecovery(t *testing.T) {
	s := testStore(t)
	holder, err := s.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(holder)
	if _, err := holder.Exec(t.Context(), `SELECT pg_advisory_xact_lock($1)`, int64(migrationLockID)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
	defer cancel()
	err = s.Migrate(ctx)
	var failure *MigrationError
	if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &failure) || failure.Stage != "acquire lock" {
		t.Fatal("lock timeout not propagated", err)
	}
	if err := holder.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatal("retry after lock timeout", err)
	}
	if err := s.VerifySchema(t.Context()); err != nil {
		t.Fatal(err)
	}
}
func TestCancellationDuringSQLRollsBack(t *testing.T) {
	s := testStore(t)
	files := fstest.MapFS{"migrations/0001_cancel.up.sql": {Data: []byte("CREATE TABLE migration_cancel_probe(id int); SELECT pg_sleep(10) /* migration_cancel_probe */;")}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.migrate(ctx, files) }()
	wait, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	// Synchronize on PostgreSQL executing the sleep, rather than guessing timings.
	for {
		var sleeping bool
		if err := s.pool.QueryRow(wait, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE query LIKE '%migration_cancel_probe%' AND wait_event='PgSleep')`).Scan(&sleeping); err != nil {
			cancel()
			<-done
			t.Fatal(err)
		}
		if sleeping {
			break
		}
		select {
		case err := <-done:
			t.Fatal("migration exited before cancellation", err)
		case <-wait.Done():
			cancel()
			<-done
			t.Fatal("SQL did not begin")
		case <-time.After(time.Millisecond * 5):
		}
	}
	cancel()
	err := <-done
	var failure *MigrationError
	if !errors.Is(err, context.Canceled) || !errors.As(err, &failure) || failure.Stage != "execute SQL" {
		t.Fatal("SQL cancellation not propagated", err)
	}
	for _, table := range []string{"migration_cancel_probe", "schema_migrations"} {
		var exists bool
		if err := s.pool.QueryRow(t.Context(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil || exists {
			t.Fatal("cancelled batch survived", table, err)
		}
	}
	if err := s.Migrate(t.Context()); err != nil {
		t.Fatal("cancelled transaction retained lock", err)
	}
}
func TestSeparatePoolsSerializeMigrationExecution(t *testing.T) {
	s := testStore(t)
	pool, err := pgxpool.NewWithConfig(t.Context(), s.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	peer := &Store{pool: pool}
	files := fstest.MapFS{"migrations/0001_once.up.sql": {Data: []byte("CREATE SEQUENCE migration_once; SELECT nextval('migration_once');")}}
	done := make(chan error, 2)
	start := make(chan struct{})
	for _, runner := range []*Store{s, peer} {
		go func() { <-start; done <- runner.migrate(t.Context(), files) }()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	var executions int
	if err := s.pool.QueryRow(t.Context(), `SELECT last_value FROM migration_once`).Scan(&executions); err != nil || executions != 1 {
		t.Fatal("migration executed more than once", executions, err)
	}
	if len(ledgerSnapshot(t, s)) != 1 {
		t.Fatal("duplicate ledger")
	}
}
