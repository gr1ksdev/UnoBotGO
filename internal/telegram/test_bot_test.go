package telegram

import (
	"github.com/malbs/UnoGoBot/internal/game"
	"log/slog"
	"time"
)

// Handler-level fixtures have a known identity without running Telegram startup.
func newTestBot(api BotAPI, service *game.Service, tokens *TokenStore, renderer *Renderer, ttl time.Duration, logger *slog.Logger) *Bot {
	b := New(api, service, tokens, renderer, ttl, logger)
	b.username = "unobot"
	b.cmdHandler.botUsername = "unobot"
	return b
}
