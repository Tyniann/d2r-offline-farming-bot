package config

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSessionDefaultsAreFiniteAndDisabled(t *testing.T) {
	var cfg SessionConfig
	cfg.applyDefaults()
	if cfg.Enabled || cfg.Run != "countess" || cfg.Difficulty != "normal" {
		t.Fatalf("selection defaults = %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.Queue, []string{"countess"}) {
		t.Fatalf("queue default = %v", cfg.Queue)
	}
	if cfg.MaxRuns != 3 || cfg.MaxDurationMs != 7200000 || cfg.StateTimeoutMs <= 0 || cfg.ExitTimeoutMs <= 0 || cfg.StartTimeoutMs <= 0 {
		t.Fatalf("budget defaults = %+v", cfg)
	}
	wantRetry := []string{
		"hard_stuck", "route_drift_exceeded", "route_segment_timeout", "route_transition_failed",
		"route_clear_no_progress", "route_threat_out_of_range", "route_mana_recovery_failed", "route_recovery_unsafe",
		"boss_combat_unprojectable", "cow_combat_no_progress", "stash_approach_failed",
		"boss_combat_no_progress",
	}
	if !reflect.DeepEqual(cfg.RetryClasses, wantRetry) {
		t.Fatalf("retry defaults = %v, want %v", cfg.RetryClasses, wantRetry)
	}
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionCustomizedRetryClassesRemainUnchanged(t *testing.T) {
	var cfg SessionConfig
	if err := yaml.Unmarshal([]byte("retry_classes:\n  - hard_stuck\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, []string{"hard_stuck"}) {
		t.Fatalf("custom retry classes changed: %v", cfg.RetryClasses)
	}
}

func TestSessionLegacyDefaultRetryClassesGainPhase17Reasons(t *testing.T) {
	var cfg SessionConfig
	if err := yaml.Unmarshal([]byte(`retry_classes:
  - hard_stuck
  - route_drift_exceeded
  - route_segment_timeout
  - route_transition_failed
`), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, defaultSessionRetryClasses) {
		t.Fatalf("migrated retry classes = %v, want %v", cfg.RetryClasses, defaultSessionRetryClasses)
	}
}

func TestSessionPhase17DefaultRetryClassesGainBossCombatReason(t *testing.T) {
	var cfg SessionConfig
	cfg.RetryClasses = append([]string(nil), phase17SessionRetryClasses...)
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, defaultSessionRetryClasses) {
		t.Fatalf("migrated retry classes = %v, want %v", cfg.RetryClasses, defaultSessionRetryClasses)
	}
}

func TestSessionPhase19DefaultRetryClassesGainCowCombatReason(t *testing.T) {
	var cfg SessionConfig
	cfg.RetryClasses = append([]string(nil), phase19SessionRetryClasses...)
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, defaultSessionRetryClasses) {
		t.Fatalf("migrated retry classes = %v, want %v", cfg.RetryClasses, defaultSessionRetryClasses)
	}
}

func TestSessionPhase25DefaultRetryClassesGainStashApproachReason(t *testing.T) {
	var cfg SessionConfig
	cfg.RetryClasses = append([]string(nil), phase25SessionRetryClasses...)
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, defaultSessionRetryClasses) {
		t.Fatalf("migrated retry classes = %v, want %v", cfg.RetryClasses, defaultSessionRetryClasses)
	}
}

func TestSessionPhase26DefaultRetryClassesGainBossCombatNoProgress(t *testing.T) {
	var cfg SessionConfig
	cfg.RetryClasses = append([]string(nil), phase26SessionRetryClasses...)
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, defaultSessionRetryClasses) {
		t.Fatalf("migrated retry classes = %v, want %v", cfg.RetryClasses, defaultSessionRetryClasses)
	}
}

func TestSessionReorderedLegacyRetryClassesRemainUnchanged(t *testing.T) {
	var cfg SessionConfig
	want := []string{"route_drift_exceeded", "hard_stuck", "route_segment_timeout", "route_transition_failed"}
	cfg.RetryClasses = append([]string(nil), want...)
	cfg.applyDefaults()
	if !reflect.DeepEqual(cfg.RetryClasses, want) {
		t.Fatalf("customized retry order changed: %v", cfg.RetryClasses)
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
		{"empty queue", func(c *SessionConfig) { c.Queue = []string{} }},
		{"empty queue entry", func(c *SessionConfig) { c.Queue = []string{"countess", ""} }},
		{"duplicate queue entry", func(c *SessionConfig) { c.Queue = []string{"countess", "countess"} }},
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
