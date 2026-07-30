//go:build !windows

package updater

func findInstallerAsset(rel *Release) *Asset {
	return nil
}

func installerFilename(tag string) string {
	return ""
}
