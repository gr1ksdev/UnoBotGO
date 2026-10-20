package telegram

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
)

func TestPollingUsesLongPollingTimeout(t *testing.T) {
	svc, err := game.NewService()
	if err != nil {
		t.Fatal(err)
	}
	api := newMockBotAPI()
	bot := newTestBot(api, svc, nil, nil, time.Minute, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- bot.Run(ctx) }()

	deadline := time.After(2 * time.Second)
	for {
		api.mu.Lock()
		calls := len(api.GetUpdatesCalls)
		api.mu.Unlock()
		if calls > 0 {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatal("polling did not start")
		case <-time.After(time.Millisecond):
		}
	}

	api.mu.Lock()
	params := api.GetUpdatesCalls[0]
	api.mu.Unlock()
	if params.Timeout != longPollingTimeoutSeconds {
		t.Fatalf("polling timeout=%d, want %d", params.Timeout, longPollingTimeoutSeconds)
	}
	if params.Limit != 100 {
		t.Fatalf("polling limit=%d, want 100", params.Limit)
	}
	if !slices.Contains(params.AllowedUpdates, "message") {
		t.Fatalf("allowed updates missing message: %v", params.AllowedUpdates)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("bot shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bot did not shut down")
	}
}
