package tasks

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RunID is the stable product identity of one registered farming run.
type RunID string

const (
	// RunIDCountess identifies the Countess run definition.
	RunIDCountess RunID = "countess"
	// RunIDMephisto identifies the Mephisto run definition.
	RunIDMephisto RunID = "mephisto"
	// RunIDSummoner identifies the Arcane Sanctuary Summoner key run.
	RunIDSummoner RunID = "summoner"
	// RunIDNihlathak identifies the Halls of Pain Nihlathak key run.
	RunIDNihlathak RunID = "nihlathak"
	// RunIDCows identifies the two-route Moo Moo Farm run.
	RunIDCows RunID = "cows"
	// RunIDLowerKurast identifies the Lower Kurast superchest run.
	RunIDLowerKurast RunID = "lower-kurast"
)

// RunCapability identifies a runtime facility required by a run definition.
// Capabilities are declarative preflight requirements, never fallback switches.
type RunCapability string

const (
	// RunCapabilityWaypointTravel requires a registered waypoint target action.
	RunCapabilityWaypointTravel RunCapability = "waypoint_travel"
	// RunCapabilityRecordedRoute requires compatible recorded-route playback.
	RunCapabilityRecordedRoute RunCapability = "recorded_route"
	// RunCapabilityEncounterProfile requires a class-compatible combat profile.
	RunCapabilityEncounterProfile RunCapability = "encounter_profile"
	// RunCapabilityLoot requires the shared pickup and disposition pipeline.
	RunCapabilityLoot RunCapability = "loot"
	// RunCapabilityTownPortal requires safe portal casting and entry.
	RunCapabilityTownPortal RunCapability = "town_portal"
	// RunCapabilityAct1TownServices requires the central Act-1 service hub.
	RunCapabilityAct1TownServices RunCapability = "act1_town_services"
	// RunCapabilityForeignTownEgress requires normalization from another act.
	RunCapabilityForeignTownEgress RunCapability = "foreign_town_egress"
	// RunCapabilityRouteClear permits threat-aware combat during bound route playback.
	RunCapabilityRouteClear RunCapability = "route_clear"
	// RunCapabilityChestSweep replaces a boss encounter with a catalog chest sweep.
	RunCapabilityChestSweep RunCapability = "chest_sweep"
)

// BossDescriptor identifies the Memory-confirmed boss selected by a run.
type BossDescriptor struct {
	NPCID uint32
	Name  string
	// RequireSuperUnique adds the runtime type-flag gate for shared base NPC IDs.
	RequireSuperUnique          bool
	AllowAnySuperUniqueFallback bool
	SearchAnchorObject          world.ObjectKind
	SearchAnchorEntrance        world.EntranceKind
}

// EncounterAction is one ordered semantic profile action before regular combat.
// Repeated actions remain separate entries so telemetry and settle state can use
// a stable action index while retaining the same pinned boss UnitID.
type EncounterAction struct {
	Hook profile.Hook
}

// RecordingSafetyReturn identifies the only post-recording return flow a run permits.
type RecordingSafetyReturn string

const (
	// RecordingSafetyReturnTownPortal requires a Memory-gated Town Portal return.
	RecordingSafetyReturnTownPortal RecordingSafetyReturn = "town_portal"
)

// RecordingStartKind identifies the Memory-confirmed recording start anchor.
type RecordingStartKind string

const (
	// RecordingStartWaypoint starts near the declared waypoint after travel.
	RecordingStartWaypoint RecordingStartKind = "waypoint"
	// RecordingStartObjectPortalArrival starts near the destination-side object portal.
	RecordingStartObjectPortalArrival RecordingStartKind = "object_portal_arrival"
)

// RecordingTerminalKind identifies the semantic evidence required at F9.
type RecordingTerminalKind string

