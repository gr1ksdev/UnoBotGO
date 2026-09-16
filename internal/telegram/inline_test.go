package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func TestInlineHandler_EmptyQuery_ZeroOneMultipleGames(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	// 1. Zero games
	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q1",
		From:  telego.User{ID: 10, FirstName: "Alice"},
		Query: "",
	})

	if len(mockAPI.AnsweredInlines) != 1 {
		t.Fatalf("expected 1 answered inline query, got %d", len(mockAPI.AnsweredInlines))
	}
	res0 := mockAPI.AnsweredInlines[0].Results
	if len(res0) != 1 {
		t.Fatalf("expected 1 article result for zero games, got %d", len(res0))
	}
	article0, ok := res0[0].(*telego.InlineQueryResultArticle)
	if !ok || article0.ID != "no_game" {
		t.Errorf("expected no_game article ID, got %+v", res0[0])
	}

	// 2. One game
	out1, _ := svc.Create(ctx, game.Actor{PlayerID: 99, ChatID: -1001}, game.CreateRequest{ChatName: "Chat 1", Rules: uno.BotRules()})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 10, ChatID: -1001}, out1.View.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 10, Revision: 0})

	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q2",
		From:  telego.User{ID: 10, FirstName: "Alice"},
		Query: "",
	})

	if len(mockAPI.AnsweredInlines) != 2 {
		t.Fatalf("expected 2 answered inlines, got %d", len(mockAPI.AnsweredInlines))
	}
	res1 := mockAPI.AnsweredInlines[1].Results
	// In lobby phase: header only
	if len(res1) == 0 {
		t.Fatalf("expected hand/header results for single game, got 0")
	}

	// 3. Multiple games
	out2, _ := svc.Create(ctx, game.Actor{PlayerID: 99, ChatID: -1002}, game.CreateRequest{ChatName: "Chat 2", Rules: uno.BotRules()})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 10, ChatID: -1002}, out2.View.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 10, Revision: 0})

	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q3",
		From:  telego.User{ID: 10, FirstName: "Alice"},
		Query: "",
	})

	if len(mockAPI.AnsweredInlines) != 3 {
		t.Fatalf("expected 3 answered inlines, got %d", len(mockAPI.AnsweredInlines))
	}
	res2 := mockAPI.AnsweredInlines[2].Results
	if len(res2) != 2 {
		t.Fatalf("expected 2 game selector articles, got %d", len(res2))
	}
}

func TestInlineHandler_PlayTurn_DrawAndPlayCard(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	// Create and start game with 2 players
	out, _ := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Group UNO", Rules: uno.BotRules()})
	gameID := out.View.GameID
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	startOut, _ := svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	currentTurnPlayer := startOut.View.CurrentTurn

	// Player of the turn queries inline with context g_<gameID>
	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q_turn",
		From:  telego.User{ID: int64(currentTurnPlayer), FirstName: "CurrentPlayer"},
		Query: "g_" + string(gameID),
	})

	if len(mockAPI.AnsweredInlines) == 0 {
		t.Fatalf("expected inline response")
	}
	results := mockAPI.AnsweredInlines[len(mockAPI.AnsweredInlines)-1].Results
	if len(results) < 2 {
		t.Fatalf("expected header + controls + hand cards, got %d results", len(results))
	}

	// Check that Draw sticker exists as a control and all results are clean stickers
	var drawToken string
	for _, r := range results {
		sticker, ok := r.(*telego.InlineQueryResultCachedSticker)
		if !ok {
			t.Fatalf("expected only sticker results for active hand, got %T", r)
		}
		if sticker.ReplyMarkup != nil {
			t.Errorf("expected clean sticker without ReplyMarkup, got: %#v", sticker.ReplyMarkup)
		}
		if sticker.StickerFileID == Stickers["option_draw"] {
			drawToken = sticker.ID
		}
	}
	if drawToken == "" {
		t.Fatalf("expected option_draw sticker in turn player inline results")
	}

	// Simulate player selecting Draw
	inlineHandler.HandleChosenInlineResult(ctx, &telego.ChosenInlineResult{
		ResultID:        drawToken,
		From:            telego.User{ID: int64(currentTurnPlayer), FirstName: "CurrentPlayer"},
		InlineMessageID: "inl_draw_msg_1",
	})

	// Wait for dispatcher chat queue to execute task
	time.Sleep(100 * time.Millisecond)

	lastMsg := mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "comprou") {
		t.Fatalf("expected draw confirmation message sent to group, got: %s", lastMsg)
	}

	// Verify sticker message was not cluttered with button edits
	if len(mockAPI.EditedMarkups) != 0 {
		t.Errorf("expected clean sticker without markup edits, got %d edits", len(mockAPI.EditedMarkups))
	}
}

