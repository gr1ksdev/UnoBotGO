package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
)

func TestDirectMiniAppLink(t *testing.T) {
	for _, username := range []string{"UnoRobotBot", "@UnoRobotBot", " @UnoRobotBot "} {
		if got := miniAppLaunchURL(username); got != "https://t.me/UnoRobotBot/ranking" {
			t.Fatalf("username %q -> %q", username, got)
		}
	}
	for _, username := range []string{"", "@", " "} {
		if miniAppLaunchURL(username) != "" {
			t.Fatal("empty username created a link")
		}
	}
}

func TestStartupBuildsRankingButtonFromSingleGetMe(t *testing.T) {
	for _, mode := range []TransportMode{TransportPolling, TransportWebhook} {
		t.Run(string(mode), func(t *testing.T) {
			api := newMockBotAPI()
			api.MeUser.Username = "UnoRobotBot"
			svc, _ := game.NewService()
			b := New(api, svc, nil, nil, time.Minute, nil)
			b.SetTransport(TransportConfig{Mode: mode, WebhookURL: "https://example.com/telegram", WebhookSecret: "derived-test-secret"})
			ready := make(chan struct{})
			b.UseSharedHTTP(func() { close(ready) })
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
			markup := b.cmdHandler.rankingMarkup()
			if markup == nil || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
				t.Fatal("missing ranking button")
			}
			button := markup.InlineKeyboard[0][0]
			if button.Text != "🌐 Ranking Global" || button.URL != "https://t.me/UnoRobotBot/ranking" {
				t.Fatal("wrong button", button)
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
