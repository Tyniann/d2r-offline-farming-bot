package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"gopkg.in/yaml.v3"
)

// Config holds application settings loaded from YAML.
type Config struct {
	App       AppConfig       `yaml:"app"`
	Runtime   RuntimeConfig   `yaml:"runtime"`
	Process   ProcessConfig   `yaml:"process"`
	Memory    MemoryConfig    `yaml:"memory"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	Loot      LootConfig      `yaml:"loot"`
	Input     InputConfig     `yaml:"input"`
	Runs      RunsConfig      `yaml:"runs"`
	Routes    RoutesConfig    `yaml:"routes"`
	Pathing   PathingConfig   `yaml:"pathing"`
	Paths     PathsConfig     `yaml:"paths"`
	Session   SessionConfig   `yaml:"session"`
	Profiles  ProfilesConfig  `yaml:"combat_profiles"`
	Town      town.Config     `yaml:"town"`

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

// TelemetryConfig selects the working-directory-relative JSONL output directory.
type TelemetryConfig struct {
	Directory string `yaml:"directory"`
}

type PathsConfig struct {
	ConfigDir string `yaml:"config_dir"`
}

// RoutesConfig selects the directory containing generic Route Contract files.
type RoutesConfig struct {
	Directory string `yaml:"directory"`
}

// LootConfig holds read-only loot model settings.
type LootConfig struct {
	Pickup        LootPickupConfig `yaml:"pickup"`
	Stash         LootStashConfig  `yaml:"stash"`
	InventoryLock [][]int          `yaml:"inventory_lock"`

	inventoryLockPresent bool `yaml:"-"`
}

// LootPickupConfig holds safety limits for hover-confirmed item pickup.
type LootPickupConfig struct {
	MaxRetries                int     `yaml:"max_retries"`
	MaxDistanceTiles          float64 `yaml:"max_distance_tiles"`
	VerifyTicks               int     `yaml:"verify_ticks"`
	VerifyTimeoutMs           int     `yaml:"verify_timeout_ms"`
	MonsterAbortDistanceTiles float64 `yaml:"monster_abort_distance_tiles"`
}

// LootStashConfig holds the hard-gated 1280x720 personal-inventory UI geometry and verification limits.
type LootStashConfig struct {
	MaxRetries      int `yaml:"max_retries"`
	VerifyTimeoutMs int `yaml:"verify_timeout_ms"`
	CloseTimeoutMs  int `yaml:"close_timeout_ms"`
	InventoryLeft   int `yaml:"inventory_left"`
	InventoryTop    int `yaml:"inventory_top"`
	InventoryCellW  int `yaml:"inventory_cell_width"`
	InventoryCellH  int `yaml:"inventory_cell_height"`
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
	Active        string               `yaml:"active"`
	StepTimeoutMs int                  `yaml:"step_timeout_ms"`
	Definitions   map[string]RunConfig `yaml:"definitions"`

	sectionPresent bool `yaml:"-"`
}

// RunConfig holds operator-selected tuning shared by every run definition.
type RunConfig struct {
	// RouteID selects a stable generic navigation route.
	RouteID string `yaml:"route_id"`
	// Combat selects the profile and regular attack tuning.
	Combat CombatConfig `yaml:"combat"`
	// Loot selects run-specific pickup and optional sell policies.
	Loot RunLootConfig `yaml:"loot"`
}

// CombatConfig holds shared boss-combat tuning.
type CombatConfig struct {
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

// RunLootConfig selects the independent pickup and sell policy files for one run.
type RunLootConfig struct {
	PickupFile string `yaml:"pickup_file"`
	SellFile   string `yaml:"sell_file"`
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
	for i := 0; i < len(value.Content)-1; i += 2 {
		if value.Content[i].Value == "countess" {
			return fmt.Errorf("runs.countess is unsupported; use runs.definitions.countess")
		}
	}
	return nil
}

func (c *RunsConfig) applyDefaults() {
	if c.StepTimeoutMs == 0 {
		c.StepTimeoutMs = 30000
	}
	if c.Definitions == nil {
		c.Definitions = map[string]RunConfig{}
	}
	for id, run := range c.Definitions {
		run.Combat.applyDefaults()
		if run.Loot.PickupFile == "" && id == "countess" {
			run.Loot.PickupFile = "pickit/countess.nip"
		}
		c.Definitions[id] = run
	}
}

func (c *CombatConfig) applyDefaults() {
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

// Run returns the config selected for id without exposing the mutable map entry.
func (c RunsConfig) Run(id string) (RunConfig, bool) {
	run, ok := c.Definitions[id]
	return run, ok
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
	cfg.Telemetry.applyDefaults()
	cfg.Routes.applyDefaults()
	cfg.Session.applyDefaults()
	cfg.Profiles.applyDefaults()
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
	c.Telemetry.applyDefaults()
	c.Routes.applyDefaults()
	c.Session.applyDefaults()
	c.Profiles.applyDefaults()
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
	if strings.TrimSpace(c.Telemetry.Directory) == "" {
		return fmt.Errorf("telemetry.directory is required")
	}
	if strings.TrimSpace(c.Routes.Directory) == "" {
		return fmt.Errorf("routes.directory is required")
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
	for id, run := range c.Runs.Definitions {
		if err := run.validate(id); err != nil {
			return err
		}
		if err := c.Profiles.validate(run.Combat.Profile, "runs.definitions."+id+".combat.profile"); err != nil {
			return err
		}
	}
	if err := c.Pathing.validate(); err != nil {
		return err
	}
	if err := c.Session.validate(); err != nil {
		return err
	}
	if err := c.Town.Validate(); err != nil {
		return err
	}
	return nil
}

func (c *TelemetryConfig) applyDefaults() {
	if c.Directory == "" {
		c.Directory = filepath.Join("logs", "telemetry")
	}
}

func (c *RoutesConfig) applyDefaults() {
	if c.Directory == "" {
		c.Directory = filepath.Join("routes", "farming")
	}
}

func (c RunConfig) validate(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("runs.definitions contains an empty run id")
	}
	if strings.TrimSpace(c.Loot.PickupFile) == "" {
		return fmt.Errorf("runs.definitions.%s.loot.pickup_file is required", id)
	}
	return c.Combat.validate("runs.definitions." + id + ".combat")
}

func (c CombatConfig) validate(path string) error {
	if strings.TrimSpace(c.Profile) == "" {
		return fmt.Errorf("%s.profile is required", path)
	}
	if c.AttackSkill != "bone_spear" {
		return fmt.Errorf("%s.attack_skill must be bone_spear", path)
	}
	if c.AttackIntervalMs <= 0 {
		return fmt.Errorf("%s.attack_interval_ms must be > 0", path)
	}
	if c.EngageDistanceTiles <= 0 {
		return fmt.Errorf("%s.engage_distance_tiles must be > 0", path)
	}
	if c.RepositionDistanceTiles <= 0 {
		return fmt.Errorf("%s.reposition_distance_tiles must be > 0", path)
	}
	if c.EngageDistanceTiles >= c.RepositionDistanceTiles {
		return fmt.Errorf("%s.engage_distance_tiles must be < reposition_distance_tiles", path)
	}
	if c.KillConfirmTicks <= 0 {
		return fmt.Errorf("%s.kill_confirm_ticks must be > 0", path)
	}
	return nil
}

func (c *LootConfig) applyDefaults() {
	c.Pickup.applyDefaults()
	c.Stash.applyDefaults()
	if c.inventoryLockPresent {
		return
	}
	c.InventoryLock = make([][]int, 4)
	for row := range c.InventoryLock {
		c.InventoryLock[row] = []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	}
}

func (c *LootStashConfig) applyDefaults() {
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.VerifyTimeoutMs == 0 {
		c.VerifyTimeoutMs = 1500
	}
	if c.CloseTimeoutMs == 0 {
		c.CloseTimeoutMs = 1500
	}
	if c.InventoryLeft == 0 {
		c.InventoryLeft = 847
	}
	if c.InventoryTop == 0 {
		c.InventoryTop = 369
	}
	if c.InventoryCellW == 0 {
		c.InventoryCellW = 33
	}
	if c.InventoryCellH == 0 {
		c.InventoryCellH = 33
	}
}

func (c *LootPickupConfig) applyDefaults() {
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.MaxDistanceTiles == 0 {
		c.MaxDistanceTiles = 8
	}
	if c.VerifyTicks == 0 {
		c.VerifyTicks = 3
	}
	if c.VerifyTimeoutMs == 0 {
		c.VerifyTimeoutMs = 1500
	}
	if c.MonsterAbortDistanceTiles == 0 {
		c.MonsterAbortDistanceTiles = 12
	}
}

func (c LootConfig) validate() error {
	if c.Pickup.MaxRetries <= 0 {
		return fmt.Errorf("loot.pickup.max_retries must be > 0")
	}
	if c.Pickup.MaxDistanceTiles <= 0 {
		return fmt.Errorf("loot.pickup.max_distance_tiles must be > 0")
	}
	if c.Pickup.VerifyTicks <= 0 {
		return fmt.Errorf("loot.pickup.verify_ticks must be > 0")
	}
	if c.Pickup.VerifyTimeoutMs <= 0 {
		return fmt.Errorf("loot.pickup.verify_timeout_ms must be > 0")
	}
	if c.Pickup.MonsterAbortDistanceTiles <= 0 {
		return fmt.Errorf("loot.pickup.monster_abort_distance_tiles must be > 0")
	}
	if c.Stash.MaxRetries <= 0 || c.Stash.VerifyTimeoutMs <= 0 || c.Stash.CloseTimeoutMs <= 0 {
		return fmt.Errorf("loot.stash retry and timeout values must be > 0")
	}
	if c.Stash.InventoryLeft < 0 || c.Stash.InventoryTop < 0 || c.Stash.InventoryCellW <= 0 || c.Stash.InventoryCellH <= 0 {
		return fmt.Errorf("loot.stash inventory geometry is invalid")
	}
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
