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

	// Existing webhook is removed safely before polling.
	mockAPI2 := newMockBotAPI()
	mockAPI2.WebhookInfo.URL = "https://example.com/webhook"
	bot2 := New(mockAPI2, svc, nil, nil, time.Minute, nil)
	ctx2, cancel2 := context.WithCancel(context.Background())
	done2 := make(chan error, 1)
	go func() { done2 <- bot2.Run(ctx2) }()
	time.Sleep(25 * time.Millisecond)
	cancel2()
	if err := <-done2; err != nil {
		t.Fatalf("polling with existing webhook: %v", err)
	}
	if len(mockAPI2.DeleteWebhookCalls) != 1 || mockAPI2.DeleteWebhookCalls[0].DropPendingUpdates {
		t.Fatalf("expected DeleteWebhook(false), got %#v", mockAPI2.DeleteWebhookCalls)
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
	if len(cmds) != 8 {
		t.Errorf("expected 8 registered commands, got %d", len(cmds))
	}
	foundReset := false
	for _, command := range cmds {
		foundReset = foundReset || command.Command == "reset"
	}
	if !foundReset {
		t.Error("reset command was not registered")
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

func TestBot_ResetUsesRecoveryLaneAndAllowsNewGame(t *testing.T) {
	svc, _ := game.NewService()
	mockAPI := newMockBotAPI()
	bot := New(mockAPI, svc, NewTokenStore(100, 10, time.Now, nil), NewRenderer(NewUserCache(100)), time.Minute, nil)
	defer bot.dispatcher.Stop(2 * time.Second)
	chatID := game.ChatID(-3001)
	ownerID := int64(10)
	if _, err := svc.Create(t.Context(), game.Actor{PlayerID: 10, ChatID: chatID}, game.CreateRequest{ChatName: "Recovery"}); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	if !bot.dispatcher.EnqueueChat(chatID, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
	}) {
		t.Fatal("failed to enqueue blocker")
	}
	waitSignal(t, started)
	for i := 0; i < ChatQueueCapacity; i++ {
		if !bot.dispatcher.EnqueueChat(chatID, func(context.Context) {}) {
			t.Fatalf("normal queue saturated early at %d", i)
		}
	}
	reset := telego.Update{UpdateID: 900, Message: &telego.Message{
		Chat: telego.Chat{ID: int64(chatID), Type: "supergroup"},
		From: &telego.User{ID: ownerID}, Text: "/reset",
	}}
	if !bot.submitUpdate(t.Context(), reset) {
		t.Fatal("reset was rejected while normal queue was saturated")
	}
	waitSignal(t, mockAPI.SentMessageSignal)
	if _, err := svc.FindChatGame(t.Context(), chatID); !errors.Is(err, game.ErrNoActiveGame) {
		t.Fatalf("reset did not remove game: %v", err)
	}
	mockAPI.mu.Lock()
	messagesAfterReset := len(mockAPI.SentMessages)
	mockAPI.mu.Unlock()
	if !bot.submitUpdate(t.Context(), reset) {
		t.Fatal("duplicate reset should be acknowledged by dedupe")
	}
	barrier := make(chan struct{})
	if !bot.dispatcher.EnqueueRecovery(chatID, func(context.Context) { close(barrier) }) {
		t.Fatal("could not enqueue recovery barrier")
	}
	waitSignal(t, barrier)
	mockAPI.mu.Lock()
	if len(mockAPI.SentMessages) != messagesAfterReset {
		mockAPI.mu.Unlock()
		t.Fatal("duplicate reset update was processed twice")
	}
	mockAPI.mu.Unlock()

	create := telego.Update{UpdateID: 901, Message: &telego.Message{
		Chat: telego.Chat{ID: int64(chatID), Type: "supergroup", Title: "Recovery"},
		From: &telego.User{ID: ownerID}, Text: "/novo",
	}}
	if !bot.submitUpdate(t.Context(), create) {
		t.Fatal("new command was not admitted after reset")
	}
	waitSignal(t, mockAPI.SentMessageSignal)
	if _, err := svc.FindChatGame(t.Context(), chatID); err != nil {
		t.Fatalf("new game failed after reset: %v", err)
	}
}
