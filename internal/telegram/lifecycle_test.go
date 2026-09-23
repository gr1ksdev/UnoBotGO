package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func waitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker")
	}
}

func flushChat(t *testing.T, b *Bot, chat game.ChatID) {
	t.Helper()
	done := make(chan struct{})
	if !b.dispatcher.EnqueueChat(chat, func(context.Context) { close(done) }) {
		t.Fatal("could not enqueue barrier")
	}
	waitSignal(t, done)
}

func createStartedGame(t *testing.T, svc *game.Service, chat game.ChatID, rules uno.Rules) game.PublicGameView {
	t.Helper()
	out, err := svc.Create(t.Context(), game.Actor{PlayerID: 11, ChatID: chat}, game.CreateRequest{ChatName: "UNO", Rules: rules})
	if err != nil {
		t.Fatal(err)
	}
	for _, player := range []uno.PlayerID{11, 22} {
		out, err = svc.Apply(t.Context(), game.Actor{PlayerID: player, ChatID: chat}, out.View.GameID, uno.Action{Type: uno.JoinGame, PlayerID: player, Revision: out.View.Revision})
		if err != nil {
			t.Fatal(err)
		}
	}
	out, err = svc.Apply(t.Context(), game.Actor{PlayerID: 11, ChatID: chat}, out.View.GameID, uno.Action{Type: uno.StartGame, PlayerID: 11, Revision: out.View.Revision})
	if err != nil {
		t.Fatal(err)
	}
	return out.View
}

// Reach the final action through the real public service API, without exposing
// a snapshot restore hook in production. Exact card/effect matrices live in uno.
func readyToFinish(t *testing.T, svc *game.Service, chat game.ChatID, rules uno.Rules, resolveWild bool) (game.PublicGameView, uno.Action) {
	t.Helper()
	view := createStartedGame(t, svc, chat, rules)
	for step := 0; step < 10000; step++ {
		pv, err := svc.PlayerView(t.Context(), game.Actor{PlayerID: view.CurrentTurn}, view.GameID)
		if err != nil {
			t.Fatal(err)
		}
		action := uno.Action{PlayerID: view.CurrentTurn, Revision: view.Revision}
		if view.Phase == uno.ChoosingColor {
			action.Type, action.Color = uno.ChooseColor, uno.Red
			if len(pv.Hand) == 0 {
				return view, action
			}
		} else {
			for _, cv := range pv.Hand {
				if cv.Playable {
					action.Type, action.CardID = uno.PlayCard, cv.Card.ID
					if len(pv.Hand) == 1 && (cv.Card.Rank < uno.Wild || !resolveWild) {
						return view, action
					}
					break
				}
			}
			if action.Type == 0 {
				action.Type = uno.DrawCard
				if pv.DrawnCardID != "" {
					action.Type = uno.PassTurn
				}
			}
		}
		out, err := svc.Apply(t.Context(), game.Actor{PlayerID: action.PlayerID, ChatID: chat}, view.GameID, action)
		if err != nil {
			t.Fatal(err)
		}
		view = out.View
	}
	t.Fatal("game did not reach a final action")
	return game.PublicGameView{}, uno.Action{}
}

