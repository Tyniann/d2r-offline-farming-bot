//go:build windows

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// DataRootLock hält den pro Datenroot eindeutigen Desktop-Core-Mutex.
type DataRootLock struct {
	handle windows.Handle
}

// AcquireDataRootLock sperrt den kanonischen Root vor API-, Hotkey- oder Inputaufbau.
func AcquireDataRootLock(root string) (*DataRootLock, error) {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return nil, &DataRootError{Code: Phase15ReasonDataRootUnavailable, Err: fmt.Errorf("data root lock requires an absolute root")}
	}
	canonical := strings.ToLower(filepath.Clean(root))
	hash := sha256.Sum256([]byte(canonical))
	name, err := windows.UTF16PtrFromString(`Local\D2ROfflineFarmingBot-` + hex.EncodeToString(hash[:16]))
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateMutex(nil, true, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, fmt.Errorf("create data root mutex: %w", err)
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		return nil, &DataRootError{Code: Phase15ReasonDataRootLocked, Err: fmt.Errorf("data root is already owned by another Core")}
	}
	return &DataRootLock{handle: handle}, nil
}

// Close gibt den gehaltenen Mutex frei.
func (l *DataRootLock) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	if err := windows.ReleaseMutex(l.handle); err != nil {
		return err
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}
