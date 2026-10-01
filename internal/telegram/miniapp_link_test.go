package telegram

import (
	"context"
	"github.com/malbs/UnoGoBot/internal/game"
	"testing"
	"time"
)

func TestMiniAppLaunchURL(t *testing.T) {
	for _, username := range []string{"UnoRobotBot", "@UnoRobotBot", " @UnoRobotBot "} {
		if got := miniAppLaunchURL(username); got != "https://t.me/UnoRobotBot/ranking" {
			t.Fatalf("got %s", got)
		}
	}
}

func TestStartupUsesIdentityForRankingButton(t *testing.T) {
	api := newMockBotAPI()
	api.MeUser.Username = "UnoRobotBot"
	svc, _ := game.NewService()
	bot := New(api, svc, nil, nil, time.Minute, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Startup runs before polling. Cancelling on readiness provides a synchronization
	// boundary without racing the dispatcher's startup writes.
	bot.UseSharedHTTP(cancel)
	if err := bot.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if api.GetMeCalls != 1 {
		t.Fatalf("getMe called %d times", api.GetMeCalls)
	}
	markup := bot.cmdHandler.rankingMarkup()
	if markup == nil {
		t.Fatal("missing ranking button")
	}
	button := markup.InlineKeyboard[0][0]
	if button.Text != "🌐 Ranking Global" || button.URL != "https://t.me/UnoRobotBot/ranking" {
		t.Fatalf("incorrect button: %#v", button)
	}
}
