// Package config owns external installation settings and fixed runtime policies.
package config

import (
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	ErrMissingDatabaseURL   = errors.New("config: DATABASE_URL is required")
	ErrMissingToken         = errors.New("config: TOKEN environment variable is required")
	ErrInvalidToken         = errors.New("config: invalid Telegram bot token format")
	ErrInvalidTurnTimeout   = errors.New("config: TURN_TIMEOUT must be a positive duration")
	ErrInvalidTelegramMode  = errors.New("config: TELEGRAM_MODE must be polling or webhook")
	ErrMissingWebhookURL    = errors.New("config: WEBHOOK_URL is required in webhook mode")
	ErrInvalidWebhookURL    = errors.New("config: WEBHOOK_URL must be an HTTPS URL with a host and a dedicated path, e.g. /telegram")
	ErrInvalidMiniAppSecret = errors.New("config: MINIAPP_SECRET must be base64 encoding of exactly 32 random bytes")
	tokenRegex              = regexp.MustCompile(`^[0-9]{3,}:[a-zA-Z0-9_-]{10,}$`)
)

type Config struct {
	Token         string
	TelegramMode  string
	DatabaseURL   string
	TurnTimeout   time.Duration
	WebAddr       string
	MiniAppSecret []byte
	WebhookURL    string // Required only when TelegramMode is webhook.
}

// Load reads .env without overriding existing environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) (*Config, error) {
	value := func(name string) string { v, _ := lookup(name); return strings.TrimSpace(v) }
	cfg := &Config{Token: value("TOKEN"), DatabaseURL: value("DATABASE_URL"), TelegramMode: "polling", TurnTimeout: 2 * time.Minute, WebAddr: ":8080"}
	if cfg.Token == "" {
		return nil, ErrMissingToken
	}
	if !tokenRegex.MatchString(cfg.Token) {
		return nil, ErrInvalidToken
	}
	if cfg.DatabaseURL == "" {
		return nil, ErrMissingDatabaseURL
	}
	if v := value("TELEGRAM_MODE"); v != "" {
		cfg.TelegramMode = strings.ToLower(v)
	}
	if cfg.TelegramMode != "polling" && cfg.TelegramMode != "webhook" {
		return nil, ErrInvalidTelegramMode
	}
	if v := value("TURN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, ErrInvalidTurnTimeout
		}
		cfg.TurnTimeout = d
	}
	if v := value("WEB_ADDR"); v != "" {
		cfg.WebAddr = v
	}
	// Preserve the existing Base64 format; do not accept larger keys or alternate encodings.
	raw, _ := lookup("MINIAPP_SECRET")
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidMiniAppSecret
	}
	cfg.MiniAppSecret = key
	if cfg.TelegramMode == "webhook" {
		cfg.WebhookURL = value("WEBHOOK_URL")
		if cfg.WebhookURL == "" {
			return nil, ErrMissingWebhookURL
		}
		u, err := url.Parse(cfg.WebhookURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return nil, ErrInvalidWebhookURL
		}
		path := u.Path
		if path == "" || path == "/" || strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/assets") || path == "/healthz" || path == "/readyz" {
			return nil, ErrInvalidWebhookURL
		}
	}
	return cfg, nil
}
