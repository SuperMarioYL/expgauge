// Package export writes the run's exposure timeline as JSONL
// (one snapshot per line) for offline analysis or dashboarding.
package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SuperMarioYL/expgauge/internal/metrics"
)

// WriteJSONL writes exposure snapshots as JSON lines to the given writer.
// Each line is a JSON object with timestamp, wall-clock, mutations, and tokens.
func WriteJSONL(w io.Writer, snapshots []metrics.Snapshot) error {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	for _, s := range snapshots {
		if err := enc.Encode(s); err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}
	}
	return bw.Flush()
}

// WriteFile writes snapshots as JSONL to a file path.
func WriteFile(path string, snapshots []metrics.Snapshot) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	return WriteJSONL(f, snapshots)
}

// ExportTimeline reads a JSONL timeline file and re-exports it.
// This is used by `expgauge export` to copy or filter the timeline.
func ExportTimeline(srcPath string, w io.Writer) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open timeline %s: %w", srcPath, err)
	}
	defer f.Close()

	bw := bufio.NewWriter(w)
	dec := json.NewDecoder(f)
	for {
		var snap metrics.Snapshot
		if err := dec.Decode(&snap); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode snapshot: %w", err)
		}
		enc := json.NewEncoder(bw)
		if err := enc.Encode(snap); err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}
	}
	return bw.Flush()
}

// AppendSnapshot appends a single snapshot to a JSONL timeline file.
func AppendSnapshot(path string, snap metrics.Snapshot) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open timeline %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(snap)
}
