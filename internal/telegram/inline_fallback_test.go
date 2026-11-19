package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func TestInlineQueryFallsBackToArticlesOnDocumentInvalid(t *testing.T) {
	api := newMockBotAPI()
	api.inlineQueryErr = errors.New(`telego: answerInlineQuery: api: 400 "Bad Request: DOCUMENT_INVALID"`)
	svc, _ := game.NewService()
	h := NewInlineHandler(api, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), time.Minute, nil, nil)

	// A started game with two players makes the caller's hand render as stickers.
	out, _ := svc.Create(context.Background(), game.Actor{PlayerID: 1, ChatID: -1001}, game.CreateRequest{ChatName: "Grupo", Rules: uno.CaseiroRules()})
	gameID := out.View.GameID
	svc.Apply(context.Background(), game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 1, Revision: 0})
	svc.Apply(context.Background(), game.Actor{PlayerID: 2, ChatID: -1001}, gameID, uno.Action{Type: uno.JoinGame, PlayerID: 2, Revision: 1})
	svc.Apply(context.Background(), game.Actor{PlayerID: 1, ChatID: -1001}, gameID, uno.Action{Type: uno.StartGame, PlayerID: 1, Revision: 2})

	// The idle player's hand is all grey stickers, which is what failed in the
	// reported scenario.
	idle := uno.PlayerID(2)
	view, _ := svc.PublicView(context.Background(), gameID)
	if view.CurrentTurn == 2 {
		idle = 1
	}

	h.HandleInlineQuery(context.Background(), &telego.InlineQuery{ID: "q1", From: telego.User{ID: int64(idle), FirstName: "Jogador"}})

	if len(api.AnsweredInlines) != 2 {
		t.Fatalf("expected the fallback retry, got %d answers", len(api.AnsweredInlines))
	}
	first := api.AnsweredInlines[0].Results
	fallback := api.AnsweredInlines[1].Results
	var stickers, articles int
	for _, r := range first {
		if _, ok := r.(*telego.InlineQueryResultCachedSticker); ok {
			stickers++
		}
	}
	for _, r := range fallback {
		if a, ok := r.(*telego.InlineQueryResultArticle); ok {
			articles++
			if a.Title == "" {
				t.Fatal("fallback article has no title")
			}
		}
	}
	if stickers == 0 || articles < stickers {
		t.Fatalf("fallback did not replace the stickers: %d stickers, %d articles", stickers, articles)
	}
}

func TestDropInvalidStickersRemovesEmptyFileIDs(t *testing.T) {
	results := []telego.InlineQueryResult{
		&telego.InlineQueryResultCachedSticker{Type: "sticker", ID: "ok", StickerFileID: "valid"},
		&telego.InlineQueryResultCachedSticker{Type: "sticker", ID: "bad", StickerFileID: ""},
		&telego.InlineQueryResultArticle{Type: "article", ID: "article"},
	}
	kept, dropped := dropInvalidStickers(results)
	if dropped != 1 || len(kept) != 2 {
		t.Fatalf("unexpected result: dropped=%d kept=%d", dropped, len(kept))
	}
	for _, r := range kept {
		if s, ok := r.(*telego.InlineQueryResultCachedSticker); ok && s.StickerFileID == "" {
			t.Fatal("kept an empty sticker file id")
		}
	}
}

func TestStickerResultsToArticlesPreservesResultID(t *testing.T) {
	results := []telego.InlineQueryResult{
		&telego.InlineQueryResultCachedSticker{
			Type:          "sticker",
			ID:            "grey_c109_3",
			StickerFileID: "invalid",
			InputMessageContent: &telego.InputTextMessageContent{
				MessageText: "🃏 Topo: 💚 7\n\nVez de: Fulano",
				ParseMode:   "HTML",
			},
		},
	}
	fallback := stickerResultsToArticles(results)
	if len(fallback) != 1 {
		t.Fatalf("unexpected fallback: %+v", fallback)
	}
	article := fallback[0].(*telego.InlineQueryResultArticle)
	if article.ID != "grey_c109_3" {
		t.Fatalf("result id changed: %s", article.ID)
	}
	if !strings.Contains(article.Title, "Topo") {
		t.Fatalf("title did not carry the card state: %s", article.Title)
	}
}
