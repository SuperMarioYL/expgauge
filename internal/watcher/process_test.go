package watcher

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadPidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.pid")
	if err := os.WriteFile(path, []byte("12345"), 0644); err != nil {
		t.Fatal(err)
	}
	pid, err := readPidFile(path)
	if err != nil {
		t.Fatalf("readPidFile: %v", err)
	}
	if pid != 12345 {
		t.Errorf("pid = %d, want 12345", pid)
	}
}

func TestReadPidFile_Whitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.pid")
	if err := os.WriteFile(path, []byte("  67890\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pid, err := readPidFile(path)
	if err != nil {
		t.Fatalf("readPidFile: %v", err)
	}
	if pid != 67890 {
		t.Errorf("pid = %d, want 67890", pid)
	}
}

func TestReadPidFile_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.pid")
	if err := os.WriteFile(path, []byte("not-a-number"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := readPidFile(path)
	if err == nil {
		t.Error("expected error for non-numeric pid file")
	}
}

func TestReadPidFile_Missing(t *testing.T) {
	_, err := readPidFile("/nonexistent/agent.pid")
	if err == nil {
		t.Error("expected error for missing pid file")
	}
}

func TestFindProcess_ExplicitPID_Self(t *testing.T) {
	pid := os.Getpid()
	proc, err := FindProcess("", "", pid)
	if err != nil {
		t.Fatalf("FindProcess(self): %v", err)
	}
	if proc.PID != pid {
		t.Errorf("pid = %d, want %d", proc.PID, pid)
	}
}

func TestFindProcess_NoConfig(t *testing.T) {
	_, err := FindProcess("", "", 0)
	if err == nil {
		t.Error("expected error when no pid/pattern configured")
	}
}

func TestNameForPID(t *testing.T) {
	pid := os.Getpid()
	name := nameForPID(pid)
	if name == "" {
		t.Error("name should not be empty")
	}
	// On macOS there's no /proc, so it falls back to pid-N format.
	if name != "pid-"+strconv.Itoa(pid) && name != "" {
		// This is fine — either /proc name or fallback.
		t.Logf("name for self: %q", name)
	}
}

func TestPidAlive_Self(t *testing.T) {
	if err := pidAlive(os.Getpid()); err != nil {
		t.Errorf("pidAlive(self) should be nil, got: %v", err)
	}
}

func TestPidAlive_Dead(t *testing.T) {
	// PID 1 on macOS is launchd, so test with a very high PID that
	// almost certainly doesn't exist.
	if err := pidAlive(999999); err == nil {
		t.Error("pidAlive(999999) should return error")
	}
}
