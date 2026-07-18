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
	if filepath.Clean(cfg.Telemetry.Directory) != filepath.Join("logs", "telemetry") {
		t.Fatalf("Telemetry.Directory = %q, want logs/telemetry", cfg.Telemetry.Directory)
	}
	if len(cfg.Loot.InventoryLock) != 4 || len(cfg.Loot.InventoryLock[0]) != 10 {
		t.Fatalf("Loot.InventoryLock shape = %dx%d, want 4x10", len(cfg.Loot.InventoryLock), len(cfg.Loot.InventoryLock[0]))
	}
	if cfg.Loot.InventoryLock[0][0] != 1 || cfg.Loot.InventoryLock[0][4] != 0 {
		t.Fatalf("Loot.InventoryLock first row = %+v, want locked columns then free columns", cfg.Loot.InventoryLock[0])
	}
	countess, ok := cfg.Runs.Run("countess")
	if !ok {
		t.Fatal("Countess run config missing")
	}
	if countess.Loot.PickupFile != "pickit/countess.nip" || countess.Loot.SellFile != "" {
		t.Fatalf("Countess loot config = %+v", countess.Loot)
	}
	mephisto, ok := cfg.Runs.Run("mephisto")
	if !ok || mephisto.Loot.PickupFile != "pickit/mephisto.nip" || mephisto.Loot.SellFile != "pickit/mephisto-sell.nip" {
		t.Fatalf("Mephisto loot config = %+v, present=%t", mephisto.Loot, ok)
	}
	if countess.Combat != mephisto.Combat {
		t.Fatalf("shared combat defaults differ: Countess=%+v Mephisto=%+v", countess.Combat, mephisto.Combat)
	}
	if cfg.Loot.Pickup.MaxRetries != 3 ||
		cfg.Loot.Pickup.MaxDistanceTiles != 8 ||
		cfg.Loot.Pickup.VerifyTicks != 3 ||
		cfg.Loot.Pickup.VerifyTimeoutMs != 1500 ||
		cfg.Loot.Pickup.MonsterAbortDistanceTiles != 12 {
		t.Fatalf("Loot.Pickup = %+v, want pickup defaults", cfg.Loot.Pickup)
	}
	if cfg.Loot.Stash.MaxRetries != 3 || cfg.Loot.Stash.VerifyTimeoutMs != 1500 ||
		cfg.Loot.Stash.CloseTimeoutMs != 1500 || cfg.Loot.Stash.InventoryLeft != 847 ||
		cfg.Loot.Stash.InventoryTop != 369 || cfg.Loot.Stash.InventoryCellW != 33 || cfg.Loot.Stash.InventoryCellH != 33 {
		t.Fatalf("Loot.Stash = %+v, want 1280x720 stash defaults", cfg.Loot.Stash)
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
	if cfg.Input.StopAfterRunHotkey != "f10" {
		t.Errorf("Input.StopAfterRunHotkey = %q, want f10", cfg.Input.StopAfterRunHotkey)
	}
	if cfg.Input.RecordingFinishHotkey != "f9" || cfg.Input.StopHotkey != "f11" {
		t.Errorf("recording/stop hotkeys = %q/%q, want f9/f11", cfg.Input.RecordingFinishHotkey, cfg.Input.StopHotkey)
	}
	if cfg.Runs.StepTimeoutMs != 30000 {
		t.Errorf("Runs.StepTimeoutMs = %d, want 30000", cfg.Runs.StepTimeoutMs)
	}
	if countess.Combat.Profile != "necro_bone_spear" {
		t.Errorf("Countess combat profile = %q, want necro_bone_spear", countess.Combat.Profile)
	}
	if countess.Combat.AttackSkill != "bone_spear" {
		t.Errorf("Countess attack skill = %q, want bone_spear", countess.Combat.AttackSkill)
	}
	profileCfg := cfg.Profiles[countess.Combat.Profile]
	if profileCfg.CharacterClass != "necromancer" || profileCfg.Hooks.TownReady[0].Skill != "bone_armor" || profileCfg.Resources.Mana.UseBelowPercent != 35 {
		t.Fatalf("combat profile = %+v", profileCfg)
	}
	if filepath.Clean(cfg.Routes.FarmingRoot) != filepath.Join("routes", "farming") {
		t.Fatalf("Routes.FarmingRoot = %q", cfg.Routes.FarmingRoot)
	}
	if cfg.Routes.LifecycleFile != "route-lifecycle.local.yaml" {
		t.Fatalf("Routes.LifecycleFile = %q", cfg.Routes.LifecycleFile)
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

func TestLoadRejectsRemovedRoutesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.Replace(string(data), "routes:\n", "routes:\n  directory: routes/farming/mrbones/nightmare\n", 1)
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "routes.directory is unsupported") {
		t.Fatalf("legacy routes.directory error = %v", err)
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
	if def.PauseHotkey != "pause" || def.RecordingFinishHotkey != "f9" || def.StopAfterRunHotkey != "f10" || def.StopHotkey != "f11" {
		t.Fatalf("hotkey defaults = pause=%q finish=%q stop-after-run=%q stop=%q", def.PauseHotkey, def.RecordingFinishHotkey, def.StopAfterRunHotkey, def.StopHotkey)
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

func TestRunLootPickupFileKeepsInventoryDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pickit-only.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
runs:
  definitions:
    countess:
      loot:
        pickup_file: pickit/custom.nip
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	run, ok := cfg.Runs.Run("countess")
	if !ok || run.Loot.PickupFile != "pickit/custom.nip" {
		t.Fatalf("Countess run config = %+v, present=%t", run, ok)
	}
	for row, cells := range cfg.Loot.InventoryLock {
		for col, cell := range cells {
			if cell != 1 {
				t.Fatalf("cell %d,%d = %d, want all locked", row, col, cell)
			}
		}
	}
}

func TestLootConfigWithOnlyInventoryLockKeepsPickupDefaults(t *testing.T) {
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
	if cfg.Loot.Pickup.MaxRetries != 3 || cfg.Loot.Pickup.VerifyTimeoutMs != 1500 {
		t.Fatalf("Pickup defaults = %+v, want populated defaults", cfg.Loot.Pickup)
	}
}

func TestLootPickupDefaultsWithExplicitInventoryLock(t *testing.T) {
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
	if cfg.Loot.Pickup.MaxRetries != 3 ||
		cfg.Loot.Pickup.MaxDistanceTiles != 8 ||
		cfg.Loot.Pickup.VerifyTicks != 3 ||
		cfg.Loot.Pickup.VerifyTimeoutMs != 1500 ||
		cfg.Loot.Pickup.MonsterAbortDistanceTiles != 12 {
		t.Fatalf("Pickup defaults = %+v, want populated defaults with explicit inventory_lock", cfg.Loot.Pickup)
	}
}

func TestLootPickupValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "max retries", content: "loot:\n  pickup:\n    max_retries: -1\n"},
		{name: "max distance", content: "loot:\n  pickup:\n    max_distance_tiles: -1\n"},
		{name: "verify ticks", content: "loot:\n  pickup:\n    verify_ticks: -1\n"},
		{name: "verify timeout", content: "loot:\n  pickup:\n    verify_timeout_ms: -1\n"},
		{name: "monster abort", content: "loot:\n  pickup:\n    monster_abort_distance_tiles: -1\n"},
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
			cfg.Loot.applyDefaults()
			if err := cfg.validate(); err == nil {
				t.Fatal("expected invalid loot.pickup config")
			}
		})
	}
}

