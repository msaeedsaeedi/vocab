//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/msaeedsaeedi/vocab/internal/database"
	"golang.org/x/sys/windows"
)

const (
	mbYesNo    = 0x00000004
	mbIconInfo = 0x00000040
	idYes      = 6
)

func ensureWallpaperConsent(db *database.DB) bool {
	var value string
	if err := db.QueryRow(`SELECT value FROM daemon_state WHERE key = 'wallpaper_consent'`).Scan(&value); err == nil && value == "accepted" {
		return true
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	text, _ := windows.UTF16PtrFromString("Vocab learns through wallpaper exposure and recall notifications. It will temporarily change your desktop wallpaper while a word is being learned. Your previous wallpaper is restored when you pause or quit.\n\nStart learning?")
	title, _ := windows.UTF16PtrFromString("Welcome to Vocab")
	answer, _, err := messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbYesNo|mbIconInfo)
	if err != syscall.Errno(0) {
		log.Printf("wallpaper consent dialog: %v", err)
		return false
	}
	if answer != idYes {
		return false
	}
	if _, err := db.Exec(`INSERT INTO daemon_state (key, value) VALUES ('wallpaper_consent', 'accepted')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		log.Printf("save wallpaper consent: %v", err)
		return false
	}
	return true
}
