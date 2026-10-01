package app

import (
	"context"
	"errors"
	"github.com/malbs/UnoGoBot/internal/config"
	"reflect"
	"testing"
	"time"
)

type migrator struct {
	events *[]string
	fail   string
}

func (m migrator) Migrate(context.Context) error {
	*m.events = append(*m.events, "migrate")
	if m.fail == "migrate" {
		return errors.New("migration failed")
	}
	return nil
}
func (m migrator) VerifySchema(context.Context) error {
	*m.events = append(*m.events, "verify")
	if m.fail == "verify" {
		return errors.New("checksum failed")
	}
	return nil
}
func TestStartupFailClosed(t *testing.T) {
	for _, fail := range []string{"", "migrate", "verify"} {
		t.Run(fail, func(t *testing.T) {
			events := []string{}
			err := Initialize(t.Context(), migrator{&events, fail}, func(context.Context) error { events = append(events, "http", "bot", "workers"); return nil })
			want := []string{"migrate", "verify", "http", "bot", "workers"}
			if fail == "migrate" {
				want = want[:1]
			} else if fail == "verify" {
				want = want[:2]
			}
			if !reflect.DeepEqual(events, want) || (err != nil) != (fail != "") {
				t.Fatal(events, err)
			}
		})
	}
}

type deadlineMigrator struct {
	deadline     time.Time
	block        bool
	verifyCalled bool
}

func (m *deadlineMigrator) Migrate(ctx context.Context) error {
	m.deadline, _ = ctx.Deadline()
	if m.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (m *deadlineMigrator) VerifySchema(context.Context) error { m.verifyCalled = true; return nil }
func TestStartupMigrationDeadlineAndApplicationContext(t *testing.T) {
	m := &deadlineMigrator{}
	start := time.Now()
	called := false
	if err := Initialize(t.Context(), m, func(ctx context.Context) error {
		called = true
		if ctx != t.Context() || ctx.Err() != nil {
			t.Fatal("functional startup inherited cancelled migration context")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	remaining := m.deadline.Sub(start)
	if !called || !m.verifyCalled || remaining < config.MigrationTimeout-time.Second || remaining > config.MigrationTimeout+time.Second {
		t.Fatal("wrong migration deadline", remaining)
	}
}
func TestStartupMigrationTimeoutStopsFunctionalStartup(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	m := &deadlineMigrator{block: true}
	called := false
	err := Initialize(ctx, m, func(context.Context) error { called = true; return nil })
	if !errors.Is(err, context.DeadlineExceeded) || called || m.verifyCalled {
		t.Fatal("timeout continued startup", err)
	}
}
func TestStartupAlreadyCancelledDoesNotMigrate(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	events := []string{}
	err := Initialize(ctx, migrator{events: &events}, func(context.Context) error { t.Fatal("started cancelled application"); return nil })
	if !errors.Is(err, context.Canceled) || len(events) != 0 {
		t.Fatal(events, err)
	}
}
