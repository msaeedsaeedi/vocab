//go:build !windows

package main

import "github.com/msaeedsaeedi/vocab/internal/state"

func ensureWallpaperConsent(*state.Store) bool { return true }
