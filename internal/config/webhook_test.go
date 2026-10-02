package config

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"testing"
)

func TestDerivedWebhookSecret(t *testing.T) {
	key := bytes.Repeat([]byte{42}, 32)
	got, err := DeriveWebhookSecret(key)
	if err != nil {
		t.Fatal(err)
	}
	// Independent protocol fixture pins the full context, not merely the helper's constant.
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("unobotgo/v2/telegram-webhook-secret/v1"))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if got != want || !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(got) {
		t.Fatal("unexpected protocol authenticator")
	}
	again, _ := DeriveWebhookSecret(key)
	if got != again {
		t.Fatal("derivation is not stable")
	}
	if got == base64.StdEncoding.EncodeToString(key) || got == base64.RawURLEncoding.EncodeToString(key) {
		t.Fatal("master key reused as protocol secret")
	}
	other := hmac.New(sha256.New, key)
	other.Write([]byte("unobotgo/v2/different-protocol/v1"))
	if got == base64.RawURLEncoding.EncodeToString(other.Sum(nil)) {
		t.Fatal("protocol separation failed")
	}
	rotated, _ := DeriveWebhookSecret(bytes.Repeat([]byte{43}, 32))
	if got == rotated {
		t.Fatal("rotation failed")
	}
	if !bytes.Equal(key, bytes.Repeat([]byte{42}, 32)) {
		t.Fatal("master key mutated")
	}
}
func TestDeriveWebhookSecretRejectsWrongKeySize(t *testing.T) {
	for _, size := range []int{0, 31, 33, 64} {
		got, err := DeriveWebhookSecret(make([]byte, size))
		if got != "" || !errors.Is(err, ErrInvalidMiniAppSecret) {
			t.Fatal("invalid master key accepted", size)
		}
	}
}
