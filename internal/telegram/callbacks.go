package telegram

import (
	"context"
	"log/slog"
	"strings"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

type CallbackHandler struct {
	bot      BotAPI
	service  *game.Service
	renderer *Renderer
	tokens   *TokenStore
	logger   *slog.Logger
}

func NewCallbackHandler(
	bot BotAPI,
	service *game.Service,
	renderer *Renderer,
	tokens *TokenStore,
	logger *slog.Logger,
) *CallbackHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CallbackHandler{
		bot:      bot,
		service:  service,
		renderer: renderer,
		tokens:   tokens,
		logger:   logger,
	}
}

func (h *CallbackHandler) HandleCallback(ctx context.Context, cq *telego.CallbackQuery) {
	if cq == nil {
		return
	}

	data := strings.TrimSpace(cq.Data)
	if data == "" || data == "noop" {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
		return
	}

	switch {
	case strings.HasPrefix(data, "view_"):
		gameID := uno.GameID(strings.TrimPrefix(data, "view_"))
		h.handleRefreshState(ctx, cq, gameID)
	case strings.HasPrefix(data, "st_"):
		tokStr := strings.TrimPrefix(data, "st_")
		h.handleStatusCheck(ctx, cq, tokStr)
	default:
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
	}
}

func (h *CallbackHandler) handleRefreshState(ctx context.Context, cq *telego.CallbackQuery, gameID uno.GameID) {
	view, err := h.service.PublicView(ctx, gameID)
	if err != nil {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ Partida não encontrada.",
		})
		return
	}

	var text string
	if view.Phase == uno.Lobby {
		text = h.renderer.RenderLobby(view)
	} else {
		text = h.renderer.RenderPublicState(view)
	}

	if cq.Message != nil {
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: cq.Message.GetChat().ID},
			MessageID:   cq.Message.GetMessageID(),
			Text:        text,
			ParseMode:   "HTML",
			ReplyMarkup: makeGameButtons(gameID),
		})
	}

	_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "Estado atualizado!",
	})
}

func (h *CallbackHandler) handleStatusCheck(ctx context.Context, cq *telego.CallbackQuery, tokStr string) {
	summary, consumed, found := h.tokens.GetActionStatus(tokStr)

	msg := "⏳ Aguardando validação pelo bot..."
	if !found {
		msg = "⚠️ Esta seleção expirou. Abra suas cartas novamente."
	} else if consumed {
		switch summary {
		case "confirmed":
			msg = "✅ Jogada confirmada!"
		case "stale":
			msg = "⚠️ Jogada antiga: a partida mudou. Abra suas cartas novamente."
		default:
			msg = "Jogada processada."
		}
	}

	_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            msg,
		ShowAlert:       false,
	})
}
