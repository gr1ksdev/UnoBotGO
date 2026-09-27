package postgres

import (
	"context"
	"strings"
	"testing"
)

func TestOpenRedactsInvalidConnection(t *testing.T) {
	_, err := Open(context.Background(), "postgres://name:private-secret@[invalid")
	if err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
