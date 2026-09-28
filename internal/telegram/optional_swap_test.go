package telegram

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func choiceArticle(t *testing.T, results []telego.InlineQueryResult, title string) *telego.InlineQueryResultArticle {
	t.Helper()
	for _, result := range results {
		if a, ok := result.(*telego.InlineQueryResultArticle); ok && a.Title == title {
			if a.ReplyMarkup != nil {
				t.Fatal("callback gameplay keyboard")
			}
			return a
		}
	}
	t.Fatalf("missing inline choice %q", title)
	return nil
}
func colorArticle(t *testing.T, results []telego.InlineQueryResult, color uno.Color) *telego.InlineQueryResultArticle {
	t.Helper()
	count := 0
	var selected *telego.InlineQueryResultArticle
	for _, result := range results {
		if a, ok := result.(*telego.InlineQueryResultArticle); ok && a.Title == "Escolha sua cor" {
			count++
			if a.ReplyMarkup != nil {
				t.Fatal("callback color keyboard")
			}
			if strings.HasSuffix(a.Description, ColorNamePT(color)) {
				selected = a
			}
		}
	}
	if count != 4 || selected == nil {
		t.Fatal("expected four existing color choices", count)
	}
	return selected
}

func TestKeepHandAvailableWithoutEligibleTargets(t *testing.T) {
	r := NewRenderer(nil)
	tokens := NewTokenStore(100, 100, time.Now, nil)
	h := NewInlineHandler(newMockBotAPI(), nil, r, tokens, time.Minute, nil, nil)
	view := game.PlayerGameView{PlayerID: 1, Public: game.PublicGameView{GameID: "game", ChatID: -1, Revision: 8, Phase: uno.ChoosingPlayer, PlayerChooserID: 1, CurrentTurn: 1,
		Players: []game.PublicPlayer{{ID: 1, Active: true}, {ID: 2, Status: uno.Left}, {ID: 3, Status: uno.WentOut}}}}
	results := h.playerChoiceResults(1, view)
	if len(results) != 1 {
		t.Fatal("self/left/placed appeared as target", len(results))
	}
	a := choiceArticle(t, results, "➡️ Manter minha mão")
	if _, status := tokens.ConsumeAction(a.ID, 2); status != ConsumeUserMismatch {
		t.Fatal(status)
	}
	token, status := tokens.ConsumeAction(a.ID, 1)
	if status != ConsumeOK || token.GameID != "game" || token.ChatID != -1 || token.Action.Type != uno.KeepHand || token.Action.TargetID != 0 || token.Action.Revision != 8 {
		t.Fatal(token, status)
	}
	if _, status = tokens.ConsumeAction(a.ID, 1); status != ConsumeAlreadyConsumed {
		t.Fatal(status)
	}
}

