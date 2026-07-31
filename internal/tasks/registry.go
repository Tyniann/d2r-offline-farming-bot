package tasks

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var runIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)

// RunSelection identifies the configured run and optional phase.
type RunSelection struct {
	// Run is the configured farming run name.
	Run string
	// Phase is an optional run phase; empty preserves the run's default behavior.
	Phase string
}

// runMachine executes step logic for a configured run name.
type runMachine interface {
	firstStep() string
	nextStep(current string) string
	usesTickTimeout(step string) bool
	allowsNonInputTick(step string) bool
	onStepEnter(step string)
	onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult
}

// RunRegistry stores immutable definitions by stable ID without World or input dependencies.
type RunRegistry struct {
	definitions map[RunID]RunDefinition
	ids         []RunID
}

// ResolvedRun pairs one immutable definition with its selected operator config.
type ResolvedRun struct {
	Definition RunDefinition
	Config     RunConfig
}

// NewRunRegistry validates and registers definitions, rejecting duplicate IDs.
func NewRunRegistry(definitions ...RunDefinition) (*RunRegistry, error) {
	registry := &RunRegistry{definitions: make(map[RunID]RunDefinition, len(definitions))}
	for i, definition := range definitions {
		if err := validateRunDefinition(definition); err != nil {
			return nil, fmt.Errorf("definition[%d]: %s: %w", i, RunReasonDefinitionInvalid, err)
		}
		if _, exists := registry.definitions[definition.ID]; exists {
			return nil, fmt.Errorf("definition[%d]: %s: duplicate id %q", i, RunReasonDefinitionInvalid, definition.ID)
		}
		registry.definitions[definition.ID] = cloneRunDefinition(definition)
		registry.ids = append(registry.ids, definition.ID)
	}
	sort.Slice(registry.ids, func(i, j int) bool { return registry.ids[i] < registry.ids[j] })
	return registry, nil
}

// DefaultRunRegistry returns the built-in product run definitions.
func DefaultRunRegistry() *RunRegistry {
	registry, err := NewRunRegistry(defaultRunDefinitions()...)
	if err != nil {
		panic(fmt.Sprintf("invalid built-in run registry: %v", err))
	}
	return registry
}

// Definition returns a defensive copy of the registered definition for id.
func (r *RunRegistry) Definition(id RunID) (RunDefinition, bool) {
	if r == nil {
		return RunDefinition{}, false
	}
	definition, ok := r.definitions[id]
	return cloneRunDefinition(definition), ok
}

// Definitions returns all registered definitions ordered by stable ID.
func (r *RunRegistry) Definitions() []RunDefinition {
	if r == nil {
		return nil
	}
	definitions := make([]RunDefinition, 0, len(r.ids))
	for _, id := range r.ids {
		definition, _ := r.Definition(id)
		definitions = append(definitions, definition)
	}
	return definitions
}

// Resolve pairs a registered definition with the config selected for the same ID.
func (r *RunRegistry) Resolve(id RunID, configs map[RunID]RunConfig) (ResolvedRun, error) {
	definition, ok := r.Definition(id)
	if !ok {
		return ResolvedRun{}, fmt.Errorf("%s: %q", RunReasonUnknown, id)
	}
	config, ok := configs[id]
	if !ok {
		return ResolvedRun{}, fmt.Errorf("%s: %q", RunReasonConfigMissing, id)
	}
	return ResolvedRun{Definition: definition, Config: config}, nil
}

// KnownRuns returns registered run names in stable order.
func KnownRuns() []string {
	definitions := DefaultRunRegistry().Definitions()
	runs := make([]string, len(definitions))
	for i, definition := range definitions {
		runs[i] = string(definition.ID)
	}
	return runs
}

