package telegram

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/mymmrac/telego"
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
// The repository resolves ties; visual positions are always unique and sequential.
func RenderGroupRanking(group ranking.GroupRanking) string {
	title := "🏆 Ranking do grupo"
	if group.MonthName != "" {
		title += " · " + group.MonthName
	}
	title += "\n\n"
	if group.Total == 0 {
		return title + "Ainda não há partidas pontuadas neste mês."
	}
	text := title
	shown := 0
	for i, entry := range group.Entries {
		line := fmt.Sprintf("%s %s · %s", placementLabel(i+1), rankingName(entry.DisplayName, entry.UserID), ranking.FormatScore(group.System, entry.Score))
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

// miniAppLaunchURL uses the bot identity already loaded by getMe at startup.
// The public HTTPS frontend URL is configured separately in BotFather.
func miniAppLaunchURL(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	return "https://t.me/" + username + "/ranking"
}
func (h *CommandHandler) rankingMarkup() *telego.InlineKeyboardMarkup {
	if h.miniAppURL == "" {
		return nil
	}
	return &telego.InlineKeyboardMarkup{InlineKeyboard: [][]telego.InlineKeyboardButton{{{Text: "🌐 Ranking Global", URL: h.miniAppURL}}}}
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
	h.reply(sendCtx, chatID, RenderGroupRanking(group), h.rankingMarkup())
}

// RenderUserMonthlyRankings formats the user's monthly points per group for private chat.
func RenderUserMonthlyRankings(rankings ranking.UserMonthlyRankings) string {
	title := "🏆 Seus rankings"
	if rankings.MonthName != "" {
		title += " · " + rankings.MonthName
	}
	title += "\n\n"

	hasUpdated := rankings.Updated != nil && len(rankings.Updated.Entries) > 0
	hasLegacy := rankings.Legacy != nil && len(rankings.Legacy.Entries) > 0

	if !hasUpdated && !hasLegacy {
		return title + "Você ainda não possui partidas pontuadas neste mês."
	}

	text := title

	renderSection := func(sec *ranking.UserMonthlyRankingSection, secHeader string, isFirst bool) {
		if sec == nil || len(sec.Entries) == 0 {
			return
		}
		prefix := ""
		if !isFirst {
			prefix = "\n\n"
		}
		header := prefix + secHeader + "\n"
		totalLine := "\n\nTotal · " + ranking.FormatScore(sec.System, sec.TotalScore)

		lines := make([]string, len(sec.Entries))
		for i, entry := range sec.Entries {
			lines[i] = fmt.Sprintf("• %s · %s", rankingGroupName(entry.GroupName, entry.ChatID), ranking.FormatScore(sec.System, entry.ScoreUnits))
		}

		bestK := -1
		for k := len(lines); k >= 0; k-- {
			var body string
			if k > 0 {
				body = strings.Join(lines[:k], "\n")
			}
			omitted := len(lines) - k
			var suffix string
			if omitted > 0 {
				if k > 0 {
					suffix = privateRankingRemaining(omitted)
				} else {
					suffix = strings.TrimPrefix(privateRankingRemaining(omitted), "\n")
				}
			}
			candidate := text + header + body + suffix + totalLine
			if messageUnits(candidate) <= rankingMessageLimit {
				bestK = k
				break
			}
		}

		if bestK >= 0 {
			var body string
			if bestK > 0 {
				body = strings.Join(lines[:bestK], "\n")
			}
			omitted := len(lines) - bestK
			var suffix string
			if omitted > 0 {
				if bestK > 0 {
					suffix = privateRankingRemaining(omitted)
				} else {
					suffix = strings.TrimPrefix(privateRankingRemaining(omitted), "\n")
				}
			}
			text += header + body + suffix + totalLine
		}
	}

	first := true
	if hasUpdated {
		renderSection(rankings.Updated, "⚡ Atualizado", first)
		first = false
	}
	if hasLegacy {
		renderSection(rankings.Legacy, "🕹️ Legado", first)
	}

	return text
}

func rankingGroupName(name string, id int64) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Grupo %d", id)
	}
	return html.EscapeString(name)
}

func privateRankingRemaining(n int) string {
	if n == 1 {
		return "\n• … e mais 1 grupo."
	}
	return fmt.Sprintf("\n• … e mais %d grupos.", n)
}

func (h *CommandHandler) handlePrivateRanking(ctx context.Context, chatID int64, userID int64) {
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	rankings, err := h.rankingService.ListUserMonthlyRankings(readCtx, userID)
	cancel()
	sendCtx, cancelSend := context.WithTimeout(ctx, 10*time.Second)
	defer cancelSend()
	if err != nil {
		h.logger.WarnContext(ctx, "failed to read private monthly ranking", "chat_id", chatID, "user_id", userID, "error", err)
		h.reply(sendCtx, chatID, "⚠️ Não foi possível consultar seu ranking. Tente /ranking novamente.", nil)
		return
	}
	h.reply(sendCtx, chatID, RenderUserMonthlyRankings(rankings), h.rankingMarkup())
}