func issueToken(t *testing.T, b *Bot, view game.PublicGameView, action uno.Action) string {
	t.Helper()
	token, err := b.tokens.CreateActionToken(action.PlayerID, view.GameID, view.ChatID, action, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// Notification channels make transport and queue tests independent of sleeps.
type lifecycleAPI struct {
	*mockBotAPI
	ready chan struct{}
	sent  chan struct{}
}

func (a *lifecycleAPI) UpdatesViaLongPolling(ctx context.Context, params *telego.GetUpdatesParams, options ...telego.LongPollingOption) (<-chan telego.Update, error) {
	updates, err := a.mockBotAPI.UpdatesViaLongPolling(ctx, params, options...)
	close(a.ready)
	return updates, err
}

func (a *lifecycleAPI) SetWebhook(ctx context.Context, params *telego.SetWebhookParams) error {
	err := a.mockBotAPI.SetWebhook(ctx, params)
	close(a.ready)
	return err
}

func (a *lifecycleAPI) SendMessage(ctx context.Context, params *telego.SendMessageParams) (*telego.Message, error) {
	msg, err := a.mockBotAPI.SendMessage(ctx, params)
	a.sent <- struct{}{}
	return msg, err
}

func TestFinalInlineActionAcrossTransports(t *testing.T) {
	for _, mode := range []TransportMode{TransportPolling, TransportWebhook} {
		for name, rules := range map[string]uno.Rules{"classic": uno.BotRules(), "caseiro": uno.CaseiroRules()} {
			t.Run(string(mode)+"/"+name, func(t *testing.T) {
				svc, _ := game.NewService()
				view, action := readyToFinish(t, svc, -1001, rules, true)
				api := &lifecycleAPI{mockBotAPI: newMockBotAPI(), ready: make(chan struct{}), sent: make(chan struct{}, 32)}
				b := New(api, svc, nil, nil, time.Minute, nil)
				b.SetTransport(TransportConfig{Mode: mode, WebhookURL: "https://example.com/telegram", WebhookSecret: "secret", ListenAddr: "127.0.0.1:0"})
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- b.Run(ctx) }()
				t.Cleanup(func() {
					cancel()
					select {
					case err := <-done:
						if err != nil {
							t.Error(err)
						}
					case <-time.After(5 * time.Second):
						t.Error("bot shutdown timed out")
					}
					b.dispatcher.Stop(time.Second)
				})
				waitSignal(t, api.ready)
				if b.renderer.botID.Load() != api.MeUser.ID {
					t.Fatal("GetMe identity not propagated")
				}
				token := issueToken(t, b, view, action)
				other := createStartedGame(t, svc, -2002, rules)
				otherToken := issueToken(t, b, other, uno.Action{Type: uno.DrawCard, PlayerID: other.CurrentTurn, Revision: other.Revision})
				update := telego.Update{UpdateID: 1, ChosenInlineResult: &telego.ChosenInlineResult{ResultID: token, From: telego.User{ID: int64(action.PlayerID), FirstName: "Winner"}}}
				if mode == TransportPolling {
					api.UpdatesChan <- update
				} else {
					body, err := json.Marshal(update)
					if err != nil {
						t.Fatal(err)
					}
					req := httptest.NewRequest(http.MethodPost, "/telegram", strings.NewReader(string(body)))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set(telego.WebhookSecretTokenHeader, "secret")
					rr := httptest.NewRecorder()
					b.webhookHandler("secret").ServeHTTP(rr, req)
					if rr.Code != http.StatusOK {
						t.Fatalf("webhook: %d", rr.Code)
					}
				}
				waitSignal(t, api.sent)
				flushChat(t, b, view.ChatID)
				api.mu.Lock()
				messages := append([]telego.SendMessageParams(nil), api.SentMessages...)
				api.mu.Unlock()
				if len(messages) != 1 || !strings.Contains(messages[0].Text, "Partida Encerrada") || strings.Contains(messages[0].Text, "Vez de") || messages[0].ReplyMarkup != nil {
					t.Fatalf("terminal confirmation: %+v", messages)
				}
				for _, id := range []uno.PlayerID{11, 22} {
					if strings.Contains(messages[0].Text, fmt.Sprintf(`href="tg://user?id=%d"`, id)) {
						t.Fatal("real player mentioned after closure")
					}
				}
				closed, err := svc.PublicView(t.Context(), view.GameID)
				if err != nil || !closed.Closed || closed.CurrentTurn != 0 || len(closed.Placements) != 2 || closed.Placements[1].WentOut {
					t.Fatalf("closure: %+v %v", closed, err)
				}
				if _, _, found := b.tokens.GetActionStatus(token); found {
					t.Fatal("finished game tokens retained")
				}
				if _, _, found := b.tokens.GetActionStatus(otherToken); !found {
					t.Fatal("other game's token invalidated")
				}
				b.inlineHandler.HandleInlineQuery(t.Context(), &telego.InlineQuery{ID: "historical", From: telego.User{ID: int64(action.PlayerID)}, Query: "g_" + string(view.GameID)})
				api.mu.Lock()
				answer := api.AnsweredInlines[len(api.AnsweredInlines)-1]
				api.mu.Unlock()
				article, ok := answer.Results[0].(*telego.InlineQueryResultArticle)
				if !ok || article.Title != "Partida encerrada" || article.ReplyMarkup != nil {
					t.Fatalf("historical button: %+v", answer)
				}
			})
		}
	}
}

func TestTimeoutQueuedBehindTerminalAction(t *testing.T) {
	for _, terminal := range []string{"victory", "cancel"} {
		t.Run(terminal, func(t *testing.T) {
			svc, _ := game.NewService()
			view, finalAction := readyToFinish(t, svc, -101, uno.BotRules(), false)
			api := newMockBotAPI()
			b := New(api, svc, nil, nil, time.Minute, nil)
			b.SetTurnTimeout(time.Nanosecond)
			defer b.dispatcher.Stop(time.Second)
			candidates := svc.ExpiredTurns(t.Context(), b.turnTimeout)
			if len(candidates) != 1 {
				t.Fatal("missing candidate")
			}
			entered, release := make(chan struct{}), make(chan struct{})
			b.dispatcher.EnqueueChat(view.ChatID, func(context.Context) { close(entered); <-release })
			waitSignal(t, entered)
			// The terminal task is admitted before the stale scheduler candidate.
			var terminalErr error
			b.dispatcher.EnqueueChat(view.ChatID, func(ctx context.Context) {
				if terminal == "cancel" {
					b.cmdHandler.handleCancelar(ctx, view.OwnerID, view.ChatID)
					return
				}
				out, err := svc.Apply(ctx, game.Actor{PlayerID: finalAction.PlayerID, ChatID: view.ChatID}, view.GameID, finalAction)
				if err == nil && out.View.Phase == uno.ChoosingColor {
					out, err = svc.Apply(ctx, game.Actor{PlayerID: finalAction.PlayerID, ChatID: view.ChatID}, view.GameID, uno.Action{Type: uno.ChooseColor, PlayerID: finalAction.PlayerID, Revision: out.View.Revision, Color: uno.Red})
				}
				terminalErr = err
				b.cmdHandler.reply(ctx, int64(view.ChatID), b.renderer.RenderPublicState(out.View), makeGameButtons(out.View))
			})
			if !b.enqueueAutoSkip(candidates[0]) {
				t.Fatal("timeout admission failed")
			}
			close(release)
			flushChat(t, b, view.ChatID)
			if terminalErr != nil {
				t.Fatal(terminalErr)
			}
			api.mu.Lock()
			defer api.mu.Unlock()
			if len(api.SentMessages) != 1 || strings.Contains(api.SentMessages[0].Text, "O tempo acabou") || strings.Contains(api.SentMessages[0].Text, "Vez de") {
				t.Fatalf("late timeout: %+v", api.SentMessages)
			}
		})
	}
}

func TestTimeoutQueueSaturationDoesNotMutate(t *testing.T) {
	svc, _ := game.NewService()
	view := createStartedGame(t, svc, -101, uno.BotRules())
	b := New(newMockBotAPI(), svc, nil, nil, time.Minute, nil)
	b.SetTurnTimeout(time.Nanosecond)
	defer b.dispatcher.Stop(time.Second)
	candidate := svc.ExpiredTurns(t.Context(), time.Nanosecond)[0]
	entered, release := make(chan struct{}), make(chan struct{})
	b.dispatcher.EnqueueChat(view.ChatID, func(context.Context) { close(entered); <-release })
	waitSignal(t, entered)
	for i := 0; i < ChatQueueCapacity; i++ {
		if !b.dispatcher.EnqueueChat(view.ChatID, func(context.Context) {}) {
			t.Fatal("unexpected saturation")
		}
	}
	accepted := b.enqueueAutoSkip(candidate)
	close(release)
	if accepted {
		t.Fatal("saturated scheduler task accepted")
	}
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil || after.Revision != view.Revision || after.CurrentTurn != view.CurrentTurn {
		t.Fatalf("saturation mutated state: %+v %v", after, err)
	}
}

func TestClosedRefreshExplicitlyRemovesKeyboard(t *testing.T) {
	svc, _ := game.NewService()
	view := createStartedGame(t, svc, -101, uno.BotRules())
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	b.renderer.SetBotID(999)
	b.cmdHandler.handleSair(t.Context(), view.CurrentTurn, view.ChatID)
	closed, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil || !closed.Closed {
		t.Fatal("departure did not close", err)
	}
	b.cbHandler.HandleCallback(t.Context(), &telego.CallbackQuery{
		ID: "old_refresh", Data: "view_" + string(view.GameID),
		Message: &telego.Message{MessageID: 1, Chat: telego.Chat{ID: int64(view.ChatID)}},
	})
	if len(api.EditedMessages) != 1 {
		t.Fatal("missing refresh")
	}
	edit := api.EditedMessages[0]
	if edit.ReplyMarkup == nil || len(edit.ReplyMarkup.InlineKeyboard) != 0 || strings.Contains(edit.Text, "Vez de") {
		t.Fatalf("terminal edit: %+v", edit)
	}
	body, err := json.Marshal(edit)
	if err != nil || !strings.Contains(string(body), `"inline_keyboard":[]`) {
		t.Fatalf("keyboard removal not serialized: %s %v", body, err)
	}
}

func TestQueuedActionAfterClosureHasNoContinuation(t *testing.T) {
	for _, history := range []int{0, 100} {
		t.Run(fmt.Sprint(history), func(t *testing.T) {
			svc, _ := game.NewService(game.WithHistoryLimit(history))
			view := createStartedGame(t, svc, -101, uno.BotRules())
			api := newMockBotAPI()
			b := New(api, svc, nil, nil, time.Minute, nil)
			b.renderer.SetBotID(999)
			defer b.dispatcher.Stop(time.Second)
			token := issueToken(t, b, view, uno.Action{Type: uno.DrawCard, PlayerID: view.CurrentTurn, Revision: view.Revision})
			entered, release := make(chan struct{}), make(chan struct{})
			b.dispatcher.EnqueueChat(view.ChatID, func(context.Context) { close(entered); <-release })
			waitSignal(t, entered)
			b.dispatcher.EnqueueChat(view.ChatID, func(ctx context.Context) { b.cmdHandler.handleCancelar(ctx, view.OwnerID, view.ChatID) })
			// Consume while cancellation is queued, just as an arriving inline update does.
			b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: token, From: telego.User{ID: int64(view.CurrentTurn)}})
			close(release)
			flushChat(t, b, view.ChatID)
			api.mu.Lock()
			messages := append([]telego.SendMessageParams(nil), api.SentMessages...)
			api.mu.Unlock()
			if len(messages) != 2 {
				t.Fatalf("messages: %+v", messages)
			}
			rejected := messages[1]
			if rejected.ReplyMarkup != nil || strings.Contains(rejected.Text, "Abra Suas cartas") || strings.Contains(rejected.Text, "Vez de") || strings.Contains(rejected.Text, fmt.Sprintf(`href="tg://user?id=%d"`, view.CurrentTurn)) {
				t.Fatalf("terminal error invites play: %+v", rejected)
			}
			b.inlineHandler.HandleInlineQuery(t.Context(), &telego.InlineQuery{ID: "old", From: telego.User{ID: int64(view.CurrentTurn)}, Query: "g_" + string(view.GameID)})
			article := api.AnsweredInlines[len(api.AnsweredInlines)-1].Results[0].(*telego.InlineQueryResultArticle)
			want := "closed_game"
			if history == 0 {
				want = "unavailable_game"
			}
			if article.ID != want || article.ReplyMarkup != nil {
				t.Fatalf("historical result: %+v", article)
			}
		})
	}
}

