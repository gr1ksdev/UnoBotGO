// Package postgres implements durable V2 storage. It is never imported by the UNO engine.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"strings"
	"time"
)

var ErrConnection = errors.New("postgres: connection unavailable (check DATABASE_URL and database service)")

// Store owns its pool; callers must Close after all operations finish.
type Store struct{ pool *pgxpool.Pool }

// Open checks connectivity, but does not apply migrations. Errors deliberately
// omit driver messages, which may contain credentials or connection parameters.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("postgres: DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("postgres: invalid DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, ErrConnection
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, ErrConnection
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// operationError preserves cancellation without exposing SQL parameters/DSNs.
func operationError(ctx context.Context, operation string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("postgres: %s: %w", operation, err)
	}
	return fmt.Errorf("postgres: %s failed", operation)
}

// Rollback still runs when the operation context has expired, but cleanup itself
// must not hold a finalization task indefinitely on a broken connection.
func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
