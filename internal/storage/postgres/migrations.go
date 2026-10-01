package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:embed migrations/*.up.sql
var migrationFiles embed.FS

var ErrSchema = errors.New("postgres: schema missing or incompatible; check migration ledger and checksums")

const migrationLockID = 71870101

var migrationName = regexp.MustCompile(`^migrations/([0-9]{4})_[A-Za-z0-9_-]+\.up\.sql$`)

type migration struct{ name, sql, checksum string }

// MigrationError exposes the failing stage and migration while retaining the
// original cause for errors.Is/As. Its log representation omits driver connection
// strings, SQL parameters and PostgreSQL Detail/Hint/Where fields.
type MigrationError struct {
	Stage     string
	Migration string
	Err       error
}

func (e *MigrationError) Unwrap() error { return e.Err }
func (e *MigrationError) Error() string {
	cause := "database operation failed"
	switch {
	case errors.Is(e.Err, context.DeadlineExceeded):
		cause = context.DeadlineExceeded.Error()
	case errors.Is(e.Err, context.Canceled):
		cause = context.Canceled.Error()
	case errors.Is(e.Err, ErrSchema):
		cause = e.Err.Error()
	default:
		var pg *pgconn.PgError
		if errors.As(e.Err, &pg) {
			cause = fmt.Sprintf("SQLSTATE %s: %s", pg.Code, pg.Message)
		}
	}
	name := ""
	if e.Migration != "" {
		name = " migration=" + e.Migration
	}
	return fmt.Sprintf("postgres: migrations stage=%s%s: %s", e.Stage, name, cause)
}
func migrationError(ctx context.Context, stage, name string, err error) error {
	if ctx.Err() != nil {
		err = errors.Join(ctx.Err(), err)
	}
	return &MigrationError{Stage: stage, Migration: name, Err: err}
}

func migrations() ([]migration, error) { return readMigrations(migrationFiles) }

// The filesystem seam is internal: production always uses embedded, immutable SQL.
func readMigrations(files fs.FS) ([]migration, error) {
	names, err := fs.Glob(files, "migrations/*.up.sql")
	if err != nil {
		return nil, &MigrationError{Stage: "discover SQL", Err: err}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, &MigrationError{Stage: "discover SQL", Err: fmt.Errorf("%w: no embedded migrations", ErrSchema)}
	}
	versions := map[string]bool{}
	result := make([]migration, 0, len(names))
	for _, name := range names {
		match := migrationName.FindStringSubmatch(name)
		if match == nil || match[1] == "0000" {
			return nil, &MigrationError{Stage: "validate SQL names", Migration: name, Err: fmt.Errorf("%w: invalid migration version/name", ErrSchema)}
		}
		if versions[match[1]] {
			return nil, &MigrationError{Stage: "validate SQL names", Migration: name, Err: fmt.Errorf("%w: duplicate migration version", ErrSchema)}
		}
		versions[match[1]] = true
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, &MigrationError{Stage: "read SQL", Migration: name, Err: err}
		}
		hash := sha256.Sum256(data)
		result = append(result, migration{name: name, sql: string(data), checksum: hex.EncodeToString(hash[:])})
	}
	return result, nil
}

// Validate the ENTIRE ledger before any pending SQL runs. Applied versions must
// form a prefix of the embedded sequence; never repair checksums or downgrade.
func pendingMigrations(list []migration, applied map[string]string) ([]migration, error) {
	expected := make(map[string]migration, len(list))
	for _, m := range list {
		expected[m.name] = m
	}
	names := make([]string, 0, len(applied))
	for name := range applied {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		m, ok := expected[name]
		if !ok {
			return nil, &MigrationError{Stage: "validate ledger", Migration: name, Err: fmt.Errorf("%w: unknown applied migration", ErrSchema)}
		}
		if applied[name] != m.checksum {
			return nil, &MigrationError{Stage: "validate ledger", Migration: name, Err: fmt.Errorf("%w: applied migration modified (checksum mismatch)", ErrSchema)}
		}
	}
	pending := []migration{}
	for _, m := range list {
		if _, ok := applied[m.name]; ok {
			if len(pending) > 0 {
				return nil, &MigrationError{Stage: "validate ledger", Migration: m.name, Err: fmt.Errorf("%w: applied versions are not an ordered prefix", ErrSchema)}
			}
		} else {
			pending = append(pending, m)
		}
	}
	return pending, nil
}

type ledgerReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readMigrationLedger(ctx context.Context, db ledgerReader) (map[string]string, error) {
	rows, err := db.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, migrationError(ctx, "read ledger", "", err)
	}
	defer rows.Close()
	applied := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			return nil, migrationError(ctx, "read ledger", "", err)
		}
		if _, exists := applied[name]; exists {
			return nil, migrationError(ctx, "read ledger", name, fmt.Errorf("%w: duplicate ledger version", ErrSchema))
		}
		applied[name] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, migrationError(ctx, "read ledger", "", err)
	}
	return applied, nil
}

// Migrate applies all pending migrations in ONE transaction, serializing runners
// across processes with the existing transaction-scoped advisory lock. Ledger
// changes and the entire SQL batch roll back together. There are no down migrations.
func (s *Store) Migrate(ctx context.Context) error { return s.migrate(ctx, migrationFiles) }
func (s *Store) migrate(ctx context.Context, files fs.FS) (err error) {
	logger := slog.Default()
	logger.InfoContext(ctx, "migrations: checking")
	defer func() {
		if err != nil {
			fields := []any{"error", err}
			var failure *MigrationError
			if errors.As(err, &failure) {
				fields = append(fields, "stage", failure.Stage, "migration", failure.Migration)
			}
			logger.ErrorContext(ctx, "migrations: failed", fields...)
		}
	}()
	if ctx.Err() != nil {
		return migrationError(ctx, "before transaction", "", ctx.Err())
	}
	list, err := readMigrations(files)
	if err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return migrationError(ctx, "begin transaction", "", err)
	}
	defer rollback(tx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(migrationLockID)); err != nil {
		return migrationError(ctx, "acquire lock", "", err)
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return migrationError(ctx, "create ledger", "", err)
	}
	applied, err := readMigrationLedger(ctx, tx)
	if err != nil {
		return err
	}
	pending, err := pendingMigrations(list, applied)
	if err != nil {
		return err
	}
	for _, m := range pending {
		logger.InfoContext(ctx, "migrations: applying", "migration", m.name)
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return migrationError(ctx, "execute SQL", m.name, err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, m.name, m.checksum); err != nil {
			return migrationError(ctx, "record ledger", m.name, err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return migrationError(ctx, "commit transaction", "", err)
	}
	if len(pending) == 0 {
		logger.InfoContext(ctx, "migrations: up to date")
	} else {
		logger.InfoContext(ctx, "migrations: complete", "applied", len(pending))
	}
	return nil
}

// VerifySchema requires exactly the known versions and original checksums.
func (s *Store) VerifySchema(ctx context.Context) error {
	list, err := migrations()
	if err != nil {
		return err
	}
	applied, err := readMigrationLedger(ctx, s.pool)
	if err != nil {
		// Keep the established ErrSchema contract for a missing/incompatible ledger.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrSchema, err)
	}
	pending, err := pendingMigrations(list, applied)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return &MigrationError{Stage: "verify schema", Migration: pending[0].name, Err: fmt.Errorf("%w: pending migration", ErrSchema)}
	}
	return nil
}
