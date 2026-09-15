package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

func TestBot_StartupChecks(t *testing.T) {
	svc, _ := game.NewService()

	// 1. Inline mode disabled check
	mockAPI := newMockBotAPI()
	mockAPI.MeUser.SupportsInlineQueries = false

	bot := New(mockAPI, svc, nil, nil, time.Minute, nil)
	err := bot.Run(context.Background())
	if !errors.Is(err, ErrInlineModeDisabled) {
		t.Fatalf("expected ErrInlineModeDisabled, got %v", err)
	}

	// 2. Webhook active check
	mockAPI2 := newMockBotAPI()
	mockAPI2.WebhookInfo.URL = "https://example.com/webhook"

	bot2 := New(mockAPI2, svc, nil, nil, time.Minute, nil)
	err2 := bot2.Run(context.Background())
	if !errors.Is(err2, ErrWebhookActive) {
		t.Fatalf("expected ErrWebhookActive, got %v", err2)
	}
}

func TestBot_RunAndShutdown(t *testing.T) {
	svc, _ := game.NewService()
	mockAPI := newMockBotAPI()

	bot := New(mockAPI, svc, nil, nil, time.Minute, nil)

	ctx, cancel := context.WithCancel(context.Background())

	runErrChan := make(chan error, 1)
	go func() {
		runErrChan <- bot.Run(ctx)
	}()

	// Wait for startup and commands registration
	time.Sleep(50 * time.Millisecond)

	cmds := mockAPI.GetRegisteredCommands()
	if len(cmds) != 7 {
		t.Errorf("expected 7 registered commands, got %d", len(cmds))
	}

	// Send an update through long polling
	mockAPI.UpdatesChan <- telego.Update{
		UpdateID: 1,
		Message: &telego.Message{
			Chat: telego.Chat{ID: -10055, Type: "supergroup", Title: "UNO Group"},
			From: &telego.User{ID: 1, FirstName: "Alice"},
			Text: "/novo",
		},
	}

	time.Sleep(50 * time.Millisecond)

	lastMsg := mockAPI.LastSentMessage()
	if lastMsg == "" {
		t.Errorf("expected bot to process /novo update from channel")
	}

	// Cancel context to initiate graceful shutdown
	cancel()

	select {
	case err := <-runErrChan:
		if err != nil {
			t.Fatalf("unexpected error from Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("bot Run did not exit cleanly within timeout")
	}
}
