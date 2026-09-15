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
