//go:build !debugcards

package telegram

import (
	"context"

	"github.com/mymmrac/telego"
)

// handleDebugCommand is a no-op in production builds: the /dar command does
// not exist at all outside an explicit debugcards build.
func (h *CommandHandler) handleDebugCommand(context.Context, *telego.Message, string, []string) bool {
	return false
}
