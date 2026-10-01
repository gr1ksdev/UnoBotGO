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
	ErrInvalidTurnTimeout   = errors.New("config: TURN_TIMEOUT must be a valid positive duration")
	ErrInvalidTelegramMode  = errors.New("config: TELEGRAM_MODE must be polling or webhook")
	ErrMissingWebhookURL    = errors.New("config: WEBHOOK_URL is required in webhook mode")
	ErrInvalidWebhookURL    = errors.New("config: WEBHOOK_URL must be an HTTPS URL with a host and dedicated path, e.g. /telegram")
	ErrInvalidMiniAppSecret = errors.New("config: MINIAPP_SECRET must be base64 encoding of exactly 32 random bytes")
	tokenRegex              = regexp.MustCompile(`^[0-9]{3,}:[a-zA-Z0-9_-]{10,}$`)
)

// Config contains only installation settings. Application policies live in defaults.go.
type Config struct {
	Token         string
	TelegramMode  string
	DatabaseURL   string
	TurnTimeout   time.Duration
	WebAddr       string
	MiniAppSecret []byte
	WebhookURL    string // Only used in webhook mode; the public endpoint is not discoverable.
}

// Load preserves exported environment values while optionally reading .env.
func Load() (*Config, error) {
	_ = godotenv.Load()
	return LoadFromLookup(os.LookupEnv)
}

func LoadFromLookup(lookup func(string) (string, bool)) (*Config, error) {
	value := func(name string) string { v, _ := lookup(name); return strings.TrimSpace(v) }
	cfg := &Config{Token: value("TOKEN"), TelegramMode: "polling", TurnTimeout: 2 * time.Minute, WebAddr: ":8080"}
	if cfg.Token == "" {
		return nil, ErrMissingToken
	}
	if !tokenRegex.MatchString(cfg.Token) {
		return nil, ErrInvalidToken
	}
	cfg.DatabaseURL = value("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, ErrMissingDatabaseURL
	}
	// Keep the existing Base64 decoder and exact 32-byte requirement.
	raw, _ := lookup("MINIAPP_SECRET")
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidMiniAppSecret
	}
	cfg.MiniAppSecret = key
	if v := value("TURN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, ErrInvalidTurnTimeout
		}
		cfg.TurnTimeout = d
	}
	if v := value("TELEGRAM_MODE"); v != "" {
		cfg.TelegramMode = strings.ToLower(v)
	}
	if cfg.TelegramMode != "polling" && cfg.TelegramMode != "webhook" {
		return nil, ErrInvalidTelegramMode
	}
	if v := value("WEB_ADDR"); v != "" {
		cfg.WebAddr = v
	}
	if cfg.TelegramMode == "webhook" {
		cfg.WebhookURL = value("WEBHOOK_URL")
		if cfg.WebhookURL == "" {
			return nil, ErrMissingWebhookURL
		}
		u, err := url.Parse(cfg.WebhookURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, ErrInvalidWebhookURL
		}
		if u.Path == "" || u.Path == "/" || strings.HasPrefix(u.Path, "/api") || strings.HasPrefix(u.Path, "/assets") || u.Path == "/healthz" || u.Path == "/readyz" {
			return nil, ErrInvalidWebhookURL
		}
	}
	return cfg, nil
}
