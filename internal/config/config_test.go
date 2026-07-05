package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
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
	if len(cfg.Loot.InventoryLock) != 4 || len(cfg.Loot.InventoryLock[0]) != 10 {
		t.Fatalf("Loot.InventoryLock shape = %dx%d, want 4x10", len(cfg.Loot.InventoryLock), len(cfg.Loot.InventoryLock[0]))
	}
	if cfg.Loot.InventoryLock[0][0] != 1 || cfg.Loot.InventoryLock[0][4] != 0 {
		t.Fatalf("Loot.InventoryLock first row = %+v, want locked columns then free columns", cfg.Loot.InventoryLock[0])
	}
	if cfg.Loot.PickitFile != "pickit/countess.nip" {
		t.Fatalf("Loot.PickitFile = %q, want pickit/countess.nip", cfg.Loot.PickitFile)
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
	if cfg.Input.Enabled {
		t.Errorf("Input.Enabled = true, want false")
	}
	if cfg.Input.PauseHotkey != "pause" {
		t.Errorf("Input.PauseHotkey = %q, want pause", cfg.Input.PauseHotkey)
	}
	if cfg.Input.StopHotkey != "f12" {
		t.Errorf("Input.StopHotkey = %q, want f12", cfg.Input.StopHotkey)
	}
	if cfg.Runs.StepTimeoutMs != 30000 {
		t.Errorf("Runs.StepTimeoutMs = %d, want 30000", cfg.Runs.StepTimeoutMs)
	}
	if cfg.Runs.Countess.Combat.Profile != "necro_bone_spear" {
		t.Errorf("Countess combat profile = %q, want necro_bone_spear", cfg.Runs.Countess.Combat.Profile)
	}
	if cfg.Runs.Countess.Combat.AttackSkill != "bone_spear" {
		t.Errorf("Countess attack skill = %q, want bone_spear", cfg.Runs.Countess.Combat.AttackSkill)
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

func TestResolvePickitPathRelativeToConfig(t *testing.T) {
	cfg := &Config{LoadedFrom: filepath.Join("configs", "config.yaml")}
	got := cfg.ResolvePath("pickit/countess.nip")
	want := filepath.Join("configs", "pickit", "countess.nip")
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
	if def.Enabled {
		t.Fatal("expected enabled=false when input section missing")
	}
	if def.PauseHotkey != "pause" || def.StopHotkey != "f12" {
		t.Fatalf("hotkey defaults = pause=%q stop=%q", def.PauseHotkey, def.StopHotkey)
	}
}

func TestLootInventoryLockDefaultsAllLockedWhenMissing(t *testing.T) {
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
	if len(cfg.Loot.InventoryLock) != 4 {
		t.Fatalf("rows = %d, want 4", len(cfg.Loot.InventoryLock))
	}
	if cfg.Loot.PickitFile != "pickit/countess.nip" {
		t.Fatalf("PickitFile = %q, want default", cfg.Loot.PickitFile)
	}
	for row, cells := range cfg.Loot.InventoryLock {
		if len(cells) != 10 {
			t.Fatalf("row %d columns = %d, want 10", row, len(cells))
		}
		for col, cell := range cells {
			if cell != 1 {
				t.Fatalf("cell %d,%d = %d, want all locked", row, col, cell)
			}
		}
	}
}

func TestLootConfigWithOnlyPickitFileKeepsInventoryDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pickit-only.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
loot:
  pickit_file: pickit/custom.nip
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Loot.PickitFile != "pickit/custom.nip" {
		t.Fatalf("PickitFile = %q, want pickit/custom.nip", cfg.Loot.PickitFile)
	}
	for row, cells := range cfg.Loot.InventoryLock {
		for col, cell := range cells {
			if cell != 1 {
				t.Fatalf("cell %d,%d = %d, want all locked", row, col, cell)
			}
		}
	}
}

func TestLootConfigWithOnlyInventoryLockKeepsPickitDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock-only.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
loot:
  inventory_lock:
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Loot.PickitFile != "pickit/countess.nip" {
		t.Fatalf("PickitFile = %q, want default", cfg.Loot.PickitFile)
	}
}

func TestLootInventoryLockValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "wrong rows",
			content: `loot:
  inventory_lock:
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
`,
		},
		{
			name: "wrong columns",
			content: `loot:
  inventory_lock:
    - [1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
`,
		},
		{
			name: "invalid value",
			content: `loot:
  inventory_lock:
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 2, 1, 1, 1, 1, 1, 1, 1]
    - [1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				App:     AppConfig{Name: "d2rbot"},
				Process: ProcessConfig{ProcessName: "D2R.exe"},
				Runtime: RuntimeConfig{PollIntervalMs: 100},
			}
			if err := yaml.Unmarshal([]byte(tc.content), cfg); err != nil {
				t.Fatal(err)
			}
			cfg.Input.applyDefaults()
			cfg.Runs.applyDefaults()
			cfg.Pathing.applyDefaults()
			if err := cfg.validate(); err == nil {
				t.Fatal("expected invalid inventory_lock")
			}
		})
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

func TestInputBindingsLoaded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bindings.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
input:
  enabled: false
  pause_hotkey: pause
  stop_hotkey: f12
  bindings:
    skills:
      teleport:
        key: f7
        button: right
      bone_spear:
        key: f8
        button: left
    belt:
      slot_1: ","
      slot_2: "."
      slot_3: "-"
      slot_4: "]"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Input.Bindings.Skills["teleport"]; got.Key != "f7" || got.Button != "right" {
		t.Fatalf("teleport binding = %+v, want f7/right", got)
	}
	if got := cfg.Input.Bindings.Skills["bone_spear"]; got.Key != "f8" || got.Button != "left" {
		t.Fatalf("bone_spear binding = %+v, want f8/left", got)
	}
	if got := cfg.Input.Bindings.Belt; got.Slot1 != "," || got.Slot2 != "." || got.Slot3 != "-" || got.Slot4 != "]" {
		t.Fatalf("belt bindings = %+v", got)
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
	cfg.Runs.applyDefaults()
	cfg.Pathing.applyDefaults()
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

func TestRunsDefaultsWhenSectionMissing(t *testing.T) {
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
	if cfg.Runs.StepTimeoutMs != 30000 {
		t.Fatalf("StepTimeoutMs = %d, want 30000", cfg.Runs.StepTimeoutMs)
	}
	if cfg.Runs.Active != "" {
		t.Fatalf("Active = %q, want empty", cfg.Runs.Active)
	}
	if cfg.Runs.Countess.Combat.AttackIntervalMs != 350 ||
		cfg.Runs.Countess.Combat.EngageDistanceTiles != 22 ||
		cfg.Runs.Countess.Combat.RepositionDistanceTiles != 32 ||
		cfg.Runs.Countess.Combat.KillConfirmTicks != 3 {
		t.Fatalf("Countess combat defaults = %+v", cfg.Runs.Countess.Combat)
	}
}

func TestRunsValidateStepTimeoutNonPositive(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Runs:    RunsConfig{StepTimeoutMs: -1},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error for negative step_timeout_ms")
	}
}

func TestRunsParsingFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
runs:
  active: countess
  step_timeout_ms: 45000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runs.Active != "countess" {
		t.Fatalf("Active = %q, want countess", cfg.Runs.Active)
	}
	if cfg.Runs.StepTimeoutMs != 45000 {
		t.Fatalf("StepTimeoutMs = %d, want 45000", cfg.Runs.StepTimeoutMs)
	}
}

func TestRunsCountessCombatParsingFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runs.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
runs:
  active: countess
  step_timeout_ms: 45000
  countess:
    combat:
      profile: necro_bone_spear
      attack_skill: bone_spear
      attack_interval_ms: 400
      engage_distance_tiles: 20
      reposition_distance_tiles: 35
      kill_confirm_ticks: 4
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Runs.Countess.Combat
	if got.AttackIntervalMs != 400 || got.EngageDistanceTiles != 20 || got.RepositionDistanceTiles != 35 || got.KillConfirmTicks != 4 {
		t.Fatalf("combat = %+v", got)
	}
}

func TestRunsCountessCombatValidation(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Runs: RunsConfig{
			StepTimeoutMs: 30000,
			Countess: CountessRunConfig{Combat: CountessCombatConfig{
				Profile:                 "necro_bone_spear",
				AttackSkill:             "bone_spear",
				AttackIntervalMs:        350,
				EngageDistanceTiles:     32,
				RepositionDistanceTiles: 32,
				KillConfirmTicks:        3,
			}},
		},
	}
	cfg.Input.applyDefaults()
	cfg.Pathing.applyDefaults()
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when engage distance is not below reposition distance")
	}
}

func TestNewLogger(t *testing.T) {
	log := NewLogger("debug")
	if log == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestNewFileLoggerWritesLogFile(t *testing.T) {
	dir := t.TempDir()
	log, file, path, err := NewFileLogger("debug", dir, "d2rbot", time.Date(2026, 7, 3, 12, 34, 56, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	if file == nil {
		t.Fatal("NewFileLogger returned nil file")
	}
	if filepath.Base(path) != "d2rbot-20260703-123456.log" {
		t.Fatalf("log path = %q", path)
	}

	log.Debug("test log entry", "value", 42)
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "test log entry") || !strings.Contains(string(content), "value=42") {
		t.Fatalf("log file content = %q", string(content))
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
