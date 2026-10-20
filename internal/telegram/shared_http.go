package telegram

import (
	"context"
	"net/http"
)

// UseSharedHTTP prevents the webhook transport from creating a second listener.
func (b *Bot) UseSharedHTTP(onReady func()) { b.externalHTTP = true; b.onReady = onReady }
func (b *Bot) WebhookHandler() http.Handler { return b.webhookHandler(b.transport.WebhookSecret) }
func (b *Bot) runSharedWebhook(ctx context.Context, cfg TransportConfig) error {
	if err := b.registerWebhook(ctx, cfg); err != nil {
		return err
	}
	b.startScheduler(ctx)
	b.startWebhookMonitor(ctx)
	if b.onReady != nil {
		b.onReady()
	}
	<-ctx.Done()
	return nil
}