func TestInlineContextAndActionBindingAcrossGroups(t *testing.T) {
	svc, _ := game.NewService()
	first := createStartedGame(t, svc, -101, uno.BotRules())
	second := createStartedGame(t, svc, -202, uno.CaseiroRules())
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	b.renderer.SetBotID(999)
	defer b.dispatcher.Stop(time.Second)
	query := func(user uno.PlayerID, q string) telego.AnswerInlineQueryParams {
		b.inlineHandler.HandleInlineQuery(t.Context(), &telego.InlineQuery{ID: "query", From: telego.User{ID: int64(user)}, Query: q})
		return api.AnsweredInlines[len(api.AnsweredInlines)-1]
	}
	selector := query(first.CurrentTurn, "")
	if len(selector.Results) != 2 {
		t.Fatal("missing multigroup selector")
	}
	for _, result := range selector.Results {
		article, ok := result.(*telego.InlineQueryResultArticle)
		if !ok || article.ReplyMarkup == nil {
			t.Fatalf("invalid selector: %T", result)
		}
		context := *article.ReplyMarkup.InlineKeyboard[0][0].SwitchInlineQueryCurrentChat
		if context != "g_"+string(first.GameID) && context != "g_"+string(second.GameID) {
			t.Fatalf("wrong selector context %s", context)
		}
	}
	foreign := query(99, "g_"+string(first.GameID))
	if len(foreign.Results) != 1 || foreign.Results[0].(*telego.InlineQueryResultArticle).ID != "unavailable_game" {
		t.Fatal("outsider obtained hand")
	}
	var drawToken string
	for _, view := range []game.PublicGameView{first, second} {
		answer := query(view.CurrentTurn, "g_"+string(view.GameID))
		if answer.CacheTime != 0 || !answer.IsPersonal {
			t.Fatal("private query cache contract changed")
		}
		for _, result := range answer.Results {
			sticker, ok := result.(*telego.InlineQueryResultCachedSticker)
			if !ok {
				t.Fatalf("expected hand sticker, got %T", result)
			}
			b.tokens.mu.Lock()
			raw := b.tokens.tokens[sticker.ID]
			var actionToken ActionToken
			if token, ok := raw.(*ActionToken); ok {
				actionToken = *token
			}
			b.tokens.mu.Unlock()
			if actionToken.Token == "" {
				continue
			}
			if actionToken.UserID != view.CurrentTurn || actionToken.GameID != view.GameID || actionToken.ChatID != view.ChatID || actionToken.Action.Revision != view.Revision || actionToken.Action.PlayerID != view.CurrentTurn {
				t.Fatalf("bad binding: %+v", actionToken)
			}
			if actionToken.Action.Type == uno.PlayCard && actionToken.Action.CardID == "" {
				t.Fatal("missing card binding")
			}
			if view.GameID == first.GameID && actionToken.Action.Type == uno.DrawCard {
				drawToken = actionToken.Token
			}
		}
	}
	if drawToken == "" {
		t.Fatal("missing draw action")
	}
	// Another user cannot consume the token, even while participating in both games.
	b.inlineHandler.HandleChosenInlineResult(t.Context(), &telego.ChosenInlineResult{ResultID: drawToken, From: telego.User{ID: 11}})
	flushChat(t, b, first.ChatID)
	unchanged, _ := svc.PublicView(t.Context(), first.GameID)
	if unchanged.Revision != first.Revision {
		t.Fatal("cross-user action accepted")
	}
	chosen := &telego.ChosenInlineResult{ResultID: drawToken, From: telego.User{ID: int64(first.CurrentTurn)}, Query: "g_" + string(second.GameID)}
	b.inlineHandler.HandleChosenInlineResult(t.Context(), chosen)
	flushChat(t, b, first.ChatID)
	afterFirst, _ := svc.PublicView(t.Context(), first.GameID)
	afterSecond, _ := svc.PublicView(t.Context(), second.GameID)
	if afterFirst.Revision != first.Revision+1 || afterSecond.Revision != second.Revision {
		t.Fatal("query redirected bound action")
	}
	b.inlineHandler.HandleChosenInlineResult(t.Context(), chosen)
	flushChat(t, b, first.ChatID)
	afterReplay, _ := svc.PublicView(t.Context(), first.GameID)
	if afterReplay.Revision != afterFirst.Revision {
		t.Fatal("one-use action replayed")
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.SentMessages) != 1 || api.SentMessages[0].ChatID.ID != int64(first.ChatID) {
		t.Fatalf("wrong destination: %+v", api.SentMessages)
	}
}