func defaultRunDefinitions() []RunDefinition {
	shared := []RunCapability{
		RunCapabilityWaypointTravel,
		RunCapabilityRecordedRoute,
		RunCapabilityEncounterProfile,
		RunCapabilityLoot,
		RunCapabilityTownPortal,
		RunCapabilityAct1TownServices,
	}
	return []RunDefinition{
		{
			ID: RunIDCountess, DisplayName: "Countess", EntryArea: world.BlackMarsh,
			RouteTerminalArea: world.TowerCellarLevel5, WaypointTarget: pathing.WaypointTargetBlackMarsh,
			Boss: BossDescriptor{
				NPCID: world.DarkStalker, Name: "Countess", RequireSuperUnique: true, AllowAnySuperUniqueFallback: true,
				SearchAnchorObject: world.ObjectKindGoodChest, SearchAnchorEntrance: world.EntranceKindTowerCellarDown,
			},
			BossEngageSequence: []EncounterAction{{Hook: profile.HookBossEngage}}, ReturnOrigin: town.OriginAct1,
			RequiredCaps: append([]RunCapability(nil), shared...),
			Recording: RecordingContract{
				InstructionsDE: "Reise zum Wegpunkt Schwarzmarsch, starte dort die Aufnahme und bewege dich bis zu deiner gewünschten Kampfposition bei der Gräfin. Beende die Aufnahme mit F9.",
				StartWaypoint:  pathing.WaypointTargetBlackMarsh, AllowedStartArea: world.BlackMarsh,
				AllowedRouteAreas: []world.AreaID{world.BlackMarsh, world.ForgottenTower, world.TowerCellarLevel1, world.TowerCellarLevel2, world.TowerCellarLevel3, world.TowerCellarLevel4, world.TowerCellarLevel5},
				TerminalArea:      world.TowerCellarLevel5, Boss: BossDescriptor{NPCID: world.DarkStalker, Name: "Countess", RequireSuperUnique: true, AllowAnySuperUniqueFallback: true, SearchAnchorObject: world.ObjectKindGoodChest, SearchAnchorEntrance: world.EntranceKindTowerCellarDown},
				TerminalMaxDistanceTiles: 80, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct1,
			},
		},
		{
			ID: RunIDMephisto, DisplayName: "Mephisto", EntryArea: world.DuranceOfHateLevel2,
			RouteTerminalArea: world.DuranceOfHateLevel3, WaypointTarget: pathing.WaypointTargetDuranceOfHateLevel2,
			Boss:               BossDescriptor{NPCID: world.Mephisto, Name: "Mephisto"},
			BossEngageSequence: []EncounterAction{{Hook: profile.HookBossEngage}, {Hook: profile.HookBossEngage}}, ReturnOrigin: town.OriginAct3,
			RequiredCaps: append(append([]RunCapability(nil), shared...), RunCapabilityForeignTownEgress),
			Recording: RecordingContract{
				InstructionsDE: "Reise zum Wegpunkt Kerker des Hasses – Ebene 2, starte dort die Aufnahme und bewege dich bis zu deiner gewünschten Kampfposition bei Mephisto. Beende die Aufnahme mit F9.",
				StartWaypoint:  pathing.WaypointTargetDuranceOfHateLevel2, AllowedStartArea: world.DuranceOfHateLevel2,
				AllowedRouteAreas: []world.AreaID{world.DuranceOfHateLevel2, world.DuranceOfHateLevel3},
				TerminalArea:      world.DuranceOfHateLevel3, Boss: BossDescriptor{NPCID: world.Mephisto, Name: "Mephisto"},
				TerminalMaxDistanceTiles: 60, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct3,
			},
		},
		{
			ID: RunIDSummoner, DisplayName: "Summoner", EntryArea: world.ArcaneSanctuary,
			RouteTerminalArea: world.ArcaneSanctuary, WaypointTarget: pathing.WaypointTargetArcaneSanctuary,
			Boss:               BossDescriptor{NPCID: world.Summoner, Name: "Summoner"},
			BossEngageSequence: nil, ClearNearbyAfterBoss: true, ReturnOrigin: town.OriginAct2,
			RouteHostileNPCIDs: []uint32{world.ArcaneSpecter, world.ArcaneHellClan, world.ArcaneGhoulLord},
			RequiredCaps:       append(append([]RunCapability(nil), shared...), RunCapabilityForeignTownEgress, RunCapabilityRouteClear),
			Recording: RecordingContract{
				InstructionsDE: "Reise zum Wegpunkt Arcane Sanctuary, starte dort die Aufnahme und bewege dich bis zu deiner gewünschten Kampfposition beim Summoner. Beende die Aufnahme mit F9.",
				StartWaypoint:  pathing.WaypointTargetArcaneSanctuary, AllowedStartArea: world.ArcaneSanctuary,
				AllowedRouteAreas: []world.AreaID{world.ArcaneSanctuary},
				TerminalArea:      world.ArcaneSanctuary, Boss: BossDescriptor{NPCID: world.Summoner, Name: "Summoner"},
				TerminalMaxDistanceTiles: 60, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct2,
			},
		},
		{
			ID: RunIDNihlathak, DisplayName: "Nihlathak", EntryArea: world.HallsOfPain,
			RouteTerminalArea: world.HallsOfVaught, WaypointTarget: pathing.WaypointTargetHallsOfPain,
			Boss:               BossDescriptor{NPCID: world.Nihlathak, Name: "Nihlathak"},
			BossEngageSequence: nil, ClearNearbyAfterBoss: true, ReturnOrigin: town.OriginAct5,
			RequiredCaps: append(append([]RunCapability(nil), shared...), RunCapabilityForeignTownEgress),
			Recording: RecordingContract{
				InstructionsDE: "Reise zum Wegpunkt Halls of Pain (Halls of Death's Calling), starte dort die Aufnahme und bewege dich bis zu deiner gewünschten Kampfposition bei Nihlathak in den Halls of Vaught. Beende die Aufnahme mit F9.",
				StartWaypoint:  pathing.WaypointTargetHallsOfPain, AllowedStartArea: world.HallsOfPain,
				AllowedRouteAreas: []world.AreaID{world.HallsOfPain, world.HallsOfVaught},
				TerminalArea:      world.HallsOfVaught, Boss: BossDescriptor{NPCID: world.Nihlathak, Name: "Nihlathak"},
				TerminalMaxDistanceTiles: 60, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct5,
			},
		},
	}
}

