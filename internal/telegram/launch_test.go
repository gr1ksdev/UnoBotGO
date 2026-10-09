package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
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

func TestGameMarkupOpensAuthorizedRoomInMiniApp(t *testing.T) {
	h := &CommandHandler{miniAppURL: miniAppLaunchURL("ConfiguredBot")}
	view := game.PublicGameView{GameID: "room-123"}
	markup := h.gameMarkup(view)
	last := markup.InlineKeyboard[len(markup.InlineKeyboard)-1][0]
	if last.Text != "Abrir partida na Mini App" || last.URL != "https://t.me/ConfiguredBot/ranking?startapp=game_room-123" {
		t.Fatal(last)
	}
	view.Closed = true
	if h.gameMarkup(view) != nil {
		t.Fatal("closed game has live launch link")
	}
}

func TestNovoEntrarRoomsAreDiscoverableWithDirectLaunch(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "ConfiguredBot", nil)
	h.miniAppURL = miniAppLaunchURL("ConfiguredBot")
	chat := telego.Chat{ID: -42, Type: "supergroup", Title: "Sala real"}
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Conta A"}, Text: "/novo@ConfiguredBot"})
	for _, id := range []int64{1, 2} {
		h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: id, FirstName: "Conta"}, Text: "/entrar@ConfiguredBot"})
		rooms, err := svc.FindPlayerGames(t.Context(), game.Actor{PlayerID: uno.PlayerID(id)})
		if err != nil || len(rooms) != 1 {
			t.Fatal("joined room not discoverable", id, err, rooms)
		}
		markup, ok := api.SentMessages[len(api.SentMessages)-1].ReplyMarkup.(*telego.InlineKeyboardMarkup)
		if !ok {
			t.Fatal("missing markup")
		}
		launch := markup.InlineKeyboard[len(markup.InlineKeyboard)-1][0]
		if launch.URL != h.miniAppURL+"?startapp=game_"+string(rooms[0].GameID) {
			t.Fatal("wrong room link", launch)
		}
	}
	outsider, _ := svc.FindPlayerGames(t.Context(), game.Actor{PlayerID: 3})
	if len(outsider) != 0 {
		t.Fatal("room leaked to outsider")
	}
}