func TestKeepHandInlineFlowStaleColorAndMultigroup(t *testing.T) {
	for _, color := range []uno.Color{uno.Red, uno.Blue} {
		t.Run(ColorNamePT(color), func(t *testing.T) {
			svc, err := game.NewService()
			if err != nil {
				t.Fatal(err)
			}
			v, cardID := readyToSwap(t, svc)
			second := createStartedGame(t, svc, -902, uno.CaseiroRules())
			api := newMockBotAPI()
			b := New(api, svc, nil, nil, time.Minute, nil)
			defer b.dispatcher.Stop(time.Second)
			actor := v.CurrentTurn
			other := uno.PlayerID(11)
			if actor == 11 {
				other = 22
			}
			choose := func(token string, user uno.PlayerID) {
				b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: token, From: telego.User{ID: int64(user), FirstName: "Chooser"}})
				flushChat(t, b, v.ChatID)
			}
			// Play through the same signed action token used by the sticker result.
			play, err := b.tokens.CreateActionToken(actor, v.GameID, v.ChatID, uno.Action{Type: uno.PlayCard, PlayerID: actor, CardID: cardID, Revision: v.Revision}, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			choose(play, actor)
			beforeActor, _ := svc.PlayerView(t.Context(), game.Actor{PlayerID: actor}, v.GameID)
			beforeOther, _ := svc.PlayerView(t.Context(), game.Actor{PlayerID: other}, v.GameID)
			results, _ := b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
			keep := choiceArticle(t, results, "➡️ Manter minha mão")
			choose(keep.ID, other)
			unchanged, _ := svc.PublicView(t.Context(), v.GameID)
			if unchanged.Revision != v.Revision+1 {
				t.Fatal("wrong user changed decision")
			}
			choose(keep.ID, actor)
			pending, _ := svc.PublicView(t.Context(), v.GameID)
			if pending.Phase != uno.ChoosingColor || pending.Revision != v.Revision+2 || pending.CurrentTurn != actor {
				t.Fatal(pending)
			}
			results, _ = b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
			oldColor := colorArticle(t, results, color)
			// A join changes revision without changing hands or resolving pending color.
			joined, err := svc.Apply(t.Context(), game.Actor{PlayerID: 33, ChatID: v.ChatID}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 33, Revision: pending.Revision})
			if err != nil {
				t.Fatal(err)
			}
			choose(oldColor.ID, actor)
			status, consumed, found := b.tokens.GetActionStatus(oldColor.ID)
			if !found || !consumed || status != "stale" {
				t.Fatal(status, consumed, found)
			}
			stable, _ := svc.PublicView(t.Context(), v.GameID)
			if stable.Revision != joined.View.Revision || stable.Phase != uno.ChoosingColor {
				t.Fatal(stable)
			}
			// A mismatched game/chat cannot route this decision into another group.
			wrong, err := b.tokens.CreateActionToken(actor, second.GameID, v.ChatID, uno.Action{Type: uno.ChooseColor, PlayerID: actor, Color: color, Revision: second.Revision}, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			choose(wrong, actor)
			untouched, _ := svc.PublicView(t.Context(), second.GameID)
			if !reflect.DeepEqual(second, untouched) {
				t.Fatal("multigroup state changed")
			}
			results, _ = b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
			finalColor := colorArticle(t, results, color)
			choose(finalColor.ID, other)
			stable, _ = svc.PublicView(t.Context(), v.GameID)
			if stable.Revision != joined.View.Revision {
				t.Fatal("wrong user chose color")
			}
			choose(finalColor.ID, actor)
			final, _ := svc.PublicView(t.Context(), v.GameID)
			if final.Revision != joined.View.Revision+1 || final.CurrentTurn != other || final.ActiveColor != color || final.Phase != uno.TakingTurn {
				t.Fatal(final)
			}
			afterActor, _ := svc.PlayerView(t.Context(), game.Actor{PlayerID: actor}, v.GameID)
			afterOther, _ := svc.PlayerView(t.Context(), game.Actor{PlayerID: other}, v.GameID)
			ids := func(v game.PlayerGameView) []uno.Card {
				var cards []uno.Card
				for _, c := range v.Hand {
					cards = append(cards, c.Card)
				}
				return cards
			}
			if !reflect.DeepEqual(ids(beforeActor), ids(afterActor)) || !reflect.DeepEqual(ids(beforeOther), ids(afterOther)) {
				t.Fatal("keep transferred cards")
			}
			choose(keep.ID, actor)
			choose(finalColor.ID, actor)
			stable, _ = svc.PublicView(t.Context(), v.GameID)
			if stable.Revision != final.Revision {
				t.Fatal("replayed decision")
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			foundKeep := false
			for _, m := range api.SentMessages {
				if strings.Contains(m.Text, "manteve sua mão") {
					foundKeep = true
					if !strings.Contains(m.Text, "🎨 Cor:") || strings.Contains(m.Text, "trocou as mãos") {
						t.Fatal(m.Text)
					}
				}
			}
			if !foundKeep {
				t.Fatal("missing public keep confirmation")
			}
		})
	}
}

