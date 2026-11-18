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
	languages     *languageCache
	applyMenu     func(context.Context, game.ChatID, groups.CommandLanguage) error
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
	case strings.HasPrefix(data, "start_"):
		h.handleStartCallback(ctx, cq, strings.TrimPrefix(data, "start_"))
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

// handleStartCallback starts a game from the lobby button. Callbacks already
// run inside the chat queue, so the mutation is serialized with every other
// action in that chat.
func (h *CallbackHandler) handleStartCallback(ctx context.Context, cq *telego.CallbackQuery, gameID string) {
	answer := func(text string, alert bool) {
		_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            text,
			ShowAlert:       alert,
		})
	}
	if cq.Message == nil || cq.From.ID <= 0 {
		answer("⚠️ Requisição inválida.", true)
		return
	}
	chatID := game.ChatID(cq.Message.GetChat().ID)
	view, err := h.service.PublicView(ctx, uno.GameID(gameID))
	if err != nil || view.ChatID != chatID {
		answer("⚠️ Partida não encontrada.", true)
		return
	}
	if view.Phase != uno.Lobby {
		answer("⚠️ A partida já foi iniciada!", true)
		return
	}
	if len(view.Players) < 2 {
		answer("⚠️ São necessários pelo menos 2 jogadores inscritos para iniciar.", true)
		return
	}

	outcome, err := startGame(ctx, h.service, h.bot, uno.PlayerID(cq.From.ID), chatID, view.GameID, view.Revision)
	if err != nil {
		switch {
		case errors.Is(err, uno.ErrStaleRevision):
			answer("⚠️ A partida mudou. Abra o estado novamente.", true)
		default:
			answer("⚠️ Não foi possível iniciar a partida.", true)
		}
		return
	}

	// The lobby message becomes a short receipt with no button at all.
	if cq.Message != nil {
		_, _ = h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telego.ChatID{ID: int64(chatID)},
			MessageID:   cq.Message.GetMessageID(),
			Text:        "🚀 <b>Partida iniciada!</b>",
			ParseMode:   telego.ModeHTML,
			ReplyMarkup: &telego.InlineKeyboardMarkup{InlineKeyboard: [][]telego.InlineKeyboardButton{}},
		})
	}
	// The actual game flow goes in a new message, exactly like /iniciar.
	sendStartFlow(ctx, h.bot, h.renderer, chatID, outcome.View)
	answer("✅ Partida iniciada!", false)
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
		text = h.renderer.RenderLobby(view, h.commandLanguage(ctx, game.ChatID(cq.Message.GetChat().ID)))
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
	case "rank", "lang":
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
			rankName = "Vitória"
		case "updated":
			targetRank = groups.Updated
			rankName = "Colocação"
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

	case "lang":
		var target groups.CommandLanguage
		var label string
		switch actionArg {
		case "pt":
			target, label = groups.CommandsPT, "Português"
		case "en":
			target, label = groups.CommandsEN, "English"
		default:
			_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
				CallbackQueryID: cq.ID,
				Text:            "⚠️ Idioma inválido.",
				ShowAlert:       true,
			})
			return
		}

		cfg, err := svc.SetCommandLanguage(ctx, targetChatID, actorID, target)
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
				Text:            "❌ Não foi possível alterar o idioma.",
				ShowAlert:       true,
			})
			return
		}

		if h.applyMenu != nil {
			_ = h.applyMenu(ctx, game.ChatID(targetChatID), target)
		}
		h.languages.set(game.ChatID(targetChatID), target)

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
			Text:            fmt.Sprintf("Idioma dos comandos alterado para %s.", label),
		})
	}
}
