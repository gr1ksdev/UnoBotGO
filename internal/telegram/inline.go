package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

type InlineHandler struct {
	bot        BotAPI
	service    *game.Service
	renderer   *Renderer
	tokens     *TokenStore
	tokenTTL   time.Duration
	dispatcher *Dispatcher
	logger     *slog.Logger
}

func NewInlineHandler(
	bot BotAPI,
	service *game.Service,
	renderer *Renderer,
	tokens *TokenStore,
	tokenTTL time.Duration,
	dispatcher *Dispatcher,
	logger *slog.Logger,
) *InlineHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if tokenTTL <= 0 {
		tokenTTL = 2 * time.Minute
	}
	return &InlineHandler{
		bot:        bot,
		service:    service,
		renderer:   renderer,
		tokens:     tokens,
		tokenTTL:   tokenTTL,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

func makeValidatingInlineMarkup(gameID uno.GameID, token string) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: "⏳ Aguardando validação", CallbackData: fmt.Sprintf("st_%s", token)},
				{Text: "🃏 Suas cartas", SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s", gameID))},
			},
		},
	}
}

func makeConfirmedInlineMarkup(gameID uno.GameID) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: "✅ Confirmado", CallbackData: "noop"},
				{Text: "🃏 Suas cartas", SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s", gameID))},
			},
		},
	}
}

func makeStaleInlineMarkup(gameID uno.GameID) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{
			{
				{Text: "❌ Seleção antiga", CallbackData: "noop"},
				{Text: "🃏 Suas cartas", SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s", gameID))},
			},
		},
	}
}

func (h *InlineHandler) HandleInlineQuery(ctx context.Context, query *telego.InlineQuery) {
	if query == nil {
		return
	}

	actorID := uno.PlayerID(query.From.ID)
	h.renderer.userCache.Put(actorID, query.From.FirstName, query.From.Username)

	rawQuery := strings.TrimSpace(query.Query)
	var results []telego.InlineQueryResult
	var nextOffset string

	if rawQuery == "" {
		// Empty query: find all active games for user
		playerGames, err := h.service.FindPlayerGames(ctx, game.Actor{PlayerID: actorID})
		if err != nil || len(playerGames) == 0 {
			results = append(results, &telego.InlineQueryResultArticle{
				Type:        "article",
				ID:          "no_game",
				Title:       "Você não está participando de nenhuma partida",
				Description: "Use /novo para criar ou /entrar em um grupo para jogar.",
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: "Você não está participando de nenhuma partida de UNO no momento. Use /novo para criar ou /entrar em um grupo para jogar!",
				},
			})
		} else if len(playerGames) == 1 {
			// Single game: open hand directly
			targetGameID := playerGames[0].GameID
			results, nextOffset = h.buildPlayerHandResults(ctx, actorID, targetGameID, query.Offset)
		} else {
			// Multiple games: render selector
			results, nextOffset = h.buildGameSelectorResults(ctx, actorID, playerGames, query.Offset)
		}
	} else if strings.HasPrefix(rawQuery, "g_") {
		targetGameID := uno.GameID(strings.TrimPrefix(rawQuery, "g_"))
		results, nextOffset = h.buildPlayerHandResults(ctx, actorID, targetGameID, query.Offset)
	} else {
		// Unknown query format
		results = append(results, &telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          "invalid_query",
			Title:       "Consulta não reconhecida",
			Description: "Abra @bot para listar suas partidas.",
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: "Consulta não reconhecida. Use @bot para ver suas partidas.",
			},
		})
	}

	params := &telego.AnswerInlineQueryParams{
		InlineQueryID: query.ID,
		Results:       results,
		CacheTime:     0,
		IsPersonal:    true,
		NextOffset:    nextOffset,
	}

	if err := h.bot.AnswerInlineQuery(ctx, params); err != nil {
		h.logger.WarnContext(ctx, "failed to answer inline query", "query_id", query.ID, "user_id", actorID, "error", err.Error())
	}
}

