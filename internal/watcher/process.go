// Package watcher detects and monitors coding-agent processes on the
// local machine. It supports locating a process by PID file, by name
// pattern (via pgrep), or by explicit PID.
package watcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Process describes a detected coding-agent process.
type Process struct {
	PID  int
	Name string
}

// FindProcess locates a coding-agent process using the provided config.
// It tries, in order: explicit pid, PID file, name pattern (pgrep).
// Returns an error if no process can be found.
func FindProcess(pidFile, namePattern string, explicitPID int) (*Process, error) {
	if explicitPID > 0 {
		if err := pidAlive(explicitPID); err != nil {
			return nil, fmt.Errorf("pid %d not alive: %w", explicitPID, err)
		}
		return &Process{PID: explicitPID, Name: nameForPID(explicitPID)}, nil
	}

	if pidFile != "" {
		pid, err := readPidFile(pidFile)
		if err == nil && pid > 0 {
			if err := pidAlive(pid); err == nil {
				return &Process{PID: pid, Name: nameForPID(pid)}, nil
			}
		}
	}

	if namePattern != "" {
		pid, err := findByName(namePattern)
		if err == nil && pid > 0 {
			return &Process{PID: pid, Name: namePattern}, nil
		}
	}

	return nil, fmt.Errorf("no coding-agent process found — configure agent.pid_file, agent.name_pattern, or pass --pid")
}

// readPidFile reads a PID from the given file path.
func readPidFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid file %s: %w", path, err)
	}
	return pid, nil
}

// findByName uses pgrep -f to find a process matching the name pattern.
func findByName(pattern string) (int, error) {
	out, err := exec.Command("pgrep", "-f", pattern).Output()
	if err != nil {
		return 0, fmt.Errorf("pgrep %q: %w", pattern, err)
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return 0, fmt.Errorf("no process matched pattern %q", pattern)
	}
	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return 0, fmt.Errorf("parse pgrep output: %w", err)
	}
	return pid, nil
}

// pidAlive returns nil if the process is running.
func pidAlive(pid int) error {
	// Signal 0 checks existence without sending a real signal.
	return syscall.Kill(pid, syscall.Signal(0))
}

// nameForPID returns a human-readable name for the PID.
func nameForPID(pid int) string {
	if comm, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm")); err == nil {
		return strings.TrimSpace(string(comm))
	}
	return fmt.Sprintf("pid-%d", pid)
}
