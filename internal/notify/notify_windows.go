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

func Send(w Word) error {
	t := toast.Notification{
		AppID: "Vocab",
		Title: "Vocab",
		Body:  "Do you remember the meaning of \"" + w.Text + "\"?",
		Actions: []toast.Action{
			{
				Type:      toast.Foreground,
				Content:   "Knew it",
				Arguments: fmt.Sprintf("--review %d --knew", w.ID),
			},
			{
				Type:      toast.Foreground,
				Content:   "Didn't know",
				Arguments: fmt.Sprintf("--review %d --knew=false", w.ID),
			},
		},
	}
	return t.Push()
}
