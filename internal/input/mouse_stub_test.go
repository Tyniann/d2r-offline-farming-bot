//go:build !windows

package input

import (
	"errors"
	"testing"
)

func TestStubMouseSenderReturnsUnsupportedPlatform(t *testing.T) {
	s := &stubMouseSender{}
	if err := s.MoveTo(0, 0); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("MoveTo err = %v, want ErrUnsupportedPlatform", err)
	}
	if err := s.ButtonDown(MouseLeft); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("ButtonDown err = %v, want ErrUnsupportedPlatform", err)
	}
	if err := s.ButtonUp(MouseLeft); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("ButtonUp err = %v, want ErrUnsupportedPlatform", err)
	}
}
