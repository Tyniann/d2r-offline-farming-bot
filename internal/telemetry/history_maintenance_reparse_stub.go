//go:build !windows

package telemetry

import (
	"os"
)

func historyPathIsReparse(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return info.Mode()&os.ModeSymlink != 0, nil
}
