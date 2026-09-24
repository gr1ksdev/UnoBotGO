package simulation

import (
	"errors"
	"testing"

	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{Players: 2, Mode: ModeClassic, Seed: 1, MaxActions: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for name, test := range map[string]struct {
		config Config
		want   error
	}{
		"too few players":  {Config{Players: 1, Mode: ModeClassic, MaxActions: 1}, ErrInvalidPlayers},
		"too many players": {Config{Players: 11, Mode: ModeClassic, MaxActions: 1}, ErrInvalidPlayers},
		"unknown mode":     {Config{Players: 2, Mode: "rapido", MaxActions: 1}, ErrInvalidMode},
		"invalid limit":    {Config{Players: 2, Mode: ModeClassic}, ErrInvalidMaxActions},
	} {
		t.Run(name, func(t *testing.T) {
			if err := test.config.Validate(); !errors.Is(err, test.want) {
				t.Fatalf("got %v, want %v", err, test.want)
			}
		})
	}
}

func TestModeMapping(t *testing.T) {
	classic, err := ParseMode(" CLASSICO ")
	if err != nil || classic != ModeClassic || classic.Rules() != uno.BotRules() {
		t.Fatalf("classic mapping failed: mode=%q err=%v", classic, err)
	}
	house, err := ParseMode("caseiro")
	if err != nil || house != ModeHouse || house.Rules() != uno.CaseiroRules() {
		t.Fatalf("house mapping failed: mode=%q err=%v", house, err)
	}
	if _, err := ParseMode("classic"); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("invalid mode accepted: %v", err)
	}
	if accented, err := ParseMode("Clássico"); err != nil || accented != ModeClassic {
		t.Fatalf("accented classic mapping failed: mode=%q err=%v", accented, err)
	}
	if numbered, err := ParseMode("2"); err != nil || numbered != ModeHouse {
		t.Fatalf("numbered house mapping failed: mode=%q err=%v", numbered, err)
	}
}
