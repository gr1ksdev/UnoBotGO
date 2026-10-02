package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

func minimalEnv() map[string]string {
	return map[string]string{"TOKEN": "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345", "DATABASE_URL": "postgres://localhost/unobot_test", "MINIAPP_SECRET": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32))}
}
func fromMap(env map[string]string) (*Config, error) {
	return LoadFromLookup(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
}
func TestMinimalConfigAndInternalPolicies(t *testing.T) {
	env := minimalEnv()
	cfg, err := fromMap(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != env["TOKEN"] || cfg.DatabaseURL != env["DATABASE_URL"] || cfg.TelegramMode != "polling" || cfg.WebAddr != ":8080" || cfg.TurnTimeout != 2*time.Minute || len(cfg.MiniAppSecret) != 32 || cfg.WebhookURL != "" {
		t.Fatal("unexpected minimal configuration")
	}
	if LogLevel != slog.LevelInfo || HistoryLimit != 100 || InlineTokenTTL != 2*time.Minute || InlineTokenLimit != 20000 || InlineTokenUserLimit != 512 || InitDataMaxAge != time.Hour || MigrationTimeout != 2*time.Minute {
		t.Fatal("fixed policies changed")
	}
}
func TestRemovedEnvironmentIsNeverRead(t *testing.T) {
	env := minimalEnv()
	cfg, err := LoadFromLookup(func(k string) (string, bool) {
		switch k {
		case "TOKEN", "DATABASE_URL", "MINIAPP_SECRET", "TELEGRAM_MODE", "TURN_TIMEOUT", "WEB_ADDR", "WEBHOOK_URL":
			v, ok := env[k]
			return v, ok
		default:
			t.Fatalf("loader unexpectedly read %s", k)
			return "invalid", true
		}
	})
	if err != nil || cfg == nil {
		t.Fatal(err)
	}
}
func TestConfigOverrides(t *testing.T) {
	env := minimalEnv()
	env["TURN_TIMEOUT"] = "45s"
	env["WEB_ADDR"] = "127.0.0.1:9090"
	env["TELEGRAM_MODE"] = " POLLING "
	cfg, err := fromMap(env)
	if err != nil || cfg.TurnTimeout != 45*time.Second || cfg.WebAddr != "127.0.0.1:9090" || cfg.TelegramMode != "polling" {
		t.Fatal("overrides failed", err)
	}
}
func TestConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		want             error
	}{
		{"missing token", "TOKEN", "", ErrMissingToken},
		{"blank token", "TOKEN", "  ", ErrMissingToken},
		{"invalid token", "TOKEN", "not-a-valid-telegram-token-123", ErrInvalidToken},
		{"missing database", "DATABASE_URL", "", ErrMissingDatabaseURL},
		{"bad mode", "TELEGRAM_MODE", "anything", ErrInvalidTelegramMode},
		{"bad duration", "TURN_TIMEOUT", "nope", ErrInvalidTurnTimeout},
		{"zero duration", "TURN_TIMEOUT", "0s", ErrInvalidTurnTimeout},
		{"negative duration", "TURN_TIMEOUT", "-1s", ErrInvalidTurnTimeout},
		{"missing secret", "MINIAPP_SECRET", "", ErrInvalidMiniAppSecret},
		{"bad base64", "MINIAPP_SECRET", "%%%", ErrInvalidMiniAppSecret},
		{"short secret", "MINIAPP_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 31)), ErrInvalidMiniAppSecret},
		{"long secret", "MINIAPP_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 33)), ErrInvalidMiniAppSecret},
		{"64 bytes rejected", "MINIAPP_SECRET", base64.StdEncoding.EncodeToString(make([]byte, 64)), ErrInvalidMiniAppSecret},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := minimalEnv()
			env[tc.key] = tc.value
			_, err := fromMap(env)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if (strings.TrimSpace(env["TOKEN"]) != "" && strings.Contains(err.Error(), env["TOKEN"])) || strings.Contains(err.Error(), env["MINIAPP_SECRET"]) && env["MINIAPP_SECRET"] != "" {
				t.Fatal("configuration error leaked secret")
			}
		})
	}
}
func TestTelegramModesAndConditionalWebhookURL(t *testing.T) {
	for _, mode := range []string{"polling", "webhook", "WEBHOOK"} {
		t.Run(mode, func(t *testing.T) {
			env := minimalEnv()
			env["TELEGRAM_MODE"] = mode
			env["WEBHOOK_URL"] = "https://bot.example/telegram"
			cfg, err := fromMap(env)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.TelegramMode == "polling" && cfg.WebhookURL != "" {
				t.Fatal("polling retained unused URL")
			}
			if cfg.TelegramMode == "webhook" && cfg.WebhookURL != env["WEBHOOK_URL"] {
				t.Fatal("webhook URL missing")
			}
		})
	}
	for _, u := range []string{"", "http://bot.example/hook", "https://", "https://bot.example/", "https://bot.example/api/hook", "https://bot.example/assets/hook", "https://bot.example/healthz", "https://bot.example/readyz", "https://user:secret@bot.example/hook", "https://bot.example/hook#fragment"} {
		t.Run(u, func(t *testing.T) {
			env := minimalEnv()
			env["TELEGRAM_MODE"] = "webhook"
			env["WEBHOOK_URL"] = u
			_, err := fromMap(env)
			want := ErrInvalidWebhookURL
			if u == "" {
				want = ErrMissingWebhookURL
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v want %v", err, want)
			}
		})
	}
	env := minimalEnv()
	env["WEBHOOK_URL"] = "invalid and ignored in polling"
	if _, err := fromMap(env); err != nil {
		t.Fatal(err)
	}
}
func TestConfigRepresentsOnlyExternalSettings(t *testing.T) {
	typ := reflect.TypeFor[Config]()
	if typ.NumField() != 7 {
		t.Fatalf("unexpected fields: %v", typ)
	}
	for _, name := range []string{"Token", "TelegramMode", "DatabaseURL", "TurnTimeout", "WebAddr", "MiniAppSecret", "WebhookURL"} {
		if _, ok := typ.FieldByName(name); !ok {
			t.Fatal("missing external field", name)
		}
	}
}
