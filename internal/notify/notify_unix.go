//go:build !windows

package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

func RegisterApp(exePath string) error {
	return fmt.Errorf("notification registration not needed on %s", runtime.GOOS)
}

func Send(w Word) error {
	switch runtime.GOOS {
	case "linux":
		return sendLinux(w)
	case "darwin":
		return sendDarwin(w)
	default:
		return fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}
}

func sendLinux(w Word) error {
	body := w.Definition
	if w.Example != "" {
		body += "\n\"" + w.Example + "\""
	}
	return exec.Command("notify-send",
		"-a", "Vocab",
		"-i", "dialog-information",
		w.Text,
		body,
	).Run()
}

func sendDarwin(w Word) error {
	script := fmt.Sprintf(
		`display notification %q with title %q subtitle %q`,
		w.Definition+" - \""+w.Example+"\"",
		"Vocab",
		w.Text,
	)
	return exec.Command("osascript", "-e", script).Run()
}
