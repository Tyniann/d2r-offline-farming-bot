package app

import (
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestValidateRunModePassiveOK(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: false}}
	log := config.NewLogger("error")
	if err := validateRunMode("", cfg, Options{}, log); err != nil {
		t.Fatalf("passive mode err = %v", err)
	}
}

func TestValidateRunModeUnknownRun(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode("mephisto", cfg, Options{}, log)
	if err == nil {
		t.Fatal("expected error for unknown run")
	}
	if !errors.Is(err, errUnknownRun) {
		t.Fatalf("err = %v, want errUnknownRun", err)
	}
}

func TestValidateRunModeDisabledInput(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: false}}
	log := config.NewLogger("error")
	err := validateRunMode("countess", cfg, Options{}, log)
	if err == nil {
		t.Fatal("expected error when input disabled")
	}
	if !errors.Is(err, errInputRequiredForRun) {
		t.Fatalf("err = %v, want errInputRequiredForRun", err)
	}
}

func TestValidateRunModeInputTestConflict(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode("countess", cfg, Options{InputTest: "belt:1"}, log)
	if err == nil {
		t.Fatal("expected error for --run with --input-test")
	}
	if !errors.Is(err, errRunInputTestConflict) {
		t.Fatalf("err = %v, want errRunInputTestConflict", err)
	}
}

func TestResolveActiveRunCLIPriority(t *testing.T) {
	cfg := &config.Config{Runs: config.RunsConfig{Active: "countess"}}
	if got := resolveActiveRun(Options{Run: "countess"}, cfg); got != "countess" {
		t.Fatalf("CLI run = %q", got)
	}
	if got := resolveActiveRun(Options{}, cfg); got != "countess" {
		t.Fatalf("config run = %q", got)
	}
}

func TestValidateRunModeKnownRunOK(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	if err := validateRunMode("countess", cfg, Options{Run: "countess"}, log); err != nil {
		t.Fatalf("countess err = %v", err)
	}
}
