package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

// RunAvailabilityContext contains only read-only identity evidence used to
// evaluate route and profile bindings. Empty live fields are never guessed.
type RunAvailabilityContext struct {
	Character         string  `json:"character"`
	CharacterClass    string  `json:"character_class,omitempty"`
	Difficulty        string  `json:"difficulty"`
	GameVersion       string  `json:"game_version"`
	MapSeed           *uint32 `json:"map_seed,omitempty"`
	LayoutFingerprint string  `json:"layout_fingerprint,omitempty"`
}

// RunsInspectReport is the deterministic JSON contract of --runs-inspect.
type RunsInspectReport struct {
	Context RunAvailabilityContext  `json:"context"`
	Runs    []tasks.RunAvailability `json:"runs"`
}

type runAvailabilityResolution struct {
	report   RunsInspectReport
	routes   map[tasks.RunID]pathing.Route
	registry *pathing.RouteRegistry
}

// ResolveRunAvailabilities evaluates every registered run without process
// attach, hotkeys, input, route playback, or session startup.
func ResolveRunAvailabilities(cfg *config.Config, context RunAvailabilityContext) (RunsInspectReport, error) {
	resolved, err := resolveRunAvailabilities(cfg, context)
	if err != nil {
		return RunsInspectReport{}, err
	}
	return resolved.report, nil
}

// ResolveRunsInspectReport derives the read-only context from session config
// and rejects combinations with any runtime or diagnostic mode.
func ResolveRunsInspectReport(cfg *config.Config, opts Options) (RunsInspectReport, error) {
	if cfg == nil {
		return RunsInspectReport{}, fmt.Errorf("runs inspect requires config")
	}
	if !opts.RunsInspect {
		return RunsInspectReport{}, fmt.Errorf("runs inspect mode is not selected")
	}
	if opts.SessionInspect || opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.Route != "" || opts.RouteName != "" || opts.RouteDifficulty != "" || opts.TownInspect || opts.TownTest != "" {
		return RunsInspectReport{}, fmt.Errorf("--runs-inspect is mutually exclusive with session, run, probe, route, town, and test modes")
	}
	context := RunAvailabilityContext{
		Character: cfg.Session.Character, Difficulty: cfg.Session.Difficulty, GameVersion: cfg.Memory.GameVersion,
	}
	return ResolveRunAvailabilities(cfg, context)
}

func resolveRunAvailabilities(cfg *config.Config, context RunAvailabilityContext) (runAvailabilityResolution, error) {
	if cfg == nil {
		return runAvailabilityResolution{}, fmt.Errorf("resolve run availability requires config")
	}
	routes, err := pathing.LoadRouteRegistry(cfg.ResolvePath(cfg.Routes.Directory))
	if err != nil {
		return runAvailabilityResolution{}, fmt.Errorf("load run route registry: %w", err)
	}
	result := runAvailabilityResolution{
		report: RunsInspectReport{Context: context}, routes: make(map[tasks.RunID]pathing.Route), registry: routes,
	}
	for _, definition := range tasks.DefaultRunRegistry().Definitions() {
		availability := tasks.RunAvailability{
			RunID: definition.ID, DisplayName: definition.DisplayName, Status: tasks.RunAvailabilityUnavailable,
		}
		runCfg, configured := cfg.Runs.Run(string(definition.ID))
		if !configured {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonConfigMissing)
			result.report.Runs = append(result.report.Runs, availability)
			continue
		}
		profileCfg, profileConfigured := cfg.Profiles[runCfg.Combat.Profile]
		if !profileConfigured {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonCapabilityMissing)
		} else if context.CharacterClass != "" && !strings.EqualFold(profileCfg.CharacterClass, context.CharacterClass) {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonProfileClassMismatch)
		}

		var route pathing.Route
		if strings.TrimSpace(runCfg.RouteID) == "" {
			availability.Route.Reason = tasks.RunReasonRouteMissing
			availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteMissing)
		} else {
			availability.Route.RouteID = runCfg.RouteID
			candidate, routeErr := routes.Get(runCfg.RouteID)
			if routeErr != nil {
				availability.Route.Reason = tasks.RunReasonRouteMissing
				availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteMissing)
			} else {
				route = candidate
				result.routes[definition.ID] = candidate
				if !routeMatchesDefinitionAndContext(candidate, definition, runCfg.Combat.Profile, context) {
					availability.Route.Reason = tasks.RunReasonRouteBindingMismatch
					availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteBindingMismatch)
				}
				if profileConfigured && !strings.EqualFold(candidate.Binding.CharacterClass, profileCfg.CharacterClass) {
					availability.Reasons = append(availability.Reasons, tasks.RunReasonProfileClassMismatch)
				}
			}
		}
		if _, supported := pathing.DefaultWaypointTargetRegistry().Action(definition.WaypointTarget); !supported {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonWaypointTargetUnsupported)
		}
		if definition.ReturnOrigin != town.OriginAct1 {
			if egress, reason := cfg.Town.EgressFor(definition.ReturnOrigin); reason != "" {
				availability.Reasons = append(availability.Reasons, tasks.RunReasonTownEgressMissing)
			} else if reason := validateTownEgressAvailability(cfg, egress, definition.ReturnOrigin, context); reason != "" {
				availability.Reasons = append(availability.Reasons, reason)
			}
		}

		availability.Reasons = uniqueSortedRunReasons(availability.Reasons)
		if len(availability.Reasons) == 0 {
			if route.Binding.LayoutFingerprint.Hash != "" && context.LayoutFingerprint == "" {
				availability.Status = tasks.RunAvailabilityRuntimeValidationRequired
				availability.Route.Reason = tasks.RunReasonRouteRuntimeValidation
				availability.Reasons = []tasks.RunReason{tasks.RunReasonRouteRuntimeValidation}
			} else if route.Binding.LayoutFingerprint.Hash != context.LayoutFingerprint {
				availability.Status = tasks.RunAvailabilityUnavailable
				availability.Route.Reason = tasks.RunReasonRouteLayoutMismatch
				availability.Reasons = []tasks.RunReason{tasks.RunReasonRouteLayoutMismatch}
			} else {
				availability.Status = tasks.RunAvailabilityAvailable
			}
		}
		result.report.Runs = append(result.report.Runs, availability)
	}
	return result, nil
}

