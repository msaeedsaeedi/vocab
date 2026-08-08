//go:build !windows

package report

// Reveal is a no-op on non-Windows platforms.
func Reveal(string) error { return nil }
