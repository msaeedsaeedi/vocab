//go:build windows

package main

import "github.com/msaeedsaeedi/vocab/internal/wallpaper"

func restoreWallpaper() error { return wallpaper.Restore() }
