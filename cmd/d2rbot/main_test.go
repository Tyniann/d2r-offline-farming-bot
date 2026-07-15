package main

import (
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestRunMissingConfig(t *testing.T) {
	err := run(filepath.Join(t.TempDir(), "missing.yaml"), app.Options{Probe: true, Verbose: true})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestShouldRunSessionDoesNotOverrideExplicitRunOrProbe(t *testing.T) {
	cfg := &config.Config{Session: config.SessionConfig{Enabled: true}}
	if shouldRunSession(cfg, app.Options{Run: "countess", RunPhase: "town-ready"}) {
		t.Fatal("explicit run must not fall through to autonomous session")
	}
	if shouldRunSession(cfg, app.Options{Probe: true}) {
		t.Fatal("probe must not fall through to autonomous session")
	}
	if !shouldRunSession(cfg, app.Options{}) {
		t.Fatal("bare enabled config should run autonomous session")
	}
}

func TestRunRunsInspectNeedsNoRuntimeOrInput(t *testing.T) {
	if err := run(filepath.Join("..", "..", "configs", "config.example.yaml"), app.Options{RunsInspect: true}); err != nil {
		t.Fatalf("runs inspect: %v", err)
	}
}
