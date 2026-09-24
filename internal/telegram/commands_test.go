package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/malbs/UnoGoBot/internal/uno"
	"github.com/mymmrac/telego"
)

func TestCommandHandler_NovoEntrarIniciarCancelar(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, err := game.NewService()
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	cmdHandler := NewCommandHandler(mockAPI, svc, renderer, tokens, "unobot", nil)

	ctx := context.Background()
	chatID := int64(-100100)

	// 1. /novo by Alice (ID 1)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 1, FirstName: "Alice", Username: "alice"},
		Text: "/novo",
	})

	lastMsg := mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Partida de UNO") || !strings.Contains(lastMsg, "Alice") {
		t.Fatalf("expected lobby message, got: %s", lastMsg)
	}

	// Verify Alice is owner but observer (0 players in lobby)
	summary, err := svc.FindChatGame(ctx, game.ChatID(chatID))
	if err != nil {
		t.Fatalf("expected game to exist: %v", err)
	}
	view, _ := svc.PublicView(ctx, summary.GameID)
	if view.OwnerID != 1 {
		t.Errorf("expected Alice (1) to be owner, got %d", view.OwnerID)
	}
	if len(view.Players) != 0 {
		t.Errorf("expected 0 players in lobby, got %d", len(view.Players))
	}

	// 2. /entrar by Bob (ID 2)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 2, FirstName: "Bob", Username: "bob"},
		Text: "/entrar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Bob") || !strings.Contains(lastMsg, "1/10") {
		t.Fatalf("expected Bob in lobby, got: %s", lastMsg)
	}

	// 3. /cancelar attempted by Bob (ID 2) -> should be forbidden because Alice (1) is owner!
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 2, FirstName: "Bob", Username: "bob"},
		Text: "/cancelar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Apenas o responsável") {
		t.Fatalf("expected forbidden error for non-owner cancel, got: %s", lastMsg)
	}

	// 4. /iniciar attempted by Bob with only 1 player -> should fail (not enough players, not forbidden!)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 2, FirstName: "Bob", Username: "bob"},
		Text: "/iniciar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "pelo menos 2 jogadores") {
		t.Fatalf("expected not enough players error, got: %s", lastMsg)
	}

	// 5. /entrar by Carol (ID 3)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 3, FirstName: "Carol", Username: "carol"},
		Text: "/entrar",
	})

	// 6. /iniciar by Carol (ID 3) — any player can start once lobby has >= 2 players!
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 3, FirstName: "Carol", Username: "carol"},
		Text: "/iniciar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Partida iniciada") {
		t.Fatalf("expected game started message, got: %s", lastMsg)
	}
	if len(mockAPI.SentStickers) == 0 {
		t.Fatalf("expected initial top card sticker to be sent on /iniciar")
	}

	// Verify reply markup on the started message has only "🃏 Suas cartas" button (no "Atualizar estado")
	lastSentParams := mockAPI.SentMessages[len(mockAPI.SentMessages)-1]
	markup, ok := lastSentParams.ReplyMarkup.(*telego.InlineKeyboardMarkup)
	if !ok || markup == nil || len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 1 {
		t.Fatalf("expected exactly 1 button in game keyboard, got: %#v", lastSentParams.ReplyMarkup)
	}
	if markup.InlineKeyboard[0][0].Text != "🃏 Suas cartas" {
		t.Fatalf("expected '🃏 Suas cartas' button text, got: %s", markup.InlineKeyboard[0][0].Text)
	}

	// Verify game is playing with numbered start (Rank < Skip) and both have 7 cards
	view2, _ := svc.PublicView(ctx, summary.GameID)
	if view2.Phase != uno.TakingTurn {
		t.Fatalf("expected TakingTurn phase, got %v", view2.Phase)
	}
	if view2.TopCard.Rank >= uno.Skip {
		t.Fatalf("expected numbered top card (< Skip), got %v", view2.TopCard.Rank)
	}
	for _, p := range view2.Players {
		if p.CardCount != 7 {
			t.Fatalf("expected player %d to have 7 cards, got %d", p.ID, p.CardCount)
		}
	}

	// 7. /estado
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 2, FirstName: "Bob", Username: "bob"},
		Text: "/estado",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Carta no topo") {
		t.Fatalf("expected state message, got: %s", lastMsg)
	}

	// 8. /cancelar by Alice (owner)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 1, FirstName: "Alice", Username: "alice"},
		Text: "/cancelar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Partida cancelada pelo responsável") {
		t.Fatalf("expected cancel message, got: %s", lastMsg)
	}

	// Verify game is closed
	viewAfter, err := svc.PublicView(ctx, summary.GameID)
	if err != nil {
		t.Fatalf("unexpected error getting public view: %v", err)
	}
	if !viewAfter.Closed {
		t.Errorf("expected game to be closed")
	}

	// Verify chat no longer has an active game
	_, errChat := svc.FindChatGame(ctx, game.ChatID(chatID))
	if !errors.Is(errChat, game.ErrNoActiveGame) {
		t.Errorf("expected ErrNoActiveGame, got %v", errChat)
	}
}

