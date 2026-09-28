package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

type CommandHandler struct {
	knownUsers    groups.UserRepository
	finalize      func(context.Context, game.Outcome) func()
	groupConfigs  groups.Repository
	groupsService *groups.Service
	bot           BotAPI
	service       *game.Service
	renderer      *Renderer
	tokens        *TokenStore
	botUsername   string
	logger        *slog.Logger
}

func (h *CommandHandler) SetGroupsService(s *groups.Service) {
	h.groupsService = s
}

func (h *CommandHandler) getGroupsService() *groups.Service {
	if h.groupsService != nil {
		return h.groupsService
	}
	if h.groupConfigs != nil {
		return &groups.Service{
			Repository: h.groupConfigs,
			LookupMembership: func(ctx context.Context, chatID, userID int64) (groups.Membership, error) {
				return lookupMembershipAPI(ctx, h.bot, chatID, userID)
			},
		}
	}
	return nil
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
	if view.Phase == uno.Lobby {
		isCaseiro := view.Rules.StackWildDrawFourOnTwo || view.Rules.StackDrawTwoOnWildFour
		classicText := "🎻 Clássico"
		caseiroText := "🏠 Caseiro"
		if isCaseiro {
			caseiroText = "✅ 🏠 Caseiro"
		} else {
			classicText = "✅ 🎻 Clássico"
		}
		return &telego.InlineKeyboardMarkup{
			InlineKeyboard: [][]telego.InlineKeyboardButton{
				{
					{
						Text:         classicText,
						CallbackData: fmt.Sprintf("mode_classic_%s", view.GameID),
					},
					{
						Text:         caseiroText,
						CallbackData: fmt.Sprintf("mode_caseiro_%s", view.GameID),
					},
				},
			},
		}
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{
					Text:                         "🃏 Suas cartas",
					SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s_%d", view.GameID, view.Revision)),
				},
			},
		},
	}
}

func makePrivateStartButtons(botUsername string) *telego.InlineKeyboardMarkup {
	username := strings.TrimPrefix(strings.TrimSpace(botUsername), "@")
	if username == "" {
		return nil
	}
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{{{
			Text: "➕ Adicionar a um grupo",
			URL:  fmt.Sprintf("https://t.me/%s?startgroup=true", username),
		}}},
	}
}

func makeGroupConfigButtons(config groups.Config) *telego.InlineKeyboardMarkup {
	classicText := "Clássico"
	caseiroText := "Caseiro"
	if config.DefaultGameMode == groups.Caseiro {
		caseiroText = "✅ Caseiro"
	} else {
		classicText = "✅ Clássico"
	}

	legacyText := "Legado"
	updatedText := "Atualizado"
	if config.RankingSystem == groups.Updated {
		updatedText = "✅ Atualizado"
	} else {
		legacyText = "✅ Legado"
	}

	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{
					Text:         classicText,
					CallbackData: fmt.Sprintf("cfg_mode_classic_%d", config.ChatID),
				},
				{
					Text:         caseiroText,
					CallbackData: fmt.Sprintf("cfg_mode_caseiro_%d", config.ChatID),
				},
			},
			{
				{
					Text:         legacyText,
					CallbackData: fmt.Sprintf("cfg_rank_legacy_%d", config.ChatID),
				},
				{
					Text:         updatedText,
					CallbackData: fmt.Sprintf("cfg_rank_updated_%d", config.ChatID),
				},
			},
		},
	}
}

