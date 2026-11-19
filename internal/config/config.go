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
	ErrMissingDatabaseURL  = errors.New("config: DATABASE_URL is required")
	ErrMissingToken        = errors.New("config: TOKEN environment variable is required")
	ErrInvalidToken        = errors.New("config: invalid Telegram bot token format")
	ErrInvalidTurnTimeout  = errors.New("config: TURN_TIMEOUT must be a positive duration")
	ErrInvalidSkipPenalty  = errors.New("config: SKIP_TIME_PENALTY must be a positive duration")
	ErrInvalidMinTurnTime  = errors.New("config: MIN_TURN_TIME must be a positive duration")
	ErrInvalidTelegramMode = errors.New("config: TELEGRAM_MODE must be polling or webhook")
	ErrMissingWebhookURL   = errors.New("config: WEBHOOK_URL is required in webhook mode")
	ErrInvalidWebhookURL   = errors.New("config: WEBHOOK_URL must be an HTTPS URL with a host and a dedicated path, e.g. /telegram")
	ErrInvalidSecretKey    = errors.New("config: SECRET_KEY must be base64 encoding of exactly 32 random bytes")
	ErrMissingSecretKey    = errors.New("config: SECRET_KEY is required in webhook mode")
	tokenRegex             = regexp.MustCompile(`^[0-9]{3,}:[a-zA-Z0-9_-]{10,}$`)
)

type Config struct {
	Token           string
	TelegramMode    string
	DatabaseURL     string
	TurnTimeout     time.Duration
	SkipTimePenalty time.Duration
	MinTurnTime     time.Duration
	WebAddr         string
	SecretKey       []byte // Required only when TelegramMode is webhook.
	WebhookURL      string // Required only when TelegramMode is webhook.
}

// Load reads .env without overriding existing environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) (*Config, error) {
	value := func(name string) string { v, _ := lookup(name); return strings.TrimSpace(v) }
	cfg := &Config{Token: value("TOKEN"), DatabaseURL: value("DATABASE_URL"), TelegramMode: "polling", TurnTimeout: 2 * time.Minute, SkipTimePenalty: 20 * time.Second, MinTurnTime: 15 * time.Second, WebAddr: ":8080"}
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
	if v := value("SKIP_TIME_PENALTY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, ErrInvalidSkipPenalty
		}
		cfg.SkipTimePenalty = d
	}
	if v := value("MIN_TURN_TIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, ErrInvalidMinTurnTime
		}
		cfg.MinTurnTime = d
	}
	if v := value("WEB_ADDR"); v != "" {
		cfg.WebAddr = v
	}
	if cfg.TelegramMode == "webhook" {
		// Preserve the existing Base64 format; do not accept larger keys or
		// alternate encodings. The key is only needed by the webhook.
		raw, _ := lookup("SECRET_KEY")
		if strings.TrimSpace(raw) == "" {
			return nil, ErrMissingSecretKey
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return nil, ErrInvalidSecretKey
		}
		cfg.SecretKey = key
		cfg.WebhookURL = value("WEBHOOK_URL")
		if cfg.WebhookURL == "" {
			return nil, ErrMissingWebhookURL
		}
		u, err := url.Parse(cfg.WebhookURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, ErrInvalidWebhookURL
		}
		path := u.Path
		if path == "" || path == "/" || path == "/healthz" || path == "/readyz" {
			return nil, ErrInvalidWebhookURL
		}
	}
	return cfg, nil
}
