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
	CombatProfile     string  `json:"combat_profile,omitempty"`
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
	report     RunsInspectReport
	routes     map[tasks.RunID]pathing.Route
	routePaths map[string]string
	routeSets  map[tasks.RunID]map[pathing.RouteRole]pathing.Route
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
	if opts.SessionInspect || opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.ObjectInspect != "" || opts.Route != "" || opts.RouteName != "" || opts.RouteDifficulty != "" || opts.TownInspect || opts.TownTest != "" {
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
	lifecycle, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		return runAvailabilityResolution{}, err
	}
	_, catalog, err := lifecycle.Snapshot()
	if err != nil {
		return runAvailabilityResolution{}, fmt.Errorf("load run route catalog: %w", err)
	}
	assignmentStore, err := NewRouteAssignmentStore(cfg)
	if err != nil {
		return runAvailabilityResolution{}, err
	}
	assignments, err := assignmentStore.Snapshot()
	if err != nil {
		return runAvailabilityResolution{}, fmt.Errorf("load route assignments: %w", err)
	}
	result := runAvailabilityResolution{
		report: RunsInspectReport{Context: context}, routes: make(map[tasks.RunID]pathing.Route), routePaths: make(map[string]string), routeSets: make(map[tasks.RunID]map[pathing.RouteRole]pathing.Route),
	}
	candidates := make(map[string]FarmingRouteCatalogEntry)
	for _, entry := range catalog.Entries {
		if entry.ID != "" {
			candidates[entry.ID] = entry
		}
	}
	for _, definition := range tasks.DefaultRunRegistry().Definitions() {
		availability := tasks.RunAvailability{
			RunID: definition.ID, DisplayName: definition.DisplayName, Status: tasks.RunAvailabilityUnavailable,
		}
		_, configured := cfg.Runs.Run(string(definition.ID))
		if !configured {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonConfigMissing)
			result.report.Runs = append(result.report.Runs, availability)
			continue
		}
		profileCfg, profileConfigured := cfg.Profiles[context.CombatProfile]
		if context.CombatProfile == "" {
			if defaultID, err := defaultEnabledCombatProfileID(cfg.Profiles, context.CharacterClass); err == nil {
				context.CombatProfile = defaultID
				profileCfg, profileConfigured = cfg.Profiles[defaultID]
			}
		}
		if !profileConfigured {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonCapabilityMissing)
		} else if context.CharacterClass != "" && !strings.EqualFold(profileCfg.CharacterClass, context.CharacterClass) {
			availability.Reasons = append(availability.Reasons, tasks.RunReasonProfileClassMismatch)
		}
		if context.CombatProfile != "" {
			if _, ok := DefaultCombatStrategyRegistry().Resolve(context.CombatProfile, string(definition.ID)); !ok {
				availability.Reasons = append(availability.Reasons, tasks.RunReasonProfileRunStrategyUnavailable)
			}
		}
		if definition.RouteSet != nil {
			resolveRouteSetAvailability(&availability, result, definition, context, assignments, candidates, profileCfg, profileConfigured)
			if _, supported := pathing.DefaultWaypointTargetRegistry().Action(definition.WaypointTarget); !supported {
				availability.Reasons = append(availability.Reasons, tasks.RunReasonWaypointTargetUnsupported)
			}
			availability.Reasons = uniqueSortedRunReasons(availability.Reasons)
			if len(availability.Reasons) == 0 {
				availability.Status = tasks.RunAvailabilityRuntimeValidationRequired
				availability.Reasons = []tasks.RunReason{tasks.RunReasonRouteRuntimeValidation}
			}
			result.report.Runs = append(result.report.Runs, availability)
			continue
		}

		var route pathing.Route
		routeID := assignments.Assignments[strings.ToLower(strings.TrimSpace(context.Character))][definition.ID]
		if strings.TrimSpace(routeID) == "" {
			availability.Route.Reason = tasks.RunReasonRouteMissing
			availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteAssignmentMissing)
		} else {
			availability.Route.RouteID = routeID
			candidate, found := candidates[routeID]
			if !found {
				availability.Route.Reason = tasks.RunReasonRouteMissing
				availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteMissing)
			} else if candidate.Status == RouteLifecycleStale {
				availability.Route.Reason = tasks.RunReasonRouteStale
				availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteStale)
			} else if candidate.Status == RouteLifecycleUnavailable {
				availability.Route.Reason = tasks.RunReasonRouteLifecycleUnavailable
				availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteLifecycleUnavailable)
			} else if candidate.ManagementStatus == RouteManagementArchived {
				availability.Route.Reason = tasks.RunReasonRouteLifecycleUnavailable
				availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteLifecycleUnavailable)
			} else {
				route = candidate.Route
				result.routes[definition.ID] = candidate.Route
				result.routePaths[candidate.ID] = candidate.Path
				if !routeMatchesDefinitionAndContext(candidate.Route, definition, context) {
					availability.Route.Reason = tasks.RunReasonRouteBindingMismatch
					availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteBindingMismatch)
				}
				if profileConfigured && !strings.EqualFold(candidate.Route.Binding.CharacterClass, profileCfg.CharacterClass) {
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

func resolveRouteSetAvailability(availability *tasks.RunAvailability, result runAvailabilityResolution, definition tasks.RunDefinition, context RunAvailabilityContext, assignments RouteAssignmentManifest, candidates map[string]FarmingRouteCatalogEntry, profileCfg config.ProfileConfig, profileConfigured bool) {
	availability.RouteRoles = make(map[pathing.RouteRole]tasks.RouteAvailability, len(definition.RouteSet.Roles))
	resolvedRoutes := make(map[pathing.RouteRole]pathing.Route, len(definition.RouteSet.Roles))
	character := strings.ToLower(strings.TrimSpace(context.Character))
	bound := assignments.RouteSets[character][definition.ID]
	var identity *pathing.RouteBinding
	for _, role := range definition.RouteSet.Roles {
		roleAvailability := tasks.RouteAvailability{}
		routeID := bound[role]
		missingReason, staleReason := roleAvailabilityReasons(role)
		if strings.TrimSpace(routeID) == "" {
			roleAvailability.Reason = tasks.RunReasonRouteMissing
			availability.Reasons = append(availability.Reasons, missingReason)
			availability.RouteRoles[role] = roleAvailability
			continue
		}
		roleAvailability.RouteID = routeID
		candidate, found := candidates[routeID]
		if !found {
			roleAvailability.Reason = tasks.RunReasonRouteMissing
			availability.Reasons = append(availability.Reasons, missingReason)
		} else if candidate.Status != RouteLifecycleValid && candidate.Status != RouteLifecycleRuntimeValidationRequired || candidate.ManagementStatus == RouteManagementArchived {
			roleAvailability.Reason = tasks.RunReasonRouteStale
			availability.Reasons = append(availability.Reasons, staleReason)
		} else if !routeMatchesRoleAndContext(candidate.Route, definition, role, context) {
			roleAvailability.Reason = tasks.RunReasonRouteBindingMismatch
			availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteSetBindingMismatch)
		} else {
			binding := candidate.Route.Binding
			if identity != nil && !sharedRouteSetIdentity(*identity, binding) {
				roleAvailability.Reason = tasks.RunReasonRouteBindingMismatch
				availability.Reasons = append(availability.Reasons, tasks.RunReasonRouteSetBindingMismatch)
			} else if identity == nil {
				identity = &binding
			}
			if profileConfigured && !strings.EqualFold(binding.CharacterClass, profileCfg.CharacterClass) {
				availability.Reasons = append(availability.Reasons, tasks.RunReasonProfileClassMismatch)
			}
			resolvedRoutes[role] = candidate.Route
			result.routePaths[candidate.ID] = candidate.Path
		}
		availability.RouteRoles[role] = roleAvailability
	}
	primary := availability.RouteRoles[definition.RouteSet.PrimaryRole]
	availability.Route = primary
	if len(resolvedRoutes) == len(definition.RouteSet.Roles) {
		result.routeSets[definition.ID] = resolvedRoutes
		if route, ok := resolvedRoutes[definition.RouteSet.PrimaryRole]; ok {
			// Existing session surfaces remain keyed by the primary route. The
			// dedicated Cow pipeline receives both immutable role IDs separately.
			result.routes[definition.ID] = route
		}
	}
}

