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
	Status                      string   `json:"status"`
	Enabled                     bool     `json:"enabled"`
	Run                         string   `json:"run"`
	Queue                       []string `json:"queue"`
	Character                   string   `json:"character"`
	Difficulty                  string   `json:"difficulty"`
	RouteID                     string   `json:"route_id"`
	RoutePath                   string   `json:"route_path,omitempty"`
	RouteLayoutFingerprint      string   `json:"route_layout_fingerprint,omitempty"`
	SetupRouteID                string   `json:"setup_route_id,omitempty"`
	SetupRoutePath              string   `json:"setup_route_path,omitempty"`
	SetupRouteLayoutFingerprint string   `json:"setup_route_layout_fingerprint,omitempty"`
	GameVersion                 string   `json:"game_version"`
	MaxRuns                     int      `json:"max_runs"`
	MaxDurationMs               int      `json:"max_duration_ms"`
	CooldownMs                  int      `json:"cooldown_ms"`
	MaxConsecutiveFailures      int      `json:"max_consecutive_failures"`
	MaxTotalRestarts            int      `json:"max_total_restarts"`
	StateTimeoutMs              int      `json:"state_timeout_ms"`
	ExitTimeoutMs               int      `json:"exit_timeout_ms"`
	StartTimeoutMs              int      `json:"start_timeout_ms"`
	RetryClasses                []string `json:"retry_classes"`
	TelemetryDirectory          string   `json:"telemetry_directory"`
	ClientWidth                 int      `json:"client_width"`
	ClientHeight                int      `json:"client_height"`
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
		Status: "disabled", Enabled: session.Enabled, Run: session.Run, Queue: append([]string(nil), session.Queue...), Character: session.Character,
		Difficulty: session.Difficulty, GameVersion: cfg.Memory.GameVersion,
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
	character, err := validateOfflineCharacter(session.Character)
	if err != nil {
		return SessionPlan{}, fmt.Errorf("session.character: %w", err)
	}
	availabilityContext, err := resolveSessionAvailabilityContext(cfg, opts, character)
	if err != nil {
		return SessionPlan{}, err
	}
	availability, err := resolveRunAvailabilities(cfg, availabilityContext)
	if err != nil {
		return SessionPlan{}, err
	}
	selected, ok := findRunAvailability(availability.report.Runs, tasks.RunID(session.Run))
	if !ok {
		return SessionPlan{}, fmt.Errorf("%s: %q", tasks.RunReasonUnknown, session.Run)
	}
	if selected.Status == tasks.RunAvailabilityUnavailable {
		return SessionPlan{}, fmt.Errorf("session.run %q unavailable: %s", session.Run, joinRunReasons(selected.Reasons))
	}
	plan.RouteID = selected.Route.RouteID
	route, ok := availability.routes[tasks.RunID(session.Run)]
	if !ok {
		return SessionPlan{}, fmt.Errorf("session.run %q unavailable: %s", session.Run, tasks.RunReasonRouteMissing)
	}
	townGraphPath := filepath.Join(cfg.ResolvePath(cfg.Town.Hub.RoutesDirectory), "graph.yaml")
	if _, graphErr := town.LoadServiceGraph(townGraphPath); graphErr != nil {
		return SessionPlan{}, fmt.Errorf("session town graph: %w", graphErr)
	}
	configDirectory := filepath.Dir(cfg.LoadedFrom)
	for _, template := range []string{
		filepath.Join(configDirectory, "ui", "character-play.png"),
		filepath.Join(configDirectory, "ui", "difficulty-dialog.png"),
		filepath.Join(configDirectory, "ui", "characters", strings.ToLower(character)+"-selected.png"),
	} {
		if _, statErr := os.Stat(template); statErr != nil {
			return SessionPlan{}, fmt.Errorf("session UI anchor %q: %w", template, statErr)
		}
	}
	profiles, err := NewPickitProfileService(cfg.ResolvePath(filepath.Join("pickit", "profiles")))
	if err != nil {
		return SessionPlan{}, fmt.Errorf("session pickit profiles: %w", err)
	}
	if setupErr := ValidateCharacterSetupConfig(cfg, profiles); setupErr != nil {
		return SessionPlan{}, fmt.Errorf("session character setup config: %w", setupErr)
	}
	assignments, err := NewPickitAssignmentStore(cfg.ResolvePath("pickit-assignments.local.yaml"), profiles)
	if err != nil {
		return SessionPlan{}, fmt.Errorf("session pickit assignments: %w", err)
	}
	for _, runID := range session.Queue {
		if _, err := assignments.Resolve(character, tasks.RunID(runID)); err != nil {
			return SessionPlan{}, fmt.Errorf("session pickit assignment %s/%s: %w", character, runID, err)
		}
	}
	plan.Status = "ready"
	plan.RoutePath = availability.routePaths[route.ID]
	plan.RouteLayoutFingerprint = route.Binding.LayoutFingerprint.Hash
	if err := bindSessionSetupRoute(&plan, availability, tasks.RunID(session.Run)); err != nil {
		return SessionPlan{}, err
	}
	return plan, nil
}

func resolveSessionAvailabilityContext(cfg *config.Config, opts Options, character string) (RunAvailabilityContext, error) {
	context := RunAvailabilityContext{
		Character: character, Difficulty: cfg.Session.Difficulty, GameVersion: cfg.Memory.GameVersion,
	}
	if opts.Loadout == nil {
		return context, nil
	}
	profileID, err := resolveRuntimeCombatProfileID(cfg, opts.Loadout)
	if err != nil {
		return RunAvailabilityContext{}, fmt.Errorf("session combat profile: %w", err)
	}
	context.CombatProfile = profileID
	context.CharacterClass = cfg.Profiles[profileID].CharacterClass
	return context, nil
}

func bindSessionSetupRoute(plan *SessionPlan, availability runAvailabilityResolution, runID tasks.RunID) error {
	roles := availability.routeSets[runID]
	if roles == nil {
		return nil
	}
	setup, ok := roles[pathing.RouteRoleLegAcquisition]
	if !ok {
		return fmt.Errorf("session.run %q unavailable: %s", runID, tasks.RunReasonLegAcquisitionRouteMissing)
	}
	plan.SetupRouteID = setup.ID
	plan.SetupRoutePath = availability.routePaths[setup.ID]
	plan.SetupRouteLayoutFingerprint = setup.Binding.LayoutFingerprint.Hash
	return nil
}

func findRunAvailability(availabilities []tasks.RunAvailability, id tasks.RunID) (tasks.RunAvailability, bool) {
	for _, availability := range availabilities {
		if availability.RunID == id {
			return availability, true
		}
	}
	return tasks.RunAvailability{}, false
}

func joinRunReasons(reasons []tasks.RunReason) string {
	values := make([]string, len(reasons))
	for i, reason := range reasons {
		values[i] = string(reason)
	}
	return strings.Join(values, ",")
}

func validateSessionInspectExclusivity(opts Options) error {
	if !opts.SessionInspect {
		return fmt.Errorf("session inspect mode is not selected")
	}
	if opts.RunsInspect || opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.Route != "" || opts.RouteName != "" || opts.RouteDifficulty != "" {
		return fmt.Errorf("--session-inspect is mutually exclusive with run, probe, route, and test modes")
	}
	return nil
}
