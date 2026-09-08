package threshold

import (
	"testing"
	"time"

	"github.com/SuperMarioYL/expgauge/internal/metrics"
)

func TestEvaluate_OK(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 7200,
		Mutations:    50,
		Tokens:       1_000_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 600,
		Mutations:    5,
		Tokens:       100_000,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusOK {
		t.Errorf("status = %s, want OK", r.Status)
	}
	if len(r.ExceededBy) != 0 {
		t.Errorf("exceeded_by = %v, want empty", r.ExceededBy)
	}
}

func TestEvaluate_Warning(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 7200,
		Mutations:    50,
		Tokens:       1_000_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 6000, // ~83% of 7200
		Mutations:    5,
		Tokens:       100_000,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusWarning {
		t.Errorf("status = %s, want WARN", r.Status)
	}
}

func TestEvaluate_ExceededWallClock(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 7200,
		Mutations:    50,
		Tokens:       1_000_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 7300,
		Mutations:    10,
		Tokens:       50_000,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusExceeded {
		t.Errorf("status = %s, want EXCEEDED", r.Status)
	}
	if len(r.ExceededBy) != 1 || r.ExceededBy[0] != "wall_clock" {
		t.Errorf("exceeded_by = %v, want [wall_clock]", r.ExceededBy)
	}
}

func TestEvaluate_ExceededMutations(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 7200,
		Mutations:    50,
		Tokens:       1_000_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 100,
		Mutations:    55,
		Tokens:       10_000,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusExceeded {
		t.Errorf("status = %s, want EXCEEDED", r.Status)
	}
	if len(r.ExceededBy) != 1 || r.ExceededBy[0] != "mutations" {
		t.Errorf("exceeded_by = %v, want [mutations]", r.ExceededBy)
	}
}

func TestEvaluate_ExceededTokens(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 7200,
		Mutations:    50,
		Tokens:       1_000_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 100,
		Mutations:    5,
		Tokens:       1_200_000,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusExceeded {
		t.Errorf("status = %s, want EXCEEDED", r.Status)
	}
	if len(r.ExceededBy) != 1 || r.ExceededBy[0] != "tokens" {
		t.Errorf("exceeded_by = %v, want [tokens]", r.ExceededBy)
	}
}

func TestEvaluate_MultipleExceeded(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 7200,
		Mutations:    50,
		Tokens:       1_000_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 8000,
		Mutations:    60,
		Tokens:       1_500_000,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusExceeded {
		t.Errorf("status = %s, want EXCEEDED", r.Status)
	}
	if len(r.ExceededBy) != 3 {
		t.Errorf("exceeded_by = %v, want 3 entries", r.ExceededBy)
	}
}

func TestEvaluate_ZeroThresholds(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 0,
		Mutations:    0,
		Tokens:       0,
	})
	exp := metrics.Exposure{
		WallClockSec: 999999,
		Mutations:    999,
		Tokens:       999_999_999,
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusOK {
		t.Errorf("with zero thresholds, status = %s, want OK", r.Status)
	}
}

func TestPct(t *testing.T) {
	tests := []struct {
		current, limit int64
		want           float64
	}{
		{50, 100, 0.5},
		{100, 100, 1.0},
		{150, 100, 1.5},
		{0, 100, 0.0},
		{50, 0, 0.0}, // zero limit returns 0
	}
	for _, tt := range tests {
		if got := pct(tt.current, tt.limit); got != tt.want {
			t.Errorf("pct(%d, %d) = %f, want %f", tt.current, tt.limit, got, tt.want)
		}
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusOK, "OK"},
		{StatusWarning, "WARN"},
		{StatusExceeded, "EXCEEDED"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestForcePause_InvalidPID(t *testing.T) {
	if err := ForcePause(0); err == nil {
		t.Error("ForcePause(0) should return error")
	}
	if err := ForcePause(-1); err == nil {
		t.Error("ForcePause(-1) should return error")
	}
}

func TestForceResume_InvalidPID(t *testing.T) {
	if err := ForceResume(0); err == nil {
		t.Error("ForceResume(0) should return error")
	}
}

// TestExposureWithRealTime ensures the engine evaluates against a real
// time-based exposure (not a mock).
func TestExposureWithRealTime(t *testing.T) {
	eng := New(Thresholds{
		WallClockSec: 3600,
		Mutations:    100,
		Tokens:       500_000,
	})
	exp := metrics.Exposure{
		WallClockSec: 0,
		Mutations:    0,
		Tokens:       0,
		StartedAt:    time.Now(),
		LastReview:   time.Now(),
	}
	r := eng.Evaluate(exp)
	if r.Status != StatusOK {
		t.Errorf("fresh exposure status = %s, want OK", r.Status)
	}
}
