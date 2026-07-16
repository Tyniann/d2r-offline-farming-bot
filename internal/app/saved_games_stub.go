//go:build !windows

package app

import (
	"fmt"
	"os"
)

func savedGamesDirectory() (string, error) {
	if home, err := os.UserHomeDir(); err == nil {
		return home, nil
	}
	return "", fmt.Errorf("Saved Games is only supported on Windows")
}

func fileInfoIsReparsePoint(interface{ Sys() any }) bool { return false }

func savedGamesPathMissing(error) bool { return false }
