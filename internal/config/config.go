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

// InputConfig holds keyboard timing, safety, and key-mapping settings.
type InputConfig struct {
	Enabled       bool      `yaml:"enabled"`
	PauseHotkey   string    `yaml:"pause_hotkey"`
	StopHotkey    string    `yaml:"stop_hotkey"`
	KeyDelayMsMin int       `yaml:"key_delay_ms_min"`
	KeyDelayMsMax int       `yaml:"key_delay_ms_max"`
	ComboHoldMs   int       `yaml:"combo_hold_ms"`
	Skills        SkillKeys `yaml:"skills"`
	Belt          BeltKeys  `yaml:"belt"`
	TownPortal    string    `yaml:"town_portal"`

	sectionPresent bool `yaml:"-"`
}

// UnmarshalYAML records whether the input section was present in the YAML document.
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

// SkillKeys maps skill bar slots 1–8 to key strings.
type SkillKeys struct {
	Slot1 string `yaml:"slot1"`
	Slot2 string `yaml:"slot2"`
	Slot3 string `yaml:"slot3"`
	Slot4 string `yaml:"slot4"`
	Slot5 string `yaml:"slot5"`
	Slot6 string `yaml:"slot6"`
	Slot7 string `yaml:"slot7"`
	Slot8 string `yaml:"slot8"`
}

// BeltKeys maps belt slots 1–4 to key strings.
type BeltKeys struct {
	Slot1 string `yaml:"slot1"`
	Slot2 string `yaml:"slot2"`
	Slot3 string `yaml:"slot3"`
	Slot4 string `yaml:"slot4"`
}

// Slots returns skill slot keys in order (slot 1 first).
func (s SkillKeys) Slots() [8]string {
	return [8]string{s.Slot1, s.Slot2, s.Slot3, s.Slot4, s.Slot5, s.Slot6, s.Slot7, s.Slot8}
}

// Slots returns belt slot keys in order (slot 1 first).
func (b BeltKeys) Slots() [4]string {
	return [4]string{b.Slot1, b.Slot2, b.Slot3, b.Slot4}
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
		c.Skills = skillKeysFromArray(def.Skills)
		c.Belt = beltKeysFromArray(def.Belt)
		c.TownPortal = def.TownPortal
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
	c.Skills.fillEmpty(def.Skills)
	c.Belt.fillEmpty(def.Belt)
}

func skillKeysFromArray(slots [8]string) SkillKeys {
	return SkillKeys{
		Slot1: slots[0], Slot2: slots[1], Slot3: slots[2], Slot4: slots[3],
		Slot5: slots[4], Slot6: slots[5], Slot7: slots[6], Slot8: slots[7],
	}
}

func beltKeysFromArray(slots [4]string) BeltKeys {
	return BeltKeys{Slot1: slots[0], Slot2: slots[1], Slot3: slots[2], Slot4: slots[3]}
}

func (s *SkillKeys) fillEmpty(def [8]string) {
	slots := s.Slots()
	for i := range slots {
		if slots[i] == "" {
			slots[i] = def[i]
		}
	}
	*s = skillKeysFromArray(slots)
}

func (b *BeltKeys) fillEmpty(def [4]string) {
	slots := b.Slots()
	for i := range slots {
		if slots[i] == "" {
			slots[i] = def[i]
		}
	}
	*b = beltKeysFromArray(slots)
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

	slots := c.Skills.Slots()
	if err := input.ValidateKeyStrings(slots[:]...); err != nil {
		return fmt.Errorf("input.skills: %w", err)
	}
	belt := c.Belt.Slots()
	if err := input.ValidateKeyStrings(belt[:]...); err != nil {
		return fmt.Errorf("input.belt: %w", err)
	}
	if err := input.ValidateKeyStrings(c.TownPortal); err != nil {
		return fmt.Errorf("input.town_portal: %w", err)
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
	return nil
}