func validateRunDefinition(definition RunDefinition) error {
	if !runIDPattern.MatchString(string(definition.ID)) {
		return fmt.Errorf("id %q must match %s", definition.ID, runIDPattern)
	}
	if strings.TrimSpace(definition.DisplayName) == "" || definition.EntryArea == world.None || definition.RouteTerminalArea == world.None {
		return fmt.Errorf("display name, entry area, and terminal area are required")
	}
	if definition.WaypointTarget == "" || definition.Boss.NPCID == 0 || strings.TrimSpace(definition.Boss.Name) == "" {
		return fmt.Errorf("waypoint target and boss descriptor are required")
	}
	// An empty BossEngageSequence is valid: combat starts with the regular
	// attack skill and skips pre-combat profile hooks such as Bone Prison.
	for i, action := range definition.BossEngageSequence {
		if action.Hook != profile.HookBossEngage {
			return fmt.Errorf("boss engage sequence[%d] must use %q", i, profile.HookBossEngage)
		}
	}
	if definition.Boss.AllowAnySuperUniqueFallback && !definition.Boss.RequireSuperUnique {
		return fmt.Errorf("boss super-unique fallback requires the super-unique identity gate")
	}
	if definition.ReturnOrigin == town.OriginActUnknown {
		return fmt.Errorf("return origin is required")
	}
	required := map[RunCapability]bool{
		RunCapabilityWaypointTravel: true, RunCapabilityRecordedRoute: true,
		RunCapabilityEncounterProfile: true, RunCapabilityLoot: true,
		RunCapabilityTownPortal: true, RunCapabilityAct1TownServices: true,
	}
	seen := make(map[RunCapability]bool, len(definition.RequiredCaps))
	for _, capability := range definition.RequiredCaps {
		if capability == "" || seen[capability] {
			return fmt.Errorf("required capabilities contain an empty or duplicate value %q", capability)
		}
		seen[capability] = true
	}
	for capability := range required {
		if !seen[capability] {
			return fmt.Errorf("%s: %s", RunReasonCapabilityMissing, capability)
		}
	}
	if definition.ReturnOrigin == town.OriginAct1 && seen[RunCapabilityForeignTownEgress] {
		return fmt.Errorf("Act-1 return must not require foreign Town egress")
	}
	if definition.ReturnOrigin != town.OriginAct1 && !seen[RunCapabilityForeignTownEgress] {
		return fmt.Errorf("%s: %s", RunReasonCapabilityMissing, RunCapabilityForeignTownEgress)
	}
	if seen[RunCapabilityRouteClear] {
		if len(definition.RouteHostileNPCIDs) == 0 {
			return fmt.Errorf("%s requires a route hostile allowlist", RunCapabilityRouteClear)
		}
		hostiles := make(map[uint32]struct{}, len(definition.RouteHostileNPCIDs))
		for _, npcID := range definition.RouteHostileNPCIDs {
			if npcID == 0 {
				return fmt.Errorf("route hostile allowlist contains zero")
			}
			if _, duplicate := hostiles[npcID]; duplicate {
				return fmt.Errorf("route hostile allowlist contains duplicate %d", npcID)
			}
			hostiles[npcID] = struct{}{}
		}
	} else if len(definition.RouteHostileNPCIDs) != 0 {
		return fmt.Errorf("route hostile allowlist requires %s", RunCapabilityRouteClear)
	}
	if err := validateRecordingContract(definition); err != nil {
		return err
	}
	return nil
}

