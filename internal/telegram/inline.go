package telegram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"slices"
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
		queryBody := strings.TrimPrefix(rawQuery, "g_")
		gameIDStr := strings.SplitN(queryBody, "_", 2)[0]
		targetGameID := uno.GameID(gameIDStr)
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
							SwitchInlineQueryCurrentChat: stringPtr(fmt.Sprintf("g_%s_%d", g.GameID, g.Revision)),
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
	if errors.Is(err, game.ErrGameClosed) {
		return []telego.InlineQueryResult{&telego.InlineQueryResultArticle{
			Type:                "article",
			ID:                  "closed_game",
			Title:               "Partida encerrada",
			Description:         "Esta partida já foi encerrada.",
			InputMessageContent: &telego.InputTextMessageContent{MessageText: "Esta partida já foi encerrada."},
		}}, ""
	}
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

	if view.Public.Phase == uno.Lobby {
		return []telego.InlineQueryResult{
			&telego.InlineQueryResultArticle{
				Type:        "article",
				ID:          fmt.Sprintf("lobby_%s", gameID),
				Title:       "A partida ainda não começou",
				Description: "Aguarde o responsável iniciar com /iniciar",
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: fmt.Sprintf("A partida no grupo <b>%s</b> ainda não foi iniciada. Aguarde o responsável usar /iniciar!", view.Public.ChatName),
					ParseMode:   "HTML",
				},
				ReplyMarkup: makeGameButtons(view.Public),
			},
		}, ""
	}

	var results []telego.InlineQueryResult
	sortedHand := sortHand(view.Hand)

	// 1. Action controls if it's the player's turn
	if view.Public.Phase == uno.ChoosingPlayer {
		return h.playerChoiceResults(actorID, view), ""
	}
	if view.Public.Phase == uno.ChoosingColor {
		if view.Public.ColorChooserID == actorID {
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
					Title:       "Escolha sua cor",
					Description: c.name,
					InputMessageContent: &telego.InputTextMessageContent{
						MessageText: c.name,
					},
				})
			}

			// 5th article: hand summary
			if len(sortedHand) > 0 {
				var descs []string
				for _, cv := range sortedHand {
					descs = append(descs, CardRepr(cv.Card))
				}
				results = append(results, &telego.InlineQueryResultArticle{
					Type:        "article",
					ID:          fmt.Sprintf("hand_%s_%d", gameID, view.Public.Revision),
					Title:       "Cartas (toque para estado do jogo):",
					Description: strings.Join(descs, ", "),
					InputMessageContent: &telego.InputTextMessageContent{
						MessageText: h.renderer.RenderPublicState(view.Public),
						ParseMode:   "HTML",
					},
				})
			}

			return results, ""
		}

		// Other player viewing hand during ChoosingColor
		chooserName := h.renderer.userCache.GetRawName(view.Public.ColorChooserID)
		results = append(results, &telego.InlineQueryResultArticle{
			Type:        "article",
			ID:          fmt.Sprintf("wait_%s_%d", gameID, view.Public.Revision),
			Title:       "Aguardando escolha de cor",
			Description: fmt.Sprintf("Aguardando %s escolher a cor.", chooserName),
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: h.renderer.RenderPublicState(view.Public),
				ParseMode:   "HTML",
			},
		})

		if len(sortedHand) > 0 {
			var descs []string
			for _, cv := range sortedHand {
				descs = append(descs, CardRepr(cv.Card))
			}
			results = append(results, &telego.InlineQueryResultArticle{
				Type:        "article",
				ID:          fmt.Sprintf("hand_%s_%d", gameID, view.Public.Revision),
				Title:       "Suas cartas (toque para estado do jogo):",
				Description: strings.Join(descs, ", "),
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: h.renderer.RenderPublicState(view.Public),
					ParseMode:   "HTML",
				},
			})
		}

		return results, ""
	} else if view.Public.Phase == uno.TakingTurn && view.Public.CurrentTurn == actorID {
		if view.DrawnCardID == "" {
			// Player can draw
			tokDraw, _ := h.tokens.CreateActionToken(actorID, gameID, view.Public.ChatID, uno.Action{
				Type:     uno.DrawCard,
				PlayerID: actorID,
				Revision: view.Public.Revision,
			}, h.tokenTTL)

			n := view.Public.DrawCounter
			if n == 0 {
				n = 1
			}
			cardWord := "carta"
			if n != 1 {
				cardWord = "cartas"
			}
			msgText := fmt.Sprintf("Comprando %d %s", n, cardWord)

			results = append(results, &telego.InlineQueryResultCachedSticker{
				Type:          "sticker",
				ID:            tokDraw,
				StickerFileID: Stickers["option_draw"],
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: msgText,
				},
			})

			if view.Public.CanCallBluff {
				tokBluff, _ := h.tokens.CreateActionToken(actorID, gameID, view.Public.ChatID, uno.Action{
					Type:     uno.CallBluff,
					PlayerID: actorID,
					Revision: view.Public.Revision,
				}, h.tokenTTL)

				results = append(results, &telego.InlineQueryResultCachedSticker{
					Type:          "sticker",
					ID:            tokBluff,
					StickerFileID: Stickers["option_bluff"],
					InputMessageContent: &telego.InputTextMessageContent{
						MessageText: "Desafiando blefe!",
					},
				})
			}
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
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: "Passar",
				},
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

	if offset < len(sortedHand) {
		end := offset + 40 // Leave room for header + controls (total < 50)
		if end > len(sortedHand) {
			end = len(sortedHand)
		}

		for i := offset; i < end; i++ {
			cv := sortedHand[i]
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

		if view.Public.Phase == uno.TakingTurn && view.Public.CurrentTurn == actorID {
			results = append(results, &telego.InlineQueryResultCachedSticker{
				Type:          "sticker",
				ID:            fmt.Sprintf("info_%s_%d", gameID, view.Public.Revision),
				StickerFileID: Stickers["option_info"],
				InputMessageContent: &telego.InputTextMessageContent{
					MessageText: h.renderer.RenderPublicState(view.Public),
					ParseMode:   "HTML",
				},
			})
		}

		nextOffset := ""
		if end < len(sortedHand) {
			if curTok, err := h.tokens.CreateCursorToken(actorID, CursorKindHand, gameID, view.Public.Revision, end, h.tokenTTL); err == nil {
				nextOffset = curTok
			}
		}

		return results, nextOffset
	}

	return results, ""
}

