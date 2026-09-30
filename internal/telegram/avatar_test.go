package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func TestAvatarSource_ErrorRedaction(t *testing.T) {
	// Dummy bot with dummy token (format: digits:35chars)
	bot, err := telego.NewBot("123456:abcdefghijklmnopqrstuvwxyz012345678", telego.WithDiscardLogger())
	if err != nil {
		t.Fatalf("failed to create dummy bot: %v", err)
	}

	src := AvatarSource{Bot: bot}
	ctx := context.Background()

	// 1. Group photo on non-existent chat
	_, _, err = src.Photo(ctx, "group", -1009999999999)
	if err == nil {
		t.Fatal("expected error for non-existent chat")
	}
	if !errors.Is(err, errPhoto) {
		t.Fatalf("expected errPhoto, got: %v", err)
	}
	if strings.Contains(err.Error(), "123456:ABC") || strings.Contains(err.Error(), "http") {
		t.Fatalf("error leaked token or url: %v", err)
	}

	// 2. User photo on non-existent user
	_, _, err = src.Photo(ctx, "user", 999999999)
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
	if !errors.Is(err, errPhoto) {
		t.Fatalf("expected errPhoto, got: %v", err)
	}
	if strings.Contains(err.Error(), "123456:ABC") || strings.Contains(err.Error(), "http") {
		t.Fatalf("error leaked token or url: %v", err)
	}
}
