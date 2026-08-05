//go:build windows

// Package tray provides the small set of desktop controls Vocab needs. This is
// a compact Win32 tray implementation: keeping it in tree avoids a GUI toolkit,
// cgo, and an extra runtime dependency.
package tray

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Actions struct {
	LearnNow func()
	Quit     func()
}

const (
	wmDestroy         = 0x0002
	wmCommand         = 0x0111
	wmContextMenu     = 0x007B
	wmUser            = 0x0400
	wmTray            = wmUser + 1
	wmRButtonUp       = 0x0205
	wmLButtonUp       = 0x0202
	pmRemove          = 0x0001
	niMessage         = 0x0001
	niIcon            = 0x0002
	niTip             = 0x0004
	nimAdd            = 0x0000
	nimDelete         = 0x0002
	nimSetVersion     = 0x0004
	mfString          = 0x0000
	mfSeparator       = 0x0800
	tpmRightButton    = 0x0002
	cmdLearnNow       = 1001
	cmdQuit           = 1002
	imageIcon         = 1
	lrDefaultSize     = 0x0040
	lrLoadFromFile    = 0x0010
	notifyIconVersion = 4
	appIconResource   = 1
	idiApplication    = 32512
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procPostMessage         = user32.NewProc("PostMessageW")
	procLoadImage           = user32.NewProc("LoadImageW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
)

type point struct{ X, Y int32 }
type msg struct {
	Hwnd           windows.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type wndClassEx struct {
	Size                               uint32
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance                           windows.Handle
	Icon, Cursor, Background, MenuName uintptr
	ClassName                          *uint16
	IconSm                             uintptr
}
type notifyIconData struct {
	Size             uint32
	Hwnd             windows.Handle
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             windows.Handle
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GuidItem         windows.GUID
	BalloonIcon      windows.Handle
}

var activeActions Actions

func Run(ctx context.Context, actions Actions) {
	activeActions = actions
	go func() { <-ctx.Done(); procPostQuitMessage.Call(0) }()
	if err := run(); err != nil {
		log.Printf("tray: %v", err)
	}
}

func run() error {
	instance, _, err := procGetModuleHandle.Call(0)
	if instance == 0 {
		return err
	}
	class, _ := windows.UTF16PtrFromString("VocabTrayWindow")
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(windowProc), Instance: windows.Handle(instance), ClassName: class}
	if atom, _, e := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return e
	}
	hwnd, _, e := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return e
	}
	defer procDestroyWindow.Call(hwnd)
	if err := addIcon(windows.Handle(hwnd)); err != nil {
		return err
	}
	defer deleteIcon(windows.Handle(hwnd))
	var m msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			return nil
		}
		if int(ret) == -1 {
			return syscall.EINVAL
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func windowProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case wmTray:
		// With NIM_SETVERSION at NOTIFYICON_VERSION_4 (Vista+), right-clicks
		// arrive as WM_CONTEXTMENU, not WM_RBUTTONUP. In that protocol lParam's
		// high word contains the icon ID, so compare only its low word. Handle
		// both forms so the menu also works if the version negotiation fails.
		event := uint32(uint16(lparam))
		if event == wmRButtonUp || event == wmLButtonUp || event == wmContextMenu {
			showMenu(windows.Handle(hwnd))
		}
	case wmCommand:
		switch uint16(wparam) {
		case cmdLearnNow:
			if activeActions.LearnNow != nil {
				go activeActions.LearnNow()
			}
		case cmdQuit:
			if activeActions.Quit != nil {
				go activeActions.Quit()
			}
		}
	case wmDestroy:
		procPostQuitMessage.Call(0)
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return r
}

func addIcon(hwnd windows.Handle) error {
	icon := loadTrayIcon()
	if icon == 0 {
		return fmt.Errorf("tray: no application icon could be loaded")
	}
	n := notifyIconData{
		Size:            uint32(unsafe.Sizeof(notifyIconData{})),
		Hwnd:            hwnd,
		ID:              1,
		Flags:           niMessage | niIcon | niTip,
		CallbackMessage: wmTray,
		Icon:            icon,
	}
	copy(n.Tip[:], windows.StringToUTF16("Vocab - Learn now or Quit"))
	if ok, _, err := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&n))); ok == 0 {
		return err
	}
	// Windows Vista+ icon version so the shell renders the icon and forwards
	// callbacks reliably on Windows 10/11. Only cbSize, hWnd, uID, and uVersion
	// are read for NIM_SETVERSION; the rest must be zero.
	n.Size = uint32(unsafe.Sizeof(notifyIconData{}))
	n.Flags = 0
	n.Icon = 0
	n.Tip = [128]uint16{}
	n.Info = [256]uint16{}
	n.InfoTitle = [64]uint16{}
	n.TimeoutOrVersion = notifyIconVersion
	if ok, _, err := procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&n))); ok == 0 {
		log.Printf("tray: NIM_SETVERSION failed: %v", err)
	}
	return nil
}

// loadTrayIcon prefers the icon.ico file installed next to the executable. It
// is loaded from disk because Go does not embed Win32 resources, so resource 1
// is usually absent; IDI_APPLICATION is only a last-resort fallback.
func loadTrayIcon() windows.Handle {
	if exe, err := os.Executable(); err == nil {
		path, _ := windows.UTF16PtrFromString(filepath.Join(filepath.Dir(exe), "icon.ico"))
		if icon, _, _ := procLoadImage.Call(0, uintptr(unsafe.Pointer(path)), imageIcon, 16, 16, lrLoadFromFile); icon != 0 {
			return windows.Handle(icon)
		}
	}
	instance, _, _ := procGetModuleHandle.Call(0)
	if icon, _, _ := procLoadImage.Call(instance, appIconResource, imageIcon, 16, 16, 0); icon != 0 {
		return windows.Handle(icon)
	}
	if icon, _, _ := procLoadImage.Call(0, idiApplication, imageIcon, 0, 0, lrDefaultSize); icon != 0 {
		return windows.Handle(icon)
	}
	return 0
}

func deleteIcon(hwnd windows.Handle) {
	n := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Hwnd: hwnd, ID: 1}
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&n)))
}

func showMenu(hwnd windows.Handle) {
	menu, _, _ := procCreatePopupMenu.Call()
	defer procDestroyMenu.Call(menu)
	learn, _ := windows.UTF16PtrFromString("Learn now")
	quit, _ := windows.UTF16PtrFromString("Quit")
	procAppendMenu.Call(menu, mfString, cmdLearnNow, uintptr(unsafe.Pointer(learn)))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	procAppendMenu.Call(menu, mfString, cmdQuit, uintptr(unsafe.Pointer(quit)))
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWindow.Call(uintptr(hwnd))
	procTrackPopupMenu.Call(menu, tpmRightButton, uintptr(p.X), uintptr(p.Y), 0, uintptr(hwnd), 0)
	// Posting WM_NULL lets the menu's modal loop exit cleanly when the user
	// clicks elsewhere, so the tray stays responsive on the first attempt.
	procPostMessage.Call(uintptr(hwnd), 0, 0, 0)
}