func (h *InlineHandler) HandleChosenInlineResult(ctx context.Context, chosen *telego.ChosenInlineResult) bool {
	if chosen == nil {
		return true
	}

	actorID := uno.PlayerID(chosen.From.ID)
	h.renderer.userCache.Put(actorID, chosen.From.FirstName, chosen.From.Username)

	tokenStr := chosen.ResultID
	if strings.HasPrefix(tokenStr, "grey_") || strings.HasPrefix(tokenStr, "hdr_") || strings.HasPrefix(tokenStr, "select_") || strings.HasPrefix(tokenStr, "no_") || strings.HasPrefix(tokenStr, "info_") || strings.HasPrefix(tokenStr, "hand_") || strings.HasPrefix(tokenStr, "wait_") {
		return true
	}

	actionToken, status := h.tokens.ConsumeAction(tokenStr, actorID)
	if status != ConsumeOK {
		h.logger.DebugContext(ctx, "ignoring unconsumed action token in chosen inline result", "status", status, "user_id", actorID)
		return true
	}

	// Schedule the state change in the designated chat worker queue to ensure in-order execution
	accepted := h.dispatcher.EnqueueChat(actionToken.ChatID, func(taskCtx context.Context) {
		actor := game.Actor{PlayerID: actorID, ChatID: actionToken.ChatID}
		outcome, err := h.service.Apply(taskCtx, actor, actionToken.GameID, actionToken.Action)
		if err != nil {
			h.replyActionError(taskCtx, actorID, tokenStr, actionToken, err)
			return
		}

		// Success!
		h.tokens.SetActionResult(tokenStr, "confirmed")
		if outcome.View.Closed {
			h.tokens.InvalidateGame(actionToken.GameID)
		}

		for _, ev := range outcome.Events {
			if ev.Type == uno.UnoAnnounced {
				unoMsg, err := h.bot.SendMessage(taskCtx, &telego.SendMessageParams{
					ChatID:    telego.ChatID{ID: int64(actionToken.ChatID)},
					Text:      fmt.Sprintf("%s <b>Gritou UNO!</b>", h.renderer.PlayerLink(ev.PlayerID, outcome.View)),
					ParseMode: "HTML",
				})
				if err == nil && unoMsg != nil {
					_ = h.bot.SetMessageReaction(taskCtx, &telego.SetMessageReactionParams{
						ChatID:    telego.ChatID{ID: int64(actionToken.ChatID)},
						MessageID: unoMsg.MessageID,
						Reaction: []telego.ReactionType{
							&telego.ReactionTypeEmoji{
								Type:  telego.ReactionEmoji,
								Emoji: "🥳",
							},
						},
					})
				}
			}
		}

		confText := h.renderer.RenderActionConfirmation(actorID, actionToken.Action, outcome)
		params := &telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: int64(actionToken.ChatID)},
			Text:      confText,
			ParseMode: "HTML",
		}
		if markup := makeGameButtons(outcome.View); markup != nil {
			params.ReplyMarkup = markup
		}
		_, _ = h.bot.SendMessage(taskCtx, params)
	})
	return accepted
}

