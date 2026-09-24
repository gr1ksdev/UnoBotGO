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

func TestCallbackHandler_RefreshAndStatus(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	tokens := NewTokenStore(100, 10, time.Now, nil)
	renderer := NewRenderer(nil)
	cbHandler := NewCallbackHandler(mockAPI, svc, renderer, tokens, nil)

	ctx := context.Background()

	// 1. Create a game for testing view_ callback
	outcome, err := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: 100}, game.CreateRequest{
		ChatName: "Test Chat",
		Rules:    uno.BotRules(),
	})
	if err != nil {
		t.Fatalf("failed to create game: %v", err)
	}
	gameID := outcome.View.GameID

	msg := &telego.Message{
		MessageID: 55,
		Chat:      telego.Chat{ID: 100},
	}

	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cq_1",
		Data:    "view_" + string(gameID),
		Message: msg,
	})

	if len(mockAPI.AnsweredCallbacks) != 1 {
		t.Fatalf("expected 1 callback answered, got %d", len(mockAPI.AnsweredCallbacks))
	}
	if mockAPI.AnsweredCallbacks[0].Text != "Estado atualizado!" {
		t.Errorf("expected 'Estado atualizado!', got %q", mockAPI.AnsweredCallbacks[0].Text)
	}
	if len(mockAPI.EditedMessages) != 1 {
		t.Fatalf("expected 1 edited message, got %d", len(mockAPI.EditedMessages))
	}

	// 2. Test status check st_<token>
	tok, _ := tokens.CreateActionToken(42, gameID, 100, uno.Action{PlayerID: 42}, time.Minute)

	// Status before consumption
	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:   "cq_2",
		Data: "st_" + tok,
	})
	if len(mockAPI.AnsweredCallbacks) != 2 {
		t.Fatalf("expected 2 callbacks answered, got %d", len(mockAPI.AnsweredCallbacks))
	}
	if !strings.Contains(mockAPI.AnsweredCallbacks[1].Text, "Aguardando validação") {
		t.Errorf("expected pending status, got: %s", mockAPI.AnsweredCallbacks[1].Text)
	}

	// Consume and set result
	_, _ = tokens.ConsumeAction(tok, 42)
	tokens.SetActionResult(tok, "confirmed")

	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:   "cq_3",
		Data: "st_" + tok,
	})
	if len(mockAPI.AnsweredCallbacks) != 3 {
		t.Fatalf("expected 3 callbacks answered, got %d", len(mockAPI.AnsweredCallbacks))
	}
	if !strings.Contains(mockAPI.AnsweredCallbacks[2].Text, "confirmada") {
		t.Errorf("expected confirmed status, got: %s", mockAPI.AnsweredCallbacks[2].Text)
	}
}

func TestCallbackHandler_ModeSelect(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	tokens := NewTokenStore(100, 10, time.Now, nil)
	renderer := NewRenderer(nil)
	cbHandler := NewCallbackHandler(mockAPI, svc, renderer, tokens, nil)

	ctx := context.Background()

	// 1. Create a game as player 1
	outcome, err := svc.Create(ctx, game.Actor{PlayerID: 1, ChatID: 100}, game.CreateRequest{
		ChatName: "Lobby Chat",
		Rules:    uno.BotRules(),
	})
	if err != nil {
		t.Fatalf("failed to create game: %v", err)
	}
	gameID := outcome.View.GameID

	msg := &telego.Message{
		MessageID: 10,
		Chat:      telego.Chat{ID: 100},
	}

	// 2. Non-owner tries to change mode -> rejected
	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cq_non_owner",
		From:    telego.User{ID: 999},
		Data:    "mode_caseiro_" + string(gameID),
		Message: msg,
	})
	if len(mockAPI.AnsweredCallbacks) != 1 || !mockAPI.AnsweredCallbacks[0].ShowAlert || !strings.Contains(mockAPI.AnsweredCallbacks[0].Text, "Apenas o responsável") {
		t.Fatalf("expected non-owner rejection, got: %+v", mockAPI.AnsweredCallbacks)
	}

	// 3. Owner changes mode to Caseiro -> succeeds
	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cq_owner_caseiro",
		From:    telego.User{ID: 1},
		Data:    "mode_caseiro_" + string(gameID),
		Message: msg,
	})
	if len(mockAPI.AnsweredCallbacks) != 2 || !strings.Contains(mockAPI.AnsweredCallbacks[1].Text, "Caseiro") {
		t.Fatalf("expected mode changed to Caseiro, got: %+v", mockAPI.AnsweredCallbacks[1])
	}
	view, _ := svc.PublicView(ctx, gameID)
	if !view.Rules.StackWildDrawFourOnTwo {
		t.Fatal("expected game rules to be updated to Caseiro")
	}

	// 4. Owner clicks Caseiro again -> already defined
	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cq_owner_caseiro_dup",
		From:    telego.User{ID: 1},
		Data:    "mode_caseiro_" + string(gameID),
		Message: msg,
	})
	if len(mockAPI.AnsweredCallbacks) != 3 || !strings.Contains(mockAPI.AnsweredCallbacks[2].Text, "já está definido") {
		t.Fatalf("expected duplicate mode notice, got: %+v", mockAPI.AnsweredCallbacks[2])
	}

	// 5. Join players and start game
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: 100}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: view.Revision})
	v2, _ := svc.PublicView(ctx, gameID)
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 2, ChatID: 100}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: v2.Revision})
	v3, _ := svc.PublicView(ctx, gameID)
	_, _ = svc.Apply(ctx, game.Actor{PlayerID: 1, ChatID: 100}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, DealerID: 1, Revision: v3.Revision})

	// 6. Attempting to change mode after start -> rejected
	cbHandler.HandleCallback(ctx, &telego.CallbackQuery{
		ID:      "cq_after_start",
		From:    telego.User{ID: 1},
		Data:    "mode_classic_" + string(gameID),
		Message: msg,
	})
	if len(mockAPI.AnsweredCallbacks) != 4 || !mockAPI.AnsweredCallbacks[3].ShowAlert || !strings.Contains(mockAPI.AnsweredCallbacks[3].Text, "já foi iniciada") {
		t.Fatalf("expected post-start rejection alert, got: %+v", mockAPI.AnsweredCallbacks[3])
	}
}
