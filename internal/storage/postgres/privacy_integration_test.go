//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
)

func TestPrivacyPersistenceAndRankingComposition(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	ctx := t.Context()
	at := time.Date(2026, 9, 15, 12, 0, 0, 0, ranking.RankingLocation)

	// 1. User privacy persistence
	p, err := s.GetUserRankingPrivacy(ctx, 1)
	if err != nil || p != false {
		t.Fatalf("expected default false for user 1, got %v, err %v", p, err)
	}

	// Toggle user 1
	p, err = s.ToggleUserRankingPrivacy(ctx, 1)
	if err != nil || p != true {
		t.Fatalf("expected toggled true for user 1, got %v, err %v", p, err)
	}
	p, _ = s.GetUserRankingPrivacy(ctx, 1)
	if !p {
		t.Fatalf("expected user 1 to be private")
	}

	// 2. Group privacy persistence
	cfg, err := s.SetRankingPrivate(ctx, 42, true)
	if err != nil || !cfg.RankingPrivate {
		t.Fatalf("expected group 42 to be private, got %+v, err %v", cfg, err)
	}

	// Seed games in group 42 (private group) and group 43 (public group)
	record := func(chat int64, system groups.RankingSystem, id string, at time.Time) {
		t.Helper()
		r := eligibleResult(t, system, 2, 1, "completed")
		r.ChatID = chat
		r.GameID = id
		r.StartedAt = at.Add(-time.Hour)
		r.FinishedAt = at
		r.Players[0].UserID = 1 // Private user
		r.Players[0].DisplayName = "Alice"
		r.Players[1].UserID = 2 // Public user
		r.Players[1].DisplayName = "Bob"
		if _, err := s.RecordCompletedGame(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// Ensure group configs
	_, _ = s.GetOrCreateGroupConfig(ctx, 42)
	_, _ = s.pool.Exec(ctx, `UPDATE group_configs SET ranking_system='updated', title='Grupo Secreto', ranking_private=true WHERE chat_id=42`)
	_, _ = s.GetOrCreateGroupConfig(ctx, 43)
	_, _ = s.pool.Exec(ctx, `UPDATE group_configs SET ranking_system='updated', title='Grupo Publico', ranking_private=false WHERE chat_id=43`)

	record(42, groups.Updated, "G1", at)
	record(43, groups.Updated, "G2", at)

	// 3. Test Global Groups Ranking
	req := ranking.GlobalRequest{Kind: "groups", System: groups.Updated, Month: ranking.MonthStart(at), Limit: 50}
	page, err := s.ReadGlobalRanking(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(page.Rows))
	}
	for _, row := range page.Rows {
		if row.ID == 42 {
			if !row.Anonymous {
				t.Fatalf("group 42 should have Anonymous: true")
			}
			// Underlying name and score must remain intact for ordering/ranking
			if row.Score != 1000 {
				t.Fatalf("group 42 score should be 1000, got %d", row.Score)
			}
		}
		if row.ID == 43 {
			if row.Anonymous {
				t.Fatalf("group 43 should have Anonymous: false")
			}
		}
	}

	// 4. Test Global Players Ranking
	req.Kind = "players"
	page, err = s.ReadGlobalRanking(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 2 {
		t.Fatalf("expected 2 players, got %d", len(page.Rows))
	}
	for _, row := range page.Rows {
		if row.ID == 1 {
			// User 1 is private -> Anonymous: true
			if !row.Anonymous {
				t.Fatalf("user 1 must have Anonymous: true")
			}
		}
		if row.ID == 2 {
			// User 2 is public -> Anonymous: false
			if row.Anonymous {
				t.Fatalf("user 2 must have Anonymous: false")
			}
		}
	}

	// 5. Test Group Detail for Private Group (Chat 42)
	// Rule: If group is private, the group AND ALL its players in detail must be Anonymous: true!
	req.Kind = "detail"
	req.GroupID = 42
	detail, err := s.ReadGlobalRanking(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Group == nil || !detail.Group.Anonymous {
		t.Fatalf("private group header must have Anonymous: true")
	}
	if len(detail.Rows) != 2 {
		t.Fatalf("expected 2 rows in detail, got %d", len(detail.Rows))
	}
	for _, row := range detail.Rows {
		if !row.Anonymous {
			t.Fatalf("all players in private group detail must have Anonymous: true, got %+v", row)
		}
	}

	// 6. Test Group Detail for Public Group (Chat 43)
	// Rule: If group is public, group is Anonymous: false, and each player respects their own privacy
	req.GroupID = 43
	detail, err = s.ReadGlobalRanking(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Group == nil || detail.Group.Anonymous {
		t.Fatalf("public group header must have Anonymous: false")
	}
	for _, row := range detail.Rows {
		if row.ID == 1 && !row.Anonymous {
			t.Fatalf("user 1 in public group detail must have Anonymous: true (respecting personal setting)")
		}
		if row.ID == 2 && row.Anonymous {
			t.Fatalf("user 2 in public group detail must have Anonymous: false")
		}
	}

	// 7. Check IsEntityAnonymous helper
	isAnon, err := s.IsEntityAnonymous(ctx, "group", 42)
	if err != nil || !isAnon {
		t.Fatalf("expected group 42 to be anonymous")
	}
	isAnon, err = s.IsEntityAnonymous(ctx, "group", 43)
	if err != nil || isAnon {
		t.Fatalf("expected group 43 to not be anonymous")
	}
	isAnon, err = s.IsEntityAnonymous(ctx, "user", 1)
	if err != nil || !isAnon {
		t.Fatalf("expected user 1 to be anonymous")
	}
	isAnon, err = s.IsEntityAnonymous(ctx, "user", 2)
	if err != nil || isAnon {
		t.Fatalf("expected user 2 to not be anonymous")
	}
}
