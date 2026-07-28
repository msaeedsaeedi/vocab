//go:build linux

package main

/*
#cgo linux pkg-config: appindicator3-0.1
#cgo linux CFLAGS: -DUSE_LEGACY_APPINDICATOR
#include "systray_linux.h"
#include <stdlib.h>
*/
import "C"
import (
	"log"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var menuClickCh = make(chan int, 8)

//export goMenuItemClicked
func goMenuItemClicked(id C.int) {
	menuClickCh <- int(id)
}

func (a *App) startSystray() {
	if rc := C.setup_indicator(); rc != 0 {
		log.Println("system tray not available on this platform")
		return
	}

	if len(appIconBytes) > 0 {
		C.set_tray_icon((*C.char)(unsafe.Pointer(&appIconBytes[0])), C.int(len(appIconBytes)))
	}

	cfg := a.cfg.Get()

	cShow := C.CString("Show Vocab")
	C.add_menu_item(1, cShow)
	C.free(unsafe.Pointer(cShow))

	cHide := C.CString("Hide Vocab")
	C.add_menu_item(2, cHide)
	C.free(unsafe.Pointer(cHide))

	C.add_separator()

	cTop := C.CString("Always on Top")
	C.add_check_item(3, cTop, boolToCInt(cfg.AlwaysOnTop))
	C.free(unsafe.Pointer(cTop))

	C.add_separator()

	cQuit := C.CString("Quit")
	C.add_menu_item(4, cQuit)
	C.free(unsafe.Pointer(cQuit))

	go a.processMenuClicks()
}

func (a *App) processMenuClicks() {
	for id := range menuClickCh {
		switch id {
		case 1:
			runtime.Show(a.ctx)
			runtime.WindowSetAlwaysOnTop(a.ctx, a.cfg.Get().AlwaysOnTop)
		case 2:
			runtime.Hide(a.ctx)
		case 3:
			cfg := a.cfg.Get()
			cfg.AlwaysOnTop = !cfg.AlwaysOnTop
			C.set_item_checked(3, boolToCInt(cfg.AlwaysOnTop))
			runtime.WindowSetAlwaysOnTop(a.ctx, cfg.AlwaysOnTop)
			a.cfg.Save()
		case 4:
			a.quitting = true
			C.remove_indicator()
			runtime.Quit(a.ctx)
		}
	}
}

func boolToCInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}