func validateRecordingContract(definition RunDefinition) error {
	contract := definition.Recording
	if strings.TrimSpace(contract.InstructionsDE) == "" || contract.StartWaypoint == "" || contract.AllowedStartArea == world.None || contract.TerminalArea == world.None {
		return fmt.Errorf("recording instructions, start waypoint, start area, and terminal area are required")
	}
	if contract.StartWaypoint != definition.WaypointTarget || contract.TerminalArea != definition.RouteTerminalArea {
		return fmt.Errorf("recording start waypoint and terminal area must match the run definition")
	}
	if contract.Boss.NPCID != definition.Boss.NPCID || contract.Boss.RequireSuperUnique != definition.Boss.RequireSuperUnique {
		return fmt.Errorf("recording boss selector must match the run boss descriptor")
	}
	if len(contract.AllowedRouteAreas) == 0 || contract.AllowedRouteAreas[0] != contract.AllowedStartArea || contract.AllowedRouteAreas[len(contract.AllowedRouteAreas)-1] != contract.TerminalArea {
		return fmt.Errorf("recording allowed areas must start and end at the declared anchors")
	}
	seenAreas := make(map[world.AreaID]bool, len(contract.AllowedRouteAreas))
	for _, area := range contract.AllowedRouteAreas {
		if area == world.None || seenAreas[area] {
			return fmt.Errorf("recording allowed areas contain an empty or duplicate area")
		}
		seenAreas[area] = true
	}
	if contract.TerminalMaxDistanceTiles <= 0 || contract.Movement != pathing.RouteMovementTeleport || contract.SafetyReturn != RecordingSafetyReturnTownPortal || contract.EgressOriginAct != definition.ReturnOrigin {
		return fmt.Errorf("recording distance, teleport movement, Town Portal return, and origin act are required")
	}
	return nil
}

func cloneRunDefinition(definition RunDefinition) RunDefinition {
	definition.BossEngageSequence = append([]EncounterAction(nil), definition.BossEngageSequence...)
	definition.RequiredCaps = append([]RunCapability(nil), definition.RequiredCaps...)
	definition.RouteHostileNPCIDs = append([]uint32(nil), definition.RouteHostileNPCIDs...)
	definition.Recording.AllowedRouteAreas = append([]world.AreaID(nil), definition.Recording.AllowedRouteAreas...)
	return definition
}

// IsKnownRun reports whether name is a registered run.
func IsKnownRun(name string) bool {
	for _, known := range KnownRuns() {
		if known == name {
			return true
		}
	}
	return false
}

func newRunMachine(sel RunSelection, cfg RunConfig) (runMachine, error) {
	definition, ok := DefaultRunRegistry().Definition(RunID(sel.Run))
	if !ok {
		return nil, fmt.Errorf("%s: %q", RunReasonUnknown, sel.Run)
	}
	switch sel.Phase {
	case "", RunPhaseTravelEntry, RunPhasePlayRoute, RunPhaseBoss, RunPhaseLootAndReturn, RunPhaseRetryReturn, RunPhaseStashPersonal, RunPhaseTownReady:
		return &runPipeline{
			definition: definition, phase: sel.Phase, combat: cfg.Combat, routeID: cfg.RouteID,
			routeCombat:             cfg.RouteCombat,
			lootPickupDistanceTiles: cfg.LootPickupDistanceTiles,
		}, nil
	default:
		return nil, fmt.Errorf("unknown run phase %q", sel.Phase)
	}
}
