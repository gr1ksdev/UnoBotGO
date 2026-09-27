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
	if c.DefaultGameMode != groups.Classic || c.RankingSystem != groups.Legacy || c.Revision != 1 {
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
	if c.DefaultGameMode != groups.Caseiro || c.RankingSystem != groups.Legacy || c.Revision != 2 {
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
	rankChanged, err := s.SetRankingSystem(ctx, 42, groups.Updated)
	if err != nil {
		t.Fatal(err)
	}
	if rankChanged.RankingSystem != groups.Updated || rankChanged.Revision != 3 {
		t.Fatalf("unexpected rank config: %+v", rankChanged)
	}

	rankAgain, err := s.SetRankingSystem(ctx, 42, groups.Updated)
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
