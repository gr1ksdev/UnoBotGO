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
