package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"time"
	"unicode/utf16"

	"github.com/malbs/UnoGoBot/internal/ranking"
)

const rankingMessageLimit = 4000 // UTF-16 units after HTML entity parsing; below 4096.

func rankingName(name string, id int64) string {
	if name == "" {
		name = strconv.FormatInt(id, 10)
	}
	return html.EscapeString(name)
}

func messageUnits(text string) int {
	n := 0
	for _, r := range html.UnescapeString(text) {
		n += utf16.RuneLen(r)
	}
	return n
}

// RenderGroupRanking is shared by /ranking and post-commit notifications.
// Equal scores share competition rank (1,1,3), independent of stable SQL order.
func RenderGroupRanking(group ranking.GroupRanking) string {
	const title = "🏆 Ranking do grupo\n\n"
	if group.Total == 0 {
		return title + "Ainda não há partidas pontuadas neste grupo."
	}
	text := title
	position := 0
	shown := 0
	for i, entry := range group.Entries {
		if i == 0 || entry.Score != group.Entries[i-1].Score {
			position = i + 1
		}
		line := fmt.Sprintf("%s %s · %s", placementLabel(position), rankingName(entry.DisplayName, entry.UserID), ranking.FormatScore(group.System, entry.Score))
		candidate := text
		if shown > 0 {
			candidate += "\n"
		}
		candidate += line
		suffix := ""
		if remaining := group.Total - int64(i+1); remaining > 0 {
			suffix = rankingRemaining(remaining)
		}
		if messageUnits(candidate+suffix) > rankingMessageLimit {
			break
		}
		text = candidate
		shown++
	}
	if remaining := group.Total - int64(shown); remaining > 0 {
		text += rankingRemaining(remaining)
	}
	return text
}

func rankingRemaining(n int64) string {
	if n == 1 {
		return "\n\n… e mais 1 jogador."
	}
	return fmt.Sprintf("\n\n… e mais %d jogadores.", n)
}

func (b *Bot) SetRankingService(service *ranking.Service) {
	b.cmdHandler.rankingService = service
}

func (h *CommandHandler) handleRanking(ctx context.Context, chatID int64) {
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	group, err := h.rankingService.ListGroupRanking(readCtx, chatID)
	cancel()
	sendCtx, cancelSend := context.WithTimeout(ctx, 10*time.Second)
	defer cancelSend()
	if err != nil {
		h.logger.WarnContext(ctx, "failed to read group ranking", "chat_id", chatID, "error", err)
		h.reply(sendCtx, chatID, "⚠️ Não foi possível consultar o ranking do grupo. Tente /ranking novamente.", nil)
		return
	}
	h.reply(sendCtx, chatID, RenderGroupRanking(group), nil)
}