func TestInlineHandler_StaleRevisionRejection(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	// Create and start game
	out, _ := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Group UNO", Rules: uno.BotRules()})
	gameID := out.View.GameID
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	startOut, _ := svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	currentTurn := startOut.View.CurrentTurn

	// Create a token with old revision 1 (current revision is 3 after StartGame)
	staleToken, _ := tokens.CreateActionToken(currentTurn, gameID, -1001, uno.Action{
		Type:     uno.DrawCard,
		PlayerID: currentTurn,
		Revision: 1, // STALE!
	}, time.Minute)

	inlineHandler.HandleChosenInlineResult(ctx, &telego.ChosenInlineResult{
		ResultID:        staleToken,
		From:            telego.User{ID: int64(currentTurn), FirstName: "CurrentPlayer"},
		InlineMessageID: "inl_stale_1",
	})

	time.Sleep(100 * time.Millisecond)

	lastMsg := mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Seleção antiga") {
		t.Fatalf("expected stale notice sent to group, got: %s", lastMsg)
	}

	// Verify sticker message was not cluttered with button edits
	if len(mockAPI.EditedMarkups) != 0 {
		t.Errorf("expected clean sticker without markup edits, got %d edits", len(mockAPI.EditedMarkups))
	}
}

func TestInlineHandler_Pagination(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	// Create game
	out, _ := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Group UNO", Rules: uno.BotRules()})
	gameID := out.View.GameID
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	startOut, _ := svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	currentTurn := startOut.View.CurrentTurn

	// Simulate drawing multiple times until hand has > 40 cards, or test pagination slicing logic
	// Each draw adds 1 card and keeps turn if card is not playable
	for i := 0; i < 40; i++ {
		rev := startOut.View.Revision + uint64(i)
		res, err := svc.Apply(ctx, game.Actor{PlayerID: currentTurn, ChatID: -1001}, gameID, uno.Action{
			Type:     uno.DrawCard,
			PlayerID: currentTurn,
			Revision: rev,
		})
		if err != nil {
			break
		}
		if res.View.CurrentTurn != currentTurn {
			currentTurn = res.View.CurrentTurn
		}
	}

	// Query inline for page 1
	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:     "q_page1",
		From:   telego.User{ID: int64(currentTurn), FirstName: "P"},
		Query:  "g_" + string(gameID),
		Offset: "",
	})

	if len(mockAPI.AnsweredInlines) == 0 {
		t.Fatalf("expected inline response")
	}
	resp1 := mockAPI.AnsweredInlines[len(mockAPI.AnsweredInlines)-1]
	// If hand had > 40 cards, next offset should be set
	view, _ := svc.PlayerView(ctx, game.Actor{PlayerID: currentTurn}, gameID)
	if len(view.Hand) > 40 {
		if resp1.NextOffset == "" {
			t.Errorf("expected NextOffset to be set for > 40 cards in hand")
		}

		// Query page 2 using NextOffset
		inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
			ID:     "q_page2",
			From:   telego.User{ID: int64(currentTurn), FirstName: "P"},
			Query:  "g_" + string(gameID),
			Offset: resp1.NextOffset,
		})
		resp2 := mockAPI.AnsweredInlines[len(mockAPI.AnsweredInlines)-1]
		if len(resp2.Results) == 0 {
			t.Errorf("expected page 2 to have cards")
		}
	}
}

