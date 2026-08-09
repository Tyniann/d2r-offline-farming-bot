package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

var supportedSessionRetryClasses = map[string]struct{}{
	"hard_stuck":                 {},
	"route_drift_exceeded":       {},
	"route_segment_timeout":      {},
	"route_transition_failed":    {},
	"route_clear_no_progress":    {},
	"route_threat_out_of_range":  {},
	"route_mana_recovery_failed": {},
	"route_recovery_unsafe":      {},
	"boss_combat_unprojectable":  {},
	"cow_combat_no_progress":     {},
}

var legacySessionRetryClasses = []string{
	"hard_stuck", "route_drift_exceeded", "route_segment_timeout", "route_transition_failed",
}

// phase17SessionRetryClasses is the exact default list shipped before boss-combat
// projection loss became retryable. Matching configs may be migrated safely.
var phase17SessionRetryClasses = []string{
	"hard_stuck", "route_drift_exceeded", "route_segment_timeout", "route_transition_failed",
	"route_clear_no_progress", "route_threat_out_of_range", "route_mana_recovery_failed", "route_recovery_unsafe",
}

// phase19SessionRetryClasses is the exact default list shipped before the Cow
// Hold watchdog gained its dedicated retry classification.
var phase19SessionRetryClasses = []string{
	"hard_stuck", "route_drift_exceeded", "route_segment_timeout", "route_transition_failed",
	"route_clear_no_progress", "route_threat_out_of_range", "route_mana_recovery_failed", "route_recovery_unsafe",
	"boss_combat_unprojectable",
}

var defaultSessionRetryClasses = []string{
	"hard_stuck", "route_drift_exceeded", "route_segment_timeout", "route_transition_failed",
	"route_clear_no_progress", "route_threat_out_of_range", "route_mana_recovery_failed", "route_recovery_unsafe",
	"boss_combat_unprojectable", "cow_combat_no_progress",
}

// SessionConfig defines finite budgets and static selection for an autonomous
// offline session. Enabled remains false unless the operator explicitly opts in.
type SessionConfig struct {
	Enabled                bool     `yaml:"enabled"`
	Run                    string   `yaml:"run"`
	Queue                  []string `yaml:"queue"`
	Character              string   `yaml:"character"`
	Difficulty             string   `yaml:"difficulty"`
	MaxRuns                int      `yaml:"max_runs"`
	MaxDurationMs          int      `yaml:"max_duration_ms"`
	CooldownMs             int      `yaml:"cooldown_ms"`
	MaxConsecutiveFailures int      `yaml:"max_consecutive_failures"`
	MaxTotalRestarts       int      `yaml:"max_total_restarts"`
	StateTimeoutMs         int      `yaml:"state_timeout_ms"`
	ExitTimeoutMs          int      `yaml:"exit_timeout_ms"`
	StartTimeoutMs         int      `yaml:"start_timeout_ms"`
	RetryClasses           []string `yaml:"retry_classes"`

	present map[string]bool `yaml:"-"`
}

// UnmarshalYAML records explicitly configured zero-valued budgets so they are
// not replaced by defaults where zero is a valid, restrictive value.
func (c *SessionConfig) UnmarshalYAML(value *yaml.Node) error {
	type sessionConfigAlias SessionConfig
	var alias sessionConfigAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*c = SessionConfig(alias)
	c.present = make(map[string]bool)
	for i := 0; i < len(value.Content)-1; i += 2 {
		c.present[value.Content[i].Value] = true
	}
	return nil
}

func (c *SessionConfig) applyDefaults() {
	if c.Run == "" {
		c.Run = "countess"
	}
	if c.Queue == nil {
		c.Queue = []string{c.Run}
	}
	if c.Difficulty == "" {
		c.Difficulty = "normal"
	}
	if !c.present["max_runs"] && c.MaxRuns == 0 {
		c.MaxRuns = 3
	}
	if !c.present["max_duration_ms"] && c.MaxDurationMs == 0 {
		c.MaxDurationMs = 7200000
	}
	if !c.present["cooldown_ms"] && c.CooldownMs == 0 {
		c.CooldownMs = 3000
	}
	if !c.present["max_consecutive_failures"] && c.MaxConsecutiveFailures == 0 {
		c.MaxConsecutiveFailures = 2
	}
	if !c.present["max_total_restarts"] && c.MaxTotalRestarts == 0 {
		c.MaxTotalRestarts = 3
	}
	if !c.present["state_timeout_ms"] && c.StateTimeoutMs == 0 {
		c.StateTimeoutMs = 30000
	}
	if !c.present["exit_timeout_ms"] && c.ExitTimeoutMs == 0 {
		c.ExitTimeoutMs = 30000
	}
	if !c.present["start_timeout_ms"] && c.StartTimeoutMs == 0 {
		c.StartTimeoutMs = 45000
	}
	if c.RetryClasses == nil {
		c.RetryClasses = append([]string(nil), defaultSessionRetryClasses...)
	} else if equalStrings(c.RetryClasses, legacySessionRetryClasses) || equalStrings(c.RetryClasses, phase17SessionRetryClasses) ||
		equalStrings(c.RetryClasses, phase19SessionRetryClasses) {
		// Exact prior defaults are safe to migrate. Any reordered, reduced, or
		// extended list is an operator decision and stays untouched.
		c.RetryClasses = append([]string(nil), defaultSessionRetryClasses...)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (c SessionConfig) validate() error {
	if c.Run == "" {
		return fmt.Errorf("session.run is required")
	}
	if len(c.Queue) == 0 {
		return fmt.Errorf("session.queue must contain at least one run")
	}
	for i, run := range c.Queue {
		if run == "" {
			return fmt.Errorf("session.queue[%d] must not be empty", i)
		}
		for first := 0; first < i; first++ {
			if c.Queue[first] == run {
				return fmt.Errorf("queue_duplicate_run: session.queue[%d]=%q duplicates session.queue[%d]", i, run, first)
			}
		}
	}
	switch c.Difficulty {
	case "normal", "nightmare", "hell":
	default:
		return fmt.Errorf("session.difficulty must be normal, nightmare, or hell")
	}
	if c.MaxRuns <= 0 {
		return fmt.Errorf("session.max_runs must be > 0")
	}
	if c.MaxDurationMs <= 0 {
		return fmt.Errorf("session.max_duration_ms must be > 0")
	}
	if c.CooldownMs < 0 {
		return fmt.Errorf("session.cooldown_ms must be >= 0")
	}
	if c.MaxConsecutiveFailures < 0 {
		return fmt.Errorf("session.max_consecutive_failures must be >= 0")
	}
	if c.MaxTotalRestarts < 0 {
		return fmt.Errorf("session.max_total_restarts must be >= 0")
	}
	if c.StateTimeoutMs <= 0 || c.ExitTimeoutMs <= 0 || c.StartTimeoutMs <= 0 {
		return fmt.Errorf("session state, exit, and start timeouts must be > 0")
	}
	seen := make(map[string]struct{}, len(c.RetryClasses))
	for _, class := range c.RetryClasses {
		if _, ok := supportedSessionRetryClasses[class]; !ok {
			return fmt.Errorf("session.retry_classes contains unsupported class %q", class)
		}
		if _, duplicate := seen[class]; duplicate {
			return fmt.Errorf("session.retry_classes contains duplicate class %q", class)
		}
		seen[class] = struct{}{}
	}
	return nil
}
