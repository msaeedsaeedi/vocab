//go:build windows

package report

import "os/exec"

// Reveal gives the user an immediate confirmation and selects the bundle they
// requested in Explorer, ready to attach to a support request.
func Reveal(path string) error {
	return exec.Command("explorer.exe", "/select,"+path).Start()
}