func (h *InlineHandler) buildGameSelectorResults(
	ctx context.Context,
	actorID uno.PlayerID,
	games []game.GameSummary,
	offsetStr string,
) ([]telego.InlineQueryResult, string) {
	offset := 0
	if offsetStr != "" {
		if cursor, err := h.tokens.GetCursor(offsetStr, actorID); err == nil && cursor.Kind == CursorKindGames {
			offset = cursor.Offset
		}
	}

	if offset >= len(games) {
		return nil, ""
	}

	end := offset + 45
	if end > len(games) {
		end = len(games)
	}

	results := make([]telego.InlineQueryResult, 0, end-offset)
	for i := offset; i < end; i++ {
		g := games[i]
		chatTitle := g.ChatName
		if chatTitle == "" {
			chatTitle = fmt.Sprintf("Grupo %d", g.ChatID)
		}

		results = append(results, &telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          fmt.Sprintf("select_%s", g.GameID),
			Title:       chatTitle,
			Description: fmt.Sprintf("Partida ativa | Fase: %v", g.Phase),
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: fmt.Sprintf("Partida em <b>%s</b>", chatTitle),
				ParseMode:   "HTML",
			},
			ReplyMarkup: &telego.InlineKeyboardMarkup{
				InlineKeyboard: [][]telego.InlineKeyboardButton{
					{
						{
							Text:                         "🃏 Abrir minha mão",
							SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s", g.GameID)),
						},
					},
				},
			},
		})
	}

	nextOffset := ""
	if end < len(games) {
		if curTok, err := h.tokens.CreateCursorToken(actorID, CursorKindGames, "", 0, end, h.tokenTTL); err == nil {
			nextOffset = curTok
		}
	}

	return results, nextOffset
}

