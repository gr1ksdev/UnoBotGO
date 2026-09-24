package simulation

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestRunnerCompletesBothModes(t *testing.T) {
	for _, test := range []struct {
		name    string
		players int
		mode    Mode
		seed    uint64
	}{
		{"classic two players", 2, ModeClassic, 20260923},
		{"house four players", 4, ModeHouse, 20260924},
		{"classic maximum players", 10, ModeClassic, 20260925},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Run(context.Background(), Config{
				Players: test.players, Mode: test.mode, Seed: test.seed, MaxActions: DefaultMaxActions,
			}, nil)
			if err != nil {
				t.Fatalf("run failed: %v", err)
			}
			if !result.Completed || result.FinalState.Phase != uno.Finished {
				t.Fatalf("game did not finish: completed=%v phase=%v", result.Completed, result.FinalState.Phase)
			}
			if len(result.FinalState.Placements) != test.players {
				t.Fatalf("got %d placements, want %d", len(result.FinalState.Placements), test.players)
			}
			if len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
			}
			if err := result.FinalState.Validate(); err != nil {
				t.Fatalf("invalid final state: %v", err)
			}
		})
	}
}

func TestRunnerIsDeterministic(t *testing.T) {
	config := Config{Players: 4, Mode: ModeHouse, Seed: 987654321, MaxActions: DefaultMaxActions}
	first, err := Run(context.Background(), config, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.DealerID != second.DealerID || !reflect.DeepEqual(first.FinalState, second.FinalState) || !reflect.DeepEqual(first.Steps, second.Steps) {
		t.Fatal("same seed produced different gameplay")
	}
}

func TestRunnerCompletesSeedMatrix(t *testing.T) {
	for seed := uint64(1); seed <= 10; seed++ {
		for _, mode := range []Mode{ModeClassic, ModeHouse} {
			result, err := Run(context.Background(), Config{
				Players: 2 + int(seed%5), Mode: mode, Seed: seed, MaxActions: DefaultMaxActions,
			}, nil)
			if err != nil || !result.Completed || len(result.Diagnostics) != 0 {
				t.Fatalf("seed=%d mode=%s: completed=%v err=%v diagnostics=%+v", seed, mode, result.Completed, err, result.Diagnostics)
			}
		}
	}
}

func TestRunnerReportsActionLimit(t *testing.T) {
	result, err := Run(context.Background(), Config{
		Players: 2, Mode: ModeClassic, Seed: 1, MaxActions: 1,
	}, nil)
	if !errors.Is(err, ErrActionLimit) {
		t.Fatalf("got %v, want ErrActionLimit", err)
	}
	if result.Completed || len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != DiagnosticLimit {
		t.Fatalf("unexpected result: completed=%v diagnostics=%+v", result.Completed, result.Diagnostics)
	}
}

func TestRunnerReportsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := Run(ctx, Config{Players: 2, Mode: ModeClassic, Seed: 1, MaxActions: DefaultMaxActions}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Kind != DiagnosticContext {
		t.Fatalf("unexpected diagnostics: %+v", result.Diagnostics)
	}
}

func TestStrategyHelpers(t *testing.T) {
	state := uno.State{
		Cards: []uno.Card{
			{ID: "red", Color: uno.Red, Rank: uno.One},
			{ID: "blue1", Color: uno.Blue, Rank: uno.Two},
			{ID: "blue2", Color: uno.Blue, Rank: uno.Three},
		},
		Players: []uno.Player{{ID: 1, Hand: []uno.CardID{"red", "blue1", "blue2"}}},
	}
	if got := preferredColor(state, 1); got != uno.Blue {
		t.Fatalf("preferred color = %v, want blue", got)
	}
	state.DrawCounter = 2
	card, ok := choosePenaltyResponse([]uno.Card{{ID: "n", Rank: uno.One}, {ID: "d", Rank: uno.DrawTwo}}, state)
	if !ok || card.ID != "d" {
		t.Fatalf("penalty response = %+v, %v", card, ok)
	}
}
