//go:build !windows

package input

import (
	"errors"
	"testing"
)

func TestStubKeySenderReturnsUnsupportedPlatform(t *testing.T) {
	s := &stubKeySender{}
	if err := s.KeyDown("a"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("KeyDown err = %v, want ErrUnsupportedPlatform", err)
	}
	if err := s.KeyUp("a"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("KeyUp err = %v, want ErrUnsupportedPlatform", err)
	}
}
