package telegram

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func TestSwapChoiceMenuFiltersTargetsAndProtectsTokens(t *testing.T) {
	renderer := NewRenderer(nil)
	renderer.SetBotID(999)
	renderer.userCache.Put(1, "Alice", "")
	renderer.userCache.Put(2, "Bob <&>", "")
	renderer.userCache.Put(3, "Carol", "")
	tokens := NewTokenStore(100, 100, time.Now, nil)
	h := NewInlineHandler(newMockBotAPI(), nil, renderer, tokens, time.Minute, nil, nil)
	view := game.PlayerGameView{
		PlayerID: 1, Hand: []game.CardView{{Card: uno.Card{ID: "private", Color: uno.Blue, Rank: uno.Nine}}},
		Public: game.PublicGameView{GameID: "swap", ChatID: -100, Revision: 7, Phase: uno.ChoosingPlayer, PlayerChooserID: 1, CurrentTurn: 1,
			Players: []game.PublicPlayer{{ID: 1, Active: true}, {ID: 2, Active: true, CardCount: 3}, {ID: 3, Active: true, CardCount: 1}, {ID: 4, Active: false}},
		},
	}
	results := h.playerChoiceResults(1, view)
	if len(results) != 3 {
		t.Fatalf("expected two targets and private hand summary: %d", len(results))
	}
	for i, target := range []uno.PlayerID{2, 3} {
		article, ok := results[i].(*telego.InlineQueryResultArticle)
		if !ok {
			t.Fatalf("unexpected result %T", results[i])
		}
		if article.ReplyMarkup != nil || !strings.Contains(article.Title, renderer.userCache.GetRawName(target)) {
			t.Fatal(article)
		}
		if _, status := tokens.ConsumeAction(article.ID, 2); status != ConsumeUserMismatch {
			t.Fatal(status)
		}
		token, status := tokens.ConsumeAction(article.ID, 1)
		if status != ConsumeOK || token.Action.Type != uno.ChoosePlayer || token.Action.TargetID != target || token.Action.Revision != 7 || token.ChatID != -100 {
			t.Fatal(token, status)
		}
		if _, status := tokens.ConsumeAction(article.ID, 1); status != ConsumeAlreadyConsumed {
			t.Fatal(status)
		}
	}
	// Sending the private summary publishes only public state.
	summary := results[2].(*telego.InlineQueryResultArticle)
	if strings.Contains(summary.InputMessageContent.(*telego.InputTextMessageContent).MessageText, "private") {
		t.Fatal("private data published")
	}
	waiting := h.playerChoiceResults(2, view)
	if len(waiting) != 2 || !strings.HasPrefix(waiting[0].(*telego.InlineQueryResultArticle).ID, "wait_") {
		t.Fatal(waiting)
	}
	confirmation := renderer.RenderActionConfirmation(1, uno.Action{Type: uno.ChoosePlayer, TargetID: 2}, game.Outcome{View: view.Public})
	if !strings.Contains(confirmation, "trocou todas as cartas") || !strings.Contains(confirmation, "Bob &lt;&amp;&gt;") {
		t.Fatal(confirmation)
	}
	if !strings.Contains(renderer.PlayerLink(1, view.Public), "id=1") || !strings.Contains(renderer.PlayerLink(2, view.Public), "id=999") {
		t.Fatal("wrong choice mention")
	}
	if GetCardStickerID(uno.Card{Rank: uno.SwapHands}) != "CAACAgEAAxkBAAER8VtqteJsR8-zG10NFeLTIZyxuZYsBQACBwkAAkkSsEU562tb90Ja3D0E" {
		t.Fatal("wrong sticker")
	}
	if GetCardStickerGreyID(uno.Card{Rank: uno.SwapHands}) != "CAACAgEAAxkBAAER8aRqtlf6ZtRKfAj02K5AnlVcRz_W_AACVAcAAkaGsEXgXGCANqlQKz0E" {
		t.Fatal("wrong unavailable sticker")
	}
}

// Draw/pass without playing until the unique swap card is available. This
// reaches the real service flow regardless of shuffle, without a restore hook.
func readyToSwap(t *testing.T, svc *game.Service) (game.PublicGameView, uno.CardID) {
	t.Helper()
	v := createStartedGame(t, svc, -901, uno.CaseiroRules())
	for i := 0; i < 220; i++ {
		pv, err := svc.PlayerView(t.Context(), game.Actor{PlayerID: v.CurrentTurn}, v.GameID)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range pv.Hand {
			if c.Card.Rank == uno.SwapHands && c.Playable {
				return v, c.Card.ID
			}
		}
		a := uno.Action{Type: uno.DrawCard, PlayerID: v.CurrentTurn, Revision: v.Revision}
		if pv.DrawnCardID != "" {
			a.Type = uno.PassTurn
		}
		out, err := svc.Apply(t.Context(), game.Actor{PlayerID: v.CurrentTurn}, v.GameID, a)
		if err != nil {
			t.Fatal(err)
		}
		v = out.View
	}
	t.Fatal("swap card never found")
	return game.PublicGameView{}, ""
}