func TestTimeoutAdvancesOnceAndMentionsResultingPlayer(t *testing.T) {
	svc, _ := game.NewService()
	view := createStartedGame(t, svc, -101, uno.BotRules())
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	b.SetTurnTimeout(time.Nanosecond)
	b.renderer.SetBotID(999)
	candidate := svc.ExpiredTurns(t.Context(), time.Nanosecond)[0]
	if !b.enqueueAutoSkip(candidate) || !b.enqueueAutoSkip(candidate) {
		t.Fatal("could not enqueue timeout")
	}
	flushChat(t, b, view.ChatID)
	after, err := svc.PublicView(t.Context(), view.GameID)
	if err != nil || after.Revision != view.Revision+1 || after.CurrentTurn == view.CurrentTurn {
		t.Fatalf("timeout state: %+v %v", after, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.SentMessages) != 1 {
		t.Fatalf("duplicate timeout notification: %+v", api.SentMessages)
	}
	msg := api.SentMessages[0]
	if !strings.Contains(msg.Text, "O tempo acabou") || msg.ReplyMarkup == nil {
		t.Fatalf("timeout notification: %+v", msg)
	}
	assertMentionTargets(t, msg.Text, b.renderer.userCache, after.CurrentTurn, 999)
}

func TestInlineHandCursorCannotCrossContext(t *testing.T) {
	svc, _ := game.NewService()
	first := createStartedGame(t, svc, -101, uno.BotRules())
	second := createStartedGame(t, svc, -202, uno.BotRules())
	api := newMockBotAPI()
	b := New(api, svc, nil, nil, time.Minute, nil)
	defer b.dispatcher.Stop(time.Second)
	for _, tc := range []struct {
		name      string
		user      uno.PlayerID
		id        uno.GameID
		revision  uint64
		wantCards int
	}{
		{"valid", first.CurrentTurn, first.GameID, first.Revision, 6},
		{"other_game", first.CurrentTurn, second.GameID, first.Revision, 7},
		{"old_revision", first.CurrentTurn, first.GameID, first.Revision - 1, 7},
		{"other_user", 11, first.GameID, first.Revision, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cursor, err := b.tokens.CreateCursorToken(tc.user, CursorKindHand, tc.id, tc.revision, 1, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			b.inlineHandler.HandleInlineQuery(t.Context(), &telego.InlineQuery{ID: tc.name, From: telego.User{ID: int64(first.CurrentTurn)}, Query: "g_" + string(first.GameID), Offset: cursor})
			results := api.AnsweredInlines[len(api.AnsweredInlines)-1].Results
			cards := 0
			for _, result := range results {
				sticker := result.(*telego.InlineQueryResultCachedSticker)
				if sticker.StickerFileID != Stickers["option_draw"] && sticker.StickerFileID != Stickers["option_info"] {
					cards++
				}
			}
			if cards != tc.wantCards {
				t.Fatalf("cursor returned %d hand cards, want %d", cards, tc.wantCards)
			}
		})
	}
}
