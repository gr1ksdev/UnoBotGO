package simulation

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/malbs/UnoGoBot/internal/uno"
)

var ErrActionLimit = errors.New("limite de jogadas atingido")

type DiagnosticKind string

const (
	DiagnosticAction    DiagnosticKind = "acao_rejeitada"
	DiagnosticInvariant DiagnosticKind = "estado_invalido"
	DiagnosticLimit     DiagnosticKind = "limite_atingido"
	DiagnosticContext   DiagnosticKind = "execucao_cancelada"
)

type Diagnostic struct {
	Kind     DiagnosticKind
	Step     int
	Revision uint64
	Message  string
}

type StateSummary struct {
	Revision      uint64
	Phase         uno.Phase
	CurrentPlayer uno.PlayerID
	Direction     int
	ActiveColor   uno.Color
	DrawCounter   int
	DrawnCardID   uno.CardID
	HandSizes     map[uno.PlayerID]int
	Placements    []uno.Placement
}

type Step struct {
	Number int
	Setup  bool
	Action uno.Action
	Card   *uno.Card
	Before StateSummary
	After  StateSummary
	Events []uno.Event
	Error  string
}

type Result struct {
	Config      Config
	StartedAt   time.Time
	FinishedAt  time.Time
	DealerID    uno.PlayerID
	Steps       []Step
	Diagnostics []Diagnostic
	FinalState  uno.State
	Completed   bool
}

type Observer func(Step)

func Run(ctx context.Context, config Config, observer Observer) (Result, error) {
	result := Result{Config: config, StartedAt: time.Now()}
	if err := config.Validate(); err != nil {
		result.FinishedAt = time.Now()
		return result, err
	}

	rng := rand.New(rand.NewPCG(config.Seed, config.Seed^0x9e3779b97f4a7c15))
	shuffle := func(ids []uno.CardID) {
		rng.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	}
	game, err := uno.NewGame("simulation", config.Mode.Rules(), uno.WithShuffler(shuffle))
	if err != nil {
		result.FinishedAt = time.Now()
		return result, err
	}

	apply := func(action uno.Action, setup bool) error {
		before := game.Snapshot()
		action.Revision = before.Revision
		step := Step{
			Number: len(result.Steps) + 1,
			Setup:  setup,
			Action: action,
			Before: summarize(before),
		}
		if action.CardID != "" {
			if card, ok := stateCard(before, action.CardID); ok {
				step.Card = &card
			}
		}
		applied, applyErr := game.Apply(action)
		if applyErr != nil {
			step.After = summarize(game.Snapshot())
			step.Error = applyErr.Error()
			result.Steps = append(result.Steps, step)
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Kind: DiagnosticAction, Step: step.Number, Revision: before.Revision,
				Message: fmt.Sprintf("%s: %v", ActionName(action.Type), applyErr),
			})
			if observer != nil {
				observer(step)
			}
			return applyErr
		}
		after := game.Snapshot()
		step.Events = slices.Clone(applied.Events)
		step.After = summarize(after)
		result.Steps = append(result.Steps, step)
		if observer != nil {
			observer(step)
		}
		if validateErr := after.Validate(); validateErr != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Kind: DiagnosticInvariant, Step: step.Number, Revision: after.Revision,
				Message: validateErr.Error(),
			})
			return validateErr
		}
		return nil
	}

	for id := uno.PlayerID(1); id <= uno.PlayerID(config.Players); id++ {
		if err := apply(uno.Action{Type: uno.JoinGame, PlayerID: id}, true); err != nil {
			return finishResult(result, game), err
		}
	}
	result.DealerID = uno.PlayerID(1 + rng.IntN(config.Players))
	if err := apply(uno.Action{Type: uno.StartGame, PlayerID: result.DealerID, DealerID: result.DealerID}, true); err != nil {
		return finishResult(result, game), err
	}

	bot := strategy{rng: rng}
	for actions := 0; ; actions++ {
		if err := ctx.Err(); err != nil {
			state := game.Snapshot()
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Kind: DiagnosticContext, Step: len(result.Steps), Revision: state.Revision, Message: err.Error(),
			})
			return finishResult(result, game), err
		}
		state := game.Snapshot()
		if state.Phase == uno.Finished {
			result.Completed = true
			return finishResult(result, game), nil
		}
		if actions >= config.MaxActions {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Kind: DiagnosticLimit, Step: len(result.Steps), Revision: state.Revision,
				Message: fmt.Sprintf("%v (%d)", ErrActionLimit, config.MaxActions),
			})
			return finishResult(result, game), ErrActionLimit
		}
		if err := apply(bot.decide(game, state), false); err != nil {
			return finishResult(result, game), err
		}
	}
}

func finishResult(result Result, game *uno.Game) Result {
	result.FinalState = game.Snapshot()
	result.FinishedAt = time.Now()
	return result
}

func summarize(state uno.State) StateSummary {
	hands := make(map[uno.PlayerID]int, len(state.Players))
	for _, player := range state.Players {
		hands[player.ID] = len(player.Hand)
	}
	return StateSummary{
		Revision:      state.Revision,
		Phase:         state.Phase,
		CurrentPlayer: state.CurrentPlayerID,
		Direction:     state.Direction,
		ActiveColor:   state.ActiveColor,
		DrawCounter:   state.DrawCounter,
		DrawnCardID:   state.DrawnCardID,
		HandSizes:     hands,
		Placements:    slices.Clone(state.Placements),
	}
}
