package telegram

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

// SetResultRepository wires synchronous closure only. No goroutine, queue,
// outbox, engine snapshot or database access is added to nonterminal actions.
func (b *Bot) SetResultRepository(repository ranking.Repository) {
	b.resultRepository = repository
	b.cmdHandler.finalize = b.finalizeOutcome
	b.inlineHandler.finalize = b.finalizeOutcome
}

// finalizeOutcome waits for COMMIT, then returns an optional notification
// replacing the handler's unscored final summary. No notification is
// returned for failed persistence, N<2 or a previously committed retry.
func (b *Bot) finalizeOutcome(ctx context.Context, outcome game.Outcome) func() {
	if outcome.Completed == nil || b.resultRepository == nil {
		return nil
	}
	result, err := b.persistResult(ctx, *outcome.Completed)
	if err != nil || result == nil {
		return nil
	}
	return func() { b.notifyPoints(ctx, *result) }
}

// persistResult acknowledges only confirmed commits. An indeterminate commit
// keeps the immutable memory result for an idempotent retry using the same ID.
func (b *Bot) persistResult(ctx context.Context, input ranking.Result) (*ranking.Result, error) {
	result, err := ranking.Prepare(input)
	if err != nil {
		b.logger.Error("invalid completed result; retained in memory", "game_id", input.GameID, "error", err)
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	commit, err := b.resultRepository.RecordCompletedGame(ctx, result)
	if err != nil {
		b.logger.Error("completed result persistence failed; retained in memory", "game_id", result.GameID, "chat_id", result.ChatID, "error", err)
		return nil, err
	}
	b.service.AcknowledgeResult(uno.GameID(result.GameID))
	b.logger.Info("completed result committed", "game_id", result.GameID, "scored", commit.Scored, "eligible_count", result.EligibleCount(), "scoring_status", result.ScoringStatus(), "already_persisted", commit.AlreadyPersisted)
	if !commit.Scored || commit.AlreadyPersisted {
		return nil, nil
	}
	return &result, nil
}

func (b *Bot) notifyPoints(ctx context.Context, result ranking.Result) {
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, err := b.api.SendMessage(sendCtx, &telego.SendMessageParams{ChatID: telego.ChatID{ID: result.ChatID}, Text: renderPoints(result), ParseMode: "HTML"})
	cancel()
	if err != nil {
		b.logger.Warn("failed to send committed game points", "game_id", result.GameID, "chat_id", result.ChatID, "error", err)
	}
	// Telegram delivery does not roll back scores or retain/requeue a DB result.
	b.cmdHandler.handleRanking(ctx, result.ChatID)
}

func renderPoints(result ranking.Result) string {
	r := result.Clone()
	// Placed users in placement order, then unplaced audit participants by ID.
	// Sorting is presentation only; it never creates an abandonment placement.
	slices.SortFunc(r.Players, func(a, b ranking.Player) int {
		if a.Eligible() != b.Eligible() {
			if a.Eligible() {
				return -1
			}
			return 1
		}
		if a.Position != b.Position {
			return a.Position - b.Position
		}
		if a.UserID < b.UserID {
			return -1
		}
		if a.UserID > b.UserID {
			return 1
		}
		return 0
	})
	var text strings.Builder
	text.WriteString("🏁 Partida encerrada\n\n")
	for i, p := range r.Players {
		if i > 0 {
			text.WriteByte('\n')
		}
		if !p.Eligible() {
			fmt.Fprintf(&text, "%s · fora do ranking", rankingName(p.DisplayName, p.UserID))
		} else {
			fmt.Fprintf(&text, "%s %s · +%s", placementLabel(p.Position), rankingName(p.DisplayName, p.UserID), ranking.FormatScore(r.RankingSystem, p.Score))
		}
	}
	return text.String()
}

// RetryPendingResults is an explicit synchronous application operation. When a
// retry newly commits, its points follow the final game message already sent by
// the original closure. Previously confirmed commits are never announced again.
func (b *Bot) RetryPendingResults(ctx context.Context) error {
	if b.resultRepository == nil {
		return errors.New("telegram: result repository is not configured")
	}
	for _, r := range b.service.PendingResults() {
		if err := ctx.Err(); err != nil {
			return err
		}
		result, err := b.persistResult(ctx, r)
		if err != nil {
			return err
		}
		if result != nil {
			b.notifyPoints(ctx, *result)
		}
	}
	return nil
}
