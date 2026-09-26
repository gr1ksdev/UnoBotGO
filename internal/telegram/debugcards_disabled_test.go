//go:build !debugcards

package telegram

import (
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
	"testing"
	"time"
)

func TestNormalBuildIgnoresDebugCommand(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	b.cmdHandler.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: -100, Type: "supergroup"}, From: &telego.User{ID: 7595607953}, Text: "/dar +4"})
	if len(api.SentMessages) != 0 {
		t.Fatal("normal build exposed debug command")
	}
}