func TestSwapInlineFlowExchangesHandsAndRejectsOldSelection(t *testing.T) {
	svc, err := game.NewService()
	if err != nil {
		t.Fatal(err)
	}
	view, cardID := readyToSwap(t, svc)
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	actor := view.CurrentTurn
	other := uno.PlayerID(11)
	if actor == 11 {
		other = 22
	}
	b.renderer.userCache.Put(other, "Opponent", "")
	before, err := svc.PlayerView(t.Context(), game.Actor{PlayerID: other}, view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	// All pages are considered: drawing to find the card may create a large hand.
	var playToken string
	offset := ""
	for {
		results, next := b.inlineHandler.buildPlayerHandResults(t.Context(), actor, view.GameID, offset)
		for _, r := range results {
			if sticker, ok := r.(*telego.InlineQueryResultCachedSticker); ok && sticker.StickerFileID == Stickers["swap_hands"] {
				playToken = sticker.ID
			}
		}
		if playToken != "" || next == "" {
			break
		}
		offset = next
	}
	if playToken == "" {
		t.Fatalf("missing playable swap %s", cardID)
	}
	b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: playToken, From: telego.User{ID: int64(actor), FirstName: "Chooser"}})
	flushChat(t, b, view.ChatID)
	pending, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Phase != uno.ChoosingPlayer {
		t.Fatal(pending.Phase)
	}
	chooserBefore, err := svc.PlayerView(t.Context(), game.Actor{PlayerID: actor}, view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	results, next := b.inlineHandler.buildPlayerHandResults(t.Context(), actor, view.GameID, "")
	if next != "" || len(results) != 2 {
		t.Fatal("expected opponent and summary")
	}
	choice := results[0].(*telego.InlineQueryResultArticle)
	oldResults, _ := b.inlineHandler.buildPlayerHandResults(t.Context(), actor, view.GameID, "")
	oldChoice := oldResults[0].(*telego.InlineQueryResultArticle)
	b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: choice.ID, From: telego.User{ID: int64(other)}})
	flushChat(t, b, view.ChatID)
	unchanged, _ := svc.PublicView(t.Context(), view.GameID)
	if unchanged.Revision != pending.Revision {
		t.Fatal("third party confirmed swap")
	}
	b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: choice.ID, From: telego.User{ID: int64(actor)}})
	flushChat(t, b, view.ChatID)
	final, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Phase != uno.TakingTurn || final.CurrentTurn != other || final.ActiveColor != view.ActiveColor {
		t.Fatal(final)
	}
	ids := func(hand []game.CardView) []uno.CardID {
		var ids []uno.CardID
		for _, c := range hand {
			ids = append(ids, c.Card.ID)
		}
		return ids
	}
	afterActor, _ := svc.PlayerView(t.Context(), game.Actor{PlayerID: actor}, view.GameID)
	afterOther, _ := svc.PlayerView(t.Context(), game.Actor{PlayerID: other}, view.GameID)
	if !slices.Equal(ids(afterActor.Hand), ids(before.Hand)) || !slices.Equal(ids(afterOther.Hand), ids(chooserBefore.Hand)) {
		t.Fatal("wrong exchanged views")
	}
	for _, token := range []string{choice.ID, oldChoice.ID} {
		b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: token, From: telego.User{ID: int64(actor)}})
		flushChat(t, b, view.ChatID)
	}
	stable, _ := svc.PublicView(t.Context(), view.GameID)
	if stable.Revision != final.Revision {
		t.Fatal("old/repeated selection applied twice")
	}
	status, consumed, found := b.tokens.GetActionStatus(oldChoice.ID)
	if !found || !consumed || status != "stale" {
		t.Fatal(status, consumed, found)
	}
}

func TestSwapUnavailableUsesGreyStickerWithoutAction(t *testing.T) {
	svc, err := game.NewService()
	if err != nil {
		t.Fatal(err)
	}
	view, cardID := readyToSwap(t, svc)
	actor := view.CurrentTurn
	pv, err := svc.PlayerView(t.Context(), game.Actor{PlayerID: actor}, view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if pv.DrawnCardID == "" {
		out, err := svc.Apply(t.Context(), game.Actor{PlayerID: actor}, view.GameID, uno.Action{
			Type: uno.DrawCard, PlayerID: actor, Revision: view.Revision,
		})
		if err != nil {
			t.Fatal(err)
		}
		view = out.View
	}
	out, err := svc.Apply(t.Context(), game.Actor{PlayerID: actor}, view.GameID, uno.Action{
		Type: uno.PassTurn, PlayerID: actor, Revision: view.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	view = out.View

	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)

	var unavailable *telego.InlineQueryResultCachedSticker
	offset := ""
	for {
		results, next := b.inlineHandler.buildPlayerHandResults(t.Context(), actor, view.GameID, offset)
		for _, result := range results {
			sticker, ok := result.(*telego.InlineQueryResultCachedSticker)
			if ok && sticker.StickerFileID == StickersGrey["swap_hands"] {
				unavailable = sticker
				break
			}
		}
		if unavailable != nil || next == "" {
			break
		}
		offset = next
	}
	if unavailable == nil {
		t.Fatalf("missing unavailable sticker for swap card %s", cardID)
	}
	if !strings.HasPrefix(unavailable.ID, "grey_") {
		t.Fatalf("unexpected unavailable result ID %q", unavailable.ID)
	}
	if _, _, found := b.tokens.GetActionStatus(unavailable.ID); found {
		t.Fatal("unavailable sticker created an action token")
	}

	before, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if !b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{
		ResultID: unavailable.ID,
		From:     telego.User{ID: int64(actor), FirstName: "Chooser"},
	}) {
		t.Fatal("unavailable sticker was not handled")
	}
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || after.CurrentTurn != before.CurrentTurn {
		t.Fatal("unavailable sticker changed the game")
	}
}
