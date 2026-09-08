// Package metrics accumulates cumulative unsupervised exposure for a
// coding-agent run. It tracks three dimensions:
//   - Wall-clock time since the last human review checkpoint
//   - State-mutating filesystem events (write/create/remove) via fsnotify
//   - Token burn estimated by parsing the agent's log file
package metrics

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Exposure is a point-in-time snapshot of cumulative unsupervised exposure.
type Exposure struct {
	WallClockSec int64     `json:"wall_clock_sec"`
	Mutations    int       `json:"mutations"`
	Tokens       int64     `json:"tokens"`
	StartedAt    time.Time `json:"started_at"`
	LastReview   time.Time `json:"last_review"`
}

// Snapshot is a timestamped exposure reading, used for JSONL export.
type Snapshot struct {
	Timestamp   time.Time `json:"ts"`
	WallClockSec int64     `json:"wall_clock_sec"`
	Mutations    int       `json:"mutations"`
	Tokens       int64     `json:"tokens"`
}

// RunState is the persisted run state written to ~/.expgauge/run.json.
type RunState struct {
	PID         int        `json:"pid"`
	StartedAt   time.Time  `json:"started_at"`
	LastReview  time.Time  `json:"last_review"`
	Mutations   int        `json:"mutations"`
	Tokens      int64      `json:"tokens"`
	WatchPaths  []string   `json:"watch_paths"`
	LogPath     string     `json:"log_path"`
	Paused      bool       `json:"paused"`
}

// Accumulator tracks live exposure for a single agent run.
type Accumulator struct {
	mu        sync.Mutex
	startedAt time.Time
	lastReview time.Time
	mutations  int
	tokens     int64
	logPath    string
	watcher    *fsnotify.Watcher
	stopCh     chan struct{}
	doneCh     chan struct{}
	logFile    *os.File
	logReader  *bufio.Reader
}

// NewAccumulator creates an accumulator that watches the given paths for
// file mutations and parses the given log file for token usage.
func NewAccumulator(watchPaths []string, logPath string) (*Accumulator, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	for _, p := range watchPaths {
		if p == "" {
			continue
		}
		if err := w.Add(p); err != nil {
			// Non-fatal: path may not exist yet.
			continue
		}
	}
	return &Accumulator{
		watcher:   w,
		logPath:   logPath,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}, nil
}

// Start begins watching for file mutations and parsing the log file.
func (a *Accumulator) Start() error {
	a.mu.Lock()
	now := time.Now()
	a.startedAt = now
	a.lastReview = now
	a.mu.Unlock()

	if a.logPath != "" {
		f, err := os.Open(a.logPath)
		if err == nil {
			a.logFile = f
			a.logReader = bufio.NewReader(f)
		}
	}

	go a.watchLoop()
	return nil
}

// watchLoop processes fsnotify events and reads the log file tail.
func (a *Accumulator) watchLoop() {
	defer close(a.doneCh)
	for {
		select {
		case <-a.stopCh:
			return
		case event, ok := <-a.watcher.Events:
			if !ok {
				return
			}
			a.handleEvent(event)
		case <-time.After(5 * time.Second):
			a.tailLog()
		}
	}
}

// handleEvent classifies a filesystem event as state-mutating.
func (a *Accumulator) handleEvent(event fsnotify.Event) {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
		// Ignore editor temp files and swap files.
		base := filepath.Base(event.Name)
		if isTempFile(base) {
			return
		}
		a.mu.Lock()
		a.mutations++
		a.mu.Unlock()
	}
}

// isTempFile returns true for common editor temporary file patterns.
func isTempFile(name string) bool {
	if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".swp") {
		return true
	}
	if strings.HasSuffix(name, "~") {
		return true
	}
	if strings.HasSuffix(name, ".tmp") {
		return true
	}
	if strings.HasPrefix(name, "4913") { // vim creates this to test writability
		return true
	}
	if strings.HasPrefix(name, ".#") {
		return true
	}
	return false
}

// tailLog reads new lines from the agent log file and extracts token counts.
var tokenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)total_tokens["\s:=]+(\d+)`),
	regexp.MustCompile(`(?i)tokens["\s:=]+(\d+)`),
	regexp.MustCompile(`(?i)usage.*?total.*?(\d+)`),
	regexp.MustCompile(`(?i)token.*?(\d{4,})`),
}

func (a *Accumulator) tailLog() {
	if a.logReader == nil {
		return
	}
	for {
		line, err := a.logReader.ReadString('\n')
		if line != "" {
			for _, re := range tokenPatterns {
				m := re.FindStringSubmatch(line)
				if len(m) >= 2 {
					if n, perr := strconv.ParseInt(m[1], 10, 64); perr == nil {
						a.mu.Lock()
						a.tokens += n
						a.mu.Unlock()
						break
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// Stop halts the accumulator and releases resources.
func (a *Accumulator) Stop() {
	select {
	case <-a.stopCh:
		return
	default:
		close(a.stopCh)
	}
	<-a.doneCh
	a.watcher.Close()
	if a.logFile != nil {
		a.logFile.Close()
	}
}

// Snapshot returns the current cumulative exposure.
func (a *Accumulator) Snapshot() Exposure {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	return Exposure{
		WallClockSec: int64(now.Sub(a.lastReview).Seconds()),
		Mutations:    a.mutations,
		Tokens:       a.tokens,
		StartedAt:    a.startedAt,
		LastReview:   a.lastReview,
	}
}

// Review marks a human review checkpoint, resetting the wall-clock timer.
func (a *Accumulator) Review() {
	a.mu.Lock()
	a.lastReview = time.Now()
	a.mu.Unlock()
}

// ToRunState converts the accumulator's current state to a RunState.
func (a *Accumulator) ToRunState(pid int, watchPaths []string) RunState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return RunState{
		PID:        pid,
		StartedAt:  a.startedAt,
		LastReview: a.lastReview,
		Mutations:  a.mutations,
		Tokens:     a.tokens,
		WatchPaths: watchPaths,
		LogPath:    a.logPath,
	}
}

// ExposureFromRunState computes live exposure from a persisted run state.
// Wall-clock is measured from last_review to now.
func ExposureFromRunState(rs RunState) Exposure {
	now := time.Now()
	return Exposure{
		WallClockSec: int64(now.Sub(rs.LastReview).Seconds()),
		Mutations:    rs.Mutations,
		Tokens:       rs.Tokens,
		StartedAt:    rs.StartedAt,
		LastReview:   rs.LastReview,
	}
}
