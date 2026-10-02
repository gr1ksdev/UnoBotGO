//go:build !debugcards

package telegram

import (
	"context"
	"github.com/mymmrac/telego"
)

func (h *CommandHandler) handleDebugCommand(context.Context, *telego.Message, string, []string) bool {
	return false
}
