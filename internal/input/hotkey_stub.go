//go:build !windows

package input

import "context"

type stubHotkeyListener struct{}

func defaultHotkeyListener() HotkeyListener {
	return &stubHotkeyListener{}
}

func (l *stubHotkeyListener) Listen(_ context.Context, _ HotkeyBindings, _ chan<- HotkeyEvent, ready chan<- error) {
	ready <- ErrUnsupportedPlatform
}
