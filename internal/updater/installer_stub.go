//go:build !windows

package updater

import "fmt"

type InstallerLauncher struct{}

func NewInstallerLauncher() *InstallerLauncher {
	return &InstallerLauncher{}
}

func (l *InstallerLauncher) Install(path string) error {
	return fmt.Errorf("installing updates is not supported on this platform")
}
