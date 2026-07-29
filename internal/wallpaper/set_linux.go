//go:build linux

package wallpaper

import (
	"fmt"
	"os/exec"
)

func Set(path string) error {
	if err := exec.Command("gsettings", "set",
		"org.gnome.desktop.background", "picture-uri", "file://"+path).Run(); err != nil {
		if err2 := exec.Command("gsettings", "set",
			"org.gnome.desktop.background", "picture-uri-dark", "file://"+path).Run(); err2 != nil {
			if err3 := exec.Command("feh", "--bg-fill", path).Run(); err3 != nil {
				return fmt.Errorf("set wallpaper: gsettings: %v; feh: %v", err, err3)
			}
		}
	}
	return nil
}
