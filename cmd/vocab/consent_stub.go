//go:build !windows

package main

import "github.com/msaeedsaeedi/vocab/internal/database"

func ensureWallpaperConsent(*database.DB) bool { return true }
