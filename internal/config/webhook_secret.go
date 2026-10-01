package config

// WebhookSecret uses the shared protocol derivation. Runtime configuration
// validates the master key before this convenience method is called.
func (c *Config) WebhookSecret() string {
	secret, _ := DeriveWebhookSecret(c.MiniAppSecret)
	return secret
}
