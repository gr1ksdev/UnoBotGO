package telegram

import (
	"context"
	"net/http"

	"github.com/mymmrac/telego"
)

// UseSharedHTTP prevents the webhook transport from creating a second listener.
func (b *Bot) UseSharedHTTP(onReady func()) { b.externalHTTP = true; b.onReady = onReady }
func (b *Bot) WebhookHandler() http.Handler { return b.webhookHandler(b.transport.WebhookSecret) }
func (b *Bot) runSharedWebhook(ctx context.Context, cfg TransportConfig) error {
	if err := b.api.SetWebhook(ctx, &telego.SetWebhookParams{URL: cfg.WebhookURL, SecretToken: cfg.WebhookSecret, AllowedUpdates: allowedUpdates, DropPendingUpdates: false}); err != nil {
		return err
	}
	b.startScheduler(ctx)
	if b.onReady != nil {
		b.onReady()
	}
	<-ctx.Done()
	return nil
}
