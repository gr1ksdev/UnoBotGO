package postgres

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.up.sql
var migrationFiles embed.FS

var ErrSchema = errors.New("postgres: schema missing or incompatible; run go run ./cmd/migrate")

type migration struct{ name, sql, checksum string }

func migrations() ([]migration, error) {
	names, err := fs.Glob(migrationFiles, "migrations/*.up.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	result := make([]migration, 0, len(names))
	for _, name := range names {
		data, err := migrationFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(data)
		result = append(result, migration{name: name, sql: string(data), checksum: hex.EncodeToString(hash[:])})
	}
	return result, nil
}

// Migrate applies all pending migrations atomically, serializing competing
// migrators with a transaction-scoped advisory lock. There is no automatic DDL
// in bot startup and no destructive down migration.
func (s *Store) Migrate(ctx context.Context) error {
	list, err := migrations()
	if err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return operationError(ctx, "begin migrations")
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71870101)`); err != nil {
		return operationError(ctx, "lock migrations")
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return operationError(ctx, "create migration ledger")
	}
	rows, err := tx.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return operationError(ctx, "read migrations")
	}
	applied := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err = rows.Scan(&name, &checksum); err != nil {
			rows.Close()
			return operationError(ctx, "read migrations")
		}
		applied[name] = checksum
	}
	rows.Close()
	if rows.Err() != nil {
		return operationError(ctx, "read migrations")
	}
	for _, m := range list {
		if checksum, ok := applied[m.name]; ok {
			if checksum != m.checksum {
				return ErrSchema
			}
			delete(applied, m.name)
			continue
		}
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return operationError(ctx, "apply "+m.name)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, m.name, m.checksum); err != nil {
			return operationError(ctx, "record migration")
		}
	}
	if len(applied) > 0 {
		return ErrSchema
	}
	if err = tx.Commit(ctx); err != nil {
		return operationError(ctx, "commit migrations")
	}
	return nil
}

// VerifySchema requires exactly the known migrations and their original checksums.
func (s *Store) VerifySchema(ctx context.Context) error {
	list, err := migrations()
	if err != nil {
		return err
	}
	rows, err := s.pool.Query(ctx, `SELECT version,checksum FROM schema_migrations`)
	if err != nil {
		return ErrSchema
	}
	defer rows.Close()
	expected := map[string]string{}
	for _, m := range list {
		expected[m.name] = m.checksum
	}
	count := 0
	for rows.Next() {
		var name, checksum string
		if err = rows.Scan(&name, &checksum); err != nil {
			return ErrSchema
		}
		if expected[name] != checksum {
			return fmt.Errorf("%w: migration checksum/version", ErrSchema)
		}
		count++
	}
	if rows.Err() != nil {
		return operationError(ctx, "verify schema")
	}
	if count != len(list) {
		return ErrSchema
	}
	return nil
}
