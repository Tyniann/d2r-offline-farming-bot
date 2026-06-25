package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "config.example.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "d2rbot" {
		t.Errorf("App.Name = %q, want d2rbot", cfg.App.Name)
	}
	if cfg.Process.ProcessName != "D2R.exe" {
		t.Errorf("Process.ProcessName = %q, want D2R.exe", cfg.Process.ProcessName)
	}
	if cfg.Process.AttachTimeoutMs != 30000 {
		t.Errorf("Process.AttachTimeoutMs = %d, want 30000", cfg.Process.AttachTimeoutMs)
	}
	if cfg.Memory.GameVersion != "3.2.92777" {
		t.Errorf("Memory.GameVersion = %q, want 3.2.92777", cfg.Memory.GameVersion)
	}
	if cfg.Input.KeyDelayMsMin != 10 {
		t.Errorf("Input.KeyDelayMsMin = %d, want 10", cfg.Input.KeyDelayMsMin)
	}
	if cfg.Input.KeyDelayMsMax != 40 {
		t.Errorf("Input.KeyDelayMsMax = %d, want 40", cfg.Input.KeyDelayMsMax)
	}
	if cfg.Input.ComboHoldMs != 200 {
		t.Errorf("Input.ComboHoldMs = %d, want 200", cfg.Input.ComboHoldMs)
	}
	skills := cfg.Input.Skills.Slots()
	for i, want := range []string{"f1", "f2", "f3", "f4", "f5", "f6", "f7", ""} {
		if skills[i] != want {
			t.Errorf("Input.Skills[%d] = %q, want %q", i, skills[i], want)
		}
	}
	belt := cfg.Input.Belt.Slots()
	for i, want := range []string{"1", "2", "3", "4"} {
		if belt[i] != want {
			t.Errorf("Input.Belt[%d] = %q, want %q", i, belt[i], want)
		}
	}
	if cfg.Input.TownPortal != "f8" {
		t.Errorf("Input.TownPortal = %q, want f8", cfg.Input.TownPortal)
	}
	if cfg.Input.Enabled {
		t.Errorf("Input.Enabled = true, want false")
	}
	if cfg.Input.PauseHotkey != "pause" {
		t.Errorf("Input.PauseHotkey = %q, want pause", cfg.Input.PauseHotkey)
	}
	if cfg.Input.StopHotkey != "f12" {
		t.Errorf("Input.StopHotkey = %q, want f12", cfg.Input.StopHotkey)
	}
	if cfg.LoadedFrom == "" {
		t.Error("LoadedFrom should be set after Load")
	}
}

func TestResolvePathRelative(t *testing.T) {
	cfg := &Config{LoadedFrom: filepath.Join("configs", "config.yaml")}
	got := cfg.ResolvePath("offsets.local.yaml")
	want := filepath.Join("configs", "offsets.local.yaml")
	if got != want {
		t.Errorf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestValidateAttachTimeoutNegative(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe", AttachTimeoutMs: -1},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for negative attach_timeout_ms")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestInputDefaultsWhenSectionMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	def := cfg.Input
	if def.KeyDelayMsMin != 10 || def.KeyDelayMsMax != 40 || def.ComboHoldMs != 200 {
		t.Fatalf("timing defaults = %+v", def)
	}
	if def.Skills.Slot1 != "f1" || def.Skills.Slot7 != "f7" || def.Skills.Slot8 != "" {
		t.Fatalf("skill defaults = %+v", def.Skills)
	}
	if def.Belt.Slot1 != "1" || def.Belt.Slot4 != "4" {
		t.Fatalf("belt defaults = %+v", def.Belt)
	}
	if def.TownPortal != "" {
		t.Fatalf("town_portal = %q, want empty default", def.TownPortal)
	}
	if def.Enabled {
		t.Fatal("expected enabled=false when input section missing")
	}
	if def.PauseHotkey != "pause" || def.StopHotkey != "f12" {
		t.Fatalf("hotkey defaults = pause=%q stop=%q", def.PauseHotkey, def.StopHotkey)
	}
}

func TestInputPartialConfigKeepsExplicitZeroDelays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
input:
  key_delay_ms_min: 0
  key_delay_ms_max: 0
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Input.KeyDelayMsMin != 0 || cfg.Input.KeyDelayMsMax != 0 {
		t.Fatalf("explicit zero delays changed: min=%d max=%d", cfg.Input.KeyDelayMsMin, cfg.Input.KeyDelayMsMax)
	}
	if cfg.Input.ComboHoldMs != 200 {
		t.Fatalf("combo_hold_ms = %d, want default 200", cfg.Input.ComboHoldMs)
	}
}

func TestInputValidateNegativeDelay(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input:   InputConfig{KeyDelayMsMin: -1},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for negative key_delay_ms_min")
	}
}

func TestInputValidateMaxLessThanMin(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input:   InputConfig{KeyDelayMsMin: 50, KeyDelayMsMax: 10},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when max < min")
	}
}

func TestInputValidateInvalidKey(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input: InputConfig{
			KeyDelayMsMin: 10,
			KeyDelayMsMax: 40,
			ComboHoldMs:   200,
			Skills:        SkillKeys{Slot1: "control"},
		},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for invalid skill key")
	}
}

func TestInputValidateInvalidAlias(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input: InputConfig{
			KeyDelayMsMin: 10,
			KeyDelayMsMax: 40,
			ComboHoldMs:   200,
			Skills:        SkillKeys{Slot1: "lctrl"},
		},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for lctrl alias")
	}
}

func TestInputValidateNegativeComboHold(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input:   InputConfig{ComboHoldMs: -1},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for negative combo_hold_ms")
	}
}

func TestInputValidateSameHotkeys(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input: InputConfig{
			PauseHotkey: "f12",
			StopHotkey:  "f12",
		},
	}
	cfg.Input.sectionPresent = true
	cfg.Input.applyDefaults()
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when pause and stop hotkeys are equal")
	}
}

func TestInputValidateEmptyHotkeys(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input:   InputConfig{PauseHotkey: "", StopHotkey: ""},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for empty hotkeys before defaults")
	}
	cfg.Input.applyDefaults()
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected valid config after defaults: %v", err)
	}
}

func TestInputValidateInvalidHotkey(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input: InputConfig{
			PauseHotkey: "pause",
			StopHotkey:  "invalid",
		},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for invalid stop hotkey")
	}
}

func TestNewLogger(t *testing.T) {
	log := NewLogger("debug")
	if log == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
