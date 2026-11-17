package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/groups"
	"github.com/malbs/UnoGoBot/internal/ranking"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

// administrationFixture prepares a running game with an owner, a stuck player
// and a second participant.
func administrationFixture(t *testing.T) (*CommandHandler, *game.Service, game.PublicGameView, *mockBotAPI) {
	t.Helper()
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)

	chat := telego.Chat{ID: -700, Type: "supergroup", Title: "Sala Admin"}
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Owner"}, Text: "/novo@unobot"})
	for _, id := range []int64{1, 2, 3} {
		h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: id, FirstName: "Jogador"}, Text: "/entrar@unobot"})
	}
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Owner"}, Text: "/iniciar@unobot"})

	summary, err := svc.FindChatGame(t.Context(), -700)
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.PublicView(t.Context(), summary.GameID)
	if err != nil {
		t.Fatal(err)
	}
	return h, svc, view, api
}

func TestPularAdvancesTheTurnForTheOwner(t *testing.T) {
	h, svc, view, api := administrationFixture(t)
	before := view.CurrentTurn
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup"},
		From: &telego.User{ID: 1, FirstName: "Owner"},
		Text: "/pular@unobot",
	})
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentTurn == before || after.Revision != view.Revision+1 {
		t.Fatalf("turn not advanced: %+v", after)
	}
	last := api.SentMessages[len(api.SentMessages)-1]
	if !strings.Contains(last.Text, "Vez de:") {
		t.Fatalf("unexpected reply: %s", last.Text)
	}
}

func TestPularIsRefusedForOrdinaryMembers(t *testing.T) {
	h, svc, view, api := administrationFixture(t)
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup"},
		From: &telego.User{ID: 3, FirstName: "Comum"},
		Text: "/pular@unobot",
	})
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != view.Revision {
		t.Fatal("unauthorized skip changed the game")
	}
	last := api.SentMessages[len(api.SentMessages)-1]
	if !strings.Contains(last.Text, "Apenas o responsável") {
		t.Fatalf("unexpected reply: %s", last.Text)
	}
}

func TestExpulsarRemovesTheRepliedPlayer(t *testing.T) {
	h, svc, view, api := administrationFixture(t)
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup"},
		From: &telego.User{ID: 1, FirstName: "Owner"},
		Text: "/expulsar@unobot",
		ReplyToMessage: &telego.Message{
			MessageID: 7,
			From:      &telego.User{ID: 3, FirstName: "Alvo"},
		},
	})
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != view.Revision+1 {
		t.Fatalf("game not changed: %+v", after)
	}
	for _, p := range after.Players {
		if p.ID == 3 && p.Active {
			t.Fatal("expelled player still active")
		}
	}
	last := api.SentMessages[len(api.SentMessages)-1]
	if !strings.Contains(last.Text, "expulso") {
		t.Fatalf("unexpected reply: %s", last.Text)
	}
}

func TestExpulsarRequiresATarget(t *testing.T) {
	h, svc, view, api := administrationFixture(t)
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup"},
		From: &telego.User{ID: 1, FirstName: "Owner"},
		Text: "/expulsar@unobot",
	})
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != view.Revision {
		t.Fatal("game changed without a target")
	}
	last := api.SentMessages[len(api.SentMessages)-1]
	if !strings.Contains(last.Text, "Responda") {
		t.Fatalf("unexpected reply: %s", last.Text)
	}
}

func TestStatsCommandReportsThePlayerHistory(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	h := NewCommandHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
	h.rankingService = &ranking.Service{Repository: &mockPrivateRankingRepo{
		onPlayerStats: func(_ context.Context, chatID, userID int64, _ time.Time) (ranking.PlayerStats, error) {
			if chatID != -700 || userID != 42 {
				t.Fatalf("unexpected scope: %d %d", chatID, userID)
			}
			return ranking.PlayerStats{System: groups.Updated, Games: 7, Wins: 2, BestPosition: 1, AvgPosition: "2.14", Score: 1200, Rank: 3, Total: 9}, nil
		},
	}}
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup", Title: "Sala Stats"},
		From: &telego.User{ID: 42, FirstName: "Freddy"},
		Text: "/stats@unobot",
	})
	last := api.SentMessages[len(api.SentMessages)-1]
	for _, want := range []string{"Sala Stats", "Partidas pontuadas: 7", "Vitórias: 2", "Melhor colocação: 1º", "Colocação média: 2.14", "Posição no ranking: 3 de 9"} {
		if !strings.Contains(last.Text, want) {
			t.Fatalf("missing %q in %s", want, last.Text)
		}
	}
}

func TestLobbyButtonStartsTheGame(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	b.cmdHandler.botUsername = "unobot"
	h := b.cmdHandler
	chat := telego.Chat{ID: -77, Type: "supergroup", Title: "Lobby Button"}

	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Owner"}, Text: "/novo@unobot"})
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 1, FirstName: "Owner"}, Text: "/entrar@unobot"})
	h.HandleMessage(t.Context(), &telego.Message{Chat: chat, From: &telego.User{ID: 2, FirstName: "Other"}, Text: "/entrar@unobot"})

	summary, err := svc.FindChatGame(t.Context(), -77)
	if err != nil {
		t.Fatal(err)
	}
	lobby := api.SentMessages[len(api.SentMessages)-1]
	if strings.Contains(lobby.Text, "Regras:") {
		t.Fatalf("rules line still present in lobby: %s", lobby.Text)
	}
	markup, ok := lobby.ReplyMarkup.(*telego.InlineKeyboardMarkup)
	if !ok || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("missing lobby button: %+v", lobby.ReplyMarkup)
	}
	button := markup.InlineKeyboard[0][0]
	if button.Text != "▶️ Iniciar" || button.CallbackData != "start_"+string(summary.GameID) {
		t.Fatalf("wrong lobby button: %+v", button)
	}

	b.cbHandler.HandleCallback(t.Context(), &telego.CallbackQuery{
		ID:      "start",
		From:    telego.User{ID: 1, FirstName: "Owner"},
		Message: &telego.Message{MessageID: 42, Chat: chat},
		Data:    button.CallbackData,
	})

	started, err := svc.PublicView(t.Context(), summary.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Phase == uno.Lobby {
		t.Fatal("lobby button did not start the game")
	}
	if len(api.EditedMessages) == 0 || strings.Contains(api.EditedMessages[0].Text, "▶️ Iniciar") != false {
		t.Fatalf("lobby message was not replaced: %+v", api.EditedMessages)
	}
	if len(api.AnsweredCallbacks) == 0 || !strings.Contains(api.AnsweredCallbacks[0].Text, "iniciada") {
		t.Fatalf("callback not answered: %+v", api.AnsweredCallbacks)
	}
}
