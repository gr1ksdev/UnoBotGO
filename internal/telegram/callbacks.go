package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

type CallbackHandler struct {
	bot           BotAPI
	service       *game.Service
	renderer      *Renderer
	tokens        *TokenStore
	logger        *slog.Logger
	groupsService *groups.Service
	knownUsers    groups.UserRepository
}

func (h *CallbackHandler) SetGroupsService(s *groups.Service) {
	h.groupsService = s
}

func (h *CallbackHandler) SetKnownUsers(r groups.UserRepository) {
	h.knownUsers = r
}

func (h *CallbackHandler) getGroupsService() *groups.Service {
	return h.groupsService
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
	if data == "" {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
		return
	}

	switch {
	case strings.HasPrefix(data, "cfg_"):
		h.handleConfigCallback(ctx, cq, data)
	case strings.HasPrefix(data, "view_"):
		// Legacy refresh route: messages sent before the state button was
		// removed may still carry it. It also clears the keyboard of a game
		// that already closed.
		gameID := uno.GameID(strings.TrimPrefix(data, "view_"))
		h.handleRefreshState(ctx, cq, gameID)
	default:
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
	}
}

// handleRefreshState re-renders a game message on demand.
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
		markup := makeGameButtons(view)
		if markup == nil {
			// An explicit empty keyboard removes the buttons of a closed game.
			markup = &telego.InlineKeyboardMarkup{InlineKeyboard: [][]telego.InlineKeyboardButton{}}
		}
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: cq.Message.GetChat().ID},
			MessageID:   cq.Message.GetMessageID(),
			Text:        text,
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: markup,
		})
	}

	_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "Estado atualizado!",
	})
}

func (h *CallbackHandler) handleConfigCallback(ctx context.Context, cq *telego.CallbackQuery, data string) {
	if cq == nil || cq.Message == nil || cq.From.ID <= 0 {
		return
	}

	msgChatID := cq.Message.GetChat().ID
	messageID := cq.Message.GetMessageID()
	actorID := cq.From.ID

	parts := strings.Split(data, "_")
	if len(parts) < 3 {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ Requisição inválida.",
			ShowAlert:       true,
		})
		return
	}

	var chatIDStr string
	actionType := parts[1]
	var actionArg string

	switch actionType {
	case "open":
		if len(parts) != 3 {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Requisição inválida.",
				ShowAlert:       true,
			})
			return
		}
		chatIDStr = parts[2]
	case "rank":
		if len(parts) != 4 {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Requisição inválida.",
				ShowAlert:       true,
			})
			return
		}
		actionArg = parts[2]
		chatIDStr = parts[3]
	default:
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ Requisição inválida.",
			ShowAlert:       true,
		})
		return
	}

	targetChatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil || targetChatID == 0 || targetChatID != msgChatID {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ Requisição inválida.",
			ShowAlert:       true,
		})
		return
	}

	svc := h.getGroupsService()
	if svc == nil {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "❌ Configuração não disponível no momento.",
			ShowAlert:       true,
		})
		return
	}

	if svc.Repository != nil {
		if _, err := svc.Repository.GetOrCreateGroupConfig(ctx, targetChatID); err != nil {
			h.logger.WarnContext(ctx, "failed to ensure group config in callback", "chat_id", targetChatID, "error", err)
		} else if h.knownUsers != nil {
			if err := h.knownUsers.ObserveGroupUser(ctx, groups.KnownUser{
				ChatID:      targetChatID,
				UserID:      actorID,
				DisplayName: observedName(cq.From),
				Username:    cq.From.Username,
				LastSeenAt:  time.Now().UTC(),
			}); err != nil {
				h.logger.WarnContext(ctx, "failed to observe user in callback", "chat_id", targetChatID, "error", err)
			}
		}
	}

	switch actionType {
	case "open":
		cfg, allowed, err := svc.CanConfigureUser(ctx, targetChatID, actorID)
		if err != nil || !allowed {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Somente administradores ou quem adicionou o bot pode alterar esta configuração.",
				ShowAlert:       true,
			})
			return
		}

		text := h.renderer.RenderGroupConfig(cfg)
		buttons := makeGroupConfigButtons(cfg)
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: targetChatID},
			MessageID:   messageID,
			Text:        text,
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: buttons,
		})
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})

	case "rank":
		var targetRank groups.RankingSystem
		var rankName string
		switch actionArg {
		case "legacy":
			targetRank = groups.Legacy
			rankName = "Legado"
		case "updated":
			targetRank = groups.Updated
			rankName = "Atualizado"
		default:
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Sistema de ranking inválido.",
				ShowAlert:       true,
			})
			return
		}

		cfg, err := svc.SetRankingSystem(ctx, targetChatID, actorID, targetRank)
		if errors.Is(err, groups.ErrForbidden) {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Somente administradores ou quem adicionou o bot pode alterar esta configuração.",
				ShowAlert:       true,
			})
			return
		}
		if errors.Is(err, groups.ErrNeedsProductDecision) {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Não é possível alterar o sistema de ranking em um grupo com pontuações já acumuladas no sistema anterior.",
				ShowAlert:       true,
			})
			return
		}
		if err != nil {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "❌ Não foi possível alterar a configuração.",
				ShowAlert:       true,
			})
			return
		}

		text := h.renderer.RenderGroupConfig(cfg)
		buttons := makeGroupConfigButtons(cfg)
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: targetChatID},
			MessageID:   messageID,
			Text:        text,
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: buttons,
		})
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            fmt.Sprintf("Sistema de ranking alterado para %s.", rankName),
		})

	}
}