const (
	// RecordingTerminalBoss requires the existing living-boss distance proof.
	RecordingTerminalBoss RecordingTerminalKind = "boss"
	// RecordingTerminalObject requires one declared object within terminal distance.
	RecordingTerminalObject RecordingTerminalKind = "object"
	// RecordingTerminalEndpoint accepts the operator's final in-area route point.
	RecordingTerminalEndpoint RecordingTerminalKind = "endpoint"
)

// RecordingContract defines the immutable start, route and terminal semantics
// used by a guided recording. It deliberately reuses the run's authoritative
// boss descriptor instead of maintaining a second NPC identity table.
type RecordingContract struct {
	RouteRole                pathing.RouteRole
	InstructionsDE           string
	StartKind                RecordingStartKind
	StartWaypoint            pathing.WaypointTargetID
	StartObjectKind          world.ObjectKind
	StartPortalFromArea      world.AreaID
	AllowedStartArea         world.AreaID
	AllowedRouteAreas        []world.AreaID
	TerminalKind             RecordingTerminalKind
	TerminalArea             world.AreaID
	Boss                     BossDescriptor
	TerminalObjectKind       world.ObjectKind
	TerminalMaxDistanceTiles float64
	Movement                 pathing.RouteMovement
	SafetyReturn             RecordingSafetyReturn
	EgressOriginAct          town.OriginAct
}

// RouteSetDefinition declares the only supported fixed-role route set for a
// run. It is metadata, not an execution graph or additional queue identity.
type RouteSetDefinition struct {
	Roles       []pathing.RouteRole
	PrimaryRole pathing.RouteRole
	Recordings  map[pathing.RouteRole]RecordingContract
}

// RunDefinition contains immutable product metadata and required capabilities.
// Operator-selected route, combat tuning, and loot files belong to RunConfig and
// must not be embedded in a definition.
type RunDefinition struct {
	ID                 RunID
	DisplayName        string
	EntryArea          world.AreaID
	RouteTerminalArea  world.AreaID
	WaypointTarget     pathing.WaypointTargetID
	Boss               BossDescriptor
	BossEngageSequence []EncounterAction
	// ClearNearbyAfterBoss enables bounded profile-aware cleanup between
	// confirmed boss death and loot handling.
	ClearNearbyAfterBoss bool
	// RouteHostileNPCIDs is the immutable living-hostile allowlist for route clear.
	RouteHostileNPCIDs []uint32
	ReturnOrigin       town.OriginAct
	RequiredCaps       []RunCapability
	Recording          RecordingContract
	RouteSet           *RouteSetDefinition
}

// RecordingForRole resolves the one recording contract declared for role.
// Existing single-route runs use the empty role.
func (d RunDefinition) RecordingForRole(role pathing.RouteRole) (RecordingContract, bool) {
	if d.RouteSet == nil {
		if role != "" {
			return RecordingContract{}, false
		}
		return d.Recording, true
	}
	contract, ok := d.RouteSet.Recordings[role]
	return contract, ok
}

// RouteRoles returns the declared roles in stable execution order.
func (d RunDefinition) RouteRoles() []pathing.RouteRole {
	if d.RouteSet == nil {
		return nil
	}
	return append([]pathing.RouteRole(nil), d.RouteSet.Roles...)
}

// HasCapability reports whether the immutable run definition declares capability.
func (d RunDefinition) HasCapability(capability RunCapability) bool {
	for _, candidate := range d.RequiredCaps {
		if candidate == capability {
			return true
		}
	}
	return false
}

// AllowsRouteHostile reports whether npcID belongs to the immutable route-clear catalog.
func (d RunDefinition) AllowsRouteHostile(npcID uint32) bool {
	for _, candidate := range d.RouteHostileNPCIDs {
		if candidate == npcID {
			return true
		}
	}
	return false
}

// RunStep identifies one state in the shared finite run lifecycle.
type RunStep string

