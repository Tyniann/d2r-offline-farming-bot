//go:build !windows

package input

import (
	"fmt"
	"log/slog"
)

func defaultMouseSender(_ *slog.Logger) MouseSender {
	return &stubMouseSender{}
}

type stubMouseSender struct{}

func (s *stubMouseSender) MoveTo(_, _ int) error {
	return fmt.Errorf("mouse move: %w", ErrUnsupportedPlatform)
}

func (s *stubMouseSender) ButtonDown(_ MouseButton) error {
	return fmt.Errorf("mouse button down: %w", ErrUnsupportedPlatform)
}

func (s *stubMouseSender) ButtonUp(_ MouseButton) error {
	return fmt.Errorf("mouse button up: %w", ErrUnsupportedPlatform)
}
