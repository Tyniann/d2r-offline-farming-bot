package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

// SessionPlan is the fully resolved, read-only operator view of Phase-7 session
// selection and finite budgets. It does not authorize process attach or input.
type SessionPlan struct {
	Status                 string   `json:"status"`
	Enabled                bool     `json:"enabled"`
	Run                    string   `json:"run"`
	Character              string   `json:"character"`
	Difficulty             string   `json:"difficulty"`
	RouteID                string   `json:"route_id"`
	RoutePath              string   `json:"route_path,omitempty"`
	RouteLayoutFingerprint string   `json:"route_layout_fingerprint,omitempty"`
	GameVersion            string   `json:"game_version"`
	MaxRuns                int      `json:"max_runs"`
	MaxDurationMs          int      `json:"max_duration_ms"`
	CooldownMs             int      `json:"cooldown_ms"`
	MaxConsecutiveFailures int      `json:"max_consecutive_failures"`
	MaxTotalRestarts       int      `json:"max_total_restarts"`
	StateTimeoutMs         int      `json:"state_timeout_ms"`
	ExitTimeoutMs          int      `json:"exit_timeout_ms"`
	StartTimeoutMs         int      `json:"start_timeout_ms"`
	RetryClasses           []string `json:"retry_classes"`
	TelemetryDirectory     string   `json:"telemetry_directory"`
	ClientWidth            int      `json:"client_width"`
	ClientHeight           int      `json:"client_height"`
}

// ResolveSessionPlan validates an autonomous-session selection without
// initializing Runtime, attaching to D2R, registering hotkeys, or sending input.
func ResolveSessionPlan(cfg *config.Config, opts Options) (SessionPlan, error) {
	if cfg == nil {
		return SessionPlan{}, fmt.Errorf("session inspect requires config")
	}
	if err := validateSessionInspectExclusivity(opts); err != nil {
		return SessionPlan{}, err
	}
	session := cfg.Session
	plan := SessionPlan{
		Status: "disabled", Enabled: session.Enabled, Run: session.Run, Character: session.Character,
		Difficulty: session.Difficulty, RouteID: cfg.Runs.Countess.RouteID, GameVersion: cfg.Memory.GameVersion,
		MaxRuns: session.MaxRuns, MaxDurationMs: session.MaxDurationMs, CooldownMs: session.CooldownMs,
		MaxConsecutiveFailures: session.MaxConsecutiveFailures, MaxTotalRestarts: session.MaxTotalRestarts,
		StateTimeoutMs: session.StateTimeoutMs, ExitTimeoutMs: session.ExitTimeoutMs, StartTimeoutMs: session.StartTimeoutMs,
		RetryClasses: append([]string(nil), session.RetryClasses...), TelemetryDirectory: cfg.Telemetry.Directory,
		ClientWidth: offlineDifficultyClientWidth, ClientHeight: offlineDifficultyClientHeight,
	}
	if !session.Enabled {
		return plan, nil
	}
	if !cfg.Input.Enabled {
		return SessionPlan{}, fmt.Errorf("session.enabled=true requires input.enabled=true")
	}
	if cfg.Runs.Active != "" {
		return SessionPlan{}, fmt.Errorf("session.enabled=true requires runs.active to be empty")
	}
	if !tasks.IsKnownRun(session.Run) {
		return SessionPlan{}, fmt.Errorf("session.run is unknown: %q", session.Run)
	}
	if session.Run != "countess" {
		return SessionPlan{}, fmt.Errorf("session.run %q has no Phase-7 full-run preflight", session.Run)
	}
	character, err := validateOfflineCharacter(session.Character)
	if err != nil {
		return SessionPlan{}, fmt.Errorf("session.character: %w", err)
	}
	if cfg.Runs.Countess.RouteID == "" {
		return SessionPlan{}, fmt.Errorf("runs.countess.route_id is required for session.run=countess")
	}
	townGraphPath := filepath.Join(cfg.ResolvePath(cfg.Town.Hub.RoutesDirectory), "graph.yaml")
	if _, err := town.LoadServiceGraph(townGraphPath); err != nil {
		return SessionPlan{}, fmt.Errorf("session town graph: %w", err)
	}
	registry, err := pathing.LoadRouteRegistry(cfg.ResolvePath(cfg.Routes.Directory))
	if err != nil {
		return SessionPlan{}, fmt.Errorf("session route registry: %w", err)
	}
	route, err := registry.Get(cfg.Runs.Countess.RouteID)
	if err != nil {
		return SessionPlan{}, fmt.Errorf("session route: %w", err)
	}
	if !strings.EqualFold(route.Binding.CharacterName, character) {
		return SessionPlan{}, fmt.Errorf("session character %q does not match route character %q", character, route.Binding.CharacterName)
	}
	if string(route.Binding.Difficulty) != session.Difficulty {
		return SessionPlan{}, fmt.Errorf("session difficulty %q does not match route difficulty %q", session.Difficulty, route.Binding.Difficulty)
	}
	if cfg.Memory.GameVersion != "" && route.Binding.GameVersion != cfg.Memory.GameVersion {
		return SessionPlan{}, fmt.Errorf("session game version %q does not match route game version %q", cfg.Memory.GameVersion, route.Binding.GameVersion)
	}
	configDirectory := filepath.Dir(cfg.LoadedFrom)
	for _, template := range []string{
		filepath.Join(configDirectory, "ui", "character-play.png"),
		filepath.Join(configDirectory, "ui", "difficulty-dialog.png"),
		filepath.Join(configDirectory, "ui", "characters", strings.ToLower(character)+"-selected.png"),
	} {
		if _, err := os.Stat(template); err != nil {
			return SessionPlan{}, fmt.Errorf("session UI anchor %q: %w", template, err)
		}
	}
	plan.Status = "ready"
	plan.RoutePath = routeSourcePath(registry, route.ID)
	plan.RouteLayoutFingerprint = route.Binding.LayoutFingerprint.Hash
	return plan, nil
}

func resolveWorkspacePath(cfg *config.Config, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if _, err := os.Stat(path); err == nil || cfg.LoadedFrom == "" {
		return path
	}
	return filepath.Join(filepath.Dir(filepath.Dir(cfg.LoadedFrom)), path)
}

func routeSourcePath(registry *pathing.RouteRegistry, routeID string) string {
	for _, entry := range registry.Entries() {
		if entry.ID == routeID && entry.Status == pathing.RouteRegistryValid {
			return entry.Path
		}
	}
	return ""
}

func validateSessionInspectExclusivity(opts Options) error {
	if !opts.SessionInspect {
		return fmt.Errorf("session inspect mode is not selected")
	}
	if opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.Route != "" || opts.RouteName != "" || opts.RouteDifficulty != "" {
		return fmt.Errorf("--session-inspect is mutually exclusive with run, probe, route, and test modes")
	}
	return nil
}