const (
	// RunStepResolveDefinition resolves immutable metadata and selected config.
	RunStepResolveDefinition RunStep = "resolve_definition"
	// RunStepPrecheck validates capabilities before runtime input.
	RunStepPrecheck RunStep = "precheck"
	// RunStepTownReadyProfile applies the ordered Town-ready profile hook.
	RunStepTownReadyProfile RunStep = "town_ready_profile"
	// RunStepAcquireAct1Waypoint reaches and opens the Act-1 waypoint.
	RunStepAcquireAct1Waypoint RunStep = "acquire_act1_waypoint"
	// RunStepSelectRunWaypoint selects the definition's registered destination.
	RunStepSelectRunWaypoint RunStep = "select_run_waypoint"
	// RunStepWaitEntryArea confirms arrival in the definition's entry area.
	RunStepWaitEntryArea RunStep = "wait_entry_area"
	// RunStepPlayBoundRoute plays the compatible recorded route.
	RunStepPlayBoundRoute RunStep = "play_bound_route"
	// RunStepChestSweep operates remaining visible hut Supertruhen and nearby racks.
	RunStepChestSweep RunStep = "chest_sweep"
	// RunStepAcquireBoss finds and pins the definition's boss UnitID.
	RunStepAcquireBoss RunStep = "acquire_boss"
	// RunStepBossEngageAction executes one indexed pre-combat encounter action.
	RunStepBossEngageAction RunStep = "boss_engage_action"
	// RunStepEngageBoss performs regular combat against the pinned boss.
	RunStepEngageBoss RunStep = "engage_boss"
	// RunStepConfirmKill confirms boss death over consistent snapshots.
	RunStepConfirmKill RunStep = "confirm_kill"
	// RunStepClearNearbyHostiles attacks nearby living encounter monsters within
	// a bounded cast budget before moving to the retained boss position.
	RunStepClearNearbyHostiles RunStep = "clear_nearby_hostiles"
	// RunStepRepositionForLoot moves to the retained boss position before scanning drops.
	RunStepRepositionForLoot RunStep = "reposition_for_loot"
	// RunStepScanAndPickLoot executes the selected run's pickup policy.
	RunStepScanAndPickLoot RunStep = "scan_and_pick_loot"
	// RunStepCastAndEnterTownPortal creates and enters the owned portal.
	RunStepCastAndEnterTownPortal RunStep = "cast_and_enter_town_portal"
	// RunStepWaitOriginTown confirms arrival in the definition's return town.
	RunStepWaitOriginTown RunStep = "wait_origin_town"
	// RunStepNormalizeToAct1Hub returns foreign origins to Rogue Encampment.
	RunStepNormalizeToAct1Hub RunStep = "normalize_to_act1_hub"
	// RunStepStashAndServices executes the validated central Town plan.
	RunStepStashAndServices RunStep = "stash_and_services"
	// RunStepReachAct1Waypoint verifies the shared handoff endpoint.
	RunStepReachAct1Waypoint RunStep = "reach_act1_waypoint"
	// RunStepComplete is the terminal successful run state.
	RunStepComplete RunStep = "complete"
)

// RunStepContract defines the only legal successors of one shared run state.
// Loading waits, timeouts, and area gates remain executor responsibilities; a
// transition outside this table is always a state-machine defect.
type RunStepContract struct {
	Step        RunStep
	AllowedNext []RunStep
}

