//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DataRootLock hält den plattformspezifischen exklusiven Datenroot-Lock.
type DataRootLock struct {
	path string
	file *os.File
}

// AcquireDataRootLock sperrt den kanonischen Root vor API-, Hotkey- oder Inputaufbau.
func AcquireDataRootLock(root string) (*DataRootLock, error) {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return nil, &DataRootError{Code: Phase15ReasonDataRootUnavailable, Err: fmt.Errorf("data root lock requires an absolute root")}
	}
	path := filepath.Join(filepath.Clean(root), ".core.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, &DataRootError{Code: Phase15ReasonDataRootLocked, Err: err}
	}
	return &DataRootLock{path: path, file: file}, nil
}

// Close gibt den gehaltenen Lock frei.
func (l *DataRootLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	closeErr := l.file.Close()
	removeErr := os.Remove(l.path)
	l.file = nil
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}