func (h *InlineHandler) buildPlayerHandResults(
	ctx context.Context,
	actorID uno.PlayerID,
	gameID uno.GameID,
	offsetStr string,
) ([]telego.InlineQueryResult, string) {
	view, err := h.service.PlayerView(ctx, game.Actor{PlayerID: actorID}, gameID)
	if err != nil {
		return []telego.InlineQueryResult{
			&telego.InlineQueryResultArticle{
				Type:        "article",
				ID:          "unavailable_game",
				Title:       "Partida não disponível ou você não participa dela",
				Description: "Abra @bot novamente para ver suas partidas ativas.",
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: "Esta partida não está disponível ou você não está participando dela.",
				},
			},
		}, ""
	}

	var results []telego.InlineQueryResult

	// 1. Header article with public game overview
	headerTitle := fmt.Sprintf("Partida: %s", view.Public.ChatName)
	if headerTitle == "Partida: " {
		headerTitle = fmt.Sprintf("Partida (ID: %s)", view.Public.GameID)
	}

	headerDesc := "Consultar estado da mesa"
	if view.Public.CurrentTurn == actorID {
		headerDesc = "👉 É A SUA VEZ!"
	} else if view.Public.CurrentTurn > 0 {
		headerDesc = fmt.Sprintf("Vez de: %s", h.renderer.userCache.GetRawName(view.Public.CurrentTurn))
	}

	results = append(results, &telego.InlineQueryResultArticle{
		Type:        "article",
		ID:          fmt.Sprintf("hdr_%s_%d", gameID, view.Public.Revision),
		Title:       headerTitle,
		Description: headerDesc,
		InputMessageContent: &telego.InputTextMessageContent{
			MessageText: h.renderer.RenderPublicState(view.Public),
			ParseMode:   "HTML",
		},
		ReplyMarkup: makeGameButtons(gameID),
	})

	// 2. Action controls if it's the player's turn
	if view.Public.Phase == uno.ChoosingColor && view.Public.ColorChooserID == actorID {
		// 4 color articles
		colors := []struct {
			color uno.Color
			name  string
		}{
			{uno.Red, "❤️ Vermelho"},
			{uno.Blue, "💙 Azul"},
			{uno.Green, "💚 Verde"},
			{uno.Yellow, "💛 Amarelo"},
		}
		for _, c := range colors {
			tok, _ := h.tokens.CreateActionToken(actorID, gameID, view.Public.ChatID, uno.Action{
				Type:     uno.ChooseColor,
				PlayerID: actorID,
				Color:    c.color,
				Revision: view.Public.Revision,
			}, h.tokenTTL)

			results = append(results, &telego.InlineQueryResultArticle{
				Type:        "article",
				ID:          tok,
				Title:       c.name,
				Description: "Escolher esta cor",
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: fmt.Sprintf("Escolhi a cor <b>%s</b>!", c.name),
					ParseMode:   "HTML",
				},
				ReplyMarkup: makeValidatingInlineMarkup(gameID, tok),
			})
		}
	} else if view.Public.Phase == uno.TakingTurn && view.Public.CurrentTurn == actorID {
		if view.DrawnCardID == "" {
			// Player can draw
			tokDraw, _ := h.tokens.CreateActionToken(actorID, gameID, view.Public.ChatID, uno.Action{
				Type:     uno.DrawCard,
				PlayerID: actorID,
				Revision: view.Public.Revision,
			}, h.tokenTTL)

			results = append(results, &telego.InlineQueryResultCachedSticker{
				Type:          "sticker",
				ID:            tokDraw,
				StickerFileID: Stickers["option_draw"],
				ReplyMarkup:   makeValidatingInlineMarkup(gameID, tokDraw),
			})
		} else {
			// Player drew already, can pass
			tokPass, _ := h.tokens.CreateActionToken(actorID, gameID, view.Public.ChatID, uno.Action{
				Type:     uno.PassTurn,
				PlayerID: actorID,
				Revision: view.Public.Revision,
			}, h.tokenTTL)

			results = append(results, &telego.InlineQueryResultCachedSticker{
				Type:          "sticker",
				ID:            tokPass,
				StickerFileID: Stickers["option_pass"],
				ReplyMarkup:   makeValidatingInlineMarkup(gameID, tokPass),
			})
		}
	}

	// 3. Hand cards with pagination
	offset := 0
	if offsetStr != "" {
		if cursor, err := h.tokens.GetCursor(offsetStr, actorID); err == nil &&
			cursor.Kind == CursorKindHand &&
			cursor.GameID == gameID &&
			cursor.Revision == view.Public.Revision {
			offset = cursor.Offset
		}
	}

	if offset < len(view.Hand) {
		end := offset + 40 // Leave room for header + controls (total < 50)
		if end > len(view.Hand) {
			end = len(view.Hand)
		}

		for i := offset; i < end; i++ {
			cv := view.Hand[i]
			if cv.Playable {
				tokPlay, _ := h.tokens.CreateActionToken(actorID, gameID, view.Public.ChatID, uno.Action{
					Type:     uno.PlayCard,
					PlayerID: actorID,
					CardID:   cv.Card.ID,
					Revision: view.Public.Revision,
				}, h.tokenTTL)

				stickerID := GetCardStickerID(cv.Card)
				results = append(results, &telego.InlineQueryResultCachedSticker{
					Type:          "sticker",
					ID:            tokPlay,
					StickerFileID: stickerID,
					ReplyMarkup:   makeValidatingInlineMarkup(gameID, tokPlay),
				})
			} else {
				greyStickerID := GetCardStickerGreyID(cv.Card)
				results = append(results, &telego.InlineQueryResultCachedSticker{
					Type:          "sticker",
					ID:            fmt.Sprintf("grey_%s_%d", cv.Card.ID, i),
					StickerFileID: greyStickerID,
					InputMessageContent: &telego.InputTextMessageContent{
						MessageText: "Esta carta não pode ser jogada agora. Abra suas cartas novamente quando for a sua vez.",
					},
				})
			}
		}

		nextOffset := ""
		if end < len(view.Hand) {
			if curTok, err := h.tokens.CreateCursorToken(actorID, CursorKindHand, gameID, view.Public.Revision, end, h.tokenTTL); err == nil {
				nextOffset = curTok
			}
		}

		return results, nextOffset
	}

	return results, ""
}

