package ranking

import (
	"testing"
	"time"
)

func TestRankingTimeMonthNamesAndTransitions(t *testing.T) {
	// 2026-09-30 23:59:59 in Sao Paulo (UTC-3: 2026-10-01 02:59:59 UTC)
	sepEnd := time.Date(2026, time.September, 30, 23, 59, 59, 999999000, RankingLocation)
	if name := MonthName(sepEnd); name != "Setembro" {
		t.Fatalf("expected Setembro, got %s", name)
	}
	if start := MonthStart(sepEnd); start.Year() != 2026 || start.Month() != time.September || start.Day() != 1 {
		t.Fatalf("expected 2026-09-01, got %v", start)
	}
	if str := MonthDateString(sepEnd); str != "2026-09-01" {
		t.Fatalf("expected 2026-09-01, got %s", str)
	}

	// Equivalent UTC timestamp: 2026-10-01 02:59:59 UTC is still September in SP
	sepEndUTC := time.Date(2026, time.October, 1, 2, 59, 59, 0, time.UTC)
	if name := MonthName(sepEndUTC); name != "Setembro" {
		t.Fatalf("expected Setembro for UTC equivalent, got %s", name)
	}
	if str := MonthDateString(sepEndUTC); str != "2026-09-01" {
		t.Fatalf("expected 2026-09-01 for UTC equivalent, got %s", str)
	}

	// 2026-10-01 00:00:00 in Sao Paulo (UTC-3: 2026-10-01 03:00:00 UTC)
	octStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, RankingLocation)
	if name := MonthName(octStart); name != "Outubro" {
		t.Fatalf("expected Outubro, got %s", name)
	}
	if str := MonthDateString(octStart); str != "2026-10-01" {
		t.Fatalf("expected 2026-10-01, got %s", str)
	}

	// All 12 months check
	expected := []string{
		"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
		"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro",
	}
	for m := 1; m <= 12; m++ {
		ts := time.Date(2026, time.Month(m), 15, 12, 0, 0, 0, RankingLocation)
		if MonthName(ts) != expected[m-1] {
			t.Errorf("month %d expected %s, got %s", m, expected[m-1], MonthName(ts))
		}
	}
}
