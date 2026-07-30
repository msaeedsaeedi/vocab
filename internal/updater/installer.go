package updater

//go:build windows
// +build windows

import (
	"fmt"
	"os/exec"
)

type WindowsInstallerLauncher struct{}

func NewWindowsInstallerLauncher() *WindowsInstallerLauncher {
	return &WindowsInstallerLauncher{}
}

func (l *WindowsInstallerLauncher) Install(path string) error {
	cmd := exec.Command(path, "/S")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch windows installer: %w", err)
	}
	return nil
}
