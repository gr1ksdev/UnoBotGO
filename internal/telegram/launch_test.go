package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func TestStartupSignalsReadyWithSingleGetMe(t *testing.T) {
	for _, mode := range []TransportMode{TransportPolling, TransportWebhook} {
		t.Run(string(mode), func(t *testing.T) {
			api := newMockBotAPI()
			api.MeUser.Username = "UnoRobotBot"
			svc, _ := game.NewService()
			b := New(api, svc, nil, nil, time.Minute, nil)
			b.SetTransport(TransportConfig{Mode: mode, WebhookURL: "https://example.com/telegram", WebhookSecret: "derived-test-secret", ListenAddr: "127.0.0.1:0"})
			ready := make(chan struct{})
			b.SetReadyCallback(func() { close(ready) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- b.Run(ctx) }()
			t.Cleanup(func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("shutdown timed out")
				}
			})
			select {
			case <-ready:
			case <-time.After(5 * time.Second):
				t.Fatal("startup timed out")
			}
			api.mu.Lock()
			calls := api.GetMeCalls
			api.mu.Unlock()
			if calls != 1 {
				t.Fatalf("getMe called %d times", calls)
			}
		})
	}
}

func TestNovoEntrarRoomsAreDiscoverable(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "ConfiguredBot", nil)
	chat := telego.Chat{ID: -42, Type: "supergroup", Title: "Sala real"}
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Conta A"}, Text: "/novo@ConfiguredBot"})
	for _, id := range []int64{1, 2} {
		h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: id, FirstName: "Conta"}, Text: "/entrar@ConfiguredBot"})
		rooms, err := svc.FindPlayerGames(t.Context(), game.Actor{PlayerID: uno.PlayerID(id)})
		if err != nil || len(rooms) != 1 {
			t.Fatal("joined room not discoverable", id, err, rooms)
		}
	}
	outsider, _ := svc.FindPlayerGames(t.Context(), game.Actor{PlayerID: 3})
	if len(outsider) != 0 {
		t.Fatal("room leaked to outsider")
	}
}
