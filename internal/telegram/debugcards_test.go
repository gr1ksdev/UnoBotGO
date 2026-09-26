//go:build debugcards

package telegram

import (
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
	"strings"
	"testing"
	"time"
)

func TestDebugCardParser(t *testing.T) {
	for _, tc := range []struct {
		text  string
		color uno.Color
		rank  uno.Rank
	}{
		{"troca", uno.NoColor, uno.SwapHands}, {"CORINGA", uno.NoColor, uno.Wild}, {"+4", uno.NoColor, uno.WildDrawFour},
		{"amarelo +2", uno.Yellow, uno.DrawTwo}, {"azul 0", uno.Blue, uno.Zero}, {"verde 9", uno.Green, uno.Nine}, {"vermelho pular", uno.Red, uno.Skip}, {"AZUL INVERTER", uno.Blue, uno.Reverse},
	} {
		color, rank, ok := parseDebugCard(strings.Fields(tc.text))
		if !ok || color != tc.color || rank != tc.rank {
			t.Fatal(tc.text, color, rank, ok)
		}
	}
	for _, text := range []string{"", "+2", "amarelo +4", "troca extra", "rosa 1", "azul 10", "azul -1", "azul 1 extra"} {
		if _, _, ok := parseDebugCard(strings.Fields(text)); ok {
			t.Fatal(text)
		}
	}
}

func TestDebugCommandDeliveryAndRejections(t *testing.T) {
	svc, _ := game.NewService()
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	v, err := svc.Create(t.Context(), game.Actor{PlayerID: 11, ChatID: -100}, game.CreateRequest{Rules: uno.CaseiroRules()})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uno.PlayerID{11, game.DebugCardsUserID} {
		v, err = svc.Apply(t.Context(), game.Actor{PlayerID: id, ChatID: -100}, v.View.GameID, uno.Action{Type: uno.JoinGame, PlayerID: id, Revision: v.View.Revision})
		if err != nil {
			t.Fatal(err)
		}
	}
	v, err = svc.Apply(t.Context(), game.Actor{PlayerID: 11, ChatID: -100}, v.View.GameID, uno.Action{Type: uno.StartGame, PlayerID: 11, Revision: v.View.Revision})
	if err != nil {
		t.Fatal(err)
	}
	msg := &telego.Message{Chat: telego.Chat{ID: -100, Type: "supergroup"}, From: &telego.User{ID: 11}, Text: "/dar +4"}
	b.cmdHandler.HandleMessage(t.Context(), msg)
	if len(api.SentMessages) != 0 {
		t.Fatal("unauthorized caller discovered command")
	}
	msg.From = &telego.User{ID: int64(game.DebugCardsUserID), FirstName: "Tester"}
	msg.MessageThreadID = 42 // Ordinary reply thread, not a forum topic.
	// Try numeric types until a drawable copy is found, independent of shuffle.
	give := func(target uno.PlayerID, reply bool) {
		before, e := svc.PlayerView(t.Context(), game.Actor{PlayerID: target}, v.View.GameID)
		if e != nil {
			t.Fatal(e)
		}
		msg.ReplyToMessage = nil
		if reply {
			msg.ReplyToMessage = &telego.Message{Chat: msg.Chat, From: &telego.User{ID: int64(target), FirstName: "Target <&>"}}
		}
		success := false
		for _, color := range []string{"vermelho", "azul", "verde", "amarelo"} {
			for n := 0; n <= 9; n++ {
				msg.Text = "/dar " + color + " " + string(rune('0'+n))
				b.cmdHandler.HandleMessage(t.Context(), msg)
				if strings.Contains(api.LastSentMessage(), "recebeu") {
					success = true
					break
				}
			}
			if success {
				break
			}
		}
		if !success {
			t.Fatal("no drawable numeric card found")
		}
		after, e := svc.PlayerView(t.Context(), game.Actor{PlayerID: target}, v.View.GameID)
		if e != nil {
			t.Fatal(e)
		}
		if len(after.Hand) != len(before.Hand)+1 {
			t.Fatal("wrong recipient")
		}
	}
	give(game.DebugCardsUserID, false)
	give(11, true)
	if !strings.Contains(api.LastSentMessage(), "Target &lt;&amp;&gt;") {
		t.Fatal("unescaped name")
	}
	before, _ := svc.PublicView(t.Context(), v.View.GameID)
	msg.ReplyToMessage.From.IsBot = true
	b.cmdHandler.HandleMessage(t.Context(), msg)
	after, _ := svc.PublicView(t.Context(), v.View.GameID)
	if before.Revision != after.Revision {
		t.Fatal("accepted bot target")
	}
	if err := b.registerCommands(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, c := range api.RegisteredCommands {
		if c.Command == "dar" {
			t.Fatal("command advertised")
		}
	}
	if strings.Contains(b.renderer.RenderHelp("testbot"), "/dar") {
		t.Fatal("debug command in help")
	}
}
