package telegram

import (
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
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
func TestCommandsAreLanguageScoped(t *testing.T) {
	for _, tc := range []struct {
		name, accepted, rejected string
		lang                     groups.CommandLanguage
	}{
		{"portuguese", "/entrar@unobot", "/join@unobot", groups.CommandsPT},
		{"english", "/join@unobot", "/entrar@unobot", groups.CommandsEN},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := newMockBotAPI()
			svc, _ := game.NewService()
			h := NewCommandHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
			if tc.lang == groups.CommandsEN {
				repo := newMockGroupRepo()
				cfg, _ := repo.GetOrCreateGroupConfig(t.Context(), -10)
				cfg.CommandLanguage = groups.CommandsEN
				repo.configs[-10] = cfg
				h.groupConfigs = repo
				h.languages.repo = repo
			}

			msg := &telego.Message{Chat: telego.Chat{ID: -10, Type: "supergroup"}, From: &telego.User{ID: 1}, Text: commandName(cmdNew, tc.lang) + "@unobot"}
			h.HandleMessage(t.Context(), msg)
			summary, err := svc.FindChatGame(t.Context(), -10)
			if err != nil {
				t.Fatal(err)
			}

			// The other language is silently ignored.
			msg.Text = tc.rejected
			msg.From = &telego.User{ID: 2}
			h.HandleMessage(t.Context(), msg)
			view, _ := svc.PublicView(t.Context(), summary.GameID)
			if len(view.Players) != 0 {
				t.Fatalf("%s was accepted in %s group", tc.rejected, tc.name)
			}

			// The configured language works.
			msg.Text = tc.accepted
			h.HandleMessage(t.Context(), msg)
			view, _ = svc.PublicView(t.Context(), summary.GameID)
			if len(view.Players) != 1 || view.Players[0].ID != 2 {
				t.Fatalf("%s failed in %s group", tc.accepted, tc.name)
			}
		})
	}
}
func TestPrivateCommandsStillAllowBare(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(nil), nil, "unobot", nil)
	for _, command := range []string{"/start", "/ajuda"} {
		h.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 1, Type: "private"}, From: &telego.User{ID: 1}, Text: command})
	}
	if len(api.SentMessages) != 2 {
		t.Fatal("private commands require username")
	}
	// English spelling is not a cross-language alias.
	h.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: 1, Type: "private"}, From: &telego.User{ID: 1}, Text: "/help"})
	if len(api.SentMessages) != 2 {
		t.Fatal("cross-language alias was accepted in private chat")
	}
}