func makeGroupWelcomeButtons(chatID int64) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{
					Text:         "⚙️ Configurar",
					CallbackData: fmt.Sprintf("cfg_open_%d", chatID),
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

	cmdName, fields, ok := parseBotCommand(msg.Text, h.botUsername)
	if !ok {
		return
	}

	// Check sender identity
	if msg.From == nil || msg.From.IsBot {
		return
	}
	if msg.SenderChat != nil {
		h.reply(ctx, msg.Chat.ID, "⚠️ Ações devem ser executadas por um usuário identificável, não como canal ou administrador anônimo.", nil)
		return
	}

	// A message thread can exist outside forums; only IsTopicMessage identifies a topic.
	if msg.IsTopicMessage {
		h.reply(ctx, msg.Chat.ID, "⚠️ Tópicos de fórum ainda não são suportados. Crie e jogue a partida no chat geral do grupo.", nil)
		return
	}

	actorID := uno.PlayerID(msg.From.ID)
	chatID := game.ChatID(msg.Chat.ID)
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"

	// Private chat command handling
	if !isGroup {
		switch cmdName {
		case "start":
			h.reply(ctx, msg.Chat.ID, h.renderer.RenderWelcome(), makePrivateStartButtons(h.botUsername))
		case "ajuda", "help":
			h.reply(ctx, msg.Chat.ID, h.renderer.RenderHelp(h.botUsername), nil)
		default:
			h.reply(ctx, msg.Chat.ID, "⚠️ Este comando só pode ser utilizado em grupos. Adicione o bot a um grupo para jogar!\n\nUse /help para mais instruções.", nil)
		}
		return
	}

	// Observed names stay in RAM during gameplay and are flushed at closure.
	if summary, err := h.service.FindChatGame(ctx, chatID); err == nil {
		_ = h.service.ObservePlayer(ctx, game.Actor{PlayerID: actorID, ChatID: chatID}, summary.GameID, observedName(*msg.From), msg.From.Username)
	}
	// Group command handling
	switch cmdName {
	case "novo":
		mode := ""
		if len(fields) > 1 {
			mode = "classic"
			if strings.EqualFold(fields[1], "caseiro") {
				mode = "caseiro"
			}
		}
		h.handleNovoObserved(ctx, actorID, chatID, msg.Chat.Title, mode, msg.From)
	case "trancar", "destrancar":
		h.handleRoomLock(ctx, actorID, chatID, cmdName == "trancar")
	case "entrar":
		h.handleEntrar(ctx, actorID, chatID)
	case "start":
		if len(fields) > 1 && fields[1] == "true" {
			h.reply(ctx, msg.Chat.ID, "👋 <b>UnoBotGO adicionado!</b>\n\nUse /novo para criar uma partida ou /help para conhecer os comandos.", nil)
			return
		}
		h.handleIniciar(ctx, actorID, chatID)
	case "iniciar":
		h.handleIniciar(ctx, actorID, chatID)
	case "cancelar", "kill":
		h.handleCancelar(ctx, actorID, chatID)
	case "sair":
		h.handleSair(ctx, actorID, chatID)
	case "estado":
		h.handleEstado(ctx, chatID)
	case "reset":
		h.HandleReset(ctx, msg, nil)
	case "config":
		h.handleConfig(ctx, msg)
	case "ajuda", "help":
		h.reply(ctx, msg.Chat.ID, h.renderer.RenderHelp(h.botUsername), nil)
	}
}

func parseBotCommand(text, botUsername string) (string, []string, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", nil, false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 || len(fields[0]) < 2 {
		return "", nil, false
	}
	cmdParts := strings.SplitN(fields[0][1:], "@", 2)
	if len(cmdParts) == 2 && botUsername != "" && !strings.EqualFold(cmdParts[1], botUsername) {
		return "", nil, false
	}
	return strings.ToLower(cmdParts[0]), fields, true
}

