//go:build !windows

package input

import (
	"image"
)

func captureClientWindow(_ WindowInfo) (*image.RGBA, error) {
	return nil, ErrUnsupportedPlatform
}
