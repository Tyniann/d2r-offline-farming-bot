package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSessionDefaultsAreFiniteAndDisabled(t *testing.T) {
	var cfg SessionConfig
	cfg.applyDefaults()
	if cfg.Enabled || cfg.Run != "countess" || cfg.Difficulty != "normal" {
		t.Fatalf("selection defaults = %+v", cfg)
	}
	if cfg.MaxRuns != 3 || cfg.MaxDurationMs != 7200000 || cfg.StateTimeoutMs <= 0 || cfg.ExitTimeoutMs <= 0 || cfg.StartTimeoutMs <= 0 {
		t.Fatalf("budget defaults = %+v", cfg)
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionExplicitZeroRestrictiveBudgetsRemainZero(t *testing.T) {
	var cfg SessionConfig
	if err := yaml.Unmarshal([]byte("cooldown_ms: 0\nmax_consecutive_failures: 0\nmax_total_restarts: 0\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.applyDefaults()
	if cfg.CooldownMs != 0 || cfg.MaxConsecutiveFailures != 0 || cfg.MaxTotalRestarts != 0 {
		t.Fatalf("explicit restrictive zeros changed: %+v", cfg)
	}
}

func TestSessionExplicitZeroRequiredBudgetIsRejected(t *testing.T) {
	var cfg SessionConfig
	if err := yaml.Unmarshal([]byte("max_runs: 0\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err == nil {
		t.Fatal("expected explicit max_runs=0 to be rejected")
	}
}

func TestSessionValidationRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SessionConfig)
	}{
		{"unbounded runs", func(c *SessionConfig) { c.MaxRuns = -1 }},
		{"unbounded duration", func(c *SessionConfig) { c.MaxDurationMs = -1 }},
		{"negative cooldown", func(c *SessionConfig) { c.CooldownMs = -1 }},
		{"invalid difficulty", func(c *SessionConfig) { c.Difficulty = "players8" }},
		{"unknown retry", func(c *SessionConfig) { c.RetryClasses = []string{"unknown"} }},
		{"duplicate retry", func(c *SessionConfig) { c.RetryClasses = []string{"hard_stuck", "hard_stuck"} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var cfg SessionConfig
			cfg.applyDefaults()
			tc.mutate(&cfg)
			if err := cfg.validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
