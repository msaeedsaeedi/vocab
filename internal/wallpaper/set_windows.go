//go:build windows

package wallpaper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/msaeedsaeedi/vocab/internal/apppaths"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modole32  = windows.NewLazySystemDLL("ole32.dll")
	moduser32 = windows.NewLazySystemDLL("user32.dll")

	procCoInitializeEx       = modole32.NewProc("CoInitializeEx")
	procCoCreateInstance     = modole32.NewProc("CoCreateInstance")
	procCoUninitialize       = modole32.NewProc("CoUninitialize")
	procSystemParametersInfo = moduser32.NewProc("SystemParametersInfoW")
)

const (
	coinitApartmentthreaded = 0x2
	clsctxInprocServer      = 0x1
	clsctxLocalServer       = 0x4
	clsctxAll               = clsctxInprocServer | clsctxLocalServer

	spiSetDeskWallpaper = 0x0014
	spifUpdateINIFile   = 0x01
	spifSendChange      = 0x02

	wmSettingChange = 0x001A
	hWndBroadcast   = 0xFFFF
	smtAbortIfHung  = 0x0002

	dwposFill = 4
)

var (
	clsIDDesktopWallpaper = windows.GUID{
		Data1: 0xC2CF3110,
		Data2: 0x460E,
		Data3: 0x4fc1,
		Data4: [8]byte{0xB9, 0xD0, 0x8A, 0x1C, 0x0C, 0x9C, 0xC4, 0xBD},
	}

	iidIDesktopWallpaper = windows.GUID{
		Data1: 0xB92B56A9,
		Data2: 0x8B55,
		Data3: 0x4E14,
		Data4: [8]byte{0x9A, 0x89, 0x01, 0x99, 0xBB, 0xB6, 0xF9, 0x3B},
	}
)

type iDesktopWallpaperVtbl struct {
	queryInterface          uintptr
	addRef                  uintptr
	release_                uintptr
	setWallpaper            uintptr
	getWallpaper            uintptr
	getMonitorDevicePathAt  uintptr
	getMonitorDevicePathCnt uintptr
	getMonitorRECT          uintptr
	setBackgroundColor      uintptr
	getBackgroundColor      uintptr
	setPosition             uintptr
	getPosition             uintptr
	setSlideshow            uintptr
	getSlideshow            uintptr
	advanceSlideshow        uintptr
	getStatus               uintptr
	enable                  uintptr
}

type wallpaperBackup struct {
	Path  string
	Style string
	Tile  string
}

func Set(path string) error {
	log.Printf("wallpaper: Set(%q)", path)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("wallpaper: set: stat file: %w", err)
	}
	log.Printf("wallpaper: file exists, size=%d bytes", info.Size())
	if info.Size() == 0 {
		return fmt.Errorf("wallpaper: set: file is empty")
	}
	if err := saveBackup(); err != nil {
		log.Printf("wallpaper: backup warning: %v", err)
	}

	if err := writeRegistry(path); err != nil {
		log.Printf("wallpaper: registry write warning: %v", err)
	}

	err = setViaDesktopWallpaper(path)
	if err != nil {
		log.Printf("wallpaper: IDesktopWallpaper failed: %v — trying SystemParametersInfo fallback", err)
		return setViaParamInfo(path)
	}
	return nil
}

// Restore puts back the wallpaper that was active before Vocab first changed
// it. If the user chose a different wallpaper in the meantime, their choice
// wins and the stale backup is simply discarded.
func Restore() error {
	backup, err := readBackup()
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}
	current, err := currentWallpaper()
	if err != nil {
		return fmt.Errorf("read current wallpaper: %w", err)
	}
	vocabPath, err := apppaths.WallpaperImagePath()
	if err != nil {
		return err
	}
	if current != vocabPath {
		return os.Remove(backupPath())
	}
	if backup.Path == "" {
		return fmt.Errorf("backup has no wallpaper path")
	}
	if err := setViaDesktopWallpaper(backup.Path); err != nil {
		if err := setViaParamInfo(backup.Path); err != nil {
			return err
		}
	}
	if err := writeRegistryValues(backup.Path, backup.Style, backup.Tile); err != nil {
		log.Printf("wallpaper: restore registry warning: %v", err)
	}
	if err := os.Remove(backupPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove backup: %w", err)
	}
	log.Print("wallpaper: restored the pre-Vocab wallpaper")
	return nil
}

func writeRegistry(path string) error {
	return writeRegistryValues(path, "10", "0")
}