// HandleReset runs through the dispatcher's independent recovery lane in
// production. resetExecution invalidates queued work only after authorization.
func (h *CommandHandler) HandleReset(ctx context.Context, msg *telego.Message, resetExecution func()) {
	if msg == nil || msg.From == nil || msg.From.IsBot {
		return
	}
	if msg.SenderChat != nil {
		h.reply(ctx, msg.Chat.ID, "⚠️ O /reset deve ser executado por um administrador identificável, não como canal ou administrador anônimo.", nil)
		return
	}
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		h.reply(ctx, msg.Chat.ID, "⚠️ Este comando só pode ser utilizado em grupos.", nil)
		return
	}
	if msg.IsTopicMessage {
		h.reply(ctx, msg.Chat.ID, "⚠️ Tópicos de fórum ainda não são suportados. Execute /reset no chat geral do grupo.", nil)
		return
	}
	cmdName, _, ok := parseBotCommand(msg.Text, h.botUsername)
	if !ok || cmdName != "reset" {
		return
	}

	actorID := uno.PlayerID(msg.From.ID)
	chatID := game.ChatID(msg.Chat.ID)
	isOwner := false
	if summary, err := h.service.FindChatGame(ctx, chatID); err == nil {
		isOwner = summary.OwnerID == actorID
	}
	isAdmin := false
	if !isOwner {
		roleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		member, err := h.bot.GetChatMember(roleCtx, &telego.GetChatMemberParams{
			ChatID: telego.ChatID{ID: msg.Chat.ID},
			UserID: msg.From.ID,
		})
		cancel()
		if err != nil {
			h.logger.WarnContext(ctx, "failed to verify reset permission", "chat_id", chatID, "user_id", actorID, "error", err.Error())
			h.reply(ctx, msg.Chat.ID, "❌ Não foi possível confirmar sua permissão de administrador. O responsável atual ainda pode usar /reset.", nil)
			return
		}
		if member != nil {
			status := member.MemberStatus()
			isAdmin = status == telego.MemberStatusCreator || status == telego.MemberStatusAdministrator
		}
		if !isAdmin {
			h.reply(ctx, msg.Chat.ID, "⚠️ Apenas o responsável pela partida ou um administrador do grupo pode usar /reset.", nil)
			return
		}
	}

	if resetExecution != nil {
		resetExecution()
	}
	resetCtx, cancelReset := context.WithTimeout(ctx, 10*time.Second)
	result, err := h.service.ResetChat(resetCtx, game.Actor{PlayerID: actorID, ChatID: chatID, ChatAdmin: isAdmin})
	cancelReset()
	if err != nil {
		if errors.Is(err, game.ErrForbidden) {
			h.reply(ctx, msg.Chat.ID, "⚠️ Sua autorização mudou antes do reset. Tente novamente como administrador do grupo.", nil)
			return
		}
		h.logger.ErrorContext(ctx, "failed to reset chat state", "chat_id", chatID, "user_id", actorID, "error", err.Error())
		h.reply(ctx, msg.Chat.ID, "❌ Não foi possível resetar o estado deste grupo.", nil)
		return
	}
	for _, gameID := range result.GameIDs {
		h.tokens.InvalidateGame(gameID)
	}
	h.logger.WarnContext(ctx, "chat state reset",
		"chat_id", chatID,
		"user_id", actorID,
		"removed_games", len(result.GameIDs),
		"removed_active", result.RemovedActive,
		"removed_history", result.RemovedHistory,
	)
	if len(result.GameIDs) == 0 {
		h.reply(ctx, msg.Chat.ID, "♻️ <b>A fila deste grupo foi renovada.</b> Não havia partida ou histórico para remover. Use /novo para iniciar.", nil)
		return
	}
	h.reply(ctx, msg.Chat.ID, "♻️ <b>Estado deste grupo resetado.</b> Tarefas e botões antigos foram invalidados. Use /novo para iniciar uma nova partida.", nil)
}