func TestLootStashValidation(t *testing.T) {
	tests := []string{
		"loot:\n  stash:\n    max_retries: -1\n",
		"loot:\n  stash:\n    verify_timeout_ms: -1\n",
		"loot:\n  stash:\n    inventory_cell_width: -1\n",
	}
	for _, content := range tests {
		var cfg Config
		if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
			t.Fatal(err)
		}
		cfg.App = AppConfig{Name: "d2rbot"}
		cfg.Process = ProcessConfig{ProcessName: "D2R.exe"}
		cfg.Runtime = RuntimeConfig{PollIntervalMs: 100}
		cfg.Input.applyDefaults()
		cfg.Runs.applyDefaults()
		cfg.Pathing.applyDefaults()
		cfg.Loot.applyDefaults()
		if err := cfg.validate(); err == nil {
			t.Fatalf("config %q unexpectedly valid", content)
		}
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

func TestInputValidateStopAfterRunHotkeyDiffersFromEmergencyStop(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{Name: "d2rbot"},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runtime: RuntimeConfig{PollIntervalMs: 100},
		Input: InputConfig{
			PauseHotkey:        "pause",
			StopAfterRunHotkey: "f11",
			StopHotkey:         "f11",
		},
	}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected error when stop-after-run and emergency hotkeys are equal")
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
	if len(cfg.Runs.Definitions) != 0 {
		t.Fatalf("missing runs section created definitions: %+v", cfg.Runs.Definitions)
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

func TestRunsRejectLegacyCountessSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.yaml")
	content := `app:
  name: d2rbot
runtime:
  poll_interval_ms: 100
process:
  process_name: D2R.exe
runs:
  countess:
    route_id: legacy
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "runs.countess is unsupported") {
		t.Fatalf("legacy schema error = %v", err)
	}
}

func TestRunDefinitionRequiresPickupPolicy(t *testing.T) {
	cfg := &Config{
		App: AppConfig{Name: "d2rbot"}, Runtime: RuntimeConfig{PollIntervalMs: 100},
		Process: ProcessConfig{ProcessName: "D2R.exe"},
		Runs: RunsConfig{StepTimeoutMs: 30000, Definitions: map[string]RunConfig{
			"countess": {Combat: CombatConfig{Profile: "necro_bone_spear", AttackSkill: "bone_spear", AttackIntervalMs: 350, EngageDistanceTiles: 22, RepositionDistanceTiles: 32, KillConfirmTicks: 3}},
		}},
	}
	cfg.Input.applyDefaults()
	cfg.Pathing.applyDefaults()
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "loot.pickup_file is required") {
		t.Fatalf("missing pickup policy error = %v", err)
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
  definitions:
    countess:
      combat:
        profile: necro_bone_spear
        attack_skill: bone_spear
        attack_interval_ms: 400
        engage_distance_tiles: 20
        reposition_distance_tiles: 35
        kill_confirm_ticks: 4
      loot:
        pickup_file: pickit/countess.nip
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	run, ok := cfg.Runs.Run("countess")
	if !ok {
		t.Fatal("Countess run config missing")
	}
	got := run.Combat
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
			Definitions: map[string]RunConfig{"countess": {Combat: CombatConfig{
				Profile:                 "necro_bone_spear",
				AttackSkill:             "bone_spear",
				AttackIntervalMs:        350,
				EngageDistanceTiles:     32,
				RepositionDistanceTiles: 32,
				KillConfirmTicks:        3,
			}, Loot: RunLootConfig{PickupFile: "pickit/countess.nip"}}},
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
	if closeErr := file.Close(); closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
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