var sharedRunStepContracts = []RunStepContract{
	{Step: RunStepResolveDefinition, AllowedNext: []RunStep{RunStepPrecheck}},
	{Step: RunStepPrecheck, AllowedNext: []RunStep{RunStepTownReadyProfile}},
	{Step: RunStepTownReadyProfile, AllowedNext: []RunStep{RunStepAcquireAct1Waypoint}},
	{Step: RunStepAcquireAct1Waypoint, AllowedNext: []RunStep{RunStepSelectRunWaypoint}},
	{Step: RunStepSelectRunWaypoint, AllowedNext: []RunStep{RunStepWaitEntryArea}},
	{Step: RunStepWaitEntryArea, AllowedNext: []RunStep{RunStepPlayBoundRoute}},
	{Step: RunStepPlayBoundRoute, AllowedNext: []RunStep{RunStepAcquireBoss, RunStepChestSweep}},
	{Step: RunStepChestSweep, AllowedNext: []RunStep{RunStepScanAndPickLoot}},
	{Step: RunStepAcquireBoss, AllowedNext: []RunStep{RunStepBossEngageAction, RunStepEngageBoss}},
	{Step: RunStepBossEngageAction, AllowedNext: []RunStep{RunStepBossEngageAction, RunStepEngageBoss}},
	{Step: RunStepEngageBoss, AllowedNext: []RunStep{RunStepConfirmKill}},
	{Step: RunStepConfirmKill, AllowedNext: []RunStep{RunStepClearNearbyHostiles, RunStepRepositionForLoot, RunStepScanAndPickLoot}},
	{Step: RunStepClearNearbyHostiles, AllowedNext: []RunStep{RunStepRepositionForLoot}},
	{Step: RunStepRepositionForLoot, AllowedNext: []RunStep{RunStepScanAndPickLoot}},
	{Step: RunStepScanAndPickLoot, AllowedNext: []RunStep{RunStepCastAndEnterTownPortal}},
	{Step: RunStepCastAndEnterTownPortal, AllowedNext: []RunStep{RunStepWaitOriginTown}},
	{Step: RunStepWaitOriginTown, AllowedNext: []RunStep{RunStepNormalizeToAct1Hub}},
	{Step: RunStepNormalizeToAct1Hub, AllowedNext: []RunStep{RunStepStashAndServices}},
	{Step: RunStepStashAndServices, AllowedNext: []RunStep{RunStepReachAct1Waypoint}},
	{Step: RunStepReachAct1Waypoint, AllowedNext: []RunStep{RunStepComplete}},
	{Step: RunStepComplete},
}

// SharedRunStepContracts returns a defensive copy of the shared transition table.
func SharedRunStepContracts() []RunStepContract {
	contracts := make([]RunStepContract, len(sharedRunStepContracts))
	for i, contract := range sharedRunStepContracts {
		contracts[i] = contract
		contracts[i].AllowedNext = append([]RunStep(nil), contract.AllowedNext...)
	}
	return contracts
}

// RunResetScope identifies state that must be discarded at the run reset barrier.
type RunResetScope string

const (
	// RunResetBossPin clears the runtime boss UnitID and position.
	RunResetBossPin RunResetScope = "boss_pin"
	// RunResetEncounterAction clears the encounter action index and settle state.
	RunResetEncounterAction RunResetScope = "encounter_action"
	// RunResetRoutePlayback cancels active route and navigator state.
	RunResetRoutePlayback RunResetScope = "route_playback"
	// RunResetProfileExecutor clears hooks, resources, pins, and cooldowns.
	RunResetProfileExecutor RunResetScope = "profile_executor"
	// RunResetLootExecutor clears pickup, stash, and skipped-item state.
	RunResetLootExecutor RunResetScope = "loot_executor"
	// RunResetTownExecutor clears planning, graph, NPC, and UI state.
	RunResetTownExecutor RunResetScope = "town_executor"
	// RunResetTelemetryBinding removes the recorder owned by the run generation.
	RunResetTelemetryBinding RunResetScope = "telemetry_binding"
)

var requiredRunResetScopes = []RunResetScope{
	RunResetBossPin,
	RunResetEncounterAction,
	RunResetRoutePlayback,
	RunResetProfileExecutor,
	RunResetLootExecutor,
	RunResetTownExecutor,
	RunResetTelemetryBinding,
}

// RequiredRunResetScopes returns every state owner that the shared reset barrier
// must clear before another run generation may execute input.
func RequiredRunResetScopes() []RunResetScope {
	return append([]RunResetScope(nil), requiredRunResetScopes...)
}

// RunAvailabilityStatus is the stable read-only result of run resolution.
type RunAvailabilityStatus string

