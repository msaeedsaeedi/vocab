//go:build !windows

// Package tray provides the small set of desktop controls Vocab needs. The
// non-Windows implementation deliberately does nothing so development builds
// remain dependency-free.
package tray

import "context"

type Actions struct {
	LearnNow func()
	Quit     func()
}

func Run(ctx context.Context, _ Actions) {
	<-ctx.Done()
}