// Failed actions use current public context, never the old token's turn.
func (h *InlineHandler) replyActionError(ctx context.Context, actorID uno.PlayerID, tokenStr string, token ActionToken, actionErr error) {
	status := "rejected"
	message := "Jogada não aceita: " + html.EscapeString(actionErr.Error()) + "."
	if errors.Is(actionErr, uno.ErrStaleRevision) {
		status = "stale"
		message = "Seleção antiga: a partida mudou."
	}
	h.tokens.SetActionResult(tokenStr, status)
	view, viewErr := h.service.PublicView(ctx, token.GameID)
	var markup *telego.InlineKeyboardMarkup
	switch {
	case viewErr != nil:
		view = game.PublicGameView{}
		message = "Esta partida não está disponível."
	case view.Closed || view.Phase == uno.Finished:
		message = "Esta partida já foi encerrada."
	default:
		// A player who left or already placed cannot reopen a private hand.
		if _, err := h.service.PlayerView(ctx, game.Actor{PlayerID: actorID, ChatID: token.ChatID}, token.GameID); err == nil {
			message += " Abra Suas cartas novamente."
			markup = makeGameButtons(view)
		}
	}
	params := &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: int64(token.ChatID)},
		Text:      fmt.Sprintf("⚠️ %s: %s", h.renderer.PlayerLink(actorID, view), message),
		ParseMode: "HTML",
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	_, _ = h.bot.SendMessage(ctx, params)
}

func sortHand(hand []game.CardView) []game.CardView {
	sorted := make([]game.CardView, len(hand))
	copy(sorted, hand)
	slices.SortStableFunc(sorted, func(a, b game.CardView) int {
		ar := colorSortRank(a.Card.Color, a.Card.Rank)
		br := colorSortRank(b.Card.Color, b.Card.Rank)
		if ar != br {
			return ar - br
		}
		if a.Card.Rank != b.Card.Rank {
			return int(a.Card.Rank) - int(b.Card.Rank)
		}
		return strings.Compare(string(a.Card.ID), string(b.Card.ID))
	})
	return sorted
}

func colorSortRank(c uno.Color, r uno.Rank) int {
	if r >= uno.Wild {
		return 99
	}
	switch c {
	case uno.Red:
		return 0
	case uno.Blue:
		return 1
	case uno.Green:
		return 2
	case uno.Yellow:
		return 3
	default:
		return 98
	}
}

// playerChoiceResults shares the wildcard's inline selection flow. Only names
// and public counts identify targets; no opponent's hand is exposed.
func (h *InlineHandler) playerChoiceResults(actorID uno.PlayerID, view game.PlayerGameView) []telego.InlineQueryResult {
	var results []telego.InlineQueryResult
	if view.Public.PlayerChooserID == actorID {
		for _, player := range view.Public.Players {
			if !player.Active || player.ID == actorID {
				continue
			}
			token, err := h.tokens.CreateActionToken(actorID, view.Public.GameID, view.Public.ChatID, uno.Action{
				Type: uno.ChoosePlayer, PlayerID: actorID, TargetID: player.ID, Revision: view.Public.Revision,
			}, h.tokenTTL)
			if err != nil {
				continue
			}
			name := h.renderer.userCache.GetRawName(player.ID)
			results = append(results, &telego.InlineQueryResultArticle{
				Type: "article", ID: token, Title: "Trocar cartas com " + name,
				Description:         fmt.Sprintf("%d carta(s)", player.CardCount),
				InputMessageContent: &telego.InputTextMessageContent{MessageText: "Escolhendo " + name + " para trocar cartas."},
			})
		}
	} else {
		results = append(results, &telego.InlineQueryResultArticle{
			Type: "article", ID: fmt.Sprintf("wait_%s_%d", view.Public.GameID, view.Public.Revision),
			Title:               "Aguardando escolha de jogador",
			Description:         h.renderer.userCache.GetRawName(view.Public.PlayerChooserID) + " está escolhendo com quem trocar cartas.",
			InputMessageContent: &telego.InputTextMessageContent{MessageText: h.renderer.RenderPublicState(view.Public), ParseMode: "HTML"},
		})
	}
	var descriptions []string
	for _, card := range sortHand(view.Hand) {
		descriptions = append(descriptions, CardRepr(card.Card))
	}
	if len(descriptions) > 0 {
		results = append(results, &telego.InlineQueryResultArticle{
			Type: "article", ID: fmt.Sprintf("hand_%s_%d", view.Public.GameID, view.Public.Revision),
			Title: "Suas cartas (toque para estado do jogo):", Description: strings.Join(descriptions, ", "),
			InputMessageContent: &telego.InputTextMessageContent{MessageText: h.renderer.RenderPublicState(view.Public), ParseMode: "HTML"},
		})
	}
	return results
}
