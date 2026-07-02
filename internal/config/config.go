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
	Active        string `yaml:"active"`
	StepTimeoutMs int    `yaml:"step_timeout_ms"`

	sectionPresent bool `yaml:"-"`
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
	if err := c.Input.validate(); err != nil {
		return err
	}
	if c.Runs.StepTimeoutMs <= 0 {
		return fmt.Errorf("runs.step_timeout_ms must be > 0")
	}
	if err := c.Pathing.validate(); err != nil {
		return err
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
