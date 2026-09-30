package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signed(token string, at time.Time, extra map[string]string) string {
	v := url.Values{"auth_date": {strconv.FormatInt(at.Unix(), 10)}, "user": {`{"id":12345,"first_name":"José 🃏"}`}}
	for k, x := range extra {
		v.Set(k, x)
	}
	lines := []string{}
	for k := range v {
		lines = append(lines, k+"="+v.Get(k))
	}
	sort.Strings(lines)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}
func TestInitData(t *testing.T) {
	now := time.Unix(1800000000, 0)
	token := "123456:test-token"
	valid := signed(token, now, map[string]string{"signature": "optional-signature"})
	for name, raw := range map[string]string{"valid": valid, "modified": valid + "x", "duplicate": valid + "&auth_date=1", "expired": signed(token, now.Add(-2*time.Hour), nil), "future": signed(token, now.Add(time.Minute), nil), "missing_user": signed(token, now, map[string]string{"user": "{}"}), "invalid_encoding": "%xx", "empty": ""} {
		t.Run(name, func(t *testing.T) {
			id, err := ValidateInitData(raw, token, now, time.Hour)
			if name == "valid" {
				if err != nil || id != 12345 {
					t.Fatal(id, err)
				}
			} else if err == nil {
				t.Fatal("accepted invalid payload")
			}
		})
	}
	if _, err := ValidateInitData(valid, "another-token", now, time.Hour); err == nil {
		t.Fatal("wrong token accepted")
	}
}
func TestReferences(t *testing.T) {
	refs, err := NewReferences(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	raw, err := refs.seal(reference{Kind: "group", ID: -100123456789, Expires: now.Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	v, err := refs.open(raw, now)
	if err != nil || v.ID != -100123456789 {
		t.Fatal(v, err)
	}
	if _, err := refs.open(raw+"a", now); err == nil {
		t.Fatal("tamper accepted")
	}
	if _, err := refs.open(raw, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired accepted")
	}
	if refs.key("user", 1) == refs.key("group", 1) {
		t.Fatal("scope collision")
	}
}
