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
	InitDataMaxAge            = time.Hour
	MigrationTimeout          = 2 * time.Minute
	WebhookDropPendingUpdates = false
)
