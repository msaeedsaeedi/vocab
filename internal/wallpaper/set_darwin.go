//go:build darwin

package wallpaper

import (
	"fmt"
	"os/exec"
)

func Set(path string) error {
	script := fmt.Sprintf(
		`tell application "System Events" to set picture of every desktop to POSIX file %q`,
		path,
	)
	return exec.Command("osascript", "-e", script).Run()
}
