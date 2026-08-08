//go:build !windows

package instance

func acquire() (func(), bool, error) {
	return func() {}, false, nil
}
