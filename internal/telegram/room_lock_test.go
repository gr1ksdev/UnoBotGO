package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func TestRoomLockCommands(t *testing.T) {
	api := newMockBotAPI()
	svc, err := game.NewService()
	if err != nil {
		t.Fatal(err)
	}
	r := NewRenderer(nil)
	h := NewCommandHandler(api, svc, r, NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
	send := func(id int64, command string, want string) {
		t.Helper()
		h.HandleMessage(t.Context(), &telego.Message{Chat: telego.Chat{ID: -10, Type: "supergroup"}, From: &telego.User{ID: id, FirstName: "Player"}, Text: command})
		if !strings.Contains(api.LastSentMessage(), want) {
			t.Fatalf("%s: %s, want %s", command, api.LastSentMessage(), want)
		}
	}
	send(99, "/trancar", "Nenhuma partida ativa")
	send(99, "/novo", "Partida de UNO")
	send(1, "/entrar", "1/10")
	send(2, "/entrar", "2/10")
	for _, active := range []bool{false, true} {
		if active {
			send(1, "/iniciar", "Partida iniciada")
		}
		send(1, "/trancar", "Apenas o responsável")
		send(99, "/trancar", "🔒 A partida foi trancada.\nNovos jogadores não poderão entrar.")
		send(99, "/trancar", "🔒 A partida já está trancada.")
		send(3, "/entrar", "🔒 Esta partida está trancada e não aceita novos jogadores.")
		send(1, "/destrancar", "Apenas o responsável")
		send(99, "/destrancar", "🔓 A partida foi destrancada.\nNovos jogadores podem entrar novamente.")
		send(99, "/destrancar", "🔓 A partida já está aberta.")
	}
	send(3, "/entrar", "entrou na partida em andamento")
	summary, _ := svc.FindChatGame(t.Context(), -10)
	view, _ := svc.PublicView(t.Context(), summary.GameID)
	if view.OwnerID != 99 || len(view.Players) != 3 || view.Phase != uno.TakingTurn {
		t.Fatal(view)
	}
	send(99, "/cancelar", "Partida cancelada")
	send(99, "/destrancar", "Nenhuma partida ativa")
	send(99, "/novo", "Partida de UNO")
	send(4, "/entrar", "1/10")
	for _, text := range []string{"/trancar", "Impede novos jogadores de entrar", "/destrancar", "Permite novas entradas"} {
		if !strings.Contains(r.RenderHelp("unobot"), text) {
			t.Fatal("help missing", text)
		}
	}
}
