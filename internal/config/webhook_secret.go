package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// This protocol context is distinct from reference/v1 and key/v1 AES/ID derivations.
const webhookSecretContext = "unobotgo/telegram/webhook-secret/v1"

// WebhookSecret derives a protocol-specific token without exposing the master key.
// Raw URL-safe Base64 satisfies Telegram secret_token: 1..256 [A-Za-z0-9_-].
func (c *Config) WebhookSecret() string {
	mac := hmac.New(sha256.New, c.MiniAppSecret)
	mac.Write([]byte(webhookSecretContext))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
