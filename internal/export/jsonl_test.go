package export

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SuperMarioYL/expgauge/internal/metrics"
)

func TestWriteJSONL(t *testing.T) {
	snapshots := []metrics.Snapshot{
		{Timestamp: time.Now(), WallClockSec: 60, Mutations: 1, Tokens: 100},
		{Timestamp: time.Now(), WallClockSec: 120, Mutations: 3, Tokens: 300},
	}
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, snapshots); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var first metrics.Snapshot
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if first.WallClockSec != 60 {
		t.Errorf("line 0 wall_clock_sec = %d, want 60", first.WallClockSec)
	}
	if first.Mutations != 1 {
		t.Errorf("line 0 mutations = %d, want 1", first.Mutations)
	}
}

func TestWriteJSONL_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, nil); err != nil {
		t.Fatalf("WriteJSONL(nil): %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty input should produce empty output, got %d bytes", buf.Len())
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "timeline.jsonl")
	snapshots := []metrics.Snapshot{
		{Timestamp: time.Now(), WallClockSec: 30, Mutations: 2, Tokens: 50},
	}
	if err := WriteFile(path, snapshots); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(data) == 0 {
		t.Error("file should not be empty")
	}
}

func TestAppendSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "timeline.jsonl")
	snap1 := metrics.Snapshot{Timestamp: time.Now(), WallClockSec: 60, Mutations: 1, Tokens: 100}
	snap2 := metrics.Snapshot{Timestamp: time.Now(), WallClockSec: 120, Mutations: 3, Tokens: 300}

	if err := AppendSnapshot(path, snap1); err != nil {
		t.Fatalf("AppendSnapshot 1: %v", err)
	}
	if err := AppendSnapshot(path, snap2); err != nil {
		t.Fatalf("AppendSnapshot 2: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestExportTimeline(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "timeline.jsonl")
	snapshots := []metrics.Snapshot{
		{Timestamp: time.Now(), WallClockSec: 60, Mutations: 1, Tokens: 100},
		{Timestamp: time.Now(), WallClockSec: 120, Mutations: 3, Tokens: 300},
	}
	if err := WriteFile(srcPath, snapshots); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	var buf bytes.Buffer
	if err := ExportTimeline(srcPath, &buf); err != nil {
		t.Fatalf("ExportTimeline: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestExportTimeline_MissingFile(t *testing.T) {
	var buf bytes.Buffer
	err := ExportTimeline("/nonexistent/timeline.jsonl", &buf)
	if err == nil {
		t.Error("expected error for missing file")
	}
}
