package main

import (
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
)

func TestRunMissingConfig(t *testing.T) {
	err := run(filepath.Join(t.TempDir(), "missing.yaml"), app.Options{Probe: true, Verbose: true})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}
