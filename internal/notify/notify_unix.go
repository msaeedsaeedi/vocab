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
	body := fmt.Sprintf("Do you remember the meaning of %q?", w.Text)
	return exec.Command("notify-send",
		"-a", "Vocab",
		"-i", "dialog-information",
		"Vocab",
		body,
	).Run()
}

func sendDarwin(w Word) error {
	script := fmt.Sprintf(
		`display notification %q with title %q`,
		"Do you remember the meaning of \""+w.Text+"\"?",
		"Vocab",
	)
	return exec.Command("osascript", "-e", script).Run()
}
