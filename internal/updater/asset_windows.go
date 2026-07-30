//go:build windows

package updater

import (
	"fmt"
	"strings"
)

func findInstallerAsset(rel *Release) *Asset {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".exe") && strings.Contains(a.ContentType, "octet-stream") {
			return &a
		}
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".exe") {
			return &a
		}
	}
	return nil
}

func installerFilename(tag string) string {
	return fmt.Sprintf("vocab_%s_installer.exe", tag)
}
