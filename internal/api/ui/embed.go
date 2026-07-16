// Package ui embeds the reproducible React production build.
package ui

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dist/*
var embedded embed.FS

// FS returns the production asset root without exposing embed internals.
func FS() (fs.FS, error) {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded UI assets: %w", err)
	}
	return assets, nil
}
