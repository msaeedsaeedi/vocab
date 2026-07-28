package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func SetEnabled(name, execPath string, enabled bool) error {
	switch runtime.GOOS {
	case "linux":
		return setLinux(name, execPath, enabled)
	case "windows":
		return setWindows(name, execPath, enabled)
	case "darwin":
		return setDarwin(name, execPath, enabled)
	}
	return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
}

func Enabled(name string) bool {
	switch runtime.GOOS {
	case "linux":
		return linuxEnabled(name)
	case "windows":
		return windowsEnabled(name)
	case "darwin":
		return darwinEnabled(name)
	}
	return false
}

func setLinux(name, execPath string, enabled bool) error {
	dir := filepath.Join(os.Getenv("HOME"), ".config", "autostart")
	p := filepath.Join(dir, name+".vocab.desktop")
	if enabled {
		content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Vocab
Exec=%s
X-GNOME-Autostart-enabled=true
`, execPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(content), 0644)
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func linuxEnabled(name string) bool {
	p := filepath.Join(os.Getenv("HOME"), ".config", "autostart", name+".vocab.desktop")
	_, err := os.Stat(p)
	return err == nil
}

func setWindows(name, execPath string, enabled bool) error {
	if enabled {
		return exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
			"/v", name+".vocab", "/t", "REG_SZ", "/d", execPath, "/f",
		).Run()
	}
	return exec.Command("reg", "delete",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", name+".vocab", "/f",
	).Run()
}

func windowsEnabled(name string) bool {
	err := exec.Command("reg", "query",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", name+".vocab",
	).Run()
	return err == nil
}

func setDarwin(name, execPath string, enabled bool) error {
	dir := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
	p := filepath.Join(dir, "com."+name+".vocab.plist")
	if enabled {
		content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.%s.vocab</string>
	<key>ProgramArguments</key>
	<array><string>%s</string></array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`, name, execPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(content), 0644)
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func darwinEnabled(name string) bool {
	p := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", "com."+name+".vocab.plist")
	_, err := os.Stat(p)
	return err == nil
}
