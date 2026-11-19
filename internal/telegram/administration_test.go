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
	return administrationFixtureWithPolicy(t, game.DefaultTurnPolicy)
}

func administrationFixtureWithPolicy(t *testing.T, policy game.TurnPolicy) (*CommandHandler, *game.Service, game.PublicGameView, *mockBotAPI) {
	t.Helper()
	api := newMockBotAPI()
	svc, _ := game.NewService(game.WithTurnPolicy(policy))
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

func TestPularAdvancesTheTurnForTheCurrentPlayer(t *testing.T) {
	h, svc, view, api := administrationFixture(t)
	before := view.CurrentTurn
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup"},
		From: &telego.User{ID: int64(before), FirstName: "Da vez"},
		Text: "/pular@unobot",
	})
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentTurn == before || after.Revision != view.Revision+1 {
		t.Fatalf("turn not advanced: %+v", after)
	}
	last := plainGameplay(api.LastSentMessage())
	if !strings.Contains(last, "foi pulado") || !strings.Contains(last, "Vez de:") {
		t.Fatalf("unexpected reply: %s", last)
	}
	if strings.Contains(last, "🃏 Topo:") || strings.Contains(last, "👥") {
		t.Fatalf("skip message leaked the detailed state: %s", last)
	}
}

func TestPularIsRefusedBeforeTheDeadline(t *testing.T) {
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
		t.Fatal("early skip changed the game")
	}
	last := api.LastSentMessage()
	if !strings.Contains(last, "Aguarde") {
		t.Fatalf("unexpected reply: %s", last)
	}
}

func TestPularIsAllowedForAnyParticipantAfterTheDeadline(t *testing.T) {
	fast := game.TurnPolicy{Initial: time.Nanosecond, Min: time.Nanosecond, Penalty: time.Nanosecond}
	h, svc, view, api := administrationFixtureWithPolicy(t, fast)
	h.HandleMessage(t.Context(), &telego.Message{
		Chat: telego.Chat{ID: -700, Type: "supergroup"},
		From: &telego.User{ID: 3, FirstName: "Comum"},
		Text: "/pular@unobot",
	})
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentTurn == view.CurrentTurn || after.Revision != view.Revision+1 {
		t.Fatalf("skip not applied: %+v", after)
	}
	last := plainGameplay(api.LastSentMessage())
	if !strings.Contains(last, "foi pulado") || !strings.Contains(last, "Vez de:") {
		t.Fatalf("unexpected reply: %s", last)
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
	if len(api.EditedMessages) != 1 {
		t.Fatalf("expected one edited lobby message: %+v", api.EditedMessages)
	}
	edit := api.EditedMessages[0]
	plainEdit := plainGameplay(edit.Text)
	if plainEdit != "🚀 Partida iniciada!" {
		t.Fatalf("lobby receipt must be only the announcement: %q", plainEdit)
	}
	if edit.ReplyMarkup == nil || len(edit.ReplyMarkup.InlineKeyboard) != 0 {
		t.Fatalf("lobby button was not removed: %+v", edit.ReplyMarkup)
	}

	// A new message carries the normal game flow, without the detailed state.
	if len(api.SentMessages) < 2 {
		t.Fatalf("expected a new start message: %+v", api.SentMessages)
	}
	flow := plainGameplay(api.LastSentMessage())
	if !strings.Contains(flow, "🚀 Partida iniciada!") || !strings.Contains(flow, "Vez de:") {
		t.Fatalf("start flow missing announcement or turn: %q", flow)
	}
	if strings.Contains(flow, "🃏 Topo:") || strings.Contains(flow, "👥") {
		t.Fatalf("start flow leaked the detailed state: %q", flow)
	}
	if len(api.AnsweredCallbacks) == 0 || !strings.Contains(api.AnsweredCallbacks[0].Text, "iniciada") {
		t.Fatalf("callback not answered: %+v", api.AnsweredCallbacks)
	}
}

func TestTextStartDoesNotRepeatTheDetailedState(t *testing.T) {
	_, _, _, api := administrationFixture(t)
	last := plainGameplay(api.LastSentMessage())
	if !strings.Contains(last, "🚀 Partida iniciada!") || !strings.Contains(last, "Vez de:") {
		t.Fatalf("start message missing announcement or turn: %q", last)
	}
	if strings.Contains(last, "🃏 Topo:") || strings.Contains(last, "👥") {
		t.Fatalf("start message leaked the detailed state: %q", last)
	}
}
