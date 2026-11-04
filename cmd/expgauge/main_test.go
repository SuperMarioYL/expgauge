package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var bareSemver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// TestVersionSurfacesAgree guards every shipped version surface against drift:
// the VERSION file and the CLI constant are bare semver, web/site.json's
// product version carries the v prefix git tags use, and both README locales
// show the bare version in their badge.
func TestVersionSurfacesAgree(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION file: %v", err)
	}
	want := strings.TrimSpace(string(raw))
	if !bareSemver.MatchString(want) {
		t.Fatalf("VERSION = %q, want bare semver without a v prefix", want)
	}
	if version != want {
		t.Errorf("CLI version = %q, want %q (VERSION file)", version, want)
	}
	if got := newRootCmd().Version; got != version {
		t.Errorf("rootCmd.Version = %q, want %q", got, version)
	}

	siteRaw, err := os.ReadFile(filepath.Join("..", "..", "web", "site.json"))
	if err != nil {
		t.Fatalf("read web/site.json: %v", err)
	}
	var site struct {
		Meta struct {
			SourceVersion string `json:"source_version"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(siteRaw, &site); err != nil {
		t.Fatalf("parse web/site.json: %v", err)
	}
	if got, wantSite := site.Meta.SourceVersion, "v"+want; got != wantSite {
		t.Errorf("web/site.json meta.source_version = %q, want %q", got, wantSite)
	}

	for _, name := range []string{"README.md", "README.en.md"} {
		readme, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(readme), "version-"+want) {
			t.Errorf("%s badge does not carry version-%s", name, want)
		}
	}
}