func validateTownEgressAvailability(cfg *config.Config, egress town.EgressConfig, origin town.OriginAct, context RunAvailabilityContext) tasks.RunReason {
	registry, err := pathing.LoadRouteRegistry(cfg.ResolvePath(egress.RoutesDirectory))
	if err != nil {
		return tasks.RunReasonTownEgressMissing
	}
	route, err := registry.Get(egress.RouteID)
	if err != nil || validateAct3EgressRoute(route) != nil || origin != town.OriginAct3 {
		return tasks.RunReasonTownEgressMissing
	}
	if context.Character != "" && !strings.EqualFold(route.Binding.CharacterName, context.Character) ||
		context.CharacterClass != "" && !strings.EqualFold(route.Binding.CharacterClass, context.CharacterClass) ||
		context.Difficulty != "" && string(route.Binding.Difficulty) != context.Difficulty ||
		context.GameVersion != "" && route.Binding.GameVersion != context.GameVersion ||
		context.MapSeed != nil && (route.Binding.MapSeed == nil || *route.Binding.MapSeed != *context.MapSeed) {
		return tasks.RunReasonTownEgressBindingMismatch
	}
	return ""
}

func routeMatchesDefinitionAndContext(route pathing.Route, definition tasks.RunDefinition, profileID string, context RunAvailabilityContext) bool {
	if len(route.Segments) == 0 || route.Segments[0].FromAreaID != definition.EntryArea || route.Segments[len(route.Segments)-1].ToAreaID != definition.RouteTerminalArea {
		return false
	}
	if context.Character != "" && !strings.EqualFold(route.Binding.CharacterName, context.Character) {
		return false
	}
	if context.Difficulty != "" && string(route.Binding.Difficulty) != context.Difficulty {
		return false
	}
	if context.GameVersion != "" && route.Binding.GameVersion != context.GameVersion {
		return false
	}
	if context.MapSeed != nil && (route.Binding.MapSeed == nil || *route.Binding.MapSeed != *context.MapSeed) {
		return false
	}
	return route.Binding.ProfileID == "" || route.Binding.ProfileID == profileID
}

func uniqueSortedRunReasons(reasons []tasks.RunReason) []tasks.RunReason {
	seen := make(map[tasks.RunReason]bool, len(reasons))
	unique := make([]tasks.RunReason, 0, len(reasons))
	for _, reason := range reasons {
		if reason != "" && !seen[reason] {
			seen[reason] = true
			unique = append(unique, reason)
		}
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i] < unique[j] })
	return unique
}