func roleAvailabilityReasons(role pathing.RouteRole) (tasks.RunReason, tasks.RunReason) {
	if role == pathing.RouteRoleLegAcquisition {
		return tasks.RunReasonLegAcquisitionRouteMissing, tasks.RunReasonLegAcquisitionRouteStale
	}
	return tasks.RunReasonCowSweepRouteMissing, tasks.RunReasonCowSweepRouteStale
}

func routeMatchesRoleAndContext(route pathing.Route, definition tasks.RunDefinition, role pathing.RouteRole, context RunAvailabilityContext) bool {
	contract, ok := definition.RecordingForRole(role)
	if !ok || route.Binding.RouteRole != role || len(route.Segments) == 0 || route.Segments[0].FromAreaID != contract.AllowedStartArea || route.Segments[len(route.Segments)-1].ToAreaID != contract.TerminalArea {
		return false
	}
	if role == pathing.RouteRoleLegAcquisition && !validLegAcquisitionSegments(route.Segments) || role == pathing.RouteRoleCowSweep && !validCowSweepSegments(route.Segments) {
		return false
	}
	if context.Character != "" && !strings.EqualFold(route.Binding.CharacterName, context.Character) || context.Difficulty != "" && string(route.Binding.Difficulty) != context.Difficulty || context.GameVersion != "" && route.Binding.GameVersion != context.GameVersion {
		return false
	}
	return true
}

func sharedRouteSetIdentity(left, right pathing.RouteBinding) bool {
	return strings.EqualFold(left.CharacterName, right.CharacterName) && strings.EqualFold(left.CharacterClass, right.CharacterClass) && left.Difficulty == right.Difficulty && left.GameVersion == right.GameVersion
}

func validateTownEgressAvailability(cfg *config.Config, egress town.EgressConfig, origin town.OriginAct, context RunAvailabilityContext) tasks.RunReason {
	route, err := town.LoadSystemEgressRoute(cfg.ResolvePath(egress.RoutesDirectory + "/" + town.SystemEgressFilename))
	if err != nil {
		return tasks.RunReasonTownEgressMissing
	}
	area, ok := town.TownAreaForAct(origin)
	if !ok || route.Contract.Act != origin || route.Contract.TownArea != area {
		return tasks.RunReasonTownEgressMissing
	}
	if context.GameVersion != "" && route.Contract.GameVersion != context.GameVersion {
		return tasks.RunReasonTownEgressBindingMismatch
	}
	if context.LayoutFingerprint != "" {
		// Anchored system egresses absorb small coordinate jitter; exact Hash
		// equality would false-negative. Runtime Start remains the authority.
		if len(route.Contract.LayoutFingerprint.Anchors) == 0 && route.Contract.LayoutFingerprint.Hash != context.LayoutFingerprint {
			return tasks.RunReasonTownEgressBindingMismatch
		}
	}
	return ""
}

func routeMatchesDefinitionAndContext(route pathing.Route, definition tasks.RunDefinition, context RunAvailabilityContext) bool {
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
	// Legacy profile_id values are tolerated on published routes but ignored for
	// availability. New recordings omit the field entirely.
	return true
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
