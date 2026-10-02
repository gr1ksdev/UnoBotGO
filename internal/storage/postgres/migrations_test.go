package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMigrationDiscoveryOrderAndChecksum(t *testing.T) {
	files := fstest.MapFS{
		"migrations/0003_third.up.sql":  {Data: []byte("SELECT 3;")},
		"migrations/0001_first.up.sql":  {Data: []byte("SELECT 1;")},
		"migrations/0002_second.up.sql": {Data: []byte("SELECT 2;")},
	}
	list, err := readMigrations(files)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, m := range list {
		names = append(names, m.name)
		hash := sha256.Sum256([]byte(m.sql))
		if m.checksum != hex.EncodeToString(hash[:]) {
			t.Fatal("wrong checksum")
		}
	}
	if !reflect.DeepEqual(names, []string{"migrations/0001_first.up.sql", "migrations/0002_second.up.sql", "migrations/0003_third.up.sql"}) {
		t.Fatal(names)
	}
	production, err := migrations()
	if err != nil || len(production) == 0 {
		t.Fatal("embedded SQL discovery", err)
	}
}
func TestMigrationDiscoveryRejectsInvalidOrDuplicateVersions(t *testing.T) {
	for _, names := range [][]string{{}, {"migrations/0000_zero.up.sql"}, {"migrations/1_short.up.sql"}, {"migrations/000x_bad.up.sql"}, {"migrations/0001_one.up.sql", "migrations/0001_duplicate.up.sql"}} {
		t.Run(fmt.Sprint(names), func(t *testing.T) {
			files := fstest.MapFS{}
			for _, name := range names {
				files[name] = &fstest.MapFile{Data: []byte("SELECT 1;")}
			}
			if _, err := readMigrations(files); !errors.Is(err, ErrSchema) {
				t.Fatal("invalid discovery accepted", err)
			}
		})
	}
}
func TestLedgerValidationAndPendingOrder(t *testing.T) {
	list, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, 2, len(list)} {
		applied := map[string]string{}
		for _, m := range list[:count] {
			applied[m.name] = m.checksum
		}
		pending, err := pendingMigrations(list, applied)
		if err != nil || !reflect.DeepEqual(pending, list[count:]) {
			t.Fatalf("count%d: %v %v", count, pending, err)
		}
		if len(applied) != count {
			t.Fatal("validation modified ledger snapshot")
		}
	}
	for _, tc := range []struct {
		name    string
		applied map[string]string
		bad     string
	}{
		{"checksum", map[string]string{list[0].name: "tampered"}, list[0].name},
		{"unknown", map[string]string{"migrations/9999_unknown.up.sql": "any"}, "migrations/9999_unknown.up.sql"},
		{"gap", map[string]string{list[1].name: list[1].checksum}, list[1].name},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pendingMigrations(list, tc.applied)
			var failure *MigrationError
			if !errors.Is(err, ErrSchema) || !errors.As(err, &failure) || failure.Stage != "validate ledger" || failure.Migration != tc.bad {
				t.Fatal(err)
			}
		})
	}
}
func TestMigrationErrorPreservesCauseAndRedactsSensitiveDetails(t *testing.T) {
	pg := &pgconn.PgError{Code: "42601", Message: "syntax error at or near THIS", Detail: "secret-row-value", Hint: "private-hint", Where: "sensitive-query"}
	err := migrationError(context.Background(), "execute SQL", "migrations/0002_invalid.up.sql", pg)
	var cause *pgconn.PgError
	if !errors.As(err, &cause) || cause != pg || !strings.Contains(err.Error(), "42601") || !strings.Contains(err.Error(), "syntax error") {
		t.Fatal("original PostgreSQL cause lost", err)
	}
	for _, private := range []string{pg.Detail, pg.Hint, pg.Where} {
		if strings.Contains(err.Error(), private) {
			t.Fatal("sensitive details logged")
		}
	}
	raw := errors.New("postgres://user:password@host/db")
	wrapped := migrationError(context.Background(), "begin transaction", "", raw)
	if !errors.Is(wrapped, raw) || strings.Contains(wrapped.Error(), raw.Error()) {
		t.Fatal("connection cause lost or leaked")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wrapped = migrationError(ctx, "acquire lock", "", pg)
	if !errors.Is(wrapped, context.Canceled) || !errors.As(wrapped, &cause) {
		t.Fatal("cancellation/original cause lost")
	}
	wrapped = &MigrationError{Stage: "read SQL", Err: fs.ErrNotExist}
	if !errors.Is(wrapped, fs.ErrNotExist) {
		t.Fatal(wrapped)
	}
}
