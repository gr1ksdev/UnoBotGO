package telegram

import "strings"

const defaultWebhookListenAddr = ":8080"

type TransportMode string

const (
	TransportPolling TransportMode = "polling"
	TransportWebhook TransportMode = "webhook"
)

type TransportConfig struct {
	Mode               TransportMode
	WebhookURL         string
	WebhookSecret      string
	ListenAddr         string
	DropPendingUpdates bool
}

func (c TransportConfig) normalized() TransportConfig {
	if c.Mode == "" {
		c.Mode = TransportPolling
	}
	c.Mode = TransportMode(strings.ToLower(string(c.Mode)))
	if c.ListenAddr == "" {
		c.ListenAddr = defaultWebhookListenAddr
	}
	return c
}