func (h *InlineHandler) HandleChosenInlineResult(ctx context.Context, chosen *telego.ChosenInlineResult) {
	if chosen == nil {
		return
	}

	actorID := uno.PlayerID(chosen.From.ID)
	h.renderer.userCache.Put(actorID, chosen.From.FirstName, chosen.From.Username)

	tokenStr := chosen.ResultID
	if strings.HasPrefix(tokenStr, "grey_") || strings.HasPrefix(tokenStr, "hdr_") || strings.HasPrefix(tokenStr, "select_") || strings.HasPrefix(tokenStr, "no_") {
		return
	}

	actionToken, status := h.tokens.ConsumeAction(tokenStr, actorID)
	if status != ConsumeOK {
		h.logger.DebugContext(ctx, "ignoring unconsumed action token in chosen inline result", "status", status, "user_id", actorID)
		return
	}

	// Schedule the state change in the designated chat worker queue to ensure in-order execution
	h.dispatcher.EnqueueChat(actionToken.ChatID, func(taskCtx context.Context) {
		actor := game.Actor{PlayerID: actorID, ChatID: actionToken.ChatID}
		outcome, err := h.service.Apply(taskCtx, actor, actionToken.GameID, actionToken.Action)
		if err != nil {
			if errors.Is(err, uno.ErrStaleRevision) {
				h.tokens.SetActionResult(tokenStr, "stale")
				staleMsg := fmt.Sprintf("⚠️ %s: Seleção antiga: a partida mudou. Abra Suas cartas novamente.", h.renderer.userCache.FormatLink(actorID))
				_, _ = h.bot.SendMessage(taskCtx, &telego.SendMessageParams{
					ChatID:      telego.ChatID{ID: int64(actionToken.ChatID)},
					Text:        staleMsg,
					ParseMode:   "HTML",
					ReplyMarkup: makeGameButtons(actionToken.GameID),
				})
				if chosen.InlineMessageID != "" {
					_, _ = h.bot.EditMessageReplyMarkup(taskCtx, &telego.EditMessageReplyMarkupParams{
						InlineMessageID: chosen.InlineMessageID,
						ReplyMarkup:     makeStaleInlineMarkup(actionToken.GameID),
					})
				}
			} else {
				h.tokens.SetActionResult(tokenStr, "rejected")
				errMsg := fmt.Sprintf("⚠️ %s: Jogada não aceita: %v. Abra Suas cartas novamente.", h.renderer.userCache.FormatLink(actorID), err)
				_, _ = h.bot.SendMessage(taskCtx, &telego.SendMessageParams{
					ChatID:      telego.ChatID{ID: int64(actionToken.ChatID)},
					Text:        errMsg,
					ParseMode:   "HTML",
					ReplyMarkup: makeGameButtons(actionToken.GameID),
				})
			}
			return
		}

		// Success!
		h.tokens.SetActionResult(tokenStr, "confirmed")
		if chosen.InlineMessageID != "" {
			_, _ = h.bot.EditMessageReplyMarkup(taskCtx, &telego.EditMessageReplyMarkupParams{
				InlineMessageID: chosen.InlineMessageID,
				ReplyMarkup:     makeConfirmedInlineMarkup(actionToken.GameID),
			})
		}

		confText := h.renderer.RenderActionConfirmation(actorID, actionToken.Action, outcome)
		_, _ = h.bot.SendMessage(taskCtx, &telego.SendMessageParams{
			ChatID:      telego.ChatID{ID: int64(actionToken.ChatID)},
			Text:        confText,
			ParseMode:   "HTML",
			ReplyMarkup: makeGameButtons(actionToken.GameID),
		})
	})
}