func TestInlineHandler_ChoosingColorCleanV1Layout(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	out, _ := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Group UNO", Rules: uno.BotRules()})
	gameID := out.View.GameID
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	startOut, _ := svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	// Find or draw a wild card for current turn
	curr := startOut.View.CurrentTurn
	var chooserID uno.PlayerID
	for i := 0; i < 50; i++ {
		pub, _ := svc.PublicView(ctx, gameID)
		if pub.Phase == uno.ChoosingColor {
			chooserID = pub.ColorChooserID
			break
		}
		curr = pub.CurrentTurn
		pv, _ := svc.PlayerView(ctx, game.Actor{PlayerID: curr}, gameID)
		var wildCardID uno.CardID
		var otherPlayableID uno.CardID
		for _, c := range pv.Hand {
			if c.Playable {
				if c.Card.Rank >= uno.Wild {
					wildCardID = c.Card.ID
					break
				} else if otherPlayableID == "" {
					otherPlayableID = c.Card.ID
				}
			}
		}
		if wildCardID != "" {
			res, err := svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
				Type:     uno.PlayCard,
				PlayerID: curr,
				CardID:   wildCardID,
				Revision: pub.Revision,
			})
			if err == nil && res.View.Phase == uno.ChoosingColor {
				chooserID = res.View.ColorChooserID
				break
			}
		} else if otherPlayableID != "" {
			_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
				Type:     uno.PlayCard,
				PlayerID: curr,
				CardID:   otherPlayableID,
				Revision: pub.Revision,
			})
		} else {
			if pv.DrawnCardID == "" {
				_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
					Type:     uno.DrawCard,
					PlayerID: curr,
					Revision: pub.Revision,
				})
			} else {
				_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
					Type:     uno.PassTurn,
					PlayerID: curr,
					Revision: pub.Revision,
				})
			}
		}
	}

	if chooserID == 0 {
		t.Skip("Could not trigger ChoosingColor within turn limit")
	}

	// 1. Query inline as the color chooser
	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q_choose_color",
		From:  telego.User{ID: int64(chooserID), FirstName: "Chooser"},
		Query: "g_" + string(gameID),
	})

	results := mockAPI.AnsweredInlines[len(mockAPI.AnsweredInlines)-1].Results
	if len(results) != 5 {
		t.Fatalf("expected exactly 5 articles for chooser (4 colors + 1 hand), got %d results", len(results))
	}

	// Verify all 5 results are Articles, 0 stickers!
	for i, r := range results {
		art, ok := r.(*telego.InlineQueryResultArticle)
		if !ok {
			t.Fatalf("result %d is not article: %T", i, r)
		}
		if i < 4 {
			if art.Title != "Escolha sua cor" {
				t.Errorf("expected title 'Escolha sua cor', got %q", art.Title)
			}
		} else {
			if art.Title != "Cartas (toque para estado do jogo):" {
				t.Errorf("expected hand summary title, got %q", art.Title)
			}
		}
	}

	// 2. Query inline as non-chooser
	nonChooserID := uno.PlayerID(1)
	if chooserID == 1 {
		nonChooserID = 2
	}
	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q_non_chooser",
		From:  telego.User{ID: int64(nonChooserID), FirstName: "NonChooser"},
		Query: "g_" + string(gameID),
	})

	ncResults := mockAPI.AnsweredInlines[len(mockAPI.AnsweredInlines)-1].Results
	if len(ncResults) == 0 {
		t.Fatalf("expected non-chooser results")
	}
	waitArt, ok := ncResults[0].(*telego.InlineQueryResultArticle)
	if !ok || waitArt.Title != "Aguardando escolha de cor" {
		t.Fatalf("expected 'Aguardando escolha de cor' article, got %+v", ncResults[0])
	}
}

func TestInlineHandler_DrawTwoStackingDisplay(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	out, _ := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Group UNO", Rules: uno.BotRules()})
	gameID := out.View.GameID
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	// Find or draw a DrawTwo card for current player
	var p2 uno.PlayerID
	for i := 0; i < 50; i++ {
		pub, _ := svc.PublicView(ctx, gameID)
		if pub.Phase != uno.TakingTurn {
			break
		}
		curr := pub.CurrentTurn
		pv, _ := svc.PlayerView(ctx, game.Actor{PlayerID: curr}, gameID)
		var d2CardID uno.CardID
		var otherPlayableID uno.CardID
		for _, c := range pv.Hand {
			if c.Playable {
				if c.Card.Rank == uno.DrawTwo {
					d2CardID = c.Card.ID
					break
				} else if otherPlayableID == "" && c.Card.Rank < uno.Wild {
					otherPlayableID = c.Card.ID
				}
			}
		}
		if d2CardID != "" {
			res, err := svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
				Type:     uno.PlayCard,
				PlayerID: curr,
				CardID:   d2CardID,
				Revision: pub.Revision,
			})
			if err == nil && res.View.DrawCounter > 0 {
				p2 = res.View.CurrentTurn
				break
			}
		} else if otherPlayableID != "" {
			_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
				Type:     uno.PlayCard,
				PlayerID: curr,
				CardID:   otherPlayableID,
				Revision: pub.Revision,
			})
		} else {
			if pv.DrawnCardID == "" {
				_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
					Type:     uno.DrawCard,
					PlayerID: curr,
					Revision: pub.Revision,
				})
			} else {
				_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
					Type:     uno.PassTurn,
					PlayerID: curr,
					Revision: pub.Revision,
				})
			}
		}
	}

	if p2 == 0 {
		t.Skip("Could not play DrawTwo within turn limit")
	}

	pub, _ := svc.PublicView(ctx, gameID)
	if pub.DrawCounter != 2 {
		t.Fatalf("expected DrawCounter 2, got %d", pub.DrawCounter)
	}

	// P2 opens inline query
	inlineHandler.HandleInlineQuery(ctx, &telego.InlineQuery{
		ID:    "q_p2_draw2",
		From:  telego.User{ID: int64(p2), FirstName: "P2"},
		Query: "g_" + string(gameID),
	})

	results := mockAPI.AnsweredInlines[len(mockAPI.AnsweredInlines)-1].Results
	var drawSticker *telego.InlineQueryResultCachedSticker
	for _, r := range results {
		if st, ok := r.(*telego.InlineQueryResultCachedSticker); ok && st.StickerFileID == Stickers["option_draw"] {
			drawSticker = st
			break
		}
	}

	if drawSticker == nil {
		t.Fatalf("expected draw option sticker")
	}
	txtContent, ok := drawSticker.InputMessageContent.(*telego.InputTextMessageContent)
	if !ok || txtContent.MessageText != "Comprando 2 cartas" {
		t.Fatalf("expected 'Comprando 2 cartas', got %+v", drawSticker.InputMessageContent)
	}

	// Verify non-DrawTwo cards are not playable
	p2View, _ := svc.PlayerView(ctx, game.Actor{PlayerID: p2}, gameID)
	for _, c := range p2View.Hand {
		if c.Card.Rank != uno.DrawTwo && c.Playable {
			t.Errorf("card %v should not be playable when DrawCounter > 0", c.Card)
		}
	}
}

