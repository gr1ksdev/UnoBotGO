//go:build integration

package postgres

import (
	"github.com/malbs/UnoGoBot/internal/groups"
	"testing"
)

func TestGroupDefaultsAndPersistence(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c, err := s.GetOrCreateGroupConfig(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultGameMode != groups.Classic || c.RankingSystem != groups.Updated || c.Revision != 1 {
		t.Fatalf("wrong defaults %+v", c)
	}
	changed, err := s.SetDefaultGameMode(ctx, 42, groups.Caseiro)
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.GetOrCreateGroupConfig(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultGameMode != groups.Caseiro || c.RankingSystem != groups.Updated || c.Revision != 2 {
		t.Fatalf("not persisted %+v", c)
	}
	again, err := s.SetDefaultGameMode(ctx, 42, groups.Caseiro)
	if err != nil {
		t.Fatal(err)
	}
	if again.Revision != changed.Revision {
		t.Fatal("no-op changed revision")
	}

	// SetRankingSystem
	rankChanged, err := s.SetRankingSystem(ctx, 42, groups.Legacy)
	if err != nil {
		t.Fatal(err)
	}
	if rankChanged.RankingSystem != groups.Legacy || rankChanged.Revision != 3 {
		t.Fatalf("unexpected rank config: %+v", rankChanged)
	}

	rankAgain, err := s.SetRankingSystem(ctx, 42, groups.Legacy)
	if err != nil {
		t.Fatal(err)
	}
	if rankAgain.Revision != rankChanged.Revision {
		t.Fatal("no-op ranking system changed revision")
	}

	// SetInstalledBy
	withInstaller, err := s.SetInstalledBy(ctx, 42, 12345)
	if err != nil {
		t.Fatal(err)
	}
	if withInstaller.InstalledByUserID == nil || *withInstaller.InstalledByUserID != 12345 {
		t.Fatalf("unexpected installer: %+v", withInstaller)
	}
}

func TestUpdatedDefaultMigrationPreservesExistingGroups(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	list, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `CREATE TABLE schema_migrations(version text PRIMARY KEY, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	// Reproduce the prior schema and ledger, then create a historical Legacy group.
	for _, m := range list {
		if m.name == "migrations/0008_default_ranking_updated.up.sql" {
			break
		}
		if _, err := s.pool.Exec(ctx, m.sql); err != nil {
			t.Fatal(err)
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, m.name, m.checksum); err != nil {
			t.Fatal(err)
		}
	}
	before, err := s.GetOrCreateGroupConfig(ctx, 100)
	if err != nil || before.RankingSystem != groups.Legacy {
		t.Fatal("historical group fixture", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetOrCreateGroupConfig(ctx, 100)
	if err != nil || after.RankingSystem != groups.Legacy || after.Revision != before.Revision {
		t.Fatal("existing group was converted", err)
	}
	created, err := s.GetOrCreateGroupConfig(ctx, 101)
	if err != nil || created.RankingSystem != groups.Updated {
		t.Fatal("new default", err)
	}
	if _, err := s.SetInstalledBy(ctx, 102, 123); err != nil {
		t.Fatal(err)
	}
	if err := s.ObserveGroupTitle(ctx, 103, "Novo grupo"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetDefaultGameMode(ctx, 104, groups.Caseiro); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{102, 103, 104} {
		c, err := s.GetOrCreateGroupConfig(ctx, id)
		if err != nil || c.RankingSystem != groups.Updated {
			t.Fatalf("creation path %d: %v", id, err)
		}
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
}
