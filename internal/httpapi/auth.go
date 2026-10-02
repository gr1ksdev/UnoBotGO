package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrAuth = errors.New("invalid Telegram authentication")

// ValidateInitData validates the original Telegram payload, not initDataUnsafe.
func ValidateInitData(raw, token string, now time.Time, maxAge time.Duration) (int64, error) {
	if len(raw) == 0 || len(raw) > 16384 || maxAge <= 0 {
		return 0, ErrAuth
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return 0, ErrAuth
	}
	lines := make([]string, 0, len(values))
	for k, v := range values {
		if len(v) != 1 {
			return 0, ErrAuth
		}
		if k != "hash" {
			lines = append(lines, k+"="+v[0])
		}
	}
	hash, err := hex.DecodeString(values.Get("hash"))
	if err != nil || len(hash) != 32 {
		return 0, ErrAuth
	}
	sort.Strings(lines)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	if !hmac.Equal(hash, mac.Sum(nil)) {
		return 0, ErrAuth
	}
	timestamp, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return 0, ErrAuth
	}
	date := time.Unix(timestamp, 0)
	if date.After(now.Add(30*time.Second)) || now.Sub(date) > maxAge {
		return 0, ErrAuth
	}
	var user struct {
		ID int64 `json:"id"`
	}
	if json.Unmarshal([]byte(values.Get("user")), &user) != nil || user.ID <= 0 {
		return 0, ErrAuth
	}
	return user.ID, nil
}
