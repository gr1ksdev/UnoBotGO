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
	return map[string]string{"TOKEN": "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ_12345", "DATABASE_URL": "postgres://localhost/unobot_test"}
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
	if cfg.Token != env["TOKEN"] || cfg.DatabaseURL != env["DATABASE_URL"] || cfg.TelegramMode != "polling" || cfg.WebAddr != ":8080" || cfg.TurnTimeout != 2*time.Minute || cfg.SkipTimePenalty != 20*time.Second || cfg.MinTurnTime != 15*time.Second || len(cfg.SecretKey) != 0 || cfg.WebhookURL != "" {
		t.Fatal("unexpected minimal configuration")
	}
	if LogLevel != slog.LevelInfo || HistoryLimit != 100 || InlineTokenTTL != 2*time.Minute || InlineTokenLimit != 20000 || InlineTokenUserLimit != 512 || MigrationTimeout != 2*time.Minute {
		t.Fatal("fixed policies changed")
	}
}
func TestRemovedEnvironmentIsNeverRead(t *testing.T) {
	env := minimalEnv()
	cfg, err := LoadFromLookup(func(k string) (string, bool) {
		switch k {
		case "TOKEN", "DATABASE_URL", "TELEGRAM_MODE", "TURN_TIMEOUT", "SKIP_TIME_PENALTY", "MIN_TURN_TIME", "WEB_ADDR", "WEBHOOK_URL", "SECRET_KEY":
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
		{"bad penalty", "SKIP_TIME_PENALTY", "nope", ErrInvalidSkipPenalty},
		{"zero penalty", "SKIP_TIME_PENALTY", "0s", ErrInvalidSkipPenalty},
		{"bad floor", "MIN_TURN_TIME", "nope", ErrInvalidMinTurnTime},
		{"negative floor", "MIN_TURN_TIME", "-2s", ErrInvalidMinTurnTime},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := minimalEnv()
			env[tc.key] = tc.value
			_, err := fromMap(env)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if (strings.TrimSpace(env["TOKEN"]) != "" && strings.Contains(err.Error(), env["TOKEN"])) || strings.Contains(err.Error(), env["SECRET_KEY"]) && env["SECRET_KEY"] != "" {
				t.Fatal("configuration error leaked secret")
			}
		})
	}
}
func TestWebhookSecretIsRequiredOnlyInWebhookMode(t *testing.T) {
	if cfg, err := fromMap(minimalEnv()); err != nil || len(cfg.SecretKey) != 0 {
		t.Fatal("polling must not require the secret", err)
	}
	valid := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32))
	for _, tc := range []struct {
		name, value string
		want        error
	}{
		{"missing", "", ErrMissingSecretKey},
		{"bad base64", "%%%", ErrInvalidSecretKey},
		{"short", base64.StdEncoding.EncodeToString(make([]byte, 31)), ErrInvalidSecretKey},
		{"long", base64.StdEncoding.EncodeToString(make([]byte, 33)), ErrInvalidSecretKey},
		{"64 bytes", base64.StdEncoding.EncodeToString(make([]byte, 64)), ErrInvalidSecretKey},
		{"valid", valid, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := minimalEnv()
			env["TELEGRAM_MODE"] = "webhook"
			env["WEBHOOK_URL"] = "https://bot.example/telegram"
			env["SECRET_KEY"] = tc.value
			cfg, err := fromMap(env)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if tc.want == nil && len(cfg.SecretKey) != 32 {
				t.Fatal("secret not loaded")
			}
			if err != nil && tc.value != "" && strings.Contains(err.Error(), tc.value) {
				t.Fatal("error leaked the secret")
			}
		})
	}
}

func TestTelegramModesAndConditionalWebhookURL(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32))
	for _, mode := range []string{"polling", "webhook", "WEBHOOK"} {
		t.Run(mode, func(t *testing.T) {
			env := minimalEnv()
			env["TELEGRAM_MODE"] = mode
			env["SECRET_KEY"] = secret
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
	for _, u := range []string{"", "http://bot.example/hook", "https://", "https://bot.example/", "https://bot.example/healthz", "https://bot.example/readyz", "https://user:secret@bot.example/hook", "https://bot.example/hook#fragment"} {
		t.Run(u, func(t *testing.T) {
			env := minimalEnv()
			env["TELEGRAM_MODE"] = "webhook"
			env["SECRET_KEY"] = secret
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
	if typ.NumField() != 9 {
		t.Fatalf("unexpected fields: %v", typ)
	}
	for _, name := range []string{"Token", "TelegramMode", "DatabaseURL", "TurnTimeout", "SkipTimePenalty", "MinTurnTime", "WebAddr", "SecretKey", "WebhookURL"} {
		if _, ok := typ.FieldByName(name); !ok {
			t.Fatal("missing external field", name)
		}
	}
}
