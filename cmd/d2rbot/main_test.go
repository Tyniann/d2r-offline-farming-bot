package main

import (
	"context"
	"os"
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
	if shouldRunSession(cfg, app.Options{Desktop: true}) {
		t.Fatal("desktop mode must never start the autonomous session")
	}
	if !shouldRunSession(cfg, app.Options{}) {
		t.Fatal("bare enabled config should run autonomous session")
	}
}

func TestValidateDesktopModeRejectsRuntimeCommands(t *testing.T) {
	if err := validateDesktopMode(app.Options{Desktop: true}); err != nil {
		t.Fatal(err)
	}
	if err := validateDesktopMode(app.Options{Desktop: true, Run: "countess"}); err == nil {
		t.Fatal("expected desktop/run conflict")
	}
}

func TestRunRunsInspectNeedsNoRuntimeOrInput(t *testing.T) {
	if err := run(filepath.Join("..", "..", "configs", "config.example.yaml"), app.Options{RunsInspect: true}); err != nil {
		t.Fatalf("runs inspect: %v", err)
	}
}

func TestLoadConfigUsesExplicitDataRootWithoutWorkingDirectoryFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(filepath.Join(root, "configs", "config.yaml"), body, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	cfg, err := loadConfig(filepath.Join("configs", "config.yaml"), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataRoot != filepath.Clean(root) || cfg.Telemetry.Directory != filepath.Join(root, "logs", "telemetry") {
		t.Fatalf("cfg.DataRoot=%q telemetry=%q", cfg.DataRoot, cfg.Telemetry.Directory)
	}
	if _, err := loadConfig("custom.yaml", root); err == nil {
		t.Fatal("custom --config was accepted with --data-root")
	}
}

func TestProvisionInstalledDataRootUsesExactlyOneCoreOperation(t *testing.T) {
	source := filepath.Join("..", "..", "configs")
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := app.BuildDefaultBundle(source, bundle); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "installed")
	result, err := provisionInstalledDataRoot(context.Background(), target, bundle, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Status != app.DataRootPublished {
		t.Fatalf("result=%+v", result)
	}
	if _, err := config.LoadFromDataRoot(target); err != nil {
		t.Fatalf("published root is not productively loadable: %v", err)
	}
	if _, err := provisionInstalledDataRoot(context.Background(), target, bundle, target); err == nil {
		t.Fatal("defaults and import were accepted together")
	}
	if _, err := provisionInstalledDataRoot(context.Background(), target, "", ""); err == nil {
		t.Fatal("missing provisioning source was accepted")
	}
}
