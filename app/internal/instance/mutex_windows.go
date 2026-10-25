//go:build windows

package instance

import (
	"errors"

	"golang.org/x/sys/windows"
)

// Acquire keeps one visualizer process alive per interactive Windows session.
func Acquire() (release func(), acquired bool, err error) {
	name, err := windows.UTF16PtrFromString(`Local\AulaCodexRgb-0C45-800A`)
	if err != nil {
		return nil, false, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(h)
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return func() { _ = windows.CloseHandle(h) }, true, nil
}

// IsRunning checks the per-session mutex without creating a new mutex when the
// visualizer is stopped. Status and control commands use this instead of
// treating an old state-file timestamp as proof that the process has exited.
func IsRunning() (bool, error) {
	name, err := windows.UTF16PtrFromString(`Local\AulaCodexRgb-0C45-800A`)
	if err != nil {
		return false, err
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = windows.CloseHandle(h)
	return true, nil
}
