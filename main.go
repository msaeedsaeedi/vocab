package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	resetDB := flag.Bool("reset-db", false, "Delete database and re-seed on next launch")
	flag.Parse()

	if *resetDB {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			dbPath := filepath.Join(homeDir, ".local", "share", "vocab", "vocab.db")
			if err := os.Remove(dbPath); err == nil {
				fmt.Println("Database deleted. Launching with fresh state...")
			} else if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Failed to remove db: %v\n", err)
			}
		}
	}

	app := NewApp()

	const (
		winW = 380
		winH = 320
	)

	err := wails.Run(&options.App{
		Title:     "Vocab",
		Width:     winW,
		Height:    winH,
		MinWidth:  winW,
		MaxWidth:  winW,
		MinHeight: winH,
		MaxHeight: winH,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.onBeforeClose,
		Linux: &linux.Options{
			WindowIsTranslucent: false,
		},
		Windows: &windows.Options{
			DisableFramelessWindowDecorations: true,
			WebviewUserDataPath:               "",
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
