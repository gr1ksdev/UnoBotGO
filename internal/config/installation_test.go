package config

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func installationEnv() map[string]string {
	return map[string]string{"TOKEN": "123456789:abcdefghij", "DATABASE_URL": "postgres://localhost/unobot_test", "MINIAPP_SECRET": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}
}
func loadInstallationEnv(env map[string]string) (*Config, error) {
	return LoadFromLookup(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
}
func TestMinimalConfigAndPolicies(t *testing.T) {
	cfg, err := loadInstallationEnv(installationEnv())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TelegramMode != "polling" || cfg.TurnTimeout != 2*time.Minute || cfg.WebAddr != ":8080" || cfg.WebhookURL != "" {
		t.Fatal("incorrect defaults")
	}
	if LogLevel != slog.LevelInfo || HistoryLimit != 100 || InlineTokenTTL != 2*time.Minute || InlineTokenLimit != 20000 || InlineTokenUserLimit != 512 || InitDataMaxAge != time.Hour || MigrationTimeout != 2*time.Minute || WebhookDropPendingUpdates {
		t.Fatal("application policies changed")
	}
}
func TestInstallationOverrides(t *testing.T) {
	env := installationEnv()
	env["TURN_TIMEOUT"] = " 3m "
	env["WEB_ADDR"] = "127.0.0.1:9090"
	cfg, err := loadInstallationEnv(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TurnTimeout != 3*time.Minute || cfg.WebAddr != "127.0.0.1:9090" {
		t.Fatal("overrides not applied")
	}
}
func TestMandatoryConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		want error
	}{{"TOKEN", ErrMissingToken}, {"DATABASE_URL", ErrMissingDatabaseURL}, {"MINIAPP_SECRET", ErrInvalidMiniAppSecret}} {
		t.Run(tc.name, func(t *testing.T) {
			env := installationEnv()
			delete(env, tc.name)
			_, err := loadInstallationEnv(env)
			if !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	env := installationEnv()
	env["TOKEN"] = "invalid-private-token"
	_, err := loadInstallationEnv(env)
	if !errors.Is(err, ErrInvalidToken) || strings.Contains(err.Error(), env["TOKEN"]) {
		t.Fatal("invalid token validation leaked data")
	}
}
func TestMiniAppSecret(t *testing.T) {
	for _, size := range []int{0, 1, 31, 32, 33, 64} {
		env := installationEnv()
		env["MINIAPP_SECRET"] = base64.StdEncoding.EncodeToString(make([]byte, size))
		cfg, err := loadInstallationEnv(env)
		if size == 32 {
			if err != nil || len(cfg.MiniAppSecret) != 32 {
				t.Fatalf("32 bytes rejected: %v", err)
			}
		} else if !errors.Is(err, ErrInvalidMiniAppSecret) {
			t.Fatalf("size %d accepted", size)
		}
	}
	env := installationEnv()
	env["MINIAPP_SECRET"] = "not-base64!"
	_, err := loadInstallationEnv(env)
	if !errors.Is(err, ErrInvalidMiniAppSecret) || strings.Contains(err.Error(), env["MINIAPP_SECRET"]) {
		t.Fatal("invalid secret validation")
	}
}
func TestTelegramMode(t *testing.T) {
	for _, mode := range []string{"polling", "webhook", " WEBHOOK ", "invalid"} {
		env := installationEnv()
		env["TELEGRAM_MODE"] = mode
		env["WEBHOOK_URL"] = "https://bot.example/telegram"
		cfg, err := loadInstallationEnv(env)
		if mode == "invalid" {
			if !errors.Is(err, ErrInvalidTelegramMode) {
				t.Fatal(err)
			}
		} else if err != nil || cfg.TelegramMode != strings.ToLower(strings.TrimSpace(mode)) {
			t.Fatal("mode validation")
		}
	}
}
func TestTurnTimeout(t *testing.T) {
	for _, value := range []string{"0s", "-1s", "no-duration"} {
		env := installationEnv()
		env["TURN_TIMEOUT"] = value
		_, err := loadInstallationEnv(env)
		if !errors.Is(err, ErrInvalidTurnTimeout) {
			t.Fatalf("accepted %q", value)
		}
	}
}
func TestWebhookURL(t *testing.T) {
	env := installationEnv()
	env["TELEGRAM_MODE"] = "webhook"
	if _, err := loadInstallationEnv(env); !errors.Is(err, ErrMissingWebhookURL) {
		t.Fatal(err)
	}
	for _, value := range []string{"http://bot.example/telegram", "https://bot.example", "https://bot.example/", "https://bot.example/api/hooks", "https://bot.example/assets/hook", "https://bot.example/healthz", "https://bot.example/readyz", "https://user:password@bot.example/telegram", "https://bot.example/telegram?secret=secret", "https://bot.example/telegram#hook"} {
		env["WEBHOOK_URL"] = value
		_, err := loadInstallationEnv(env)
		if !errors.Is(err, ErrInvalidWebhookURL) || strings.Contains(err.Error(), value) {
			t.Fatalf("URL validation: %q", value)
		}
	}
	env["WEBHOOK_URL"] = "https://bot.example/telegram"
	if _, err := loadInstallationEnv(env); err != nil {
		t.Fatal(err)
	}
	env["TELEGRAM_MODE"] = "polling"
	env["WEBHOOK_URL"] = "invalid"
	cfg, err := loadInstallationEnv(env)
	if err != nil || cfg.WebhookURL != "" {
		t.Fatal("polling should not read webhook URL")
	}
}
func TestOnlyInstallationSettingsAreRead(t *testing.T) {
	env := installationEnv()
	allowed := map[string]bool{"TOKEN": true, "DATABASE_URL": true, "MINIAPP_SECRET": true, "TURN_TIMEOUT": true, "TELEGRAM_MODE": true, "WEB_ADDR": true, "WEBHOOK_URL": true}
	cfg, err := LoadFromLookup(func(k string) (string, bool) {
		if !allowed[k] {
			t.Fatalf("unexpected configuration lookup: %s", k)
		}
		v, ok := env[k]
		return v, ok
	})
	if err != nil || reflect.TypeOf(*cfg).NumField() != len(allowed) {
		t.Fatal("config contains non-installation policies")
	}
}
func TestWebhookSecretDerivation(t *testing.T) {
	cfg, err := loadInstallationEnv(installationEnv())
	if err != nil {
		t.Fatal(err)
	}
	secret := cfg.WebhookSecret()
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`).MatchString(secret) {
		t.Fatal("invalid Telegram secret_token format")
	}
	if secret != cfg.WebhookSecret() {
		t.Fatal("derivation not deterministic")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) != sha256.Size {
		t.Fatal("invalid derived bytes")
	}
	if bytes.Equal(decoded, cfg.MiniAppSecret) {
		t.Fatal("master key exposed")
	}
	mac := hmac.New(sha256.New, cfg.MiniAppSecret)
	mac.Write([]byte("unobotgo/v2/telegram-webhook-secret/v1"))
	if !bytes.Equal(decoded, mac.Sum(nil)) {
		t.Fatal("protocol context changed")
	}
	for _, other := range []string{"unobotgo/reference/v1", "unobotgo/key/v1", "unobotgo/telegram/webhook-secret/v2"} {
		mac := hmac.New(sha256.New, cfg.MiniAppSecret)
		mac.Write([]byte(other))
		if bytes.Equal(decoded, mac.Sum(nil)) {
			t.Fatal("missing domain separation")
		}
	}
	cfg.MiniAppSecret[0] ^= 1
	if cfg.WebhookSecret() == secret {
		t.Fatal("key rotation did not rotate token")
	}
}
