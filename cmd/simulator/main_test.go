package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInteractive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--seed", "20260923", "--quiet", "--output", path}, strings.NewReader("2\nclassico\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Partida concluída") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("report not created: %v", err)
	}
}

func TestRunRejectsInvalidPlayers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--players", "1", "--mode", "classico"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "entre 2 e 10") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}
