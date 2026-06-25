package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds application settings loaded from YAML.
type Config struct {
	App     AppConfig     `yaml:"app"`
	Runtime RuntimeConfig `yaml:"runtime"`
	Process ProcessConfig `yaml:"process"`
	Memory  MemoryConfig  `yaml:"memory"`
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
	return nil
}
