package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionMatchesVersionFile guards against version drift between the
// CLI constant and the VERSION file at the repo root.
func TestVersionMatchesVersionFile(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION file: %v", err)
	}
	want := strings.TrimSpace(string(raw))
	if version != want {
		t.Errorf("CLI version = %q, want %q (VERSION file)", version, want)
	}
	if got := newRootCmd().Version; got != version {
		t.Errorf("rootCmd.Version = %q, want %q", got, version)
	}
}
