package metrics

import (
	"testing"
	"time"
)

func TestExposureFromRunState(t *testing.T) {
	now := time.Now()
	rs := RunState{
		StartedAt:   now.Add(-2 * time.Hour),
		LastReview:  now.Add(-30 * time.Minute),
		Mutations:   15,
		Tokens:      250_000,
	}
	exp := ExposureFromRunState(rs)
	// Wall-clock should be ~30 minutes (1800 seconds), within a few seconds.
	if exp.WallClockSec < 1790 || exp.WallClockSec > 1810 {
		t.Errorf("wall_clock_sec = %d, want ~1800", exp.WallClockSec)
	}
	if exp.Mutations != 15 {
		t.Errorf("mutations = %d, want 15", exp.Mutations)
	}
	if exp.Tokens != 250_000 {
		t.Errorf("tokens = %d, want 250000", exp.Tokens)
	}
}

func TestAccumulator_WallClock(t *testing.T) {
	acc, err := NewAccumulator(nil, "")
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	time.Sleep(2 * time.Second)
	exp := acc.Snapshot()
	if exp.WallClockSec < 1 {
		t.Errorf("wall_clock_sec = %d, want >= 1 after 2s sleep", exp.WallClockSec)
	}
}

func TestAccumulator_Review(t *testing.T) {
	acc, err := NewAccumulator(nil, "")
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	time.Sleep(2 * time.Second)
	exp := acc.Snapshot()
	firstWall := exp.WallClockSec
	if firstWall < 1 {
		t.Fatalf("first wall_clock_sec = %d, want >= 1", firstWall)
	}

	acc.Review()
	time.Sleep(1 * time.Second)
	exp = acc.Snapshot()
	if exp.WallClockSec >= firstWall {
		t.Errorf("after review, wall_clock_sec = %d, should be < %d", exp.WallClockSec, firstWall)
	}
}

func TestAccumulator_ToRunState(t *testing.T) {
	acc, err := NewAccumulator([]string{"/tmp"}, "/tmp/agent.log")
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	rs := acc.ToRunState(12345, []string{"/tmp"})
	if rs.PID != 12345 {
		t.Errorf("pid = %d, want 12345", rs.PID)
	}
	if rs.LogPath != "/tmp/agent.log" {
		t.Errorf("log_path = %q, want /tmp/agent.log", rs.LogPath)
	}
	if len(rs.WatchPaths) != 1 || rs.WatchPaths[0] != "/tmp" {
		t.Errorf("watch_paths = %v, want [/tmp]", rs.WatchPaths)
	}
	if rs.StartedAt.IsZero() {
		t.Error("started_at should not be zero")
	}
	if rs.LastReview.IsZero() {
		t.Error("last_review should not be zero")
	}
}

func TestIsTempFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".file.go.swp", true},
		{"file.go~", true},
		{"file.tmp", true},
		{"4913", true},
		{".#file.go", true},
		{"main.go", false},
		{"config.yaml", false},
		{"data.json", false},
	}
	for _, tt := range tests {
		if got := isTempFile(tt.name); got != tt.want {
			t.Errorf("isTempFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
