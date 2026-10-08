//go:build integration

package postgres

import (
	"fmt"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"testing"
	"time"
)

func TestSharedInlineWebAppRankingAndProfile(t *testing.T) {
	for _, system := range []groups.RankingSystem{groups.Legacy, groups.Updated} {
		t.Run(string(system), func(t *testing.T) {
			s := prepareRanking(t, system)
			first := sampleResult("inline-result", system)
			second := sampleResult("webapp-result", system)
			second.Origin = "webapp"
			for _, r := range []ranking.Result{first, second, second} {
				if _, err := s.RecordCompletedGame(t.Context(), r); err != nil {
					t.Fatal(err)
				}
			}
			page, err := s.ReadGlobalRanking(t.Context(), ranking.GlobalRequest{Kind: "players", System: system, Month: ranking.MonthStart(first.FinishedAt), Limit: 10, LookupID: 1})
			if err != nil || len(page.Rows) != 1 {
				t.Fatal(err, page)
			}
			if page.Rows[0].Score != first.Players[0].Score*2 {
				t.Fatal("separate or duplicated ranking", page)
			}
			profile, err := s.ReadProfile(t.Context(), 1, nil)
			if err != nil || len(profile.History) != 2 || len(profile.Stats) != 1 || profile.Stats[0].Games != 2 {
				t.Fatal("duplicated history/stats", err, profile)
			}
			origins := map[string]bool{}
			for _, h := range profile.History {
				origins[h.Origin] = true
			}
			if !origins["inline"] || !origins["webapp"] {
				t.Fatal(origins)
			}
			if _, err = s.pool.Exec(t.Context(), `UPDATE group_configs SET ranking_private=true WHERE chat_id=42`); err != nil {
				t.Fatal(err)
			}
			private, err := s.ReadProfile(t.Context(), 1, nil)
			if err != nil || private.History[0].Group != "Grupo anônimo" {
				t.Fatal("private group name leak", err, private)
			}
			result, err := s.ReadResult(t.Context(), second.GameID, 999)
			if err != nil || len(result) != 0 {
				t.Fatal("other player's result leaked", err)
			}
		})
	}
}

func TestProfileKeysetDoesNotDuplicateWhenNewGameArrives(t *testing.T) {
	s := prepareRanking(t, groups.Updated)
	sample := sampleResult("history", groups.Updated)
	for i := 0; i < 25; i++ {
		r := sample.Clone()
		r.GameID = fmt.Sprintf("history-%02d", i)
		if _, err := s.RecordCompletedGame(t.Context(), r); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.ReadProfile(t.Context(), 1, nil)
	if err != nil || len(first.History) != 21 {
		t.Fatal(err, len(first.History))
	}
	last := first.History[19]
	newer := sample.Clone()
	newer.GameID = "new-arrival"
	newer.FinishedAt = newer.FinishedAt.Add(time.Second)
	if _, err := s.RecordCompletedGame(t.Context(), newer); err != nil {
		t.Fatal(err)
	}
	next, err := s.ReadProfile(t.Context(), 1, &ranking.HistoryKey{Finished: last.Finished, ID: last.ID})
	if err != nil || len(next.History) != 5 {
		t.Fatal(err, next)
	}
	seen := map[string]bool{}
	for _, h := range first.History[:20] {
		seen[h.ID] = true
	}
	for _, h := range next.History {
		if seen[h.ID] {
			t.Fatal("duplicate after pagination", h.ID)
		}
		seen[h.ID] = true
	}
	if len(seen) != 25 {
		t.Fatal("missing historical entries", len(seen))
	}
}
