package main

import (
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
)

func TestRunInputTestFlagRequiresConfig(t *testing.T) {
	err := run(filepath.Join(t.TempDir(), "missing.yaml"), app.Options{
		InputTest:          "belt:1",
		InputTestObserveMs: 3000,
	})
	if err == nil {
		t.Fatal("expected error for missing config with --input-test")
	}
}
