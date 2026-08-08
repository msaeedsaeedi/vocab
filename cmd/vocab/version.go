package main

// Build metadata injected at link time via -ldflags "-X main.version=..." etc.
// Kept in the main package so goreleaser's -X main.* flags keep working.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
