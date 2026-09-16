package config

import (
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	env := map[string]string{
		"TOKEN": "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
	}
	lookup := func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}

	cfg, err := LoadFromLookup(lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Token != env["TOKEN"] {
		t.Errorf("expected token %q, got %q", env["TOKEN"], cfg.Token)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected log level info, got %v", cfg.LogLevel)
	}
	if cfg.HistoryLimit != 100 {
		t.Errorf("expected history limit 100, got %d", cfg.HistoryLimit)
	}
	if cfg.InlineTokenTTL != 2*time.Minute {
		t.Errorf("expected TTL 2m, got %v", cfg.InlineTokenTTL)
	}
	if cfg.InlineTokenLimit != 20000 {
		t.Errorf("expected token limit 20000, got %d", cfg.InlineTokenLimit)
	}
	if cfg.InlineTokenUserLim != 512 {
		t.Errorf("expected user limit 512, got %d", cfg.InlineTokenUserLim)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	env := map[string]string{
		"TOKEN":                   "987654321:XYZ_secret_token_12345",
		"LOG_LEVEL":               "debug",
		"HISTORY_LIMIT":           "50",
		"INLINE_TOKEN_TTL":        "5m",
		"INLINE_TOKEN_LIMIT":      "5000",
		"INLINE_TOKEN_USER_LIMIT": "128",
	}
	lookup := func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}

	cfg, err := LoadFromLookup(lookup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("expected debug log level, got %v", cfg.LogLevel)
	}
	if cfg.HistoryLimit != 50 {
		t.Errorf("expected history limit 50, got %d", cfg.HistoryLimit)
	}
	if cfg.InlineTokenTTL != 5*time.Minute {
		t.Errorf("expected TTL 5m, got %v", cfg.InlineTokenTTL)
	}
	if cfg.InlineTokenLimit != 5000 {
		t.Errorf("expected token limit 5000, got %d", cfg.InlineTokenLimit)
	}
	if cfg.InlineTokenUserLim != 128 {
		t.Errorf("expected user limit 128, got %d", cfg.InlineTokenUserLim)
	}
}

func TestLoadConfig_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		expectedErr error
	}{
		{
			name:        "missing token",
			env:         map[string]string{},
			expectedErr: ErrMissingToken,
		},
		{
			name: "empty token",
			env: map[string]string{
				"TOKEN": "   ",
			},
			expectedErr: ErrMissingToken,
		},
		{
			name: "invalid token format",
			env: map[string]string{
				"TOKEN": "not-a-valid-telegram-token",
			},
			expectedErr: ErrInvalidToken,
		},
		{
			name: "invalid log level",
			env: map[string]string{
				"TOKEN":     "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"LOG_LEVEL": "unknown",
			},
			expectedErr: ErrInvalidLogLevel,
		},
		{
			name: "negative history limit",
			env: map[string]string{
				"TOKEN":         "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"HISTORY_LIMIT": "-1",
			},
			expectedErr: ErrInvalidHistoryLimit,
		},
		{
			name: "invalid history limit string",
			env: map[string]string{
				"TOKEN":         "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"HISTORY_LIMIT": "abc",
			},
			expectedErr: ErrInvalidHistoryLimit,
		},
		{
			name: "negative TTL",
			env: map[string]string{
				"TOKEN":            "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"INLINE_TOKEN_TTL": "-10s",
			},
			expectedErr: ErrInvalidTokenTTL,
		},
		{
			name: "zero TTL",
			env: map[string]string{
				"TOKEN":            "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"INLINE_TOKEN_TTL": "0s",
			},
			expectedErr: ErrInvalidTokenTTL,
		},
		{
			name: "invalid token limit",
			env: map[string]string{
				"TOKEN":              "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"INLINE_TOKEN_LIMIT": "0",
			},
			expectedErr: ErrInvalidTokenLimit,
		},
		{
			name: "invalid user limit",
			env: map[string]string{
				"TOKEN":                   "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345",
				"INLINE_TOKEN_USER_LIMIT": "-5",
			},
			expectedErr: ErrInvalidUserTokenLim,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			_, err := LoadFromLookup(lookup)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
			// Verify token secret is never printed in error message
			if tokenVal, ok := tt.env["TOKEN"]; ok && tokenVal != "" {
				if strings.Contains(err.Error(), tokenVal) {
					t.Errorf("error message leaked token secret: %s", err.Error())
				}
			}
		})
	}
}

func TestLoadWebhookConfig(t *testing.T) {
	lookup := func(k string) (string, bool) {
		m := map[string]string{"TOKEN": "123456789:abcdefghij", "TELEGRAM_MODE": "webhook", "WEBHOOK_URL": "https://bot.example/hook", "WEBHOOK_SECRET": "abc_DEF-123"}
		v, ok := m[k]
		return v, ok
	}
	cfg, err := LoadFromLookup(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramMode != "webhook" || cfg.WebhookListenAddr != ":8080" || cfg.WebhookDropPending {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
func TestLoadRejectsInvalidWebhookConfig(t *testing.T) {
	base := map[string]string{"TOKEN": "123456789:abcdefghij", "TELEGRAM_MODE": "webhook", "WEBHOOK_URL": "http://bot.example/hook", "WEBHOOK_SECRET": "secret"}
	_, err := LoadFromLookup(func(k string) (string, bool) { v, ok := base[k]; return v, ok })
	if !errors.Is(err, ErrInvalidWebhookURL) {
		t.Fatalf("err=%v", err)
	}
}
