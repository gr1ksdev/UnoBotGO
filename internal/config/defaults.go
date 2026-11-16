package config

import (
	"log/slog"
	"time"
)

// Fixed application policies, deliberately not configurable via environment.
const (
	LogLevel                  = slog.LevelInfo
	HistoryLimit              = 100
	InlineTokenTTL            = 2 * time.Minute
	InlineTokenLimit          = 20000
	InlineTokenUserLimit      = 512
	MigrationTimeout          = 2 * time.Minute
	ShutdownTimeout           = 12 * time.Second
	WebhookDropPendingUpdates = false
)