func (h *CommandHandler) handleNovo(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID, chatTitle, mode string) {
	h.handleNovoObserved(ctx, actorID, chatID, chatTitle, mode, nil)
}
func (h *CommandHandler) handleNovoObserved(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID, chatTitle, mode string, user *telego.User) {
	config := groups.Defaults(int64(chatID))
	if h.groupConfigs != nil {
		var err error
		config, err = h.groupConfigs.GetOrCreateGroupConfig(ctx, int64(chatID))
		if err != nil {
			h.logger.Warn("failed to load group config", "chat_id", chatID, "error", err)
			h.reply(ctx, int64(chatID), "❌ Não foi possível carregar a configuração do grupo. Tente novamente.", nil)
			return
		}
	}
	if h.knownUsers != nil && user != nil {
		if err := h.knownUsers.ObserveGroupUser(ctx, groups.KnownUser{ChatID: int64(chatID), UserID: user.ID, DisplayName: observedName(*user), Username: user.Username, LastSeenAt: time.Now().UTC()}); err != nil {
			h.logger.Warn("failed to observe group user", "chat_id", chatID, "error", err)
		}
	}
	if mode == "" {
		mode = string(config.DefaultGameMode)
	}
	rules := uno.BotRules()
	if strings.EqualFold(mode, "caseiro") {
		rules = uno.CaseiroRules()
	}
	req := game.CreateRequest{
		GroupConfig: config.Snapshot(),
		ChatName:    chatTitle,
		Rules:       rules,
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
		case errors.Is(err, uno.ErrAlreadyFinished):
			h.reply(ctx, int64(chatID), "🏁 Você já terminou esta partida e não pode entrar novamente.", nil)
		case errors.Is(err, uno.ErrAlreadyJoined):
			h.reply(ctx, int64(chatID), "⚠️ Você já está inscrito nesta partida!", nil)
		case errors.Is(err, game.ErrRoomLocked):
			h.reply(ctx, int64(chatID), "🔒 Esta partida está trancada e não aceita novos jogadores.", nil)
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

	var notify func()
	if h.finalize != nil {
		notify = h.finalize(ctx, outcome)
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
	if notify != nil {
		notify()
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

func (h *CommandHandler) handleRoomLock(ctx context.Context, actorID uno.PlayerID, chatID game.ChatID, locked bool) {
	summary, err := h.service.FindChatGame(ctx, chatID)
	if err != nil {
		h.reply(ctx, int64(chatID), "⚠️ Nenhuma partida ativa encontrada neste grupo.", nil)
		return
	}
	_, changed, err := h.service.SetLocked(ctx, game.Actor{PlayerID: actorID, ChatID: chatID}, summary.GameID, locked)
	if err != nil {
		if errors.Is(err, game.ErrForbidden) {
			h.reply(ctx, int64(chatID), "⚠️ Apenas o responsável pela partida pode trancar ou destrancar.", nil)
		} else {
			h.reply(ctx, int64(chatID), "⚠️ Não foi possível alterar as entradas desta partida.", nil)
		}
		return
	}
	text := "🔓 A partida foi destrancada.\nNovos jogadores podem entrar novamente."
	if locked {
		text = "🔒 A partida foi trancada.\nNovos jogadores não poderão entrar."
	}
	if !changed {
		text = "🔓 A partida já está aberta."
		if locked {
			text = "🔒 A partida já está trancada."
		}
	}
	h.reply(ctx, int64(chatID), text, nil)
}

func (h *CommandHandler) handleConfig(ctx context.Context, msg *telego.Message) {
	if msg == nil || msg.From == nil {
		return
	}
	chatID := msg.Chat.ID
	actorID := msg.From.ID

	svc := h.getGroupsService()
	if svc == nil {
		h.reply(ctx, chatID, "❌ Configuração não disponível no momento.", nil)
		return
	}

	if svc.Repository != nil {
		if _, err := svc.Repository.GetOrCreateGroupConfig(ctx, chatID); err != nil {
			h.logger.WarnContext(ctx, "failed to ensure group config in /config", "chat_id", chatID, "error", err)
		} else if h.knownUsers != nil {
			if err := h.knownUsers.ObserveGroupUser(ctx, groups.KnownUser{
				ChatID:      chatID,
				UserID:      actorID,
				DisplayName: observedName(*msg.From),
				Username:    msg.From.Username,
				LastSeenAt:  time.Now().UTC(),
			}); err != nil {
				h.logger.WarnContext(ctx, "failed to observe user in /config", "chat_id", chatID, "error", err)
			}
		}
	}

	cfg, allowed, err := svc.CanConfigureUser(ctx, chatID, actorID)
	if err != nil && !errors.Is(err, groups.ErrForbidden) {
		h.logger.WarnContext(ctx, "failed to check config permissions", "chat_id", chatID, "user_id", actorID, "error", err)
	}

	if !allowed {
		h.reply(ctx, chatID, "⚠️ Somente administradores ou quem adicionou o bot pode alterar esta configuração.", nil)
		return
	}

	text := h.renderer.RenderGroupConfig(cfg)
	buttons := makeGroupConfigButtons(cfg)
	h.reply(ctx, chatID, text, buttons)
}

func (h *CommandHandler) HandleMyChatMember(ctx context.Context, update *telego.ChatMemberUpdated) {
	if update == nil {
		return
	}
	chatID := update.Chat.ID
	isGroup := update.Chat.Type == "group" || update.Chat.Type == "supergroup"
	if !isGroup {
		return
	}

	if update.OldChatMember == nil || update.NewChatMember == nil {
		return
	}

	oldStatus := update.OldChatMember.MemberStatus()
	newStatus := update.NewChatMember.MemberStatus()

	wasAbsent := oldStatus == telego.MemberStatusLeft || oldStatus == telego.MemberStatusBanned
	isPresent := newStatus == telego.MemberStatusMember || newStatus == telego.MemberStatusAdministrator

	if !wasAbsent || !isPresent {
		return
	}

	svc := h.getGroupsService()
	if update.From.ID > 0 && !update.From.IsBot {
		if svc != nil {
			if _, err := svc.RecordInstallation(ctx, chatID, update.From.ID); err != nil {
				h.logger.WarnContext(ctx, "failed to record bot installation", "chat_id", chatID, "user_id", update.From.ID, "error", err)
			}
		}
		if h.knownUsers != nil {
			if err := h.knownUsers.ObserveGroupUser(ctx, groups.KnownUser{
				ChatID:      chatID,
				UserID:      update.From.ID,
				DisplayName: observedName(update.From),
				Username:    update.From.Username,
				LastSeenAt:  time.Now().UTC(),
			}); err != nil {
				h.logger.WarnContext(ctx, "failed to observe installer user", "chat_id", chatID, "error", err)
			}
		}
	}

	config := groups.Defaults(chatID)
	if svc != nil && svc.Repository != nil {
		if c, err := svc.Repository.GetOrCreateGroupConfig(ctx, chatID); err == nil {
			config = c
		}
	}

	text := h.renderer.RenderGroupWelcome(config)
	buttons := makeGroupWelcomeButtons(chatID)
	h.reply(ctx, chatID, text, buttons)
}
