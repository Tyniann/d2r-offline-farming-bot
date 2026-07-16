//go:build !windows

package api

import "fmt"

// OpenBrowser reports that the Phase-11 browser launcher is Windows-only.
func OpenBrowser(string) error {
	return fmt.Errorf("open default browser: unsupported platform")
}