func TestInlineHandler_UnoAnnouncedReaction(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	dispatcher := NewDispatcher(nil, nil)
	defer dispatcher.Stop(2 * time.Second)

	inlineHandler := NewInlineHandler(mockAPI, svc, renderer, tokens, time.Minute, dispatcher, nil)
	ctx := context.Background()

	out, _ := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Group UNO", Rules: uno.BotRules()})
	gameID := out.View.GameID
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	unoAnnouncedTriggered := false
	for step := 0; step < 120; step++ {
		pub, err := svc.PublicView(ctx, gameID)
		if err != nil || pub.Closed {
			break
		}
		if pub.Phase == uno.ChoosingColor {
			_, _ = svc.Apply(ctx, game.Actor{PlayerID: pub.ColorChooserID, ChatID: -1001}, gameID, uno.Action{
				Type:     uno.ChooseColor,
				PlayerID: pub.ColorChooserID,
				Color:    uno.Red,
				Revision: pub.Revision,
			})
			continue
		}
		curr := pub.CurrentTurn
		pv, _ := svc.PlayerView(ctx, game.Actor{PlayerID: curr}, gameID)
		var playCardID uno.CardID
		for _, c := range pv.Hand {
			if c.Playable {
				playCardID = c.Card.ID
				break
			}
		}
		if playCardID != "" {
			if len(pv.Hand) == 2 {
				tok, _ := tokens.CreateActionToken(curr, gameID, -1001, uno.Action{
					Type:     uno.PlayCard,
					PlayerID: curr,
					CardID:   playCardID,
					Revision: pub.Revision,
				}, time.Minute)

				inlineHandler.HandleChosenInlineResult(ctx, &telego.ChosenInlineResult{
					ResultID: tok,
					From:     telego.User{ID: int64(curr), FirstName: "UnoPlayer"},
				})

				time.Sleep(150 * time.Millisecond)

				reactions := mockAPI.GetSentReactions()
				if len(reactions) == 0 {
					t.Fatalf("expected SetMessageReaction to be called on UNO announcement")
				}
				reaction := reactions[0]
				if len(reaction.Reaction) == 0 {
					t.Fatalf("expected at least 1 reaction")
				}
				emojiReaction, ok := reaction.Reaction[0].(*telego.ReactionTypeEmoji)
				if !ok || emojiReaction.Emoji != "🥳" {
					t.Fatalf("expected 🥳 emoji reaction, got %+v", reaction.Reaction[0])
				}
				unoAnnouncedTriggered = true
				break
			}
			_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
				Type:     uno.PlayCard,
				PlayerID: curr,
				CardID:   playCardID,
				Revision: pub.Revision,
			})
		} else {
			if pv.DrawnCardID == "" {
				_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
					Type:     uno.DrawCard,
					PlayerID: curr,
					Revision: pub.Revision,
				})
			} else {
				_, _ = svc.Apply(ctx, game.Actor{PlayerID: curr, ChatID: -1001}, gameID, uno.Action{
					Type:     uno.PassTurn,
					PlayerID: curr,
					Revision: pub.Revision,
				})
			}
		}
	}

	if !unoAnnouncedTriggered {
		t.Skip("Could not reach 2-to-1 card state within step limit")
	}
}
