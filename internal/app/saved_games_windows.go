//go:build windows

package app

import (
	"errors"

	"golang.org/x/sys/windows"
)

func savedGamesDirectory() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_SavedGames, 0)
}

func fileInfoIsReparsePoint(info interface{ Sys() any }) bool {
	data, ok := info.Sys().(*windows.Win32FileAttributeData)
	return ok && data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func savedGamesPathMissing(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}
