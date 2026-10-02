package telegram

import (
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
	"testing"
	"time"
)

func TestGroupCommandsRequireOwnUsername(t *testing.T) {
	for _, chatType := range []string{"group", "supergroup"} {
		for _, command := range []string{"/novo", "/help", "/join", "/reset", "/novo@otherbot", "/reset@otherbot"} {
			api := newMockBotAPI()
			svc, _ := game.NewService()
			h := NewCommandHandler(api, svc, NewRenderer(nil), nil, "unobot", nil)
			msg := &telego.Message{Chat: telego.Chat{ID: -10, Type: chatType}, From: &telego.User{ID: 1}, Text: command, SenderChat: &telego.Chat{ID: -11}, IsTopicMessage: true}
			h.HandleMessage(t.Context(), msg)
			h.HandleReset(t.Context(), msg, func() { t.Fatal("unaddressed reset ran") })
			if len(api.SentMessages) != 0 {
				t.Fatalf("%s %s responded without target", chatType, command)
			}
		}
	}
}
func TestJoinAlias(t *testing.T) {
	for _, command := range []string{"/join@unobot", "/JOIN@UNOBOT", "/entrar@unobot"} {
		api := newMockBotAPI()
		svc, _ := game.NewService()
		h := NewCommandHandler(api, svc, NewRenderer(nil), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
		msg := &telego.Message{Chat: telego.Chat{ID: -10, Type: "supergroup"}, From: &telego.User{ID: 1}, Text: "/novo@unobot"}
		h.HandleMessage(t.Context(), msg)
		summary, err := svc.FindChatGame(t.Context(), -10)
		if err != nil {
			t.Fatal(err)
		}
		msg.Text = command
		msg.From = &telego.User{ID: 2}
		h.HandleMessage(t.Context(), msg)
		view, err := svc.PublicView(t.Context(), summary.GameID)
		if err != nil || len(view.Players) != 1 || view.Players[0].ID != 2 {
			t.Fatalf("alias failed: %s", command)
		}
		// A duplicate joins the same flow and cannot create a second player.
		h.HandleMessage(t.Context(), msg)
		view, _ = svc.PublicView(t.Context(), summary.GameID)
		if len(view.Players) != 1 {
			t.Fatal("duplicate join")
		}
	}
}
func TestPrivateCommandsStillAllowBare(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(nil), nil, "unobot", nil)
	for _, command := range []string{"/start", "/help"} {
		h.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 1, Type: "private"}, From: &telego.User{ID: 1}, Text: command})
	}
	if len(api.SentMessages) != 2 {
		t.Fatal("private commands require username")
	}
}
