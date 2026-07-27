//go:build !windows

package app

import (
	"context"
	"os"
)

func openDesktopHandshakePipe(_ context.Context, pipeName string) (desktopHandshakePipe, error) {
	return os.OpenFile(pipeName, os.O_WRONLY, 0)
}
