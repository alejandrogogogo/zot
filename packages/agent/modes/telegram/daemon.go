package telegram

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PIDPath returns the location of the bot's pid file.
func PIDPath(zotHome string) string {
	return filepath.Join(zotHome, "bot.pid")
}

// LogPath returns the location of the bot's log file (stdout+stderr
// from a detached `zot bot start`).
func LogPath(zotHome string) string {
	return filepath.Join(zotHome, "logs", "bot.log")
}

// WritePID persists pid to bot.pid. Overwrites any existing file.
func WritePID(zotHome string, pid int) error {
	p := PIDPath(zotHome)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(strconv.Itoa(pid)+"\n"), 0o644)
}

// ReadPID returns the pid stored in bot.pid, or 0 if the file doesn't
// exist. Returns an error for any other read/parse failure.
func ReadPID(zotHome string) (int, error) {
	b, err := os.ReadFile(PIDPath(zotHome))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}

// RemovePID deletes the pid file if it exists.
func RemovePID(zotHome string) error {
	err := os.Remove(PIDPath(zotHome))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// IsRunning returns (pid, true) if a live process with the recorded
// pid exists, or (pid, false) if the pid file points to a dead process.
// Stale pid files are left in place; the caller may remove them.
func IsRunning(zotHome string) (int, bool, error) {
	pid, err := ReadPID(zotHome)
	if err != nil {
		return 0, false, err
	}
	if pid <= 0 {
		return 0, false, nil
	}
	alive, err := processAlive(pid)
	if err != nil {
		return pid, false, nil
	}
	return pid, alive, nil
}

// StopProcess asks pid to exit and waits up to graceful for it to stop,
// then escalates to a forced kill. Returns nil if the process is gone.
func StopProcess(pid int, graceful time.Duration) error {
	return stopProcess(pid, graceful)
}
