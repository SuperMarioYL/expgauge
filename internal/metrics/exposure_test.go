package metrics

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestExposureFromRunState(t *testing.T) {
	now := time.Now()
	rs := RunState{
		StartedAt:  now.Add(-2 * time.Hour),
		LastReview: now.Add(-30 * time.Minute),
		Mutations:  15,
		Tokens:     250_000,
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
	logPath := filepath.Join(t.TempDir(), "agent.log")
	if err := os.WriteFile(logPath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	acc, err := NewAccumulator([]string{"/tmp"}, logPath)
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
	if rs.LogPath != logPath {
		t.Errorf("log_path = %q, want %q", rs.LogPath, logPath)
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

func TestReviewAt_ResetsExposureWindow(t *testing.T) {
	acc, err := NewAccumulator(nil, "")
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	// Seed mutations via a write event and tokens via a crafted log reader.
	acc.handleEvent(fsnotify.Event{Name: "/proj/main.go", Op: fsnotify.Write})
	acc.mu.Lock()
	acc.logReader = bufio.NewReader(strings.NewReader("total_tokens: 250000\n"))
	acc.mu.Unlock()
	acc.tailLog()

	exp := acc.Snapshot()
	if exp.Mutations != 1 {
		t.Fatalf("mutations = %d, want 1 (seed)", exp.Mutations)
	}
	if exp.Tokens != 250000 {
		t.Fatalf("tokens = %d, want 250000 (seed)", exp.Tokens)
	}

	acc.ReviewAt(time.Now().Add(time.Second))
	exp = acc.Snapshot()
	if exp.Mutations != 0 {
		t.Errorf("mutations after review = %d, want 0", exp.Mutations)
	}
	if exp.Tokens != 0 {
		t.Errorf("tokens after review = %d, want 0", exp.Tokens)
	}
	if exp.WallClockSec < 0 {
		t.Errorf("wall_clock after review = %d, want >= 0", exp.WallClockSec)
	}
	rs := acc.ToRunState(0, nil)
	if rs.Reviews != 1 {
		t.Errorf("reviews = %d, want 1", rs.Reviews)
	}
}

func TestAdoptDiskState_NewerReviewRestartsWindow(t *testing.T) {
	acc, err := NewAccumulator(nil, "")
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	acc.handleEvent(fsnotify.Event{Name: "/proj/main.go", Op: fsnotify.Write})

	future := time.Now().Add(time.Minute)
	acc.AdoptDiskState(RunState{LastReview: future, Reviews: 3, Mutations: 99, Tokens: 999})

	exp := acc.Snapshot()
	if exp.Mutations != 0 {
		t.Errorf("mutations after adopt = %d, want 0 (window restarted)", exp.Mutations)
	}
	if exp.Tokens != 0 {
		t.Errorf("tokens after adopt = %d, want 0 (window restarted)", exp.Tokens)
	}
	rs := acc.ToRunState(0, nil)
	if rs.Reviews != 3 {
		t.Errorf("reviews after adopt = %d, want 3 (carried from disk)", rs.Reviews)
	}
	if !rs.LastReview.Equal(future) {
		t.Errorf("last_review after adopt = %v, want %v", rs.LastReview, future)
	}
}

func TestAdoptDiskState_StaleDiskStateIgnored(t *testing.T) {
	acc, err := NewAccumulator(nil, "")
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	before := acc.Snapshot()
	acc.AdoptDiskState(RunState{LastReview: time.Now().Add(-time.Hour), Reviews: 0, Mutations: 5})
	after := acc.Snapshot()
	if after.Mutations != before.Mutations {
		t.Errorf("mutations changed from %d to %d on stale adopt, want unchanged", before.Mutations, after.Mutations)
	}
}

func TestStart_LogTailBeginsAtEOF(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent.log")
	if err := os.WriteFile(logPath, []byte("tokens: 1000000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	acc, err := NewAccumulator(nil, logPath)
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	// Historical content written before Start must not count.
	acc.tailLog()
	if exp := acc.Snapshot(); exp.Tokens != 0 {
		t.Fatalf("tokens = %d after tailing pre-existing log, want 0", exp.Tokens)
	}

	// Lines appended after Start must count.
	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("total_tokens: 7000\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	acc.tailLog()
	if exp := acc.Snapshot(); exp.Tokens != 7000 {
		t.Errorf("tokens = %d after appending a line, want 7000", exp.Tokens)
	}
}

// A watch path that cannot be attached must fail instead of silently
// reporting zero mutations for the rest of the run.
func TestNewAccumulator_UnwatchablePathFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := NewAccumulator([]string{missing}, ""); err == nil {
		t.Fatal("NewAccumulator accepted an unwatchable path, want error")
	}
}

// A configured agent log that cannot be opened must fail instead of silently
// disabling token tracking.
func TestStart_MissingLogFails(t *testing.T) {
	acc, err := NewAccumulator(nil, filepath.Join(t.TempDir(), "agent.log"))
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	// Start closes the watcher when it fails, so there is nothing to Stop.
	if err := acc.Start(); err == nil {
		t.Fatal("Start accepted a missing agent log, want error")
	}
}

func appendLogLine(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

// An agent that truncates its log mid-run must not end token counting.
func TestTailLog_SurvivesTruncation(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent.log")
	if err := os.WriteFile(logPath, []byte("total_tokens: 5000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	acc, err := NewAccumulator(nil, logPath)
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	appendLogLine(t, logPath, "total_tokens: 7000\n")
	acc.tailLog()
	if got := acc.Snapshot().Tokens; got != 7000 {
		t.Fatalf("tokens before truncation = %d, want 7000", got)
	}

	if err := os.Truncate(logPath, 0); err != nil {
		t.Fatal(err)
	}
	appendLogLine(t, logPath, "total_tokens: 9000\n")
	acc.tailLog()
	if got := acc.Snapshot().Tokens; got != 16000 {
		t.Errorf("tokens after truncation = %d, want 16000", got)
	}
}

// A logrotate-style rotation that moves the path to a new file must not end
// token counting.
func TestTailLog_SurvivesReplacement(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "agent.log")
	if err := os.WriteFile(logPath, []byte("total_tokens: 4000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	acc, err := NewAccumulator(nil, logPath)
	if err != nil {
		t.Fatalf("NewAccumulator: %v", err)
	}
	if err := acc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer acc.Stop()

	appendLogLine(t, logPath, "total_tokens: 6000\n")
	acc.tailLog()
	if got := acc.Snapshot().Tokens; got != 6000 {
		t.Fatalf("tokens before rotation = %d, want 6000", got)
	}

	if err := os.Rename(logPath, logPath+".1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("total_tokens: 8000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	acc.tailLog()
	if got := acc.Snapshot().Tokens; got != 14000 {
		t.Errorf("tokens after rotation = %d, want 14000", got)
	}
}
