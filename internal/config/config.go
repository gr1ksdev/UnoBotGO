package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	ErrMissingToken         = errors.New("config: TOKEN environment variable is required")
	ErrInvalidToken         = errors.New("config: invalid Telegram bot token format")
	ErrInvalidLogLevel      = errors.New("config: invalid LOG_LEVEL")
	ErrInvalidHistoryLimit  = errors.New("config: HISTORY_LIMIT must be non-negative")
	ErrInvalidTokenTTL      = errors.New("config: INLINE_TOKEN_TTL must be positive")
	ErrInvalidTokenLimit    = errors.New("config: INLINE_TOKEN_LIMIT must be positive")
	ErrInvalidUserTokenLim  = errors.New("config: INLINE_TOKEN_USER_LIMIT must be positive")
	ErrInvalidTurnTimeout   = errors.New("config: TURN_TIMEOUT must be positive")
	ErrInvalidTelegramMode  = errors.New("config: TELEGRAM_MODE must be polling or webhook")
	ErrMissingWebhookURL    = errors.New("config: WEBHOOK_URL is required in webhook mode")
	ErrInvalidWebhookURL    = errors.New("config: WEBHOOK_URL must be an https URL with a host")
	ErrMissingWebhookSecret = errors.New("config: WEBHOOK_SECRET is required in webhook mode")
	ErrInvalidWebhookSecret = errors.New("config: WEBHOOK_SECRET has invalid format")
	ErrInvalidWebhookAddr   = errors.New("config: WEBHOOK_LISTEN_ADDR must not be empty")
	ErrInvalidWebhookDrop   = errors.New("config: WEBHOOK_DROP_PENDING_UPDATES must be boolean")

	tokenRegex         = regexp.MustCompile(`^[0-9]{3,}:[a-zA-Z0-9_-]{10,}$`)
	webhookSecretRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)
)

type Config struct {
	Token              string
	LogLevel           slog.Level
	HistoryLimit       int
	InlineTokenTTL     time.Duration
	InlineTokenLimit   int
	InlineTokenUserLim int
	TurnTimeout        time.Duration
	TelegramMode       string
	WebhookURL         string
	WebhookSecret      string
	WebhookListenAddr  string
	WebhookDropPending bool
}

// Load loads configuration from environment variables, optionally reading from .env if present.
// Existing environment variables are preserved.
func Load() (*Config, error) {
	_ = godotenv.Load()
	return LoadFromLookup(os.LookupEnv)
}

// LoadFromLookup loads configuration using a custom environment lookup function.
func LoadFromLookup(lookup func(string) (string, bool)) (*Config, error) {
	token, ok := lookup("TOKEN")
	if !ok || strings.TrimSpace(token) == "" {
		return nil, ErrMissingToken
	}
	token = strings.TrimSpace(token)
	if !tokenRegex.MatchString(token) {
		return nil, ErrInvalidToken
	}

	cfg := &Config{
		Token:              token,
		LogLevel:           slog.LevelInfo,
		HistoryLimit:       100,
		InlineTokenTTL:     2 * time.Minute,
		InlineTokenLimit:   20000,
		InlineTokenUserLim: 512,
		TurnTimeout:        2 * time.Minute,
		TelegramMode:       "polling",
		WebhookListenAddr:  ":8080",
	}

	if val, ok := lookup("LOG_LEVEL"); ok && strings.TrimSpace(val) != "" {
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "debug":
			cfg.LogLevel = slog.LevelDebug
		case "info":
			cfg.LogLevel = slog.LevelInfo
		case "warn", "warning":
			cfg.LogLevel = slog.LevelWarn
		case "error":
			cfg.LogLevel = slog.LevelError
		default:
			return nil, fmt.Errorf("%w: %q", ErrInvalidLogLevel, val)
		}
	}

	if val, ok := lookup("HISTORY_LIMIT"); ok && strings.TrimSpace(val) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || n < 0 {
			return nil, ErrInvalidHistoryLimit
		}
		cfg.HistoryLimit = n
	}

	if val, ok := lookup("INLINE_TOKEN_TTL"); ok && strings.TrimSpace(val) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(val))
		if err != nil || d <= 0 {
			return nil, ErrInvalidTokenTTL
		}
		cfg.InlineTokenTTL = d
	}

	if val, ok := lookup("INLINE_TOKEN_LIMIT"); ok && strings.TrimSpace(val) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || n <= 0 {
			return nil, ErrInvalidTokenLimit
		}
		cfg.InlineTokenLimit = n
	}

	if val, ok := lookup("INLINE_TOKEN_USER_LIMIT"); ok && strings.TrimSpace(val) != "" {
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || n <= 0 {
			return nil, ErrInvalidUserTokenLim
		}
		cfg.InlineTokenUserLim = n
	}
	if val, ok := lookup("TURN_TIMEOUT"); ok && strings.TrimSpace(val) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(val))
		if err != nil || d <= 0 {
			return nil, ErrInvalidTurnTimeout
		}
		cfg.TurnTimeout = d
	}
	if val, ok := lookup("TELEGRAM_MODE"); ok && strings.TrimSpace(val) != "" {
		cfg.TelegramMode = strings.ToLower(strings.TrimSpace(val))
	}
	if cfg.TelegramMode != "polling" && cfg.TelegramMode != "webhook" {
		return nil, ErrInvalidTelegramMode
	}
	if val, ok := lookup("WEBHOOK_URL"); ok {
		cfg.WebhookURL = strings.TrimSpace(val)
	}
	if val, ok := lookup("WEBHOOK_SECRET"); ok {
		cfg.WebhookSecret = strings.TrimSpace(val)
	}
	if val, ok := lookup("WEBHOOK_LISTEN_ADDR"); ok && strings.TrimSpace(val) != "" {
		cfg.WebhookListenAddr = strings.TrimSpace(val)
	}
	if cfg.WebhookListenAddr == "" {
		return nil, ErrInvalidWebhookAddr
	}
	if val, ok := lookup("WEBHOOK_DROP_PENDING_UPDATES"); ok && strings.TrimSpace(val) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(val))
		if err != nil {
			return nil, ErrInvalidWebhookDrop
		}
		cfg.WebhookDropPending = parsed
	}
	if cfg.TelegramMode == "webhook" {
		if cfg.WebhookURL == "" {
			return nil, ErrMissingWebhookURL
		}
		parsed, err := url.Parse(cfg.WebhookURL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, ErrInvalidWebhookURL
		}
		if cfg.WebhookSecret == "" {
			return nil, ErrMissingWebhookSecret
		}
		if !webhookSecretRegex.MatchString(cfg.WebhookSecret) {
			return nil, ErrInvalidWebhookSecret
		}
	}

	return cfg, nil
}
