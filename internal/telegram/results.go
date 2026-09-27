package telegram

import (
	"context"
	"errors"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"time"
)

// SetResultRepository wires synchronous closure only. No goroutine, queue,
// outbox, engine snapshot or database access is added to nonterminal actions.
// A product scoring policy is intentionally not enabled yet: the final public
// audit result is persisted with needs_product_decision and no points.
func (b *Bot) SetResultRepository(repository ranking.Repository) {
	b.resultRepository = repository
	b.cmdHandler.finalize = b.finalizeOutcome
	b.inlineHandler.finalize = b.finalizeOutcome
}
func (b *Bot) finalizeOutcome(ctx context.Context, outcome game.Outcome) {
	if outcome.Completed == nil || b.resultRepository == nil {
		return
	}
	b.persistResult(ctx, outcome.Completed.Clone())
}
func (b *Bot) persistResult(ctx context.Context, result ranking.Result) bool {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	commit, err := b.resultRepository.RecordCompletedGame(ctx, result)
	if err != nil {
		b.logger.Error("completed result persistence failed; retained in memory", "game_id", result.GameID, "chat_id", result.ChatID, "error", err)
		return false
	}
	b.service.AcknowledgeResult(uno.GameID(result.GameID))
	b.logger.Info("completed result committed", "game_id", result.GameID, "scored", commit.Scored, "already_persisted", commit.AlreadyPersisted)
	return true
}

// RetryPendingResults is an explicit synchronous application operation; it never
// runs in a background worker or reads private game runtime.
func (b *Bot) RetryPendingResults(ctx context.Context) error {
	for _, r := range b.service.PendingResults() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !b.persistResult(ctx, r) {
			return errors.New("telegram: pending result persistence failed")
		}
	}
	return nil
}
