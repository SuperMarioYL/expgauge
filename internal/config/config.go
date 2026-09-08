// Package config loads expgauge configuration from ~/.expgauge.yaml.
// It parses a minimal YAML subset covering the expgauge config schema:
// top-level sections, nested key/value pairs, and string lists.
package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Thresholds defines when cumulative exposure triggers a forced review.
type Thresholds struct {
	WallClockSec int64 `json:"wall_clock_sec"`
	Mutations    int   `json:"mutations"`
	Tokens       int64 `json:"tokens"`
}

// AgentConfig describes how to locate the coding-agent process.
type AgentConfig struct {
	PidFile     string `json:"pid_file"`
	LogPath     string `json:"log_path"`
	NamePattern string `json:"name_pattern"`
}

// Config is the parsed expgauge configuration.
type Config struct {
	WatchPaths []string    `json:"watch_paths"`
	Thresholds Thresholds  `json:"thresholds"`
	Agent      AgentConfig  `json:"agent"`
}

// Default returns the built-in default configuration.
// Default thresholds: 2h wall-clock, 50 mutations, 1M tokens.
func Default() Config {
	return Config{
		WatchPaths: []string{},
		Thresholds: Thresholds{
			WallClockSec: 2 * 60 * 60, // 2 hours
			Mutations:    50,
			Tokens:       1_000_000,
		},
		Agent: AgentConfig{},
	}
}

// ConfigPath returns the default config file location: ~/.expgauge.yaml.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home dir: %w", err)
	}
	return filepath.Join(home, ".expgauge.yaml"), nil
}

// StateDir returns the directory for run state and timeline files.
func StateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home dir: %w", err)
	}
	return filepath.Join(home, ".expgauge"), nil
}

// Load reads the config from ~/.expgauge.yaml, falling back to defaults
// when the file is absent or partially specified.
func Load() (*Config, error) {
	return LoadFromPath("")
}

// LoadFromPath reads config from the given path. An empty path uses the
// default ~/.expgauge.yaml location. Missing file returns defaults.
func LoadFromPath(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		p, err := ConfigPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := parseYAML(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// parseYAML parses a minimal YAML subset matching the expgauge config
// schema: top-level section headers (key:), nested key: value pairs,
// and string list items (- value). Comments (#) and blank lines are
// skipped. Quoted values are unquoted.
func parseYAML(data []byte, cfg *Config) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var section string
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if indent == 0 {
			if strings.HasSuffix(trimmed, ":") {
				section = strings.TrimSuffix(trimmed, ":")
			}
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			val = unquote(val)
			if section == "watch_paths" {
				cfg.WatchPaths = append(cfg.WatchPaths, val)
			}
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := unquote(strings.TrimSpace(parts[1]))
		applyKV(cfg, section, key, val)
	}
	return scanner.Err()
}

// applyKV sets a single config field based on the current section and key.
func applyKV(cfg *Config, section, key, val string) {
	switch section {
	case "thresholds":
		switch key {
		case "wall_clock_sec":
			cfg.Thresholds.WallClockSec = parseInt64(val, cfg.Thresholds.WallClockSec)
		case "mutations":
			cfg.Thresholds.Mutations = parseInt(val, cfg.Thresholds.Mutations)
		case "tokens":
			cfg.Thresholds.Tokens = parseInt64(val, cfg.Thresholds.Tokens)
		}
	case "agent":
		switch key {
		case "pid_file":
			cfg.Agent.PidFile = val
		case "log_path":
			cfg.Agent.LogPath = val
		case "name_pattern":
			cfg.Agent.NamePattern = val
		}
	}
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseInt64(s string, fallback int64) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func parseInt(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
