//go:build !windows

package process_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestStubAPICompilesOnNonWindows(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := process.New(log, "D2R.exe")
	if svc == nil {
		t.Fatal("New() returned nil")
	}
}
