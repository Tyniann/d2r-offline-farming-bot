//go:build windows

package input

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

func captureClientWindow(win WindowInfo) (*image.RGBA, error) {
	if win.ClientWidth <= 0 || win.ClientHeight <= 0 {
		return nil, fmt.Errorf("capture client %dx%d: %w", win.ClientWidth, win.ClientHeight, ErrInvalidClientArea)
	}
	rect := image.Rect(
		win.ClientLeft,
		win.ClientTop,
		win.ClientLeft+win.ClientWidth,
		win.ClientTop+win.ClientHeight,
	)
	img, err := screenshot.CaptureRect(rect)
	if err != nil {
		return nil, fmt.Errorf("capture client rect %v: %w: %w", rect, ErrWindowCaptureFailed, err)
	}
	if img.Bounds().Dx() != win.ClientWidth || img.Bounds().Dy() != win.ClientHeight {
		return nil, fmt.Errorf("capture client size %dx%d, want %dx%d: %w", img.Bounds().Dx(), img.Bounds().Dy(), win.ClientWidth, win.ClientHeight, ErrWindowCaptureFailed)
	}
	return img, nil
}
