//go:build integration

package postgres

import (
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/rankingimport"
	"testing"
)

func TestImportStagingIdempotencyPreservesDuplicateEntries(t *testing.T) {
	s := prepareRanking(t, groups.Legacy)
	source, err := rankingimport.New(42, 1, "Gabriel - 25\nGabriel - 12\n🎮 Nome - 21\ninvalid")
	if err != nil {
		t.Fatal(err)
	}
	staged, err := s.StageRankingImport(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	source.CreatedBy = 2
	retry, err := s.StageRankingImport(t.Context(), source)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID != staged.ID || retry.CreatedBy != 1 || len(staged.Entries) != 4 || retry.Entries[0].ID != staged.Entries[0].ID || staged.Entries[0].ID == staged.Entries[1].ID {
		t.Fatal("idempotency/duplicates failed")
	}
	var n int
	if err = s.pool.QueryRow(t.Context(), `SELECT count(*) FROM player_group_stats`).Scan(&n); err != nil || n != 0 {
		t.Fatal("staging awarded points", n, err)
	}
	if err = s.pool.QueryRow(t.Context(), `SELECT count(*) FROM ranking_import_entries`).Scan(&n); err != nil || n != 4 {
		t.Fatal("retry duplicated entries", n, err)
	}
}
