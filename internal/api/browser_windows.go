//go:build windows

package api

import (
	"fmt"
	"os/exec"
)

// OpenBrowser opens the token-bearing bootstrap URL without logging it.
func OpenBrowser(url string) error {
	if url == "" {
		return fmt.Errorf("browser URL is required")
	}
	if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		return fmt.Errorf("open default browser: %w", err)
	}
	return nil
}
