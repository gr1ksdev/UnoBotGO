package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

type CommandHandler struct {
	bot         BotAPI
	service     *game.Service
	renderer    *Renderer
	tokens      *TokenStore
	botUsername string
	logger      *slog.Logger
}

func NewCommandHandler(
	bot BotAPI,
	service *game.Service,
	renderer *Renderer,
	tokens *TokenStore,
	botUsername string,
	logger *slog.Logger,
) *CommandHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommandHandler{
		bot:         bot,
		service:     service,
		renderer:    renderer,
		tokens:      tokens,
		botUsername: strings.ToLower(botUsername),
		logger:      logger,
	}
}

func stringPtr(s string) *string {
	return &s
}

func makeGameButtons(view game.PublicGameView) *telego.InlineKeyboardMarkup {
	if view.GameID == "" || view.Closed || view.Phase == uno.Finished {
		return nil
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{
					Text:                         "🃏 Suas cartas",
					SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s", view.GameID)),
				},
			},
		},
	}
}

func (h *CommandHandler) reply(ctx context.Context, chatID int64, text string, markup *telego.InlineKeyboardMarkup) {
	params := &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: chatID},
		Text:      text,
		ParseMode: "HTML",
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	_, err := h.bot.SendMessage(ctx, params)
	if err != nil {
		h.logger.WarnContext(ctx, "failed to send reply message", "chat_id", chatID, "error", err.Error())
	}
}

func (h *CommandHandler) HandleMessage(ctx context.Context, msg *telego.Message) {
	if msg == nil || msg.Text == "" {
		return
	}

	// Update user in presentation cache if From is available
	if msg.From != nil {
		h.renderer.userCache.Put(uno.PlayerID(msg.From.ID), msg.From.FirstName, msg.From.Username)
	}

	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(text, "/") {
		return
	}

	// Split command and target bot, e.g. /novo@UnoBot
	fields := strings.Fields(text)
	cmdFull := fields[0][1:] // strip '/'
	cmdParts := strings.SplitN(cmdFull, "@", 2)
	cmdName := strings.ToLower(cmdParts[0])

	if len(cmdParts) == 2 {
		targetBot := cmdParts[1]
		if h.botUsername != "" && !strings.EqualFold(targetBot, h.botUsername) {
			// Command addressed to another bot, ignore
			return
		}
	}

	// Check sender identity
	if msg.From == nil || msg.From.IsBot {
		return
	}
	if msg.SenderChat != nil {
		h.reply(ctx, msg.Chat.ID, "⚠️ Ações devem ser executadas por um usuário identificável, não como canal ou administrador anônimo.", nil)
		return
	}

	// Check forum topic
	if msg.IsTopicMessage || msg.MessageThreadID != 0 {
		h.reply(ctx, msg.Chat.ID, "⚠️ Tópicos de fórum ainda não são suportados. Crie e jogue a partida no chat geral do grupo.", nil)
		return
	}

	actorID := uno.PlayerID(msg.From.ID)
	chatID := game.ChatID(msg.Chat.ID)
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"

	// Private chat command handling
	if !isGroup {
		switch cmdName {
		case "start", "ajuda", "help":
			h.reply(ctx, msg.Chat.ID, h.renderer.RenderHelp(h.botUsername), nil)
		default:
			h.reply(ctx, msg.Chat.ID, "⚠️ Este comando só pode ser utilizado em grupos. Adicione o bot a um grupo para jogar!\n\nUse /ajuda para mais instruções.", nil)
		}
		return
	}

	// Group command handling
	switch cmdName {
	case "novo":
		mode := "classic"
		if len(fields) > 1 && strings.EqualFold(fields[1], "caseiro") {
			mode = "caseiro"
		}
		h.handleNovo(ctx, actorID, chatID, msg.Chat.Title, mode)
	case "entrar":
		h.handleEntrar(ctx, actorID, chatID)
	case "iniciar", "start":
		h.handleIniciar(ctx, actorID, chatID)
	case "cancelar", "kill":
		h.handleCancelar(ctx, actorID, chatID)
	case "sair":
		h.handleSair(ctx, actorID, chatID)
	case "estado":
		h.handleEstado(ctx, chatID)
	case "ajuda", "help":
		h.reply(ctx, msg.Chat.ID, h.renderer.RenderHelp(h.botUsername), nil)
	}
}

