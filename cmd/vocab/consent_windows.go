//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/msaeedsaeedi/vocab/internal/state"
	"golang.org/x/sys/windows"
)

const (
	mbYesNo    = 0x00000004
	mbIconInfo = 0x00000040
	idYes      = 6
)

func ensureWallpaperConsent(store *state.Store) bool {
	if store.WallpaperConsentAccepted() {
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
	if err := store.AcceptWallpaperConsent(); err != nil {
		log.Printf("%v", err)
		return false
	}
	return true
}
