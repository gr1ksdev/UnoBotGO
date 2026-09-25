package simulation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestExplainSpecialSteps(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want string
	}{
		{
			name: "skip",
			step: playStep(uno.Card{Color: uno.Red, Rank: uno.Skip}, StateSummary{}, StateSummary{}, uno.Event{Type: uno.PlayerSkipped, PlayerID: 2}),
			want: "perdeu a vez",
		},
		{
			name: "reverse two players",
			step: playStep(uno.Card{Color: uno.Blue, Rank: uno.Reverse}, StateSummary{}, StateSummary{}, uno.Event{Type: uno.PlayerSkipped, PlayerID: 2}),
			want: "funciona como bloqueio",
		},
		{
			name: "reverse direction",
			step: playStep(uno.Card{Color: uno.Green, Rank: uno.Reverse}, StateSummary{}, StateSummary{Direction: -1}, uno.Event{Type: uno.DirectionChanged, Direction: -1}),
			want: "anti-horário",
		},
		{
			name: "draw two stack",
			step: playStep(uno.Card{Color: uno.Yellow, Rank: uno.DrawTwo}, StateSummary{DrawCounter: 2}, StateSummary{DrawCounter: 4}),
			want: "empilhada",
		},
		{
			name: "wild",
			step: playStep(uno.Card{Rank: uno.Wild}, StateSummary{}, StateSummary{}),
			want: "escolher a nova cor",
		},
		{
			name: "wild draw four",
			step: playStep(uno.Card{Rank: uno.WildDrawFour}, StateSummary{}, StateSummary{}),
			want: "Coringa +4",
		},
		{
			name: "color choice",
			step: Step{Action: uno.Action{Type: uno.ChooseColor, PlayerID: 1, Color: uno.Blue}, After: StateSummary{DrawCounter: 4}},
			want: "Azul",
		},
		{
			name: "penalty",
			step: Step{Action: uno.Action{Type: uno.DrawCard, PlayerID: 1}, Before: StateSummary{DrawCounter: 6}, Events: []uno.Event{{Type: uno.CardsDrawn, PlayerID: 1, Count: 6}}},
			want: "penalidade de 6",
		},
		{
			name: "bluff caught",
			step: Step{Action: uno.Action{Type: uno.CallBluff, PlayerID: 2}, Events: []uno.Event{{Type: uno.BluffCalled, PlayerID: 2, TargetID: 1, Success: true, Count: 4}}},
			want: "acertou",
		},
		{
			name: "bluff failed",
			step: Step{Action: uno.Action{Type: uno.CallBluff, PlayerID: 2}, Events: []uno.Event{{Type: uno.BluffCalled, PlayerID: 2, TargetID: 1, Count: 6}}},
			want: "errou",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			explanations := ExplainStep(test.step)
			if len(explanations) != 1 || !strings.Contains(explanations[0], test.want) {
				t.Fatalf("explanations = %q, want substring %q", explanations, test.want)
			}
		})
	}
}

func TestReportContainsStatsSpecialsAndDiagnostics(t *testing.T) {
	result, err := Run(context.Background(), Config{
		Players: 4, Mode: ModeHouse, Seed: 20260924, MaxActions: DefaultMaxActions,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stats := CollectStats(result)
	if stats.BluffsCalled == 0 || stats.PenaltyEvents == 0 {
		t.Fatalf("expected bluff and penalty statistics, got %+v", stats)
	}
	report := RenderMarkdown(result)
	for _, want := range []string{
		"# Relatório da simulação UNO", "## Estatísticas gerais", "## Estatísticas por jogador",
		"**Fim:**", "**Tempo total da simulação:**", "## Histórico completo da partida",
		"## Jogadas especiais explicadas", "Coringa +4", "desafiou o +4", "Nenhum erro ou violação",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report does not contain %q", want)
		}
	}
	for _, step := range result.Steps {
		rowStart := fmt.Sprintf("| %d | %d → %d |", step.Number, step.Before.Revision, step.After.Revision)
		if !strings.Contains(report, rowStart) {
			t.Fatalf("complete history is missing step %d", step.Number)
		}
	}

	result.Diagnostics = append(result.Diagnostics, Diagnostic{Kind: DiagnosticLimit, Step: 9, Revision: 8, Message: "limite"})
	if report := RenderMarkdown(result); !strings.Contains(report, "`limite_atingido`") {
		t.Fatal("diagnostic missing from report")
	}
}

func TestWriteReport(t *testing.T) {
	result := Result{
		Config:    Config{Players: 2, Mode: ModeClassic, Seed: 42, MaxActions: 1},
		StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0),
	}
	path := filepath.Join(t.TempDir(), "nested", "report.md")
	if err := WriteReport(path, result); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), "Semente:** `42`") {
		t.Fatal("written report is missing seed")
	}
}

func TestDefaultReportPathIncludesMilliseconds(t *testing.T) {
	started := time.Date(2026, 9, 23, 21, 38, 58, 123456789, time.UTC)
	got := filepath.ToSlash(DefaultReportPath(started, 42))
	want := ".reports/simulations/partida_2026-09-23_21-38-58_123_seed-42.md"
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func playStep(card uno.Card, before, after StateSummary, events ...uno.Event) Step {
	return Step{
		Action: uno.Action{Type: uno.PlayCard, PlayerID: 1, CardID: card.ID},
		Card:   &card,
		Before: before,
		After:  after,
		Events: events,
	}
}

func TestCaughtBluffStatisticsFromEvents(t *testing.T) {
	stats := CollectStats(Result{Config: Config{Players: 2}, Steps: []Step{{
		Action: uno.Action{Type: uno.CallBluff, PlayerID: 2},
		Events: []uno.Event{{Type: uno.BluffCalled, PlayerID: 2, TargetID: 1, Success: true, Count: 4}, {Type: uno.CardsDrawn, PlayerID: 1, Count: 4}},
	}}})
	if stats.BluffsCalled != 1 || stats.BluffsCaught != 1 || stats.PenaltyEvents != 1 || stats.Players[1].CardsDrawn != 4 || stats.Players[2].BluffsCaught != 1 {
		t.Fatal(stats)
	}
}

func TestSwapHandsSimulationAndReport(t *testing.T) {
	result, err := Run(t.Context(), Config{Players: 4, Mode: ModeHouse, Seed: 20260924, MaxActions: DefaultMaxActions}, nil)
	if err != nil {
		t.Fatal(err)
	}
	swaps := 0
	for _, step := range result.Steps {
		if step.Action.Type != uno.ChoosePlayer {
			continue
		}
		swaps++
		actor, target := step.Action.PlayerID, step.Action.TargetID
		if actor == target || step.After.HandSizes[actor] != step.Before.HandSizes[target] || step.After.HandSizes[target] != step.Before.HandSizes[actor] || step.After.ActiveColor != step.Before.ActiveColor {
			t.Fatal(step)
		}
		if !strings.Contains(DescribeStep(step), "trocou todas as cartas") || len(ExplainStep(step)) != 1 {
			t.Fatal("missing swap description")
		}
	}
	if swaps == 0 || CollectStats(result).HandSwaps != swaps {
		t.Fatal("no swaps recorded")
	}
	report := RenderMarkdown(result)
	for _, want := range []string{"Trocar cartas", "escolhendo jogador", "trocou todas as cartas", "Trocas de mãos"} {
		if !strings.Contains(report, want) {
			t.Fatal("missing", want)
		}
	}
}
