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
	if c.DefaultGameMode != groups.Caseiro || c.RankingSystem != groups.Updated || c.Revision != 1 {
		t.Fatalf("wrong defaults %+v", c)
	}
	changed, err := s.SetDefaultGameMode(ctx, 42, groups.Classic)
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.GetOrCreateGroupConfig(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultGameMode != groups.Classic || c.RankingSystem != groups.Updated || c.Revision != 2 {
		t.Fatalf("not persisted %+v", c)
	}
	again, err := s.SetDefaultGameMode(ctx, 42, groups.Classic)
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
	// Reproduce schema and ledger prior to migration 0008.
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

	// Caso B (pré-0008): grupo histórico criado com o default anterior (Legacy).
	beforeLegacy, err := s.GetOrCreateGroupConfig(ctx, 100)
	if err != nil || beforeLegacy.RankingSystem != groups.Legacy {
		t.Fatal("historical legacy group fixture", err)
	}

	// Caso C (pré-0008): grupo histórico configurado explicitamente com Updated.
	beforeUpdated, err := s.SetRankingSystem(ctx, 200, groups.Updated)
	if err != nil || beforeUpdated.RankingSystem != groups.Updated {
		t.Fatal("historical updated group fixture", err)
	}

	// Aplicar migration 0008.
	if err := s.Migrate(ctx); err != nil {
		t.Fatal("migrate 0008", err)
	}

	// Caso B pós-0008: grupo Legacy existente DEVE permanecer Legacy com mesma revisão e modo.
	afterLegacy, err := s.GetOrCreateGroupConfig(ctx, 100)
	if err != nil || afterLegacy.RankingSystem != groups.Legacy || afterLegacy.Revision != beforeLegacy.Revision || afterLegacy.DefaultGameMode != beforeLegacy.DefaultGameMode {
		t.Fatalf("existing legacy group was converted or altered: before=%+v after=%+v", beforeLegacy, afterLegacy)
	}

	// Caso C pós-0008: grupo Updated existente DEVE permanecer Updated com mesma revisão e modo.
	afterUpdated, err := s.GetOrCreateGroupConfig(ctx, 200)
	if err != nil || afterUpdated.RankingSystem != groups.Updated || afterUpdated.Revision != beforeUpdated.Revision || afterUpdated.DefaultGameMode != beforeUpdated.DefaultGameMode {
		t.Fatalf("existing updated group was altered: before=%+v after=%+v", beforeUpdated, afterUpdated)
	}

	// Caso A pós-0008: novos grupos criados DEVEM assumir Updated por padrão em todas as vias.
	created, err := s.GetOrCreateGroupConfig(ctx, 101)
	if err != nil || created.RankingSystem != groups.Updated || created.Revision != 1 {
		t.Fatalf("new group default: %+v", created)
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

	// Idempotência: rodar Migrate novamente não altera nada e novos grupos continuam Updated.
	if err := s.Migrate(ctx); err != nil {
		t.Fatal("re-migrate error", err)
	}
	stableLegacy, err := s.GetOrCreateGroupConfig(ctx, 100)
	if err != nil || stableLegacy.RankingSystem != groups.Legacy || stableLegacy.Revision != beforeLegacy.Revision {
		t.Fatalf("re-migrate altered legacy group: %+v", stableLegacy)
	}
	stableUpdated, err := s.GetOrCreateGroupConfig(ctx, 200)
	if err != nil || stableUpdated.RankingSystem != groups.Updated || stableUpdated.Revision != beforeUpdated.Revision {
		t.Fatalf("re-migrate altered updated group: %+v", stableUpdated)
	}
	postRemigrate, err := s.GetOrCreateGroupConfig(ctx, 105)
	if err != nil || postRemigrate.RankingSystem != groups.Updated {
		t.Fatalf("group created after re-migrate: %+v", postRemigrate)
	}
}