const (
	// RunAvailabilityAvailable means all statically checkable requirements match.
	RunAvailabilityAvailable RunAvailabilityStatus = "available"
	// RunAvailabilityUnavailable means at least one stable blocking reason exists.
	RunAvailabilityUnavailable RunAvailabilityStatus = "unavailable"
	// RunAvailabilityRuntimeValidationRequired defers the live layout check until arrival.
	RunAvailabilityRuntimeValidationRequired RunAvailabilityStatus = "runtime_validation_required"
)

// RunReason is a stable machine-readable Phase-10 preflight or runtime reason.
type RunReason string

const (
	// RunReasonUnknown reports an unregistered run ID.
	RunReasonUnknown RunReason = "run_unknown"
	// RunReasonConfigMissing reports a definition without selected operator config.
	RunReasonConfigMissing RunReason = "run_config_missing"
	// RunReasonDefinitionInvalid reports an internally inconsistent definition.
	RunReasonDefinitionInvalid RunReason = "run_definition_invalid"
	// RunReasonCapabilityMissing reports an absent required runtime facility.
	RunReasonCapabilityMissing RunReason = "run_capability_missing"
	// RunReasonRouteMissing reports that no configured route candidate exists.
	RunReasonRouteMissing RunReason = "route_missing"
	// RunReasonRouteBindingMismatch reports incompatible static route metadata.
	RunReasonRouteBindingMismatch RunReason = "route_binding_mismatch"
	// RunReasonRouteLayoutMismatch reports a live fingerprint mismatch.
	RunReasonRouteLayoutMismatch RunReason = "route_layout_mismatch"
	// RunReasonRouteRuntimeValidation reports a live fingerprint that is not observable yet.
	RunReasonRouteRuntimeValidation RunReason = "route_runtime_validation_required"
	// RunReasonRouteStale reports a lifecycle-invalidated Farming route.
	RunReasonRouteStale RunReason = "route_stale"
	// RunReasonRouteLifecycleUnavailable reports unusable or inconsistent lifecycle metadata.
	RunReasonRouteLifecycleUnavailable RunReason = "route_lifecycle_unavailable"
	// RunReasonRouteAssignmentMissing reports an absent character/run assignment.
	RunReasonRouteAssignmentMissing RunReason = "route_assignment_missing"
	// RunReasonLegAcquisitionRouteMissing reports a missing Wirt route role.
	RunReasonLegAcquisitionRouteMissing RunReason = "leg_acquisition_route_missing"
	// RunReasonLegAcquisitionRouteStale reports an unusable Wirt route role.
	RunReasonLegAcquisitionRouteStale RunReason = "leg_acquisition_route_stale"
	// RunReasonCowSweepRouteMissing reports a missing Cow sweep route role.
	RunReasonCowSweepRouteMissing RunReason = "cow_sweep_route_missing"
	// RunReasonCowSweepRouteStale reports an unusable Cow sweep route role.
	RunReasonCowSweepRouteStale RunReason = "cow_sweep_route_stale"
	// RunReasonRouteSetBindingMismatch reports incompatible fixed-role identities.
	RunReasonRouteSetBindingMismatch RunReason = "route_set_binding_mismatch"
	// RunReasonProfileClassMismatch reports a character/profile class mismatch.
	RunReasonProfileClassMismatch RunReason = "profile_class_mismatch"
	// RunReasonProfileRunStrategyUnavailable reports that the active combat profile has no registered strategy for the run.
	RunReasonProfileRunStrategyUnavailable RunReason = "profile_run_strategy_unavailable"
	// RunReasonCharacterProfileRunIncompatible is retained for older telemetry/history only.
	// Phase 21.1 replaces live availability checks with profile_run_strategy_unavailable.
	RunReasonCharacterProfileRunIncompatible RunReason = "character_profile_run_incompatible"
	// RunReasonWaypointTargetUnsupported reports a target without registered UI action.
	RunReasonWaypointTargetUnsupported RunReason = "waypoint_target_unsupported"
	// RunReasonWaypointUIUnconfirmed reports a missing Memory-confirmed waypoint UI.
	RunReasonWaypointUIUnconfirmed RunReason = "waypoint_ui_unconfirmed"
	// RunReasonWaypointDestinationTimeout reports missing destination confirmation.
	RunReasonWaypointDestinationTimeout RunReason = "waypoint_destination_timeout"
	// RunReasonTownPortalDestinationTimeout reports that one bounded portal retry did not reach town.
	RunReasonTownPortalDestinationTimeout RunReason = "town_portal_destination_timeout"
	// RunReasonUnexpectedArea reports a valid but disallowed current area.
	RunReasonUnexpectedArea RunReason = "unexpected_area"
	// RunReasonChestSweepEmpty reports that the recorded path showed no Supertruhe.
	RunReasonChestSweepEmpty RunReason = "chest_sweep_empty"
	// RunReasonBossNotFound reports that the configured boss was not acquired.
	RunReasonBossNotFound RunReason = "boss_not_found"
	// RunReasonBossPinLost reports a lost or changed boss UnitID.
	RunReasonBossPinLost RunReason = "boss_pin_lost"
	// RunReasonEncounterActionFailed reports a terminal indexed hook failure.
	RunReasonEncounterActionFailed RunReason = "encounter_action_failed"
	// RunReasonBossKillUnconfirmed reports an exhausted kill-confirmation budget.
	RunReasonBossKillUnconfirmed RunReason = "boss_kill_unconfirmed"
	// RunReasonBossCombatNoProgress reports that Hammerdin boss combat sent
	// neither a confirmed close-range hammer nor a bounded teleport in time.
	RunReasonBossCombatNoProgress RunReason = "boss_combat_no_progress"
	// RunReasonLootPolicyInvalid reports an invalid pickup or sell policy.
	RunReasonLootPolicyInvalid RunReason = "loot_policy_invalid"
	// RunReasonItemTierUnknown reports missing authoritative base-tier data.
	RunReasonItemTierUnknown RunReason = "item_tier_unknown"
	// RunReasonItemClassificationConflict reports incompatible item dispositions.
	RunReasonItemClassificationConflict RunReason = "item_classification_conflict"
	// RunReasonItemIdentifyFailed reports failed or unverified identification.
	RunReasonItemIdentifyFailed RunReason = "item_identify_failed"
	// RunReasonItemSellFailed reports failed or unverified sale.
	RunReasonItemSellFailed RunReason = "item_sell_failed"
	// RunReasonTownEgressMissing reports an absent foreign-town egress.
	RunReasonTownEgressMissing RunReason = "town_egress_missing"
	// RunReasonTownEgressBindingMismatch reports incompatible egress metadata.
	RunReasonTownEgressBindingMismatch RunReason = "town_egress_binding_mismatch"
	// RunReasonHubTransferUnsupported reports an unregistered act transfer.
	RunReasonHubTransferUnsupported RunReason = "hub_transfer_unsupported"
	// RunReasonTownServiceVerifyTimeout reports exhausted Town verification.
	RunReasonTownServiceVerifyTimeout RunReason = "town_service_verify_timeout"
)

// RouteAvailability describes the selected route candidate without starting playback.
type RouteAvailability struct {
	RouteID string    `json:"route_id,omitempty"`
	Reason  RunReason `json:"reason,omitempty"`
}

// RunAvailability is the deterministic read-only view consumed by CLI and later GUI code.
type RunAvailability struct {
	RunID       RunID                                   `json:"run_id"`
	DisplayName string                                  `json:"display_name"`
	Status      RunAvailabilityStatus                   `json:"status"`
	Reasons     []RunReason                             `json:"reasons,omitempty"`
	Route       RouteAvailability                       `json:"route"`
	RouteRoles  map[pathing.RouteRole]RouteAvailability `json:"route_roles,omitempty"`
}
