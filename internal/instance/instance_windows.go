//go:build windows

package instance

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const mutexName = `Local\Vocab.DesktopDaemon`

func acquire() (func(), bool, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, false, fmt.Errorf("singleton name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if handle == 0 {
		return nil, false, fmt.Errorf("create singleton mutex: %w", err)
	}
	if err == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(handle)
		return nil, true, nil
	}
	return func() { _ = windows.CloseHandle(handle) }, false, nil
}
