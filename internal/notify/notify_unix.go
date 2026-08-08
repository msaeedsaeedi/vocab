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

func setActivationCallback(func(arguments string)) {}

func sendStatus(message string) error {
	return exec.Command("notify-send", "-a", "Vocab", "Vocab", message).Run()
}

func sendStatusLink(message, _, _ string) error {
	return sendStatus(message)
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
	body := fmt.Sprintf("Do you remember the meaning of %q?\nvocab -review %d -rating 0|1|2  (0=forgot, 1=struggled, 2=knew)", w.Text, w.ID)
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

func sendProduction(w Word) error {
	switch runtime.GOOS {
	case "linux":
		body := fmt.Sprintf("Can you use %q in a sentence?\nvocab -produce %d -produced  (or omit -produced)", w.Text, w.ID)
		return exec.Command("notify-send",
			"-a", "Vocab",
			"-i", "dialog-information",
			"Vocab",
			body,
		).Run()
	case "darwin":
		script := fmt.Sprintf(
			`display notification %q with title %q`,
			"Can you use \""+w.Text+"\" in a sentence?\nvocab -produce "+fmt.Sprint(w.ID)+" [-produced]",
			"Vocab",
		)
		return exec.Command("osascript", "-e", script).Run()
	default:
		return fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}
}
