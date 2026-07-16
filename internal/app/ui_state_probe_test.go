package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestValidateUIStateProbeLabel(t *testing.T) {
	for _, label := range []string{"gameplay", "quit-menu", "difficulty-dialog"} {
		if err := validateUIStateProbeLabel(label); err != nil {
			t.Fatalf("label %q error = %v", label, err)
		}
	}
	for _, label := range []string{"", "Quit Menu", "../menu", "menu_1"} {
		if err := validateUIStateProbeLabel(label); err == nil {
			t.Fatalf("label %q should fail", label)
		}
	}
}

func TestBuildUIStateProbeArtifactSeparatesStableAndVolatileBytes(t *testing.T) {
	at := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	captures := []memory.UIBufferCapture{
		{At: at, Anchor: 1, Bytes: []byte{0, 4, 7, 0}},
		{At: at.Add(time.Millisecond), Anchor: 1, Bytes: []byte{0, 4, 8, 0}},
		{At: at.Add(2 * time.Millisecond), Anchor: 1, Bytes: []byte{0, 4, 9, 0}},
	}
	artifact, err := buildUIStateProbeArtifactValues("quit-menu", "test", "in_game", true, "", false, false, true, captures)
	if err != nil {
		t.Fatalf("buildUIStateProbeArtifactValues() error = %v", err)
	}
	if artifact.SampleCount != 3 || artifact.BufferSize != 4 || artifact.AnchorIndex != 1 {
		t.Fatalf("artifact shape = %+v", artifact)
	}
	if !artifact.QuitMenuOpen {
		t.Fatal("quit-menu flag was not preserved")
	}
	if len(artifact.StableNonZero) != 1 || artifact.StableNonZero[0].Index != 1 || artifact.StableNonZero[0].RelativeOffset != 0 || artifact.StableNonZero[0].Value != 4 {
		t.Fatalf("stable bytes = %+v", artifact.StableNonZero)
	}
	if len(artifact.VolatileOffsets) != 1 || artifact.VolatileOffsets[0].Index != 2 || artifact.VolatileOffsets[0].RelativeOffset != 1 {
		t.Fatalf("volatile offsets = %+v", artifact.VolatileOffsets)
	}
	if len(artifact.RawHexSamples) != 3 || artifact.StableHash == "" {
		t.Fatalf("raw/hash missing = %+v", artifact)
	}
}

func TestSaveUIStateProbeArtifactPublishesJSON(t *testing.T) {
	dir := t.TempDir()
	artifact := uiStateProbeArtifact{SchemaVersion: 1, CapturedAt: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC), Label: "gameplay"}
	path, err := saveUIStateProbeArtifact(dir, artifact)
	if err != nil {
		t.Fatalf("saveUIStateProbeArtifact() error = %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("path = %q, want dir %q", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("artifact is empty")
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".ui-state-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files = %v, err=%v", matches, err)
	}
}

func TestResolveActiveRunDisabledForUIStateProbe(t *testing.T) {
	cfg := fullCountessConfig(t)
	cfg.Runs.Active = "countess"
	if got := resolveActiveRun(Options{UIStateProbe: "gameplay"}, cfg); got != "" {
		t.Fatalf("active run = %q, want empty", got)
	}
}
