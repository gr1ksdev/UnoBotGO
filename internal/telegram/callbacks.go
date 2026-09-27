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
	if data == "" || data == "noop" {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
		return
	}

	switch {
	case strings.HasPrefix(data, "mode_"):
		h.handleModeSelect(ctx, cq, data)
	case strings.HasPrefix(data, "view_"):
		gameID := uno.GameID(strings.TrimPrefix(data, "view_"))
		h.handleRefreshState(ctx, cq, gameID)
	case strings.HasPrefix(data, "st_"):
		tokStr := strings.TrimPrefix(data, "st_")
		h.handleStatusCheck(ctx, cq, tokStr)
	case strings.HasPrefix(data, "cfg_"):
		h.handleConfigCallback(ctx, cq, data)
	default:
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
	}
}

func (h *CallbackHandler) handleModeSelect(ctx context.Context, cq *telego.CallbackQuery, data string) {
	var mode string
	var gameID uno.GameID
	if strings.HasPrefix(data, "mode_classic_") {
		mode = "classic"
		gameID = uno.GameID(strings.TrimPrefix(data, "mode_classic_"))
	} else if strings.HasPrefix(data, "mode_caseiro_") {
		mode = "caseiro"
		gameID = uno.GameID(strings.TrimPrefix(data, "mode_caseiro_"))
	} else {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{CallbackQueryID: cq.ID})
		return
	}

	view, err := h.service.PublicView(ctx, gameID)
	if err != nil || view.Closed {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ Partida não encontrada ou já encerrada.",
			ShowAlert:       true,
		})
		return
	}

	if view.Phase != uno.Lobby {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ A partida já foi iniciada, não é possível alterar o modo!",
			ShowAlert:       true,
		})
		return
	}

	if uno.PlayerID(cq.From.ID) != view.OwnerID {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "⚠️ Apenas o responsável pela partida pode alterar o modo.",
			ShowAlert:       true,
		})
		return
	}

	isCaseiroCurrent := view.Rules.StackWildDrawFourOnTwo || view.Rules.StackDrawTwoOnWildFour
	if (mode == "caseiro" && isCaseiroCurrent) || (mode == "classic" && !isCaseiroCurrent) {
		modeName := "Clássico 🎻"
		if mode == "caseiro" {
			modeName = "Caseiro 🏠"
		}
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            fmt.Sprintf("O modo já está definido como %s", modeName),
		})
		return
	}

	var newRules uno.Rules
	var modeName string
	if mode == "caseiro" {
		newRules = uno.CaseiroRules()
		modeName = "Caseiro 🏠"
	} else {
		newRules = uno.BotRules()
		modeName = "Clássico 🎻"
	}

	var chatID game.ChatID
	if cq.Message != nil {
		chatID = game.ChatID(cq.Message.GetChat().ID)
	}
	actor := game.Actor{PlayerID: uno.PlayerID(cq.From.ID), ChatID: chatID}

	outcome, err := h.service.SetRules(ctx, actor, gameID, newRules)
	if err != nil {
		h.logger.WarnContext(ctx, "failed to update mode", "game_id", gameID, "error", err.Error())
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "❌ Não foi possível alterar o modo. Tente novamente.",
			ShowAlert:       true,
		})
		return
	}

	if cq.Message != nil {
		text := h.renderer.RenderLobby(outcome.View)
		markup := makeGameButtons(outcome.View)
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: cq.Message.GetChat().ID},
			MessageID:   cq.Message.GetMessageID(),
			Text:        text,
			ParseMode:   "HTML",
			ReplyMarkup: markup,
		})
	}

	_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            fmt.Sprintf("Modo alterado para %s", modeName),
	})
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
		markup := makeGameButtons(view)
		if markup == nil {
			// Explicit empty keyboard removes buttons from the existing message.
			markup = &telego.InlineKeyboardMarkup{InlineKeyboard: [][]telego.InlineKeyboardButton{}}
		}
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: cq.Message.GetChat().ID},
			MessageID:   cq.Message.GetMessageID(),
			Text:        text,
			ParseMode:   "HTML",
			ReplyMarkup: markup,
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
	case "mode", "rank":
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

	if h.knownUsers != nil {
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

	svc := h.getGroupsService()
	if svc == nil {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "❌ Configuração não disponível no momento.",
			ShowAlert:       true,
		})
		return
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

	case "mode":
		var targetMode groups.Mode
		var modeName string
		switch actionArg {
		case "classic":
			targetMode = groups.Classic
			modeName = "Clássico"
		case "caseiro":
			targetMode = groups.Caseiro
			modeName = "Caseiro"
		default:
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Modo inválido.",
				ShowAlert:       true,
			})
			return
		}

		cfg, err := svc.SetDefaultGameMode(ctx, targetChatID, actorID, targetMode)
		if errors.Is(err, groups.ErrForbidden) {
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Somente administradores ou quem adicionou o bot pode alterar esta configuração.",
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
			Text:            fmt.Sprintf("Modo padrão alterado para %s.", modeName),
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
