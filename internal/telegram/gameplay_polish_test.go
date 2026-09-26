package telegram

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
)

func polishFixture() (*Renderer, game.PublicGameView) {
	c := NewUserCache(20)
	c.Put(11, "Freddy", "")
	c.Put(22, "Mezi", "")
	c.Put(33, "Jeesttin", "")
	c.Put(44, "Novo", "")
	r := NewRenderer(c)
	r.SetBotID(999)
	return r, game.PublicGameView{Phase: uno.TakingTurn, CurrentTurn: 22, Direction: 1, ActiveColor: uno.Red, Rules: uno.CaseiroRules(), TopCard: &uno.Card{Color: uno.Red, Rank: uno.Four}, Order: []uno.PlayerID{11, 22, 33}, Players: []game.PublicPlayer{{ID: 11, Active: true, CardCount: 3}, {ID: 22, Active: true, CardCount: 3}, {ID: 33, Active: true, CardCount: 3}}}
}
func plainGameplay(s string) string { return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "") }
func requireGameplay(t *testing.T, text string, want ...string) {
	t.Helper()
	for _, fragment := range want {
		if !strings.Contains(text, fragment) {
			t.Fatalf("missing %q in %s", fragment, text)
		}
	}
}
func TestGameplayCardColorAndPenalty(t *testing.T) {
	for _, rules := range []uno.Rules{uno.BotRules(), uno.CaseiroRules()} {
		for _, rank := range []uno.Rank{uno.Four, uno.Skip, uno.Reverse, uno.DrawTwo, uno.Wild, uno.WildDrawFour, uno.SwapHands} {
			t.Run(fmt.Sprintf("caseiro=%v/rank=%d", rules.AllowSwapHands, rank), func(t *testing.T) {
				r, v := polishFixture()
				v.Rules = rules
				v.TopCard.Rank = rank
				colorless := rank == uno.Wild || rank == uno.WildDrawFour || rank == uno.SwapHands
				if colorless {
					v.TopCard.Color = uno.NoColor
				}
				if rank == uno.DrawTwo || rank == uno.WildDrawFour {
					v.DrawCounter = 6
				}
				text := plainGameplay(r.RenderPublicState(v))
				requireGameplay(t, text, "🃏 Topo:", "🎯 Vez: 👉 Mezi", "👥 Mezi → Jeesttin → Freddy")
				if strings.Contains(text, "🎨 Cor:") != colorless {
					t.Fatalf("wrong color visibility: %s", text)
				}
				if v.DrawCounter > 0 {
					requireGameplay(t, text, "⚠️ Compra acumulada: 6 cartas")
				}
				for _, obsolete := range []string{"Cor ativa", "Carta no topo", "Jogadores em jogo", "Penalidade acumulada", "Colocações:", " | ", "Vez de:"} {
					if strings.Contains(text, obsolete) {
						t.Fatalf("obsolete %q: %s", obsolete, text)
					}
				}
			})
		}
	}
}
func TestGameplayOrderFollowsDirectionAndLateJoin(t *testing.T) {
	for _, dir := range []int{1, -1} {
		r, v := polishFixture()
		v.Direction = dir
		want := "👥 Mezi → Jeesttin → Freddy"
		if dir < 0 {
			want = "👥 Mezi → Freddy → Jeesttin"
		}
		requireGameplay(t, plainGameplay(r.RenderPublicState(v)), want)
		// Physical insertion before current clockwise; after current counter-clockwise.
		if dir > 0 {
			v.Order = []uno.PlayerID{11, 44, 22, 33}
		} else {
			v.Order = []uno.PlayerID{11, 22, 44, 33}
		}
		v.Players = append(v.Players, game.PublicPlayer{ID: 44, Active: true, CardCount: 7})
		requireGameplay(t, plainGameplay(r.RenderPublicState(v)), want+" → Novo")
		v.Players[0].Active = false
		text := plainGameplay(r.RenderPublicState(v))
		if strings.Contains(text, "Freddy") {
			t.Fatal("finished player remains in active order", text)
		}
	}
}
func TestGameplayActionEffectResultAndFinish(t *testing.T) {
	r, v := polishFixture()
	out := game.Outcome{View: v, Events: []uno.Event{{Type: uno.DirectionChanged, Direction: -1}, {Type: uno.PlayerSkipped, PlayerID: 33}, {Type: uno.PlayerWon, PlayerID: 11, Position: 1}}}
	out.View.TopCard = &uno.Card{Color: uno.Yellow, Rank: uno.Reverse}
	out.View.Placements = []uno.Placement{{PlayerID: 11, Position: 1, WentOut: true}}
	out.View.Order = []uno.PlayerID{22, 33}
	out.View.Players[0].Active = false
	text := plainGameplay(r.RenderActionConfirmation(11, uno.Action{Type: uno.PlayCard}, out))
	requireGameplay(t, text, "Freddy jogou 💛 🔄 Inverter!\n🔄 O sentido foi invertido.\n🚫 Jeesttin foi pulado.\n🥇 Freddy terminou em 1º lugar!", "🏅 Classificação: 🥇 Freddy", "🎯 Vez: 👉 Mezi", "👥 Mezi → Jeesttin")
	// Without a DirectionChanged event a Skip must not invent a reversal.
	out.Events = []uno.Event{{Type: uno.PlayerSkipped, PlayerID: 33}}
	out.View.TopCard = &uno.Card{Color: uno.Red, Rank: uno.Skip}
	text = plainGameplay(r.RenderActionConfirmation(11, uno.Action{Type: uno.PlayCard}, out))
	if strings.Contains(text, "sentido foi invertido") {
		t.Fatal(text)
	}
	requireGameplay(t, text, "\n🚫 Jeesttin foi pulado.")
	out.View.Closed = true
	out.View.Phase = uno.Finished
	out.View.CloseReason = game.Completed
	out.View.Placements = []uno.Placement{{PlayerID: 11, Position: 1}, {PlayerID: 22, Position: 2}, {PlayerID: 33, Position: 3}, {PlayerID: 44, Position: 4}}
	out.Events = append(out.Events, uno.Event{Type: uno.PlayerWon, PlayerID: 22, Position: 2})
	text = plainGameplay(r.RenderActionConfirmation(22, uno.Action{Type: uno.PlayCard}, out))
	requireGameplay(t, text, "🥈 Mezi terminou em 2º lugar!", "🏆 Partida encerrada\n\n🥇 Freddy\n🥈 Mezi\n🥉 Jeesttin\n4º Novo")
	if strings.Count(text, "Partida encerrada") != 1 || strings.Contains(text, "chegou ao fim") || strings.Contains(text, "🎯 Vez:") || strings.Contains(text, "👥") {
		t.Fatal(text)
	}
	for position := 1; position <= 5; position++ {
		out.Events = []uno.Event{{Type: uno.PlayerWon, PlayerID: 22, Position: position}}
		text = plainGameplay(r.RenderActionConfirmation(22, uno.Action{Type: uno.PlayCard}, out))
		requireGameplay(t, text, fmt.Sprintf("Mezi terminou em %dº lugar!", position))
		if position > 3 {
			requireGameplay(t, text, "🏅 Mezi terminou")
		}
	}
}
func TestGameplayChoicesMentionsEscapingAndPlacements(t *testing.T) {
	r, v := polishFixture()
	r.userCache.Put(11, "Freddy <&>", "")
	for _, phase := range []uno.Phase{uno.TakingTurn, uno.ChoosingColor, uno.ChoosingPlayer} {
		v.Phase = phase
		v.ColorChooserID = 22
		v.PlayerChooserID = 22
		text := r.RenderPublicState(v)
		assertMentionTargets(t, text, r.userCache, 22, 999)
		requireGameplay(t, text, "Freddy &lt;&amp;&gt;")
		if phase == uno.ChoosingPlayer {
			requireGameplay(t, text, "escolher um jogador para trocar cartas")
		}
		if phase == uno.ChoosingColor {
			requireGameplay(t, text, "escolher a cor")
		}
	}
	v.Phase = uno.TakingTurn
	v.CurrentTurn = 33
	v.TopCard = &uno.Card{Color: uno.NoColor, Rank: uno.WildDrawFour}
	v.ActiveColor = uno.Yellow
	v.DrawCounter = 4
	text := r.RenderActionConfirmation(22, uno.Action{Type: uno.ChooseColor, Color: uno.Yellow}, game.Outcome{View: v})
	assertMentionTargets(t, text, r.userCache, 33, 999)
	requireGameplay(t, plainGameplay(text), "Mezi escolheu 💛 Amarelo!", "🎨 Cor: 💛 Amarelo", "Compra acumulada: 4 cartas")
	v.TopCard = &uno.Card{Color: uno.NoColor, Rank: uno.SwapHands}
	v.DrawCounter = 0
	text = r.RenderActionConfirmation(22, uno.Action{Type: uno.ChoosePlayer, TargetID: 11}, game.Outcome{View: v})
	assertMentionTargets(t, text, r.userCache, 33, 999)
	requireGameplay(t, text, "trocou todas as cartas", "🎨 Cor:")
	v.Placements = []uno.Placement{{PlayerID: 11, Position: 1}, {PlayerID: 22, Position: 2}}
	v.Order = []uno.PlayerID{33}
	v.Players[0].Active = false
	v.Players[1].Active = false
	text = plainGameplay(r.RenderPublicState(v))
	requireGameplay(t, text, "🏅 Classificação\n🥇 Freddy &lt;&amp;&gt;\n🥈 Mezi", "👥 Jeesttin")
	if strings.Contains(text, " | ") {
		t.Fatal(text)
	}
}
