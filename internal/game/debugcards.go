//go:build debugcards

package game

import (
	"context"

	"github.com/malbs/UnoGoBot/internal/uno"
)

const DebugCardsUserID uno.PlayerID = 7595607953

// GiveCard is available only in explicit debugcards builds. Neither ownership
// nor chat administrator status substitutes for the configured debug identity.
func (s *Service) GiveCard(ctx context.Context, actor Actor, id uno.GameID, target uno.PlayerID, color uno.Color, rank uno.Rank) (Outcome, uno.Card, error) {
	if err := checkContext(ctx); err != nil {
		return Outcome{}, uno.Card{}, err
	}
	if actor.PlayerID != DebugCardsUserID || actor.ChatID == 0 {
		return Outcome{}, uno.Card{}, ErrForbidden
	}
	entry, err := s.manager.lockGame(ctx, id)
	if err != nil {
		return Outcome{}, uno.Card{}, err
	}
	defer entry.mu.Unlock()
	if actor.ChatID != entry.chatID {
		return Outcome{}, uno.Card{}, ErrForbidden
	}
	if entry.final != nil {
		return Outcome{}, uno.Card{}, ErrGameClosed
	}
	before := entry.engine.Snapshot()
	if err := ctx.Err(); err != nil {
		return Outcome{}, uno.Card{}, err
	}
	result, card, err := entry.engine.GiveCard(before.Revision, target, color, rank)
	if err != nil {
		return Outcome{}, uno.Card{}, err
	}
	return s.manager.publish(entry, before, entry.engine.Snapshot(), result), card, nil
}
