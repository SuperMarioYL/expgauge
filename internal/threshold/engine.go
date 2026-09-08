// Package threshold evaluates accumulated exposure against configurable
// thresholds and triggers a forced review pause when any threshold is crossed.
package threshold

import (
	"fmt"
	"runtime"
	"syscall"

	"github.com/SuperMarioYL/expgauge/internal/metrics"
)

// Status describes how the current exposure relates to configured thresholds.
type Status int

const (
	// StatusOK means no threshold has been crossed.
	StatusOK Status = iota
	// StatusWarning means at least one dimension is at or above 80% of threshold.
	StatusWarning
	// StatusExceeded means at least one threshold has been crossed.
	StatusExceeded
)

// String returns a human-readable status label.
func (s Status) String() string {
	switch s {
	case StatusOK:
		return "OK"
	case StatusWarning:
		return "WARN"
	case StatusExceeded:
		return "EXCEEDED"
	default:
		return "UNKNOWN"
	}
}

// Thresholds defines the per-dimension review-trigger limits.
type Thresholds struct {
	WallClockSec int64
	Mutations    int
	Tokens       int64
}

// Engine evaluates exposure against a fixed set of thresholds.
type Engine struct {
	Thresholds Thresholds
}

// New creates an engine with the given thresholds.
func New(t Thresholds) *Engine {
	return &Engine{Thresholds: t}
}

// Result captures the outcome of an evaluation.
type Result struct {
	Status     Status
	ExceededBy []string
	WallClockPct float64
	MutationsPct float64
	TokensPct  float64
}

// Evaluate checks the exposure against all three thresholds.
func (e *Engine) Evaluate(exp metrics.Exposure) Result {
	var exceeded []string
	wallPct := pct(exp.WallClockSec, e.Thresholds.WallClockSec)
	mutPct := pct(int64(exp.Mutations), int64(e.Thresholds.Mutations))
	tokPct := pct(exp.Tokens, e.Thresholds.Tokens)

	if exp.WallClockSec >= e.Thresholds.WallClockSec && e.Thresholds.WallClockSec > 0 {
		exceeded = append(exceeded, "wall_clock")
	}
	if exp.Mutations >= e.Thresholds.Mutations && e.Thresholds.Mutations > 0 {
		exceeded = append(exceeded, "mutations")
	}
	if exp.Tokens >= e.Thresholds.Tokens && e.Thresholds.Tokens > 0 {
		exceeded = append(exceeded, "tokens")
	}

	st := StatusOK
	if len(exceeded) > 0 {
		st = StatusExceeded
	} else if wallPct >= 0.8 || mutPct >= 0.8 || tokPct >= 0.8 {
		st = StatusWarning
	}

	return Result{
		Status:       st,
		ExceededBy:   exceeded,
		WallClockPct: wallPct,
		MutationsPct: mutPct,
		TokensPct:    tokPct,
	}
}

// pct returns current/limit as a fraction in [0, 1+].
func pct(current, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	return float64(current) / float64(limit)
}

// ForcePause sends SIGTSTP to the agent process so it stops until reviewed.
// On non-Unix systems this is a no-op (returns nil) since signals are not
// available; the TUI review prompt provides the blocking there.
func ForcePause(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTSTP)
}

// ForceResume sends SIGCONT to resume a paused agent process.
func ForceResume(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGCONT)
}
