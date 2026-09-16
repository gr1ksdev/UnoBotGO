package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/malbs/UnoGoBot/internal/game"
	"github.com/mymmrac/telego"
)

func TestWebhookHandlerSecurityAndDecode(t *testing.T) {
	svc, _ := game.NewService()
	b := New(newMockBotAPI(), svc, nil, nil, time.Minute, nil)
	h := b.webhookHandler("secret")
	for _, tc := range []struct {
		name, method, secret, body string
		want                       int
	}{
		{"method", "GET", "secret", "{}", http.StatusMethodNotAllowed},
		{"secret", "POST", "wrong", "{}", http.StatusForbidden},
		{"json", "POST", "secret", "nope", http.StatusBadRequest},
		{"valid", "POST", "secret", `{"update_id":42}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/", strings.NewReader(tc.body))
			req.Header.Set(telego.WebhookSecretTokenHeader, tc.secret)
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("status=%d want %d", rr.Code, tc.want)
			}
		})
	}
	_ = json.Valid
}

func TestUpdateDeduper(t *testing.T) {
	now := time.Unix(100, 0)
	d := newUpdateDeduper(func() time.Time { return now })
	if !d.reserve(1) || d.reserve(1) {
		t.Fatal("duplicate should be rejected")
	}
	d.commit(1)
	if d.reserve(1) {
		t.Fatal("committed duplicate should be rejected")
	}
	if !d.reserve(2) {
		t.Fatal("new update rejected")
	}
	d.release(2)
	if !d.reserve(2) {
		t.Fatal("released update should be accepted")
	}
}

func TestWebhookStartupAlwaysSetsWebhook(t *testing.T) {
	api := newMockBotAPI()
	svc, _ := game.NewService()
	b := New(api, svc, nil, nil, time.Minute, nil)
	b.SetTransport(TransportConfig{Mode: TransportWebhook, WebhookURL: "https://example.com/telegram", WebhookSecret: "secret", ListenAddr: "127.0.0.1:0"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Run(ctx) }()
	deadline := time.After(2 * time.Second)
	for {
		api.mu.Lock()
		n := len(api.SetWebhookCalls)
		api.mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("SetWebhook not called")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if api.SetWebhookCalls[0].SecretToken != "secret" || api.SetWebhookCalls[0].DropPendingUpdates {
		t.Fatalf("unexpected params: %#v", api.SetWebhookCalls[0])
	}
}
