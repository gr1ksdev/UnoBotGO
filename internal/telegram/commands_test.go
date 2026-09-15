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

	// 3. /iniciar attempted by Bob (ID 2) -> should be forbidden because Alice (1) is owner!
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 2, FirstName: "Bob", Username: "bob"},
		Text: "/iniciar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Apenas o responsável") {
		t.Fatalf("expected forbidden error for non-owner start, got: %s", lastMsg)
	}

	// 4. /iniciar attempted by Alice with only 1 player (Bob) -> should fail (not enough players)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 1, FirstName: "Alice", Username: "alice"},
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

	// 6. /iniciar by Alice (ID 1)
	cmdHandler.HandleMessage(ctx, &telego.Message{
		Chat: telego.Chat{ID: chatID, Type: "supergroup", Title: "UNO Fun"},
		From: &telego.User{ID: 1, FirstName: "Alice", Username: "alice"},
		Text: "/iniciar",
	})
	lastMsg = mockAPI.LastSentMessage()
	if !strings.Contains(lastMsg, "Partida iniciada") {
		t.Fatalf("expected game started message, got: %s", lastMsg)
	}

	// Verify game is playing
	view2, _ := svc.PublicView(ctx, summary.GameID)
	if view2.Phase != uno.TakingTurn {
		t.Fatalf("expected phase TakingTurn, got %v", view2.Phase)
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
}