func (h *CommandHandler) handleNovo(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID, chatTitle, mode string) {
	rules := uno.BotRules()
	if strings.EqualFold(mode, "caseiro") {
		rules = uno.CaseiroRules()
	}
	req := game.CreateRequest{
		ChatName: chatTitle,
		Rules:    rules,
	}
	actor := game.Actor{PlayerID: actorID, ChatID: chatID}

	outcome, err := h.service.Create(ctx, actor, req)
	if err != nil {
		if errors.Is(err, game.ErrChatOccupied) {
			h.reply(ctx, int64(chatID), "⚠️ Já existe uma partida aberta ou em andamento neste grupo.\nUse /estado para ver ou /cancelar para encerrar.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Não foi possível criar a partida. Tente novamente.", nil)
		return
	}

	h.reply(ctx, int64(chatID), h.renderer.RenderLobby(outcome.View), makeGameButtons(outcome.View))
}

func (h *CommandHandler) handleEntrar(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID) {
	summary, err := h.service.FindChatGame(ctx, chatID)
	if err != nil {
		if errors.Is(err, game.ErrNoActiveGame) || errors.Is(err, game.ErrGameNotFound) {
			h.reply(ctx, int64(chatID), "⚠️ Nenhuma partida aberta neste grupo. Use /novo para criar uma.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Erro ao buscar partida.", nil)
		return
	}

	actor := game.Actor{PlayerID: actorID, ChatID: chatID}
	action := uno.Action{
		Type:     uno.JoinGame,
		PlayerID: actorID,
		Revision: summary.Revision,
	}

	outcome, err := h.service.Apply(ctx, actor, summary.GameID, action)
	if errors.Is(err, uno.ErrStaleRevision) {
		// Retry once with updated summary
		if freshSummary, freshErr := h.service.FindChatGame(ctx, chatID); freshErr == nil {
			action.Revision = freshSummary.Revision
			outcome, err = h.service.Apply(ctx, actor, freshSummary.GameID, action)
		}
	}

	if err != nil {
		switch {
		case errors.Is(err, uno.ErrAlreadyJoined):
			h.reply(ctx, int64(chatID), "⚠️ Você já está inscrito nesta partida!", nil)
		case errors.Is(err, uno.ErrPlayerLimit):
			h.reply(ctx, int64(chatID), "⚠️ A partida já atingiu o limite de 10 jogadores.", nil)
		case errors.Is(err, uno.ErrGameFinished), errors.Is(err, game.ErrGameClosed):
			h.reply(ctx, int64(chatID), "⚠️ A partida já foi finalizada.", nil)
		case errors.Is(err, uno.ErrLobbyClosed):
			h.reply(ctx, int64(chatID), "⚠️ Esta partida não permite entrada tardia.", nil)
		default:
			h.reply(ctx, int64(chatID), "❌ Não foi possível entrar na partida.", nil)
		}
		return
	}

	if outcome.View.Phase == uno.Lobby {
		h.reply(ctx, int64(chatID), h.renderer.RenderLobby(outcome.View), makeGameButtons(outcome.View))
	} else {
		msg := fmt.Sprintf("✅ %s entrou na partida em andamento!\n\n%s",
			h.renderer.PlayerLink(actorID, outcome.View),
			h.renderer.RenderPublicState(outcome.View),
		)
		h.reply(ctx, int64(chatID), msg, makeGameButtons(outcome.View))
	}
}

func (h *CommandHandler) handleIniciar(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID) {
	summary, err := h.service.FindChatGame(ctx, chatID)
	if err != nil {
		if errors.Is(err, game.ErrNoActiveGame) || errors.Is(err, game.ErrGameNotFound) {
			h.reply(ctx, int64(chatID), "⚠️ Nenhuma partida aberta neste grupo. Use /novo para criar uma.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Erro ao buscar partida.", nil)
		return
	}

	if summary.Phase != uno.Lobby {
		h.reply(ctx, int64(chatID), "⚠️ A partida já foi iniciada!", nil)
		return
	}

	actor := game.Actor{PlayerID: actorID, ChatID: chatID}
	action := uno.Action{
		Type:     uno.StartGame,
		PlayerID: actorID,
		Revision: summary.Revision,
	}

	outcome, err := h.service.Apply(ctx, actor, summary.GameID, action)
	if err != nil {
		switch {
		case errors.Is(err, game.ErrForbidden):
			h.reply(ctx, int64(chatID), "⚠️ Apenas o responsável pela partida pode iniciá-la.", nil)
		case errors.Is(err, uno.ErrNotEnoughPlayers):
			h.reply(ctx, int64(chatID), "⚠️ São necessários pelo menos 2 jogadores inscritos para iniciar.", nil)
		default:
			h.reply(ctx, int64(chatID), "❌ Não foi possível iniciar a partida.", nil)
		}
		return
	}

	// Send initial top card sticker to the chat
	if outcome.View.TopCard != nil {
		stickerID := GetCardStickerID(*outcome.View.TopCard)
		if stickerID != "" {
			_, errSticker := h.bot.SendSticker(ctx, &telego.SendStickerParams{
				ChatID:  telego.ChatID{ID: int64(chatID)},
				Sticker: telego.InputFile{FileID: stickerID},
			})
			if errSticker != nil {
				h.logger.WarnContext(ctx, "failed to send initial top card sticker", "chat_id", chatID, "error", errSticker.Error())
			}
		}
	}

	msg := "🚀 <b>Partida iniciada!</b>\n\n" + h.renderer.RenderPublicState(outcome.View)
	h.reply(ctx, int64(chatID), msg, makeGameButtons(outcome.View))
}

func (h *CommandHandler) handleCancelar(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID) {
	summary, err := h.service.FindChatGame(ctx, chatID)
	if err != nil {
		if errors.Is(err, game.ErrNoActiveGame) || errors.Is(err, game.ErrGameNotFound) {
			h.reply(ctx, int64(chatID), "⚠️ Nenhuma partida ativa encontrada neste grupo.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Erro ao buscar partida.", nil)
		return
	}

	actor := game.Actor{PlayerID: actorID, ChatID: chatID}
	action := uno.Action{
		Type:     uno.CancelGame,
		PlayerID: actorID,
		Revision: summary.Revision,
	}

	_, err = h.service.Apply(ctx, actor, summary.GameID, action)
	if err != nil {
		if errors.Is(err, game.ErrForbidden) {
			h.reply(ctx, int64(chatID), "⚠️ Apenas o responsável pela partida pode cancelá-la.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Não foi possível cancelar a partida.", nil)
		return
	}

	h.tokens.InvalidateGame(summary.GameID)
	h.reply(ctx, int64(chatID), "🛑 <b>Partida cancelada pelo responsável.</b>", nil)
}

func (h *CommandHandler) handleSair(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID) {
	summary, err := h.service.FindChatGame(ctx, chatID)
	if err != nil {
		if errors.Is(err, game.ErrNoActiveGame) || errors.Is(err, game.ErrGameNotFound) {
			h.reply(ctx, int64(chatID), "⚠️ Nenhuma partida ativa encontrada neste grupo.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Erro ao buscar partida.", nil)
		return
	}

	actor := game.Actor{PlayerID: actorID, ChatID: chatID}
	action := uno.Action{
		Type:     uno.LeaveGame,
		PlayerID: actorID,
		Revision: summary.Revision,
	}

	outcome, err := h.service.Apply(ctx, actor, summary.GameID, action)
	if err != nil {
		if errors.Is(err, uno.ErrUnknownPlayer) || errors.Is(err, game.ErrNotParticipant) {
			h.reply(ctx, int64(chatID), "⚠️ Você não está participando desta partida.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Não foi possível sair da partida.", nil)
		return
	}

	h.tokens.InvalidateUserGame(summary.GameID, actorID)

	if outcome.View.Closed {
		h.tokens.InvalidateGame(summary.GameID)
		msg := fmt.Sprintf("👋 %s saiu da partida.\n\n%s",
			h.renderer.PlayerLink(actorID, outcome.View),
			h.renderer.RenderPublicState(outcome.View),
		)
		h.reply(ctx, int64(chatID), msg, nil)
	} else {
		msg := fmt.Sprintf("👋 %s saiu da partida.\n\n%s",
			h.renderer.PlayerLink(actorID, outcome.View),
			h.renderer.RenderPublicState(outcome.View),
		)
		h.reply(ctx, int64(chatID), msg, makeGameButtons(outcome.View))
	}
}

func (h *CommandHandler) handleEstado(ctx context.Context, chatID game.ChatID) {
	summary, err := h.service.FindChatGame(ctx, chatID)
	if err != nil {
		if errors.Is(err, game.ErrNoActiveGame) || errors.Is(err, game.ErrGameNotFound) {
			h.reply(ctx, int64(chatID), "⚠️ Nenhuma partida ativa ou recente encontrada neste grupo. Use /novo para criar uma.", nil)
			return
		}
		h.reply(ctx, int64(chatID), "❌ Erro ao buscar partida.", nil)
		return
	}

	view, err := h.service.PublicView(ctx, summary.GameID)
	if err != nil {
		h.reply(ctx, int64(chatID), "❌ Erro ao consultar estado da partida.", nil)
		return
	}

	if view.Phase == uno.Lobby {
		h.reply(ctx, int64(chatID), h.renderer.RenderLobby(view), makeGameButtons(view))
	} else {
		h.reply(ctx, int64(chatID), h.renderer.RenderPublicState(view), makeGameButtons(view))
	}
}
