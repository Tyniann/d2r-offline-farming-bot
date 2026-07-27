//go:build windows

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func openDesktopHandshakePipe(ctx context.Context, pipeName string) (desktopHandshakePipe, error) {
	if !strings.HasPrefix(strings.ToLower(pipeName), `\\.\pipe\d2rbot-desktop-`) || strings.Contains(pipeName[len(`\\.\pipe\`):], `\`) {
		return nil, fmt.Errorf("desktop handshake pipe name is invalid")
	}
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return nil, err
	}
	for {
		handle, openErr := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_WRITE_THROUGH, 0)
		if openErr == nil {
			return os.NewFile(uintptr(handle), "desktop-handshake"), nil
		}
		if !errors.Is(openErr, windows.ERROR_PIPE_BUSY) && !errors.Is(openErr, windows.ERROR_FILE_NOT_FOUND) {
			return nil, fmt.Errorf("open desktop handshake pipe: %w", openErr)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
