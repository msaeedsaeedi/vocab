//go:build windows

package main

import (
	"os/exec"
)

// revealReport gives the user an immediate confirmation and selects the bundle
// they requested in Explorer, ready to attach to a support request.
func revealReport(path string) error {
	return exec.Command("explorer.exe", "/select,"+path).Start()
}
