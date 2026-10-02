package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/config"
	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

func TestSharedWebhookUsesDerivedSecretAndPreservesPendingUpdates(t *testing.T) {
	master := bytes.Repeat([]byte{42}, 32)
	secret, err := config.DeriveWebhookSecret(master)
	if err != nil {
		t.Fatal(err)
	}
	api := newMockBotAPI()
	svc, _ := game.NewService()
	b := New(api, svc, nil, nil, time.Minute, nil)
	b.SetTransport(TransportConfig{Mode: TransportWebhook, WebhookURL: "https://example.com/telegram", WebhookSecret: secret})
	handler := b.WebhookHandler()
	// Starting twice must re-register, including after secret rotation.
	for _, key := range [][]byte{master, bytes.Repeat([]byte{43}, 32)} {
		derived, _ := config.DeriveWebhookSecret(key)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := b.runSharedWebhook(ctx, TransportConfig{WebhookURL: "https://example.com/telegram", WebhookSecret: derived}); err != nil {
			t.Fatal(err)
		}
	}
	api.mu.Lock()
	calls := append([]telego.SetWebhookParams(nil), api.SetWebhookCalls...)
	api.mu.Unlock()
	if len(calls) != 2 || calls[0].SecretToken != secret || calls[0].SecretToken == calls[1].SecretToken {
		t.Fatal("secret not installed/rotated")
	}
	for _, call := range calls {
		if call.DropPendingUpdates || call.URL != "https://example.com/telegram" || len(call.AllowedUpdates) == 0 {
			t.Fatal("webhook behavior changed")
		}
	}
	for _, tc := range []struct {
		header string
		status int
	}{
		{"", http.StatusForbidden}, {"wrong", http.StatusForbidden},
		{base64.StdEncoding.EncodeToString(master), http.StatusForbidden},
		{calls[1].SecretToken, http.StatusForbidden}, {secret, http.StatusOK},
	} {
		req := httptest.NewRequest(http.MethodPost, "/telegram", strings.NewReader(`{"update_id":42}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(telego.WebhookSecretTokenHeader, tc.header)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.status {
			t.Fatalf("got %d want %d", response.Code, tc.status)
		}
	}
	b.dispatcher.Stop(time.Second)
}
