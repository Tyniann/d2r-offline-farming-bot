//go:build windows

package app

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func savedGamesDirectory() (string, error) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_SavedGames, 0)
	if err == nil {
		return path, nil
	}
	if !savedGamesPathMissing(err) {
		return "", err
	}

	// Manche lokale Windows-Profile besitzen den physischen Standardordner,
	// aber keinen registrierten Saved-Games-Known-Folder. Nur für diesen
	// FILE_NOT_FOUND-Fall wird der ebenfalls per Known Folder aufgelöste
	// Profilroot verwendet; umgeleitete registrierte Pfade behalten Vorrang.
	profile, profileErr := windows.KnownFolderPath(windows.FOLDERID_Profile, 0)
	if profileErr != nil {
		return "", err
	}
	fallback := filepath.Join(profile, "Saved Games")
	info, statErr := os.Lstat(fallback)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || fileInfoIsReparsePoint(info) {
		return "", err
	}
	return fallback, nil
}

func fileInfoIsReparsePoint(info interface{ Sys() any }) bool {
	data, ok := info.Sys().(*windows.Win32FileAttributeData)
	return ok && data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func savedGamesPathMissing(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}
