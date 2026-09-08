// Command expgauge is a run-level exposure gauge for coding agents on
// local models. It tracks cumulative unsupervised exposure (wall-clock,
// state mutations, token burn) and forces a human review checkpoint at
// configurable thresholds.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/expgauge/internal/config"
	"github.com/SuperMarioYL/expgauge/internal/export"
	"github.com/SuperMarioYL/expgauge/internal/metrics"
	"github.com/SuperMarioYL/expgauge/internal/threshold"
	"github.com/SuperMarioYL/expgauge/internal/ui"
	"github.com/SuperMarioYL/expgauge/internal/watcher"
)

const (
	stateFileName     = "run.json"
	timelineFileName  = "timeline.jsonl"
	snapshotInterval  = time.Minute
)

var (
	watchPID    int
	watchPaths  []string
	watchLog    string
	exportOut   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "expgauge",
		Short: "Run-level exposure gauge for coding agents",
		Long:  "expgauge tracks cumulative unsupervised exposure of coding agents on local models — wall-clock time, state mutations, and token burn since the last human review — and forces a pause at configurable thresholds.",
	}
	rootCmd.AddCommand(newWatchCmd(), newStatusCmd(), newReviewCmd(), newExportCmd())
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Start monitoring an agent and show live exposure",
		RunE:  runWatch,
	}
	cmd.Flags().IntVarP(&watchPID, "pid", "p", 0, "agent process PID (overrides config)")
	cmd.Flags().StringSliceVarP(&watchPaths, "paths", "w", nil, "directories to watch for mutations")
	cmd.Flags().StringVarP(&watchLog, "log", "l", "", "agent log file path for token parsing")
	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current run's cumulative exposure",
		RunE:  runStatus,
	}
}

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review",
		Short: "Mark a review checkpoint (resets wall-clock)",
		RunE:  runReview,
	}
}

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the run's exposure timeline as JSONL",
		RunE:  runExport,
	}
	cmd.Flags().StringVarP(&exportOut, "output", "o", "", "output file (default: stdout)")
	return cmd
}

// runWatch starts the live monitoring loop and TUI badge.
func runWatch(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(watchPaths) > 0 {
		cfg.WatchPaths = watchPaths
	}
	if watchLog != "" {
		cfg.Agent.LogPath = watchLog
	}

	// Locate the agent process. This is best-effort: if no PID is found,
	// we still track filesystem mutations and show the badge.
	pid := watchPID
	if pid == 0 {
		proc, perr := watcher.FindProcess(cfg.Agent.PidFile, cfg.Agent.NamePattern, 0)
		if perr == nil {
			pid = proc.PID
		}
	}

	if len(cfg.WatchPaths) == 0 {
		cwd, _ := os.Getwd()
		cfg.WatchPaths = []string{cwd}
	}

	acc, err := metrics.NewAccumulator(cfg.WatchPaths, cfg.Agent.LogPath)
	if err != nil {
		return fmt.Errorf("create accumulator: %w", err)
	}
	if err := acc.Start(); err != nil {
		return err
	}
	defer acc.Stop()

	// Persist initial state so status/review/export work during the run.
	rs := acc.ToRunState(pid, cfg.WatchPaths)
	_ = saveState(rs)

	// Background state + timeline saver.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go stateSaver(ctx, acc, pid, cfg.WatchPaths)

	eng := threshold.New(threshold.Thresholds{
		WallClockSec: cfg.Thresholds.WallClockSec,
		Mutations:    cfg.Thresholds.Mutations,
		Tokens:       cfg.Thresholds.Tokens,
	})

	onReview := func() {
		acc.Review()
		_ = saveState(acc.ToRunState(pid, cfg.WatchPaths))
	}

	badge := ui.NewBadge(acc.Snapshot, eng, pid, onReview)
	if err := ui.Run(badge); err != nil {
		return fmt.Errorf("run badge: %w", err)
	}

	// Final state save on exit.
	_ = saveState(acc.ToRunState(pid, cfg.WatchPaths))
	return nil
}

// stateSaver periodically writes run state and appends timeline snapshots.
func stateSaver(ctx context.Context, acc *metrics.Accumulator, pid int, paths []string) {
	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()
	tlPath, _ := timelinePath()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			exp := acc.Snapshot()
			_ = saveState(acc.ToRunState(pid, paths))
			_ = export.AppendSnapshot(tlPath, metrics.Snapshot{
				Timestamp:    time.Now(),
				WallClockSec: exp.WallClockSec,
				Mutations:    exp.Mutations,
				Tokens:       exp.Tokens,
			})
		}
	}
}

// runStatus prints the current run's cumulative exposure.
func runStatus(cmd *cobra.Command, args []string) error {
	rs, err := loadState()
	if err != nil {
		return fmt.Errorf("no active run found (run 'expgauge watch' first): %w", err)
	}
	exp := metrics.ExposureFromRunState(*rs)

	fmt.Printf("expgauge status\n")
	fmt.Printf("  pid:         %d\n", rs.PID)
	fmt.Printf("  started:     %s\n", rs.StartedAt.Format(time.RFC3339))
	fmt.Printf("  last review: %s\n", rs.LastReview.Format(time.RFC3339))
	fmt.Printf("  wall-clock:  %s\n", fmtDuration(exp.WallClockSec))
	fmt.Printf("  mutations:   %d\n", exp.Mutations)
	fmt.Printf("  tokens:      %s\n", fmtTokens(exp.Tokens))
	if rs.Paused {
		fmt.Printf("  state:       PAUSED (awaiting review)\n")
	}
	return nil
}

// runReview marks a human review checkpoint, resetting the wall-clock timer.
func runReview(cmd *cobra.Command, args []string) error {
	rs, err := loadState()
	if err != nil {
		return fmt.Errorf("no active run found (run 'expgauge watch' first): %w", err)
	}
	rs.LastReview = time.Now()
	rs.Paused = false
	if err := saveState(*rs); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	fmt.Println("review checkpoint recorded — wall-clock timer reset")
	return nil
}

// runExport writes the run's exposure timeline as JSONL.
func runExport(cmd *cobra.Command, args []string) error {
	tlPath, err := timelinePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(tlPath); err != nil {
		return fmt.Errorf("no timeline found (run 'expgauge watch' first): %w", err)
	}
	if exportOut != "" {
		return export.ExportTimeline(tlPath, fileWriter(exportOut))
	}
	return export.ExportTimeline(tlPath, os.Stdout)
}

// --- state file helpers ---

func statePath() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

func timelinePath() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, timelineFileName), nil
}

func saveState(rs metrics.RunState) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadState() (*metrics.RunState, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rs metrics.RunState
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	return &rs, nil
}

func fileWriter(path string) *os.File {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create %s: %v\n", path, err)
		os.Exit(1)
	}
	return f
}

// fmtDuration renders seconds as h m s.
func fmtDuration(sec int64) string {
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// fmtTokens renders token counts with K/M suffixes.
func fmtTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
