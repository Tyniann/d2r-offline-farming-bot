//go:build !windows

package input

import "log/slog"

type unsupportedWindowAPI struct{}

func defaultWindowAPI(_ *slog.Logger) windowAPI {
	return &unsupportedWindowAPI{}
}

func (u *unsupportedWindowAPI) FindMainWindow(_ uint32, _ string) (nativeWindow, error) {
	return 0, ErrUnsupportedPlatform
}

func (u *unsupportedWindowAPI) ClientArea(_ nativeWindow) (WindowInfo, error) {
	return WindowInfo{}, ErrUnsupportedPlatform
}

func (u *unsupportedWindowAPI) Activate(_ nativeWindow) error {
	return ErrUnsupportedPlatform
}

func (u *unsupportedWindowAPI) IsForeground(_ nativeWindow) bool {
	return false
}
