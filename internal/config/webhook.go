package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// This versioned protocol context separates the webhook authenticator from
// AES-GCM references, cursor signing and every other use of MINIAPP_SECRET.
const webhookSecretContext = "unobotgo/v2/telegram-webhook-secret/v1"

// DeriveWebhookSecret returns a 43-character [A-Za-z0-9_-] authenticator,
// compatible with Telegram secret_token. The master key is never sent to Telegram.
// Rotating MINIAPP_SECRET rotates this value; startup setWebhook installs it.
func DeriveWebhookSecret(master []byte) (string, error) {
	if len(master) != 32 {
		return "", ErrInvalidMiniAppSecret
	}
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte(webhookSecretContext))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
