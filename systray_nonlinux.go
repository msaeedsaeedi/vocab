//go:build !linux

package main

import (
	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startSystray() {
	go systray.Run(a.onSystrayReady, nil)
}

func (a *App) onSystrayReady() {
	systray.SetIcon(appIconBytes)
	systray.SetTooltip("Vocab -- Vocabulary Widget")

	showItem := systray.AddMenuItem("Show Vocab", "Show the widget")
	hideItem := systray.AddMenuItem("Hide Vocab", "Hide the widget")
	systray.AddSeparator()
	topItem := systray.AddMenuItem("Always on Top", "Toggle always on top")
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Quit the application")

	cfg := a.cfg.Get()
	if cfg.AlwaysOnTop {
		topItem.Check()
	}

	go func() {
		for range showItem.ClickedCh {
			runtime.Show(a.ctx)
			runtime.WindowSetAlwaysOnTop(a.ctx, a.cfg.Get().AlwaysOnTop)
		}
	}()

	go func() {
		for range hideItem.ClickedCh {
			runtime.Hide(a.ctx)
		}
	}()

	go func() {
		for range topItem.ClickedCh {
			cfg := a.cfg.Get()
			cfg.AlwaysOnTop = !cfg.AlwaysOnTop
			if cfg.AlwaysOnTop {
				topItem.Check()
			} else {
				topItem.Uncheck()
			}
			runtime.WindowSetAlwaysOnTop(a.ctx, cfg.AlwaysOnTop)
			a.cfg.Save()
		}
	}()

	go func() {
		<-quitItem.ClickedCh
		a.quitting = true
		systray.Quit()
		runtime.Quit(a.ctx)
	}()
}
