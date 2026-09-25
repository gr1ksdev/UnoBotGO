package telegram

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
)

func TestUserCache_HtmlEscapeAndEviction(t *testing.T) {
	cache := NewUserCache(2)
	renderer := NewRenderer(cache)
	renderer.SetBotID(999)

	cache.Put(1, "Alice <alert>", "alice_evil")
	cache.Put(2, "Bob & Co", "")

	link1 := renderer.PlayerLink(1, game.PublicGameView{})
	if !strings.Contains(link1, "Alice &lt;alert&gt; (@alice_evil)") {
		t.Errorf("expected escaped html link, got: %s", link1)
	}

	link2 := renderer.PlayerLink(2, game.PublicGameView{})
	if !strings.Contains(link2, "Bob &amp; Co") {
		t.Errorf("expected escaped html link, got: %s", link2)
	}

	// Put 3rd -> 1 should be evicted from cache and fallback to Jogador 1
	cache.Put(3, "Carol", "carol")

	link1Fallback := renderer.PlayerLink(1, game.PublicGameView{})
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

func TestRenderer_WelcomeAndHelp(t *testing.T) {
	renderer := NewRenderer(nil)
	welcome := renderer.RenderWelcome()
	if !strings.Contains(welcome, "Bem-vindo ao UnoBotGO") || !strings.Contains(welcome, "/help") {
		t.Fatalf("unexpected welcome: %s", welcome)
	}
	if strings.Contains(welcome, "V2") || strings.Contains(welcome, "Golang") || strings.Contains(welcome, "desenvolvida em Go") {
		t.Fatalf("welcome exposed implementation details: %s", welcome)
	}

	help := renderer.RenderHelp("ExampleBot")
	if strings.Count(help, "<blockquote>") != 1 || strings.Count(help, "</blockquote>") != 1 {
		t.Fatalf("help blockquote is malformed: %s", help)
	}
	for _, expected := range []string{"<b>/start</b>", "<b>/help</b>", "<b>/novo</b>", "<b>/reset</b>", "@ExampleBot", "Go (Golang)", "@unopybot"} {
		if !strings.Contains(help, expected) {
			t.Fatalf("help is missing %q: %s", expected, help)
		}
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

func TestRenderer_RenderCallBluffConfirmation(t *testing.T) {
	cache := NewUserCache(100)
	cache.Put(10, "Alice", "alice")
	cache.Put(20, "Bob", "bob")
	renderer := NewRenderer(cache)

	// 1. Bluff caught
	outcomeCaught := game.Outcome{
		View: game.PublicGameView{
			GameID: "g1",
			Phase:  uno.TakingTurn,
			Players: []game.PublicPlayer{
				{ID: 10, CardCount: 5, Active: true},
				{ID: 20, CardCount: 3, Active: true},
			},
		},
		Events: []uno.Event{
			{Type: uno.BluffCalled, PlayerID: 20, TargetID: 10, Success: true, Count: 4},
			{Type: uno.CardsDrawn, PlayerID: 10, Count: 4},
		},
	}
	textCaught := renderer.RenderActionConfirmation(20, uno.Action{Type: uno.CallBluff, PlayerID: 20}, outcomeCaught)
	if !strings.Contains(textCaught, "Blefe pego!") || !strings.Contains(textCaught, "Alice") || !strings.Contains(textCaught, "4 cartas") {
		t.Errorf("unexpected caught confirmation text: %s", textCaught)
	}

	// 2. Bluff failed
	outcomeFailed := game.Outcome{
		View: game.PublicGameView{
			GameID: "g1",
			Phase:  uno.TakingTurn,
			Players: []game.PublicPlayer{
				{ID: 10, CardCount: 1, Active: true},
				{ID: 20, CardCount: 8, Active: true},
			},
		},
		Events: []uno.Event{
			{Type: uno.BluffCalled, PlayerID: 20, TargetID: 10, Success: false, Count: 6},
			{Type: uno.CardsDrawn, PlayerID: 20, Count: 6},
		},
	}
	textFailed := renderer.RenderActionConfirmation(20, uno.Action{Type: uno.CallBluff, PlayerID: 20}, outcomeFailed)
	if !strings.Contains(textFailed, "não blefou!") || !strings.Contains(textFailed, "Alice") || !strings.Contains(textFailed, "Bob") || !strings.Contains(textFailed, "6 cartas") {
		t.Errorf("unexpected failed confirmation text: %s", textFailed)
	}
}

func TestRendererMentionTargetsFollowResultingTurn(t *testing.T) {
	cache := NewUserCache(100)
	cache.Put(11, `Alice <&"'>`, "alice")
	cache.Put(22, "Bob", "bob")
	cache.Put(33, "Carol", "")
	r := NewRenderer(cache)
	r.SetBotID(999)
	for _, tc := range []struct {
		name                string
		phase               uno.Phase
		closed              bool
		turn, chooser, real uno.PlayerID
	}{
		{"lobby", uno.Lobby, false, 0, 0, 0},
		{"alice_turn", uno.TakingTurn, false, 11, 0, 11},
		{"bob_turn", uno.TakingTurn, false, 22, 0, 22},
		{"color", uno.ChoosingColor, false, 11, 11, 11},
		{"finished", uno.Finished, true, 0, 0, 0},
		{"closed_overrides_turn", uno.TakingTurn, true, 11, 0, 0},
		{"phase_overrides_turn", uno.Finished, false, 11, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := game.PublicGameView{
				GameID: "test", OwnerID: 11, Rules: uno.BotRules(), Phase: tc.phase,
				Closed: tc.closed, CurrentTurn: tc.turn, ColorChooserID: tc.chooser,
				Direction: 1, ActiveColor: uno.Red, TopCard: &uno.Card{Color: uno.Red, Rank: uno.One},
				Players: []game.PublicPlayer{{ID: 11, Active: true, CardCount: 1}, {ID: 22, Active: true, CardCount: 3}, {ID: 33}},
				Order:   []uno.PlayerID{11, 22}, Placements: []uno.Placement{{PlayerID: 33, Position: 1, WentOut: true}},
			}
			text := r.RenderPublicState(v)
			if tc.phase == uno.Lobby {
				text = r.RenderLobby(v)
			}
			assertMentionTargets(t, text, cache, tc.real, 999)
			for _, action := range []uno.ActionType{uno.PlayCard, uno.DrawCard, uno.PassTurn, uno.ChooseColor} {
				out := game.Outcome{View: v, Events: []uno.Event{
					{Type: uno.CardsDrawn, PlayerID: 11, Count: 2},
					{Type: uno.PlayerSkipped, PlayerID: 22},
					{Type: uno.PlayerWon, PlayerID: 33, Position: 1},
				}}
				confirmation := r.RenderActionConfirmation(11, uno.Action{Type: action, Color: uno.Blue}, out)
				assertMentionTargets(t, confirmation, cache, tc.real, 999)
				if (tc.closed || tc.phase == uno.Finished) && strings.Contains(confirmation, "Vez de") {
					t.Fatalf("turn after closure: %s", confirmation)
				}
			}
		})
	}
}

func assertMentionTargets(t *testing.T, text string, cache *UserCache, real uno.PlayerID, botID int64) {
	t.Helper()
	links := regexp.MustCompile(`<a href="tg://user\?id=([0-9]+)">([^<]*)</a>`).FindAllStringSubmatch(text, -1)
	if len(links) == 0 {
		t.Fatalf("missing mentions: %s", text)
	}
	for _, link := range links {
		found := false
		for _, id := range []uno.PlayerID{11, 22, 33} {
			if link[2] != html.EscapeString(cache.GetRawName(id)) {
				continue
			}
			found = true
			want := botID
			if id == real {
				want = int64(id)
			}
			if link[1] != strconv.FormatInt(want, 10) {
				t.Errorf("name %s targets %s, want %d", link[2], link[1], want)
			}
		}
		if !found {
			t.Errorf("unknown or improperly escaped name: %s", link[2])
		}
	}
}

func TestRendererMissingIdentityAndContext(t *testing.T) {
	r := NewRenderer(nil)
	r.userCache.Put(11, "A <&>", "")
	view := game.PublicGameView{Phase: uno.TakingTurn, CurrentTurn: 11}
	if got := r.PlayerLink(11, view); got != "A &lt;&amp;&gt;" {
		t.Fatalf("renderer without bot identity: %s", got)
	}
	r.SetBotID(999)
	if got := r.PlayerLink(11, game.PublicGameView{}); got != `<a href="tg://user?id=999">A &lt;&amp;&gt;</a>` {
		t.Fatal(got)
	}
	if got := r.PlayerLink(44, game.PublicGameView{}); got != `<a href="tg://user?id=999">Jogador 44</a>` {
		t.Fatal(got)
	}
}

func TestGameButtonsRespectLifecycle(t *testing.T) {
	for _, phase := range []uno.Phase{uno.Lobby, uno.TakingTurn, uno.ChoosingColor, uno.Finished} {
		v := game.PublicGameView{GameID: "context", Phase: phase}
		markup := makeGameButtons(v)
		if phase == uno.Finished {
			if markup != nil {
				t.Fatal("terminal keyboard")
			}
			continue
		}
		if phase == uno.Lobby {
			if markup == nil || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 2 {
				t.Fatalf("expected lobby mode buttons: %+v", markup)
			}
			if markup.InlineKeyboard[0][0].CallbackData != "mode_classic_context" || markup.InlineKeyboard[0][1].CallbackData != "mode_caseiro_context" {
				t.Fatalf("unexpected lobby callback data: %+v", markup)
			}
		} else {
			if markup == nil || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 || *markup.InlineKeyboard[0][0].SwitchInlineQueryCurrentChat != "g_context_0" {
				t.Fatalf("contextual hand button changed: %+v", markup)
			}
		}
		v.Closed = true
		if makeGameButtons(v) != nil {
			t.Fatal("closed view offered keyboard")
		}
	}
}
