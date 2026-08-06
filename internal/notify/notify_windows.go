//go:build windows

package notify

import (
	"fmt"
	"path/filepath"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

func RegisterApp(exePath string) error {
	return toast.SetAppData(toast.AppData{
		AppID:         "Vocab",
		ActivationExe: exePath,
		IconPath:      filepath.Join(filepath.Dir(exePath), "icon.ico"),
	})
}

func setActivationCallback(callback func(arguments string)) {
	toast.SetActivationCallback(func(arguments string, _ []toast.UserData) {
		callback(arguments)
	})
}

func Send(w Word) error {
	t := toast.Notification{
		AppID: "Vocab",
		Title: "Vocab",
		Body:  "Do you remember the meaning of \"" + w.Text + "\"?",
		Actions: []toast.Action{
			{
				Type:      toast.Foreground,
				Content:   "Knew it",
				Arguments: fmt.Sprintf("--review %d --rating 2", w.ID),
			},
			{
				Type:      toast.Foreground,
				Content:   "Struggled",
				Arguments: fmt.Sprintf("--review %d --rating 1", w.ID),
			},
			{
				Type:      toast.Foreground,
				Content:   "Forgot",
				Arguments: fmt.Sprintf("--review %d --rating 0", w.ID),
			},
		},
	}
	return t.Push()
}

func sendStatus(message string) error {
	n := toast.Notification{
		AppID: "Vocab",
		Title: "Vocab",
		Body:  message,
	}
	return n.Push()
}

func sendProduction(w Word) error {
	t := toast.Notification{
		AppID: "Vocab",
		Title: "Vocab",
		Body:  "Can you use \"" + w.Text + "\" in a sentence?",
		Actions: []toast.Action{
			{
				Type:      toast.Foreground,
				Content:   "Got it",
				Arguments: fmt.Sprintf("--produce %d --produced", w.ID),
			},
			{
				Type:      toast.Foreground,
				Content:   "Couldn't",
				Arguments: fmt.Sprintf("--produce %d", w.ID),
			},
		},
	}
	return t.Push()
}