func writeRegistryValues(path, style, tile string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Control Panel\Desktop`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("Wallpaper", path); err != nil {
		return fmt.Errorf("set Wallpaper: %w", err)
	}
	if err := k.SetStringValue("WallpaperStyle", style); err != nil {
		return fmt.Errorf("set WallpaperStyle: %w", err)
	}
	if err := k.SetStringValue("TileWallpaper", tile); err != nil {
		return fmt.Errorf("set TileWallpaper: %w", err)
	}
	log.Print("wallpaper: registry keys written")
	return nil
}

func currentWallpaper() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	path, _, err := k.GetStringValue("Wallpaper")
	return path, err
}

func backupPath() string {
	dir, err := apppaths.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "wallpaper-backup.json")
}

func saveBackup() error {
	path := backupPath()
	if path == "" {
		return fmt.Errorf("find backup path")
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	wallpaper, err := currentWallpaper()
	if err != nil {
		return err
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	style, _, _ := k.GetStringValue("WallpaperStyle")
	tile, _, _ := k.GetStringValue("TileWallpaper")
	data, err := json.Marshal(wallpaperBackup{Path: wallpaper, Style: style, Tile: tile})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func readBackup() (wallpaperBackup, error) {
	var backup wallpaperBackup
	data, err := os.ReadFile(backupPath())
	if err != nil {
		return backup, err
	}
	return backup, json.Unmarshal(data, &backup)
}

func setViaDesktopWallpaper(path string) error {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentthreaded)
	needsUninit := hr == 0
	if hr != 0 && hr != uintptr(windows.S_FALSE) {
		return fmt.Errorf("CoInitializeEx: 0x%X", hr)
	}
	if needsUninit {
		defer procCoUninitialize.Call()
	}
	log.Printf("wallpaper: COM initialized (S_FALSE=%v, needsUninit=%v)", hr == uintptr(windows.S_FALSE), needsUninit)

	var pUnk unsafe.Pointer
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsIDDesktopWallpaper)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(&iidIDesktopWallpaper)),
		uintptr(unsafe.Pointer(&pUnk)),
	)
	if hr != 0 {
		return fmt.Errorf("CoCreateInstance DesktopWallpaper: 0x%X", hr)
	}
	log.Print("wallpaper: IDesktopWallpaper COM instance created")

	vtbl := *(**iDesktopWallpaperVtbl)(pUnk)

	wpath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		syscall.SyscallN(vtbl.release_, uintptr(pUnk))
		return fmt.Errorf("UTF16: %w", err)
	}

	hr, _, _ = syscall.SyscallN(
		vtbl.setWallpaper,
		uintptr(pUnk),
		0,
		uintptr(unsafe.Pointer(wpath)),
	)
	if hr != 0 {
		syscall.SyscallN(vtbl.release_, uintptr(pUnk))
		return fmt.Errorf("SetWallpaper: 0x%X", hr)
	}
	log.Print("wallpaper: SetWallpaper succeeded")

	hr, _, _ = syscall.SyscallN(
		vtbl.setPosition,
		uintptr(pUnk),
		uintptr(dwposFill),
	)
	if hr != 0 {
		log.Printf("wallpaper: SetPosition: 0x%X (non-fatal)", hr)
	} else {
		log.Print("wallpaper: SetPosition FILL succeeded")
	}

	syscall.SyscallN(vtbl.release_, uintptr(pUnk))
	return nil
}

func setViaParamInfo(path string) error {
	log.Print("wallpaper: fallback SystemParametersInfoW")
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("UTF16: %w", err)
	}

	ret, _, lastErr := procSystemParametersInfo.Call(
		uintptr(spiSetDeskWallpaper),
		0,
		uintptr(unsafe.Pointer(ptr)),
		uintptr(spifUpdateINIFile|spifSendChange),
	)
	if ret == 0 {
		return fmt.Errorf("SystemParametersInfoW: %v", lastErr)
	}

	broadcastSettingChange()

	log.Print("wallpaper: SystemParametersInfoW succeeded")
	return nil
}

func broadcastSettingChange() {
	traySettings, _ := windows.UTF16PtrFromString("TraySettings")
	procSMTO := moduser32.NewProc("SendMessageTimeoutW")
	procSMTO.Call(
		hWndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(traySettings)),
		smtAbortIfHung,
		500,
		0,
	)
	procPM := moduser32.NewProc("PostMessageW")
	procPM.Call(hWndBroadcast, wmSettingChange, 0, 0)
}
