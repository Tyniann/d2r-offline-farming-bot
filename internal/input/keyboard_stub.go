//go:build !windows

package input

import (
	"fmt"
	"log/slog"
)

func defaultKeySender(_ *slog.Logger) KeySender {
	return &stubKeySender{}
}

type stubKeySender struct{}

func (s *stubKeySender) KeyDown(_ Key) error {
	return fmt.Errorf("keyboard down: %w", ErrUnsupportedPlatform)
}

func (s *stubKeySender) KeyUp(_ Key) error {
	return fmt.Errorf("keyboard up: %w", ErrUnsupportedPlatform)
}