func TestCommandHandler_FiltersAndAliases(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	cmdHandler := NewCommandHandler(mockAPI, svc, renderer, tokens, "unobot", nil)
	ctx := context.Background()

	// 1. Command addressed to another bot: /novo@otherbot -> ignored!
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: -1001, Type: "supergroup"},
		From: &telego.User{ID: 1, FirstName: "User"},
		Text: "/novo@otherbot",
	})
	if len(mockAPI.SentMessages) != 0 {
		t.Fatalf("expected no response to command addressed to other bot")
	}

	// 2. Command from a bot: ignored!
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: -1001, Type: "supergroup"},
		From: &telego.User{ID: 2, FirstName: "BotUser", IsBot: true},
		Text: "/novo",
	})
	if len(mockAPI.SentMessages) != 0 {
		t.Fatalf("expected no response to bot sender")
	}

	// 3. Command as channel / sender chat -> rejected
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat:       telego.Chat{ID: -1001, Type: "supergroup"},
		From:       &telego.User{ID: 1, FirstName: "User"},
		SenderChat: &telego.Chat{ID: -10099, Type: "channel"},
		Text:       "/novo",
	})
	if !strings.Contains(mockAPI.LastSentMessage(), "não como canal") {
		t.Fatalf("expected sender chat warning, got: %s", mockAPI.LastSentMessage())
	}

	// 4. Forum topic message -> rejected
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat:            telego.Chat{ID: -1001, Type: "supergroup"},
		From:            &telego.User{ID: 1, FirstName: "User"},
		IsTopicMessage:  true,
		MessageThreadID: 42,
		Text:            "/novo",
	})
	if !strings.Contains(mockAPI.LastSentMessage(), "Tópicos de fórum ainda não são suportados") {
		t.Fatalf("expected forum topic warning, got: %s", mockAPI.LastSentMessage())
	}

	// 5. Private chat /ajuda vs /novo
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: 100, Type: "private"},
		From: &telego.User{ID: 100, FirstName: "User"},
		Text: "/novo",
	})
	if !strings.Contains(mockAPI.LastSentMessage(), "Este comando só pode ser utilizado em grupos") {
		t.Fatalf("expected private chat group warning, got: %s", mockAPI.LastSentMessage())
	}

	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: 100, Type: "private"},
		From: &telego.User{ID: 100, FirstName: "User"},
		Text: "/ajuda",
	})
	if !strings.Contains(mockAPI.LastSentMessage(), "Como Jogar") {
		t.Fatalf("expected help text in private chat, got: %s", mockAPI.LastSentMessage())
	}

	// 6. /kill alias in group
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: -1002, Type: "supergroup"},
		From: &telego.User{ID: 1, FirstName: "User"},
		Text: "/novo",
	})
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: -1002, Type: "supergroup"},
		From: &telego.User{ID: 1, FirstName: "User"},
		Text: "/kill",
	})
	if !strings.Contains(mockAPI.LastSentMessage(), "Partida cancelada") {
		t.Fatalf("expected /kill to act as cancel alias, got: %s", mockAPI.LastSentMessage())
	}

	// 7. Case-insensitive bot username command: /novo@UNOBOT works with unobot
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: -1003, Type: "supergroup", Title: "Test Group"},
		From: &telego.User{ID: 10, FirstName: "Creator"},
		Text: "/novo@UNOBOT",
	})
	if !strings.Contains(mockAPI.LastSentMessage(), "Partida de UNO") {
		t.Fatalf("expected /novo@UNOBOT to work, got: %s", mockAPI.LastSentMessage())
	}
	// Verify creator is NOT auto-enrolled (0 players in lobby)
	gSummary, err := svc.FindChatGame(ctx, game.ChatID(-1003))
	if err != nil {
		t.Fatalf("expected game created at -1003: %v", err)
	}
	gView, _ := svc.PublicView(ctx, gSummary.GameID)
	if len(gView.Players) != 0 {
		t.Fatalf("expected 0 players in lobby, got %d", len(gView.Players))
	}
}

