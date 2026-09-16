package telegram

import (
	"strings"
	"testing"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestUserCache_HtmlEscapeAndEviction(t *testing.T) {
	cache := NewUserCache(2)

	cache.Put(1, "Alice <alert>", "alice_evil")
	cache.Put(2, "Bob & Co", "")

	link1 := cache.FormatLink(1)
	if !strings.Contains(link1, "Alice &lt;alert&gt; (@alice_evil)") {
		t.Errorf("expected escaped html link, got: %s", link1)
	}

	link2 := cache.FormatLink(2)
	if !strings.Contains(link2, "Bob &amp; Co") {
		t.Errorf("expected escaped html link, got: %s", link2)
	}

	// Put 3rd -> 1 should be evicted from cache and fallback to Jogador 1
	cache.Put(3, "Carol", "carol")

	link1Fallback := cache.FormatLink(1)
	if !strings.Contains(link1Fallback, "Jogador 1") {
		t.Errorf("expected evicted user to fallback to Jogador 1, got: %s", link1Fallback)
	}
}

func TestCardRepr(t *testing.T) {
	red7 := uno.Card{ID: "c1", Color: uno.Red, Rank: uno.Seven}
	reprRed7 := CardRepr(red7)
	if reprRed7 != "❤️ 7" {
		t.Errorf("expected '❤️ 7', got %q", reprRed7)
	}

	wild := uno.Card{ID: "c2", Color: uno.NoColor, Rank: uno.Wild}
	reprWild := CardRepr(wild)
	if reprWild != "🌈 Coringa" {
		t.Errorf("expected '🌈 Coringa', got %q", reprWild)
	}

	wild4 := uno.Card{ID: "c3", Color: uno.NoColor, Rank: uno.WildDrawFour}
	reprWild4 := CardRepr(wild4)
	if reprWild4 != "🌈+4 Coringa Comprar 4" {
		t.Errorf("expected '🌈+4 Coringa Comprar 4', got %q", reprWild4)
	}

	blueDraw := uno.Card{ID: "c4", Color: uno.Blue, Rank: uno.DrawTwo}
	reprBlueDraw := CardRepr(blueDraw)
	if !strings.Contains(reprBlueDraw, "+2") {
		t.Errorf("expected +2 in draw card, got %q", reprBlueDraw)
	}
}

func TestRenderer_RenderLobbyAndState(t *testing.T) {
	cache := NewUserCache(10)
	cache.Put(10, "Alice", "alice")
	cache.Put(20, "Bob", "bob")
	renderer := NewRenderer(cache)

	lobbyView := game.PublicGameView{
		GameID:  "g1",
		ChatID:  -1001,
		OwnerID: 10,
		Rules:   uno.BotRules(),
		Players: []game.PublicPlayer{{ID: 10, CardCount: 0}, {ID: 20, CardCount: 0}},
		Phase:   uno.Lobby,
	}

	lobbyText := renderer.RenderLobby(lobbyView)
	if !strings.Contains(lobbyText, "Alice") || !strings.Contains(lobbyText, "Bob") {
		t.Errorf("expected players in lobby text: %s", lobbyText)
	}
	if !strings.Contains(lobbyText, "/iniciar") {
		t.Errorf("expected /iniciar mentioned for 2+ players: %s", lobbyText)
	}

	topCard := uno.Card{ID: "c10", Color: uno.Green, Rank: uno.Five}
	playView := game.PublicGameView{
		GameID:      "g1",
		ChatID:      -1001,
		OwnerID:     10,
		Rules:       uno.BotRules(),
		Phase:       uno.TakingTurn,
		CurrentTurn: 20,
		Direction:   1,
		ActiveColor: uno.Green,
		TopCard:     &topCard,
		Order:       []uno.PlayerID{10, 20},
		Players: []game.PublicPlayer{
			{ID: 10, CardCount: 4, Active: true},
			{ID: 20, CardCount: 1, Active: true},
		},
	}

	stateText := renderer.RenderPublicState(playView)
	if strings.Contains(stateText, "UnoBotGO") {
		t.Errorf("expected no UnoBotGO header in state text: %s", stateText)
	}
	if strings.Contains(stateText, "cartas") {
		t.Errorf("expected no cartas count in state text: %s", stateText)
	}
	if !strings.Contains(stateText, "UNO!") {
		t.Errorf("expected UNO warning for 1 card: %s", stateText)
	}
	if !strings.Contains(stateText, "💚 5") {
		t.Errorf("expected top card in state text: %s", stateText)
	}
}

func TestRenderer_RenderActionConfirmation(t *testing.T) {
	cache := NewUserCache(10)
	cache.Put(10, "Alice", "alice")
	cache.Put(20, "Bob", "bob")
	renderer := NewRenderer(cache)

	topCard := uno.Card{ID: "c1", Color: uno.Red, Rank: uno.Three}
	outcome := game.Outcome{
		View: game.PublicGameView{
			GameID:      "g1",
			ChatID:      -1001,
			Phase:       uno.TakingTurn,
			CurrentTurn: 20,
			Direction:   -1,
			ActiveColor: uno.Red,
			TopCard:     &topCard,
			Order:       []uno.PlayerID{10, 20},
			Players: []game.PublicPlayer{
				{ID: 10, CardCount: 3, Active: true},
				{ID: 20, CardCount: 5, Active: true},
			},
		},
		Events: []uno.Event{
			{Type: uno.CardPlayed, PlayerID: 10, CardID: "c1"},
			{Type: uno.DirectionChanged, PlayerID: 10, Direction: -1},
		},
	}

	text := renderer.RenderActionConfirmation(10, uno.Action{Type: uno.PlayCard, PlayerID: 10}, outcome)
	if !strings.Contains(text, "Alice") || !strings.Contains(text, "jogou") || !strings.Contains(text, "invertido") {
		t.Errorf("unexpected action confirmation text: %s", text)
	}
}
