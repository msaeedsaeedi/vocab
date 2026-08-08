// Package apppaths centralizes every on-disk location Vocab uses, so the
// various subsystems agree on where state, logs, and reports live.
package apppaths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DataDir is the per-user directory that holds vocab.db and the daemon's
// ephemeral state. On Windows it is %AppData%\vocab; elsewhere
// $XDG_DATA_HOME/vocab (falling back to ~/.local/share/vocab).
func DataDir() (string, error) {
	if runtime.GOOS == "windows" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home dir: %w", err)
		}
		return filepath.Join(home, "AppData", "Roaming", "vocab"), nil
	}
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "vocab"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "vocab"), nil
}

// LogDir returns the directory that holds vocab.log and its rotated copy.
func LogDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}

// ReportDir returns the directory that holds diagnostic report bundles.
func ReportDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reports"), nil
}

// CommandPath returns the mailbox file that lets a short-lived CLI invocation
// signal a running daemon.
func CommandPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon-command"), nil
}

// WallpaperImagePath returns where the rendered wallpaper image is written.
func WallpaperImagePath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "wallpaper.jpg"), nil
}
