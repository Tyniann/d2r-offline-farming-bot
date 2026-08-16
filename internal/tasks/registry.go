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
				StartKind:      RecordingStartWaypoint, StartWaypoint: pathing.WaypointTargetBlackMarsh, AllowedStartArea: world.BlackMarsh,
				AllowedRouteAreas: []world.AreaID{world.BlackMarsh, world.ForgottenTower, world.TowerCellarLevel1, world.TowerCellarLevel2, world.TowerCellarLevel3, world.TowerCellarLevel4, world.TowerCellarLevel5},
				TerminalKind:      RecordingTerminalBoss, TerminalArea: world.TowerCellarLevel5, Boss: BossDescriptor{NPCID: world.DarkStalker, Name: "Countess", RequireSuperUnique: true, AllowAnySuperUniqueFallback: true, SearchAnchorObject: world.ObjectKindGoodChest, SearchAnchorEntrance: world.EntranceKindTowerCellarDown},
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
				StartKind:      RecordingStartWaypoint, StartWaypoint: pathing.WaypointTargetDuranceOfHateLevel2, AllowedStartArea: world.DuranceOfHateLevel2,
				AllowedRouteAreas: []world.AreaID{world.DuranceOfHateLevel2, world.DuranceOfHateLevel3},
				TerminalKind:      RecordingTerminalBoss, TerminalArea: world.DuranceOfHateLevel3, Boss: BossDescriptor{NPCID: world.Mephisto, Name: "Mephisto"},
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
				StartKind:      RecordingStartWaypoint, StartWaypoint: pathing.WaypointTargetArcaneSanctuary, AllowedStartArea: world.ArcaneSanctuary,
				AllowedRouteAreas: []world.AreaID{world.ArcaneSanctuary},
				TerminalKind:      RecordingTerminalBoss, TerminalArea: world.ArcaneSanctuary, Boss: BossDescriptor{NPCID: world.Summoner, Name: "Summoner"},
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
				StartKind:      RecordingStartWaypoint, StartWaypoint: pathing.WaypointTargetHallsOfPain, AllowedStartArea: world.HallsOfPain,
				AllowedRouteAreas: []world.AreaID{world.HallsOfPain, world.HallsOfVaught},
				TerminalKind:      RecordingTerminalBoss, TerminalArea: world.HallsOfVaught, Boss: BossDescriptor{NPCID: world.Nihlathak, Name: "Nihlathak"},
				TerminalMaxDistanceTiles: 60, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct5,
			},
		},
		{
			ID: RunIDCows, DisplayName: "Kuh-Level", EntryArea: world.StonyField,
			RouteTerminalArea: world.MooMooFarm, WaypointTarget: pathing.WaypointTargetStonyField,
			ReturnOrigin:       town.OriginAct1,
			RequiredCaps:       append(append([]RunCapability(nil), shared...), RunCapabilityRouteClear),
			RouteHostileNPCIDs: []uint32{world.HellBovine, world.CowKing},
			RouteSet: &RouteSetDefinition{
				Roles:       []pathing.RouteRole{pathing.RouteRoleLegAcquisition, pathing.RouteRoleCowSweep},
				PrimaryRole: pathing.RouteRoleCowSweep,
				Recordings: map[pathing.RouteRole]RecordingContract{
					pathing.RouteRoleLegAcquisition: {
						RouteRole:      pathing.RouteRoleLegAcquisition,
						InstructionsDE: "Reise zum Wegpunkt Stony Field. Ein vorheriger Clear ist nicht nötig. Starte dort die Aufnahme, betrete das bereits geöffnete rote Portal nach Tristram und bewege dich bis in die Nähe von Wirts Körper. Klicke Wirt nicht an. Beende die Aufnahme mit F9.",
						StartKind:      RecordingStartWaypoint, StartWaypoint: pathing.WaypointTargetStonyField,
						AllowedStartArea: world.StonyField, AllowedRouteAreas: []world.AreaID{world.StonyField, world.Tristram},
						TerminalKind: RecordingTerminalObject, TerminalArea: world.Tristram, TerminalObjectKind: world.ObjectKindWirtsBody,
						TerminalMaxDistanceTiles: 20, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct1,
					},
					pathing.RouteRoleCowSweep: {
						RouteRole:      pathing.RouteRoleCowSweep,
						InstructionsDE: "Öffne das Kuh-Level manuell und räume es vor der Aufnahme vollständig. Starte die Aufnahme in der Moo Moo Farm nahe dem roten Ankunftsportal, folge deiner vollständigen Farming-Schleife und beende sie am gewünschten Endpunkt mit F9.",
						StartKind:      RecordingStartObjectPortalArrival, StartObjectKind: world.ObjectKindPermanentPortal, StartPortalFromArea: world.RogueEncampment,
						AllowedStartArea: world.MooMooFarm, AllowedRouteAreas: []world.AreaID{world.MooMooFarm},
						TerminalKind: RecordingTerminalEndpoint, TerminalArea: world.MooMooFarm,
						TerminalMaxDistanceTiles: 20, Movement: pathing.RouteMovementTeleport, SafetyReturn: RecordingSafetyReturnTownPortal, EgressOriginAct: town.OriginAct1,
					},
				},
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
	if definition.WaypointTarget == "" {
		return fmt.Errorf("waypoint target is required")
	}
	if definition.RouteSet == nil && (definition.Boss.NPCID == 0 || strings.TrimSpace(definition.Boss.Name) == "") {
		return fmt.Errorf("boss descriptor is required for a single-route run")
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
	if definition.RouteSet == nil {
		if err := validateRecordingContract(definition, definition.Recording, true); err != nil {
			return err
		}
	} else if err := validateRouteSetDefinition(definition); err != nil {
		return err
	}
	return nil
}

func validateRecordingContract(definition RunDefinition, contract RecordingContract, singleRoute bool) error {
	if strings.TrimSpace(contract.InstructionsDE) == "" || contract.AllowedStartArea == world.None || contract.TerminalArea == world.None {
		return fmt.Errorf("recording instructions, start area, and terminal area are required")
	}
	switch contract.StartKind {
	case RecordingStartWaypoint:
		if contract.StartWaypoint == "" {
			return fmt.Errorf("recording waypoint start requires a waypoint target")
		}
	case RecordingStartObjectPortalArrival:
		if contract.StartObjectKind != world.ObjectKindPermanentPortal || contract.StartPortalFromArea == world.None || contract.StartWaypoint != "" {
			return fmt.Errorf("recording portal-arrival start requires permanent portal origin and no waypoint")
		}
	default:
		return fmt.Errorf("recording start kind %q unsupported", contract.StartKind)
	}
	switch contract.TerminalKind {
	case RecordingTerminalBoss:
		if contract.Boss.NPCID == 0 || strings.TrimSpace(contract.Boss.Name) == "" {
			return fmt.Errorf("recording boss terminal requires boss evidence")
		}
	case RecordingTerminalObject:
		if contract.TerminalObjectKind == world.ObjectKindUnknown || contract.Boss.NPCID != 0 {
			return fmt.Errorf("recording object terminal requires one object and no boss")
		}
	case RecordingTerminalEndpoint:
		if contract.Boss.NPCID != 0 || contract.TerminalObjectKind != world.ObjectKindUnknown {
			return fmt.Errorf("recording endpoint terminal must not require boss or object")
		}
	default:
		return fmt.Errorf("recording terminal kind %q unsupported", contract.TerminalKind)
	}
	if singleRoute {
		if contract.StartKind != RecordingStartWaypoint || contract.StartWaypoint != definition.WaypointTarget || contract.TerminalArea != definition.RouteTerminalArea {
			return fmt.Errorf("recording start waypoint and terminal area must match the run definition")
		}
		if contract.Boss.NPCID != definition.Boss.NPCID || contract.Boss.RequireSuperUnique != definition.Boss.RequireSuperUnique {
			return fmt.Errorf("recording boss selector must match the run boss descriptor")
		}
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

func validateRouteSetDefinition(definition RunDefinition) error {
	set := definition.RouteSet
	if definition.ID != RunIDCows || set == nil || len(set.Roles) != 2 || set.PrimaryRole != pathing.RouteRoleCowSweep || len(set.Recordings) != 2 || definition.Recording.InstructionsDE != "" {
		return fmt.Errorf("cows route set must declare exactly leg_acquisition and primary cow_sweep")
	}
	want := []pathing.RouteRole{pathing.RouteRoleLegAcquisition, pathing.RouteRoleCowSweep}
	for index, role := range want {
		if set.Roles[index] != role {
			return fmt.Errorf("cows route role[%d] got %q, want %q", index, set.Roles[index], role)
		}
		contract, ok := set.Recordings[role]
		if !ok || contract.RouteRole != role {
			return fmt.Errorf("cows route role %q has no matching recording contract", role)
		}
		if err := validateRecordingContract(definition, contract, false); err != nil {
			return fmt.Errorf("cows route role %q: %w", role, err)
		}
	}
	leg := set.Recordings[pathing.RouteRoleLegAcquisition]
	if leg.StartKind != RecordingStartWaypoint || leg.AllowedStartArea != world.StonyField || leg.TerminalKind != RecordingTerminalObject || leg.TerminalArea != world.Tristram || leg.TerminalObjectKind != world.ObjectKindWirtsBody {
		return fmt.Errorf("leg_acquisition recording anchors are invalid")
	}
	sweep := set.Recordings[pathing.RouteRoleCowSweep]
	if sweep.StartKind != RecordingStartObjectPortalArrival || sweep.AllowedStartArea != world.MooMooFarm || sweep.TerminalKind != RecordingTerminalEndpoint || sweep.TerminalArea != world.MooMooFarm {
		return fmt.Errorf("cow_sweep recording anchors are invalid")
	}
	return nil
}

func cloneRunDefinition(definition RunDefinition) RunDefinition {
	definition.BossEngageSequence = append([]EncounterAction(nil), definition.BossEngageSequence...)
	definition.RequiredCaps = append([]RunCapability(nil), definition.RequiredCaps...)
	definition.RouteHostileNPCIDs = append([]uint32(nil), definition.RouteHostileNPCIDs...)
	definition.Recording.AllowedRouteAreas = append([]world.AreaID(nil), definition.Recording.AllowedRouteAreas...)
	if definition.RouteSet != nil {
		set := *definition.RouteSet
		set.Roles = append([]pathing.RouteRole(nil), definition.RouteSet.Roles...)
		set.Recordings = make(map[pathing.RouteRole]RecordingContract, len(definition.RouteSet.Recordings))
		for role, contract := range definition.RouteSet.Recordings {
			contract.AllowedRouteAreas = append([]world.AreaID(nil), contract.AllowedRouteAreas...)
			set.Recordings[role] = contract
		}
		definition.RouteSet = &set
	}
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
	// Cow owns a dedicated productive pipeline, but controlled recovery is the
	// same portal-to-Town flow used by every other run. Keep that narrow phase
	// on runPipeline so Cow failures cannot strand the character in Area 39.
	if definition.ID == RunIDCows && sel.Phase != RunPhaseRetryReturn {
		if sel.Phase != "" {
			return nil, fmt.Errorf("unknown run phase %q", sel.Phase)
		}
		return newCowPipeline(definition, cfg), nil
	}
	switch sel.Phase {
	case "", RunPhaseTravelEntry, RunPhasePlayRoute, RunPhaseBoss, RunPhaseLootAndReturn, RunPhaseRetryReturn, RunPhaseStashPersonal, RunPhaseTownReady:
		return &runPipeline{
			definition: definition, phase: sel.Phase,
			core: pipelineCoreState{combat: cfg.Combat, routeID: cfg.RouteID, routeCombat: cfg.RouteCombat, lootPickupDistanceTiles: cfg.LootPickupDistanceTiles},
		}, nil
	default:
		return nil, fmt.Errorf("unknown run phase %q", sel.Phase)
	}
}