func TestSwapRendererUsesResolvedEvents(t *testing.T) {
	r, v := polishFixture()
	v.TopCard = &uno.Card{Rank: uno.SwapHands}
	v.ActiveColor = uno.Blue
	for _, kind := range []uno.EventType{uno.HandsSwapped, uno.HandKept} {
		text := plainGameplay(r.RenderActionConfirmation(11, uno.Action{Type: uno.ChooseColor, Color: uno.Blue}, game.Outcome{View: v, Events: []uno.Event{{Type: kind, PlayerID: 11, TargetID: 22}}}))
		requireGameplay(t, text, "Freddy jogou", "Trocar cartas", "🎨 Cor: 💙 Azul")
		if kind == uno.HandKept {
			requireGameplay(t, text, "Freddy manteve sua mão.")
			if strings.Contains(text, "trocou as mãos") {
				t.Fatal(text)
			}
		} else {
			requireGameplay(t, text, "Freddy trocou as mãos com Mezi.")
		}
	}
	for _, action := range []uno.Action{{Type: uno.ChoosePlayer, TargetID: 22}, {Type: uno.KeepHand}} {
		text := r.RenderActionConfirmation(11, action, game.Outcome{View: v})
		if strings.Contains(text, "trocou as mãos") || strings.Contains(text, "manteve sua mão") {
			t.Fatal("selection claimed resolved effect", text)
		}
		requireGameplay(t, text, "escolher a cor")
	}
}

func TestSwapInlineTargetDepartureInvalidatesColor(t *testing.T) {
	svc, err := game.NewService()
	if err != nil {
		t.Fatal(err)
	}
	v, cardID := readyToSwap(t, svc)
	actor := v.CurrentTurn
	other := uno.PlayerID(11)
	if actor == 11 {
		other = 22
	}
	out, err := svc.Apply(t.Context(), game.Actor{PlayerID: 33, ChatID: v.ChatID}, v.GameID, uno.Action{Type: uno.JoinGame, PlayerID: 33, Revision: v.Revision})
	if err != nil {
		t.Fatal(err)
	}
	out, err = svc.Apply(t.Context(), game.Actor{PlayerID: actor}, v.GameID, uno.Action{Type: uno.PlayCard, PlayerID: actor, CardID: cardID, Revision: out.View.Revision})
	if err != nil {
		t.Fatal(err)
	}
	b := New(newMockBotAPI(), svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	b.renderer.userCache.Put(other, "Target", "")
	choose := func(token string) {
		b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: token, From: telego.User{ID: int64(actor)}})
		flushChat(t, b, v.ChatID)
	}
	results, _ := b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
	choose(choiceArticle(t, results, "🔄 Trocar com Target").ID)
	results, _ = b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
	old := colorArticle(t, results, uno.Green)
	pending, _ := svc.PublicView(t.Context(), v.GameID)
	out, err = svc.Apply(t.Context(), game.Actor{PlayerID: other, ChatID: v.ChatID}, v.GameID, uno.Action{Type: uno.LeaveGame, PlayerID: other, Revision: pending.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if out.View.Phase != uno.ChoosingPlayer {
		t.Fatal(out.View)
	}
	choose(old.ID)
	status, _, _ := b.tokens.GetActionStatus(old.ID)
	if status != "stale" {
		t.Fatal(status)
	}
	results, _ = b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
	for _, result := range results {
		if a, ok := result.(*telego.InlineQueryResultArticle); ok && a.Title == "🔄 Trocar com Target" {
			t.Fatal("departed target offered")
		}
	}
	choose(choiceArticle(t, results, "➡️ Manter minha mão").ID)
	results, _ = b.inlineHandler.buildPlayerHandResults(t.Context(), actor, v.GameID, "")
	choose(colorArticle(t, results, uno.Green).ID)
	final, _ := svc.PublicView(t.Context(), v.GameID)
	if final.Phase != uno.TakingTurn || final.CurrentTurn != 33 || final.ActiveColor != uno.Green {
		t.Fatal(final)
	}
}
