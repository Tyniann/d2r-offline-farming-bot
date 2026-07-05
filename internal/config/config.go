package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"gopkg.in/yaml.v3"
)

// Config holds application settings loaded from YAML.
type Config struct {
	App     AppConfig     `yaml:"app"`
	Runtime RuntimeConfig `yaml:"runtime"`
	Process ProcessConfig `yaml:"process"`
	Memory  MemoryConfig  `yaml:"memory"`
	Loot    LootConfig    `yaml:"loot"`
	Input   InputConfig   `yaml:"input"`
	Runs    RunsConfig    `yaml:"runs"`
	Pathing PathingConfig `yaml:"pathing"`
	Paths   PathsConfig   `yaml:"paths"`

	// LoadedFrom is the path passed to [Load] (used to resolve relative file paths).
	LoadedFrom string `yaml:"-"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	LogLevel string `yaml:"log_level"`
}

type RuntimeConfig struct {
	PollIntervalMs int `yaml:"poll_interval_ms"`
}

type ProcessConfig struct {
	ProcessName     string `yaml:"process_name"`
	AttachTimeoutMs int    `yaml:"attach_timeout_ms"`
}

// MemoryConfig holds probe-related settings (offsets remain in Go/YAML override files).
type MemoryConfig struct {
	GameVersion string `yaml:"game_version"`
	OffsetsFile string `yaml:"offsets_file"`
}

type PathsConfig struct {
	ConfigDir string `yaml:"config_dir"`
}

// LootConfig holds read-only loot model settings.
type LootConfig struct {
	PickitFile    string  `yaml:"pickit_file"`
	InventoryLock [][]int `yaml:"inventory_lock"`

	inventoryLockPresent bool `yaml:"-"`
}

// UnmarshalYAML records whether inventory_lock was present.
func (c *LootConfig) UnmarshalYAML(value *yaml.Node) error {
	type lootConfigAlias LootConfig
	var alias lootConfigAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*c = LootConfig(alias)
	for i := 0; i < len(value.Content)-1; i += 2 {
		if value.Content[i].Value == "inventory_lock" {
			c.inventoryLockPresent = true
			break
		}
	}
	return nil
}

// InputConfig holds keyboard timing, safety settings, and explicit in-game bindings.
type InputConfig struct {
	Enabled       bool                `yaml:"enabled"`
	PauseHotkey   string              `yaml:"pause_hotkey"`
	StopHotkey    string              `yaml:"stop_hotkey"`
	KeyDelayMsMin int                 `yaml:"key_delay_ms_min"`
	KeyDelayMsMax int                 `yaml:"key_delay_ms_max"`
	ComboHoldMs   int                 `yaml:"combo_hold_ms"`
	Bindings      InputBindingsConfig `yaml:"bindings"`

	sectionPresent bool `yaml:"-"`
}

// InputBindingsConfig maps bot actions to the operator's D2R hotkeys.
type InputBindingsConfig struct {
	Skills map[string]SkillBindingConfig `yaml:"skills"`
	Belt   BeltBindingsConfig            `yaml:"belt"`
}

// SkillBindingConfig maps a skill selector key to the mouse button used to cast it.
type SkillBindingConfig struct {
	Key    string `yaml:"key"`
	Button string `yaml:"button"`
}

// BeltBindingsConfig maps belt columns to their in-game hotkeys.
type BeltBindingsConfig struct {
	Slot1 string `yaml:"slot_1"`
	Slot2 string `yaml:"slot_2"`
	Slot3 string `yaml:"slot_3"`
	Slot4 string `yaml:"slot_4"`
}

// UnmarshalYAML records whether the input section was present.
func (c *InputConfig) UnmarshalYAML(value *yaml.Node) error {
	type inputConfigAlias InputConfig
	var alias inputConfigAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*c = InputConfig(alias)
	c.sectionPresent = true
	return nil
}

// RunsConfig holds active run selection and step timing defaults.
type RunsConfig struct {
	Active        string            `yaml:"active"`
	StepTimeoutMs int               `yaml:"step_timeout_ms"`
	Countess      CountessRunConfig `yaml:"countess"`

	sectionPresent bool `yaml:"-"`
}

// CountessRunConfig holds Countess-specific run tuning.
type CountessRunConfig struct {
	// Combat tunes the optional Countess kill phase.
	Combat CountessCombatConfig `yaml:"combat"`
}

// CountessCombatConfig holds Countess kill-phase combat tuning.
type CountessCombatConfig struct {
	// Profile selects the fixed MVP combat profile.
	Profile string `yaml:"profile"`
	// AttackSkill names the configured attack skill.
	AttackSkill string `yaml:"attack_skill"`
	// AttackIntervalMs throttles real attack inputs.
	AttackIntervalMs int `yaml:"attack_interval_ms"`
	// EngageDistanceTiles is the desired distance after combat repositioning.
	EngageDistanceTiles float64 `yaml:"engage_distance_tiles"`
	// RepositionDistanceTiles triggers teleport repositioning when exceeded.
	RepositionDistanceTiles float64 `yaml:"reposition_distance_tiles"`
	// KillConfirmTicks confirms death after consecutive valid absence ticks.
	KillConfirmTicks int `yaml:"kill_confirm_ticks"`
}

// UnmarshalYAML records whether the runs section was present in the YAML document.
func (c *RunsConfig) UnmarshalYAML(value *yaml.Node) error {
	type runsConfigAlias RunsConfig
	var alias runsConfigAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*c = RunsConfig(alias)
	c.sectionPresent = true
	return nil
}

func (c *RunsConfig) applyDefaults() {
	if c.StepTimeoutMs == 0 {
		c.StepTimeoutMs = 30000
	}
	c.Countess.Combat.applyDefaults()
}

func (c *CountessCombatConfig) applyDefaults() {
	if c.Profile == "" {
		c.Profile = "necro_bone_spear"
	}
	if c.AttackSkill == "" {
		c.AttackSkill = "bone_spear"
	}
	if c.AttackIntervalMs == 0 {
		c.AttackIntervalMs = 350
	}
	if c.EngageDistanceTiles == 0 {
		c.EngageDistanceTiles = 22
	}
	if c.RepositionDistanceTiles == 0 {
		c.RepositionDistanceTiles = 32
	}
	if c.KillConfirmTicks == 0 {
		c.KillConfirmTicks = 3
	}
}

// Load reads and validates a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg.LoadedFrom = path
	cfg.Input.applyDefaults()
	cfg.Runs.applyDefaults()
	cfg.Pathing.applyDefaults()
	cfg.Loot.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ResolvePath resolves rel against the directory of the loaded config file.
func (c *Config) ResolvePath(rel string) string {
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	if c.LoadedFrom == "" {
		return rel
	}
	return filepath.Join(filepath.Dir(c.LoadedFrom), rel)
}

func (c *Config) validate() error {
	c.Loot.applyDefaults()
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}
	if c.Process.ProcessName == "" {
		return fmt.Errorf("process.process_name is required")
	}
	if c.Runtime.PollIntervalMs <= 0 {
		return fmt.Errorf("runtime.poll_interval_ms must be > 0")
	}
	if c.Process.AttachTimeoutMs < 0 {
		return fmt.Errorf("process.attach_timeout_ms must be >= 0")
	}
	if err := c.Loot.validate(); err != nil {
		return err
	}
	if err := c.Input.validate(); err != nil {
		return err
	}
	if c.Runs.StepTimeoutMs <= 0 {
		return fmt.Errorf("runs.step_timeout_ms must be > 0")
	}
	if err := c.Runs.Countess.Combat.validate(); err != nil {
		return err
	}
	if err := c.Pathing.validate(); err != nil {
		return err
	}
	return nil
}

func (c CountessCombatConfig) validate() error {
	if c.Profile != "necro_bone_spear" {
		return fmt.Errorf("runs.countess.combat.profile must be necro_bone_spear")
	}
	if c.AttackSkill != "bone_spear" {
		return fmt.Errorf("runs.countess.combat.attack_skill must be bone_spear")
	}
	if c.AttackIntervalMs <= 0 {
		return fmt.Errorf("runs.countess.combat.attack_interval_ms must be > 0")
	}
	if c.EngageDistanceTiles <= 0 {
		return fmt.Errorf("runs.countess.combat.engage_distance_tiles must be > 0")
	}
	if c.RepositionDistanceTiles <= 0 {
		return fmt.Errorf("runs.countess.combat.reposition_distance_tiles must be > 0")
	}
	if c.EngageDistanceTiles >= c.RepositionDistanceTiles {
		return fmt.Errorf("runs.countess.combat.engage_distance_tiles must be < reposition_distance_tiles")
	}
	if c.KillConfirmTicks <= 0 {
		return fmt.Errorf("runs.countess.combat.kill_confirm_ticks must be > 0")
	}
	return nil
}

func (c *LootConfig) applyDefaults() {
	if c.PickitFile == "" {
		c.PickitFile = "pickit/countess.nip"
	}
	if c.inventoryLockPresent {
		return
	}
	c.InventoryLock = make([][]int, 4)
	for row := range c.InventoryLock {
		c.InventoryLock[row] = []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	}
}

func (c LootConfig) validate() error {
	if len(c.InventoryLock) != 4 {
		return fmt.Errorf("loot.inventory_lock must have 4 rows")
	}
	for row, cells := range c.InventoryLock {
		if len(cells) != 10 {
			return fmt.Errorf("loot.inventory_lock row %d must have 10 columns", row)
		}
		for col, cell := range cells {
			if cell != 0 && cell != 1 {
				return fmt.Errorf("loot.inventory_lock row %d column %d must be 0 or 1", row, col)
			}
		}
	}
	return nil
}

func (c *InputConfig) applyDefaults() {
	def := input.DefaultKeyboardConfig()
	if !c.sectionPresent {
		c.Enabled = false
		c.PauseHotkey = "pause"
		c.StopHotkey = "f12"
		c.KeyDelayMsMin = def.KeyDelayMsMin
		c.KeyDelayMsMax = def.KeyDelayMsMax
		c.ComboHoldMs = def.ComboHoldMs
		return
	}

	if c.PauseHotkey == "" {
		c.PauseHotkey = "pause"
	}
	if c.StopHotkey == "" {
		c.StopHotkey = "f12"
	}
	if c.ComboHoldMs == 0 {
		c.ComboHoldMs = def.ComboHoldMs
	}
}

func (c *InputConfig) validate() error {
	if c.KeyDelayMsMin < 0 {
		return fmt.Errorf("input.key_delay_ms_min must be >= 0")
	}
	if c.KeyDelayMsMax < c.KeyDelayMsMin {
		return fmt.Errorf("input.key_delay_ms_max must be >= input.key_delay_ms_min")
	}
	if c.ComboHoldMs < 0 {
		return fmt.Errorf("input.combo_hold_ms must be >= 0")
	}
	if c.PauseHotkey == "" {
		return fmt.Errorf("input.pause_hotkey is required")
	}
	if c.StopHotkey == "" {
		return fmt.Errorf("input.stop_hotkey is required")
	}
	if c.PauseHotkey == c.StopHotkey {
		return fmt.Errorf("input.pause_hotkey and input.stop_hotkey must differ")
	}
	if err := input.ValidateKeyStrings(c.PauseHotkey); err != nil {
		return fmt.Errorf("input.pause_hotkey: %w", err)
	}
	if err := input.ValidateKeyStrings(c.StopHotkey); err != nil {
		return fmt.Errorf("input.stop_hotkey: %w", err)
	}
	if err := c.Bindings.validate(); err != nil {
		return err
	}
	return nil
}

func (c InputBindingsConfig) validate() error {
	for name, binding := range c.Skills {
		if binding.Key == "" {
			return fmt.Errorf("input.bindings.skills.%s.key is required", name)
		}
		if err := input.ValidateKeyStrings(binding.Key); err != nil {
			return fmt.Errorf("input.bindings.skills.%s.key: %w", name, err)
		}
		switch binding.Button {
		case string(input.MouseLeft), string(input.MouseRight):
		default:
			return fmt.Errorf("input.bindings.skills.%s.button must be left or right", name)
		}
	}
	for slot, key := range c.Belt.keys() {
		if err := input.ValidateKeyStrings(key); err != nil {
			return fmt.Errorf("input.bindings.belt.slot_%d: %w", slot, err)
		}
	}
	return nil
}

func (c BeltBindingsConfig) keys() map[int]string {
	return map[int]string{
		1: c.Slot1,
		2: c.Slot2,
		3: c.Slot3,
		4: c.Slot4,
	}
}
