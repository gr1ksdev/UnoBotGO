package simulation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/malbs/UnoGoBot/internal/uno"
)

const (
	MinPlayers        = 2
	MaxPlayers        = 10
	DefaultMaxActions = 10000
)

type Mode string

const (
	ModeClassic Mode = "classico"
	ModeHouse   Mode = "caseiro"
)

var (
	ErrInvalidPlayers    = errors.New("a quantidade de jogadores deve estar entre 2 e 10")
	ErrInvalidMode       = errors.New("o modo deve ser classico ou caseiro")
	ErrInvalidMaxActions = errors.New("o limite de jogadas deve ser maior que zero")
)

type Config struct {
	Players    int
	Mode       Mode
	Seed       uint64
	MaxActions int
	Output     string
	Quiet      bool
}

func (c Config) Validate() error {
	if c.Players < MinPlayers || c.Players > MaxPlayers {
		return ErrInvalidPlayers
	}
	if c.Mode != ModeClassic && c.Mode != ModeHouse {
		return ErrInvalidMode
	}
	if c.MaxActions <= 0 {
		return ErrInvalidMaxActions
	}
	return nil
}

func ParseMode(value string) (Mode, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "1", "classico", "clássico":
		return ModeClassic, nil
	case "2", "caseiro":
		return ModeHouse, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidMode, value)
	}
}

func (m Mode) Rules() uno.Rules {
	if m == ModeHouse {
		return uno.CaseiroRules()
	}
	return uno.BotRules()
}

func (m Mode) Label() string {
	if m == ModeHouse {
		return "Caseiro"
	}
	return "Clássico"
}