func TestCommandHandler_ReplyMarkup_OmitsNull(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, err := game.NewService()
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	renderer := NewRenderer(NewUserCache(100))
	tokens := NewTokenStore(1000, 100, time.Now, nil)
	cmdHandler := NewCommandHandler(mockAPI, svc, renderer, tokens, "unobot", nil)

	ctx := context.Background()
	// Trigger a message without markup (e.g., /ajuda)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: 100, Type: "private"},
		From: &telego.User{ID: 100, FirstName: "User"},
		Text: "/ajuda",
	})

	if len(mockAPI.SentMessages) == 0 {
		t.Fatalf("expected a message to be sent")
	}

	lastParams := mockAPI.SentMessages[len(mockAPI.SentMessages)-1]
	if lastParams.ReplyMarkup != nil {
		t.Errorf("expected ReplyMarkup to be nil, got: %#v", lastParams.ReplyMarkup)
	}

	data, err := json.Marshal(lastParams)
	if err != nil {
		t.Fatalf("failed to marshal params to JSON: %v", err)
	}

	if strings.Contains(string(data), "reply_markup") {
		t.Errorf("expected JSON to omit reply_markup, but got: %s", string(data))
	}
}

func TestCommandHandler_ResetByOwnerInvalidatesStateAndTokens(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	tokens := NewTokenStore(100, 10, time.Now, nil)
	handler := NewCommandHandler(mockAPI, svc, NewRenderer(NewUserCache(100)), tokens, "unobot", nil)
	ctx := context.Background()
	chatID := int64(-2001)
	owner := int64(10)

	handler.HandleMessage(ctx, &telego.Message{Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "Reset"}, From: &telego.User{ID: owner}, Text: "/novo"})
	summary, err := svc.FindChatGame(ctx, game.ChatID(chatID))
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.CreateActionToken(uno.PlayerID(owner), summary.GameID, game.ChatID(chatID), uno.Action{PlayerID: uno.PlayerID(owner)}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler.HandleMessage(ctx, &telego.Message{Chat: telego.Chat{ID: chatID, Type: "supergroup"}, From: &telego.User{ID: owner}, Text: "/reset@UNOBOT"})
	if !strings.Contains(mockAPI.LastSentMessage(), "Estado deste grupo resetado") {
		t.Fatalf("unexpected reset reply: %s", mockAPI.LastSentMessage())
	}
	if _, err := svc.FindChatGame(ctx, game.ChatID(chatID)); !errors.Is(err, game.ErrNoActiveGame) {
		t.Fatalf("game survived reset: %v", err)
	}
	if _, status := tokens.ConsumeAction(token, uno.PlayerID(owner)); status != ConsumeNotFound {
		t.Fatalf("old token survived reset: %v", status)
	}
	handler.HandleMessage(ctx, &telego.Message{Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "Reset"}, From: &telego.User{ID: owner}, Text: "/novo"})
	if _, err := svc.FindChatGame(ctx, game.ChatID(chatID)); err != nil {
		t.Fatalf("new game failed after reset: %v", err)
	}
}

