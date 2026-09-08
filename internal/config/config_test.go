package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Thresholds.WallClockSec != 7200 {
		t.Errorf("default wall_clock_sec = %d, want 7200", cfg.Thresholds.WallClockSec)
	}
	if cfg.Thresholds.Mutations != 50 {
		t.Errorf("default mutations = %d, want 50", cfg.Thresholds.Mutations)
	}
	if cfg.Thresholds.Tokens != 1_000_000 {
		t.Errorf("default tokens = %d, want 1000000", cfg.Thresholds.Tokens)
	}
}

func TestLoadFromPath_MissingFile(t *testing.T) {
	cfg, err := LoadFromPath("/nonexistent/expgauge.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Thresholds.WallClockSec != 7200 {
		t.Errorf("should fall back to default, got %d", cfg.Thresholds.WallClockSec)
	}
}

func TestParseYAML(t *testing.T) {
	yaml := `# expgauge config
watch_paths:
  - /home/user/project
  - /home/user/project/sub

thresholds:
  wall_clock_sec: 3600
  mutations: 30
  tokens: 500000

agent:
  pid_file: /tmp/agent.pid
  log_path: /tmp/agent.log
  name_pattern: "qwen|deepseek"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expgauge.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if len(cfg.WatchPaths) != 2 {
		t.Errorf("watch_paths = %v, want 2 entries", cfg.WatchPaths)
	}
	if cfg.WatchPaths[0] != "/home/user/project" {
		t.Errorf("watch_paths[0] = %q, want /home/user/project", cfg.WatchPaths[0])
	}
	if cfg.Thresholds.WallClockSec != 3600 {
		t.Errorf("wall_clock_sec = %d, want 3600", cfg.Thresholds.WallClockSec)
	}
	if cfg.Thresholds.Mutations != 30 {
		t.Errorf("mutations = %d, want 30", cfg.Thresholds.Mutations)
	}
	if cfg.Thresholds.Tokens != 500000 {
		t.Errorf("tokens = %d, want 500000", cfg.Thresholds.Tokens)
	}
	if cfg.Agent.PidFile != "/tmp/agent.pid" {
		t.Errorf("pid_file = %q, want /tmp/agent.pid", cfg.Agent.PidFile)
	}
	if cfg.Agent.LogPath != "/tmp/agent.log" {
		t.Errorf("log_path = %q, want /tmp/agent.log", cfg.Agent.LogPath)
	}
	if cfg.Agent.NamePattern != "qwen|deepseek" {
		t.Errorf("name_pattern = %q, want qwen|deepseek", cfg.Agent.NamePattern)
	}
}

func TestParseYAML_PartialConfig(t *testing.T) {
	yaml := `thresholds:
  mutations: 10
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expgauge.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if cfg.Thresholds.Mutations != 10 {
		t.Errorf("mutations = %d, want 10", cfg.Thresholds.Mutations)
	}
	// Other fields should keep defaults.
	if cfg.Thresholds.WallClockSec != 7200 {
		t.Errorf("wall_clock_sec should be default, got %d", cfg.Thresholds.WallClockSec)
	}
	if cfg.Thresholds.Tokens != 1_000_000 {
		t.Errorf("tokens should be default, got %d", cfg.Thresholds.Tokens)
	}
}

func TestParseYAML_CommentsAndBlanks(t *testing.T) {
	yaml := `# comment line

thresholds:
  # nested comment
  wall_clock_sec: 1800

agent:
  name_pattern: deepseek
`
	dir := t.TempDir()
	path := filepath.Join(dir, "expgauge.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if cfg.Thresholds.WallClockSec != 1800 {
		t.Errorf("wall_clock_sec = %d, want 1800", cfg.Thresholds.WallClockSec)
	}
	if cfg.Agent.NamePattern != "deepseek" {
		t.Errorf("name_pattern = %q, want deepseek", cfg.Agent.NamePattern)
	}
}
