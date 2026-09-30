package config

import (
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"
)

type Web struct {
	Addr             string
	LaunchURL        string
	Secret           []byte
	InitDataMaxAge   time.Duration
	MigrationTimeout time.Duration
}

// LoadWeb is separate so development tools that do not serve HTTP need no secret.
func LoadWeb(lookup func(string) (string, bool)) (Web, error) {
	w := Web{Addr: ":8080", InitDataMaxAge: time.Hour, MigrationTimeout: 2 * time.Minute}
	old, _ := lookup("WEBHOOK_LISTEN_ADDR")
	addr, _ := lookup("WEB_ADDR")
	old = strings.TrimSpace(old)
	addr = strings.TrimSpace(addr)
	if old != "" {
		w.Addr = old
	}
	if addr != "" {
		w.Addr = addr
	}
	if old != "" && addr != "" && old != addr {
		return w, errors.New("WEB_ADDR conflicts with WEBHOOK_LISTEN_ADDR")
	}
	raw, _ := lookup("MINIAPP_SECRET")
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return w, errors.New("MINIAPP_SECRET must be base64 encoding of 32 random bytes")
	}
	w.Secret = key
	w.LaunchURL, _ = lookup("MINIAPP_LAUNCH_URL")
	if w.LaunchURL != "" {
		u, err := url.Parse(w.LaunchURL)
		if err != nil || u.Scheme != "https" || u.Host != "t.me" || u.User != nil || len(strings.Split(strings.Trim(u.Path, "/"), "/")) < 2 {
			return w, errors.New("MINIAPP_LAUNCH_URL must be an HTTPS Telegram Mini App direct link")
		}
	}
	for name, target := range map[string]*time.Duration{"TELEGRAM_INITDATA_MAX_AGE": &w.InitDataMaxAge, "MIGRATION_TIMEOUT": &w.MigrationTimeout} {
		if v, ok := lookup(name); ok {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return w, errors.New("invalid duration: " + name)
			}
			*target = d
		}
	}
	return w, nil
}