func TestCommandHandler_ResetByAdminAndRejectMember(t *testing.T) {
	for _, test := range []struct {
		name    string
		member  telego.ChatMember
		allowed bool
	}{
		{"creator", &telego.ChatMemberOwner{Status: telego.MemberStatusCreator}, true},
		{"administrator", &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}, true},
		{"member", &telego.ChatMemberMember{Status: telego.MemberStatusMember}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			mockAPI := newMockBotAPI()
			mockAPI.ChatMembers[20] = test.member
			svc, _ := game.NewService()
			handler := NewCommandHandler(mockAPI, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
			ctx := context.Background()
			chatID := int64(-2100)
			handler.HandleMessage(ctx, &telego.Message{Chat: telego.Chat{ID: chatID, Type: "supergroup"}, From: &telego.User{ID: 10}, Text: "/novo"})
			handler.HandleMessage(ctx, &telego.Message{Chat: telego.Chat{ID: chatID, Type: "supergroup"}, From: &telego.User{ID: 20}, Text: "/reset"})
			_, err := svc.FindChatGame(ctx, game.ChatID(chatID))
			if test.allowed && !errors.Is(err, game.ErrNoActiveGame) {
				t.Fatalf("authorized reset left game: %v", err)
			}
			if !test.allowed && err != nil {
				t.Fatalf("denied reset changed game: %v", err)
			}
			if !test.allowed && !strings.Contains(mockAPI.LastSentMessage(), "Apenas o responsável") {
				t.Fatalf("unexpected denial: %s", mockAPI.LastSentMessage())
			}
		})
	}
}

func TestCommandHandler_AdminResetWithoutGameAndRoleFailure(t *testing.T) {
	mockAPI := newMockBotAPI()
	mockAPI.ChatMembers[20] = &telego.ChatMemberAdministrator{Status: telego.MemberStatusAdministrator}
	svc, _ := game.NewService()
	handler := NewCommandHandler(mockAPI, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
	msg := &telego.Message{Chat: telego.Chat{ID: -2200, Type: "supergroup"}, From: &telego.User{ID: 20}, Text: "/reset"}
	handler.HandleMessage(context.Background(), msg)
	if !strings.Contains(mockAPI.LastSentMessage(), "fila deste grupo foi renovada") {
		t.Fatalf("unexpected empty reset reply: %s", mockAPI.LastSentMessage())
	}

	mockAPI.ChatMemberErr = errors.New("telegram unavailable")
	msg.Chat.ID = -2201
	handler.HandleMessage(context.Background(), msg)
	if !strings.Contains(mockAPI.LastSentMessage(), "confirmar sua permissão") {
		t.Fatalf("unexpected role failure reply: %s", mockAPI.LastSentMessage())
	}
}

func TestCommandHandler_ResetRejectsUnsupportedSendersAndChats(t *testing.T) {
	mockAPI := newMockBotAPI()
	svc, _ := game.NewService()
	handler := NewCommandHandler(mockAPI, svc, NewRenderer(NewUserCache(100)), NewTokenStore(100, 10, time.Now, nil), "unobot", nil)
	ctx := context.Background()

	handler.HandleReset(ctx, &telego.Message{
		Chat: telego.Chat{ID: -2300, Type: "supergroup"}, From: &telego.User{ID: 1},
		SenderChat: &telego.Chat{ID: -2300, Type: "supergroup"}, Text: "/reset",
	}, nil)
	if !strings.Contains(mockAPI.LastSentMessage(), "administrador identificável") {
		t.Fatalf("anonymous reset was not rejected: %s", mockAPI.LastSentMessage())
	}
	handler.HandleReset(ctx, &telego.Message{
		Chat: telego.Chat{ID: -2300, Type: "supergroup"}, From: &telego.User{ID: 1},
		IsTopicMessage: true, MessageThreadID: 7, Text: "/reset",
	}, nil)
	if !strings.Contains(mockAPI.LastSentMessage(), "chat geral") {
		t.Fatalf("topic reset was not rejected: %s", mockAPI.LastSentMessage())
	}
	handler.HandleReset(ctx, &telego.Message{
		Chat: telego.Chat{ID: 1, Type: "private"}, From: &telego.User{ID: 1}, Text: "/reset",
	}, nil)
	if !strings.Contains(mockAPI.LastSentMessage(), "só pode ser utilizado em grupos") {
		t.Fatalf("private reset was not rejected: %s", mockAPI.LastSentMessage())
	}
}
