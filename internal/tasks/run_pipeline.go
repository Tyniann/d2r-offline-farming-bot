package tasks

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// RunPhaseTravelEntry selects travel from the Act-1 hub to the definition's entry area.
	RunPhaseTravelEntry = "travel-entry"
	// RunPhasePlayRoute selects travel through the definition's complete bound route.
	RunPhasePlayRoute = "play-route"
	// RunPhaseBoss selects boss acquisition, encounter actions, and combat.
	RunPhaseBoss = "boss"
	// RunPhaseLootAndReturn selects loot, portal return, and stash recovery.
	RunPhaseLootAndReturn = "loot-and-return"
	// RunPhaseRetryReturn selects the input-minimal portal and foreign-town
	// normalization path used before retrying a failed run in a fresh game.
	RunPhaseRetryReturn = "retry-return"
	// RunPhaseStashPersonal selects transfer-free Act-1 personal-stash navigation and opening.
	RunPhaseStashPersonal = "stash-personal"
	// RunPhaseTownReady selects the isolated class-profile Town-ready hook.
	RunPhaseTownReady = "town-ready"

	pipelineStepPrecheck            = "precheck"
	pipelineStepApplyTownProfile    = "town_ready_profile"
	pipelineStepAcquireTownWaypoint = "acquire_town_waypoint"
	pipelineStepOpenWaypoint        = "open_waypoint"
	pipelineStepSelectRunWaypoint   = "select_run_waypoint"
	pipelineStepWaitEntryArea       = "wait_entry_area"
	pipelineStepPlayRoute           = "play_bound_route"
	pipelineStepAcquireBoss         = "acquire_boss"
	pipelineStepEngageBoss          = "engage_boss"
	pipelineStepClearNearbyHostiles = "clear_nearby_hostiles"
	pipelineStepRepositionForLoot   = "reposition_for_loot"
	pipelineStepWaitForDrops        = "wait_for_drops"
	pipelineStepScanLoot            = "scan_loot"
	pipelineStepPickLoot            = "pick_loot"
	pipelineStepCastTownPortal      = "cast_town_portal"
	pipelineStepEnterTownPortal     = "enter_town_portal"
	pipelineStepWaitOriginTown      = "wait_origin_town"
	pipelineStepPlayTownEgress      = "play_town_egress"
	pipelineStepOpenOriginWaypoint  = "open_origin_waypoint"
	pipelineStepSelectHubWaypoint   = "select_hub_waypoint"
	pipelineStepWaitHubArea         = "wait_hub_area"
	pipelineStepOpenStash           = "open_personal_stash"
	pipelineStepStashItems          = "stash_items"
	pipelineStepCloseStash          = "close_personal_stash"
	pipelineStepPrepareTown         = "prepare_town_handoff"
	pipelineStepComplete            = "complete"

	waypointSelectSettleDelay = 500 * time.Millisecond
	dropStableTicks           = 3
	lootNoTargetStableTicks   = 3
	postKillLootDistanceTiles = 4
	defaultLootPickupDistance = 8
	// lootApproachMaxDistanceTiles caps post-kill / route loot chase teleports.
	// Beyond this, the candidate is handed to pickup as too_far without yanking
	// the character multiple screens away before town portal.
	lootApproachMaxDistanceTiles float64 = 20
	lootRepositionRetryDelay             = 500 * time.Millisecond
	lootRepositionMaxAttempts            = 3
	routeLootRadiusTiles         float64 = 30
	routeThreatApproachSettle            = 500 * time.Millisecond
	// A single incomplete identity/route projection is a read-side flake, not
	// an internal route contract violation. No input is allowed during this
	// grace; sustained unavailability still fails closed.
	routeProgressUnavailableGrace = 2 * time.Second
	// Memory exposes integer world positions. A one-tile diagonal input can
	// therefore resolve to a much smaller positive projection onto the sent
	// command vector. Any unambiguous forward component is objective progress;
	// zero, lateral, and backward movement still consume the bounded retry.
	routeThreatApproachProgressEpsilonTiles         = 0.01
	routeThreatApproachMaxFailures                  = 3
	bossApproachSettle                              = 700 * time.Millisecond
	postBossCleanupRadiusTiles              float64 = 18
	nihlathakCleanupRadiusTiles             float64 = 30
	postBossCleanupMaxCasts                         = 20
	nihlathakCleanupMaxCasts                        = 40
	postBossCleanupStableTicks                      = 3
	nihlathakCleanupNoProgressTimeout               = 3 * time.Second
)

// runPipeline executes one immutable run definition or a thin isolated-phase alias.
// Persistent executor state belongs to this generation and is cleared at the
// runner's central reset barrier before another generation may start.
type runPipeline struct {
	definition             RunDefinition
	phase                  string
	routeID                string
	combat                 CombatConfig
	routeCombat            RouteCombatConfig
	routeThreat            RouteThreatController
	navStarted             bool
	resumeAfterPrecheckSet bool
	resumeAfterPrecheck    string
	chestFallbackStarted   bool
	targetSeen             bool
	targetUnitID           uint32
	targetPosition         world.Position
	targetPositionSet      bool
	targetAbsentTicks      int
	dropStableTicks        int
	lootScanHasTarget      bool
	lootPickupActive       bool
	lootNoTargetTicks      int
	routeStarted           bool
	// routeProgressUnavailableSince guards transient read-side projection loss
	// without letting an unavailable RoutePlayer stall the run indefinitely.
	routeProgressUnavailableSince    time.Time
	routeProgressUnavailableSnapshot time.Time
	egressStarted                    bool
	encounterActionIndex             int
	encounterActionStarted           bool
	bossKillEmitted                  bool
	bossApproachPending              bool
	bossApproachAttempted            bool
	bossApproachAt                   time.Time
	bossApproachSnapshot             time.Time
	cleanupTargetUnitID              uint32
	cleanupCastCount                 int
	cleanupNoTargetTicks             int
	cleanupLastProgressAt            time.Time
	// cleanupSkippedUnitIDs prevents an unprojectable hostile from pinning the
	// best-effort Nihlathak cleanup while another nearby target remains usable.
	cleanupSkippedUnitIDs    map[uint32]bool
	lootPickupDistanceTiles  float64
	postKillTeleportAttempts int
	postKillTeleportAt       time.Time
	postKillTeleportSnapshot time.Time
	lootApproachTarget       LootTarget
	lootApproachTargetSet    bool
	lootApproachAttempts     int
	lootApproachAt           time.Time
	lootApproachSnapshot     time.Time
	// lootPickupRecovered bounds post-fail item teleports to one attempt per UnitID.
	lootPickupRecovered       map[uint32]bool
	lootRecoveryPending       bool
	lootRecoveryTarget        LootTarget
	lootRecoveryTeleportSent  bool
	lootRecoveryAt            time.Time
	lootRecoverySnapshot      time.Time
	lootRecoveryMaxDistance   float64
	routeLootPointSet         bool
	routeLootSegmentIndex     int
	routeLootPointIndex       int
	routeLootScanned          bool
	routeApproachTargetUnitID uint32
	routeApproachOrigin       world.Position
	routeApproachGoal         world.Position
	routeApproachSentAt       time.Time
	routeApproachSnapshotAt   time.Time
	routeApproachPending      bool
	routeApproachFailures     int
	// routeApproachExhaustedUnitID suppresses further local movement for one
	// blocker after the bounded attempts. The shared no-progress watchdog, not
	// this low-level targeting inconvenience, owns any later run termination.
	routeApproachExhaustedUnitID uint32
	// cowNoProgressRecoveryStage stages Cow soft-exit: retarget → approach → fail.
	// Real objective progress resets it so a later stall may recover again.
	cowNoProgressRecoveryStage  int
	cowNoProgressApproachUnitID uint32
	// portalRecovered bounds post-fail portal teleports to one attempt per portal UnitID.
	portalRecovered            map[uint32]bool
	portalRecoveryPending      bool
	portalRecoveryUnitID       uint32
	portalRecoveryPos          world.Position
	portalRecoveryTeleportSent bool
	portalRecoveryAt           time.Time
	portalRecoverySnapshot     time.Time
	// suppressRouteLoot reserves setup inventory capacity while still reusing
	// the established route Hold/Clear controller.
	suppressRouteLoot      bool
	requireTerminalSafe    bool
	terminalSafeSnapshots  int
	terminalSafeSnapshotAt time.Time
}

type routeClearObjectiveObserver interface {
	ObserveObjectiveProgress(world.State) bool
}

type routeClearApproachObserver interface {
	ObserveRouteClearApproachProgress()
}

// routeClearNoProgressRetargetObserver rotates a stuck Cow group target before the
// shared no-progress soft exit consumes the session retry budget.
type routeClearNoProgressRetargetObserver interface {
	ObserveNoProgressRetarget(currentUnitID uint32) (world.Monster, bool)
}

const (
	cowNoProgressStageNone       = 0
	cowNoProgressStageRetargeted = 1
	cowNoProgressStageApproached = 2
)

func (c *runPipeline) effectiveDefinition() RunDefinition {
	return c.definition
}

func (c *runPipeline) handlesResources(step string) bool {
	return step == pipelineStepPlayRoute &&
		c.routeCombat.Enabled &&
		c.definition.HasCapability(RunCapabilityRouteClear)
}

func (c *runPipeline) resetGeneration() {
	c.navStarted = false
	c.resumeAfterPrecheckSet = false
	c.resumeAfterPrecheck = ""
	c.chestFallbackStarted = false
	c.targetSeen = false
	c.targetUnitID = 0
	c.targetPosition = world.Position{}
	c.targetPositionSet = false
	c.targetAbsentTicks = 0
	c.dropStableTicks = 0
	c.lootScanHasTarget = false
	c.lootPickupActive = false
	c.lootNoTargetTicks = 0
	c.routeStarted = false
	c.resetRouteProgressUnavailable()
	c.routeThreat.Reset(nil)
	c.egressStarted = false
	c.encounterActionIndex = 0
	c.encounterActionStarted = false
	c.bossKillEmitted = false
	c.resetBossApproach()
	c.resetPostBossCleanup()
	c.resetPostKillReposition()
	c.resetLootApproach()
	c.resetLootPickupRecovery()
	c.resetRouteLoot()
	c.resetRouteThreatApproach()
	c.cowNoProgressRecoveryStage = cowNoProgressStageNone
	c.cowNoProgressApproachUnitID = 0
	c.resetPortalEntryRecovery()
	c.resetTerminalSafe()
}

func (c *runPipeline) firstStep() string {
	return pipelineStepPrecheck
}

func (c *runPipeline) nextStep(current string) string {
	if c.phase == RunPhaseTownReady {
		switch current {
		case pipelineStepPrecheck:
			return pipelineStepApplyTownProfile
		case pipelineStepApplyTownProfile:
			return pipelineStepComplete
		case pipelineStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.phase == RunPhaseStashPersonal {
		switch current {
		case pipelineStepPrecheck:
			return pipelineStepOpenStash
		case pipelineStepOpenStash:
			return pipelineStepStashItems
		case pipelineStepStashItems:
			return pipelineStepCloseStash
		case pipelineStepCloseStash:
			return pipelineStepComplete
		case pipelineStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.phase == RunPhaseBoss {
		switch current {
		case pipelineStepPrecheck:
			return pipelineStepAcquireBoss
		case pipelineStepAcquireBoss:
			return pipelineStepEngageBoss
		case pipelineStepEngageBoss:
			return ""
		default:
			return ""
		}
	}
	if c.phase == RunPhaseLootAndReturn {
		switch current {
		case pipelineStepPrecheck:
			return pipelineStepWaitForDrops
		case pipelineStepWaitForDrops:
			return pipelineStepScanLoot
		case pipelineStepScanLoot:
			if c.lootScanHasTarget {
				return pipelineStepPickLoot
			}
			return pipelineStepCastTownPortal
		case pipelineStepPickLoot:
			return pipelineStepCastTownPortal
		case pipelineStepCastTownPortal:
			return pipelineStepEnterTownPortal
		case pipelineStepEnterTownPortal:
			return pipelineStepWaitOriginTown
		case pipelineStepWaitOriginTown:
			if foreignTownOrigin(c.effectiveDefinition().ReturnOrigin) {
				return pipelineStepPlayTownEgress
			}
			return pipelineStepOpenStash
		case pipelineStepPlayTownEgress:
			return pipelineStepOpenOriginWaypoint
		case pipelineStepOpenOriginWaypoint:
			return pipelineStepSelectHubWaypoint
		case pipelineStepSelectHubWaypoint:
			return pipelineStepWaitHubArea
		case pipelineStepWaitHubArea:
			return pipelineStepOpenStash
		case pipelineStepOpenStash:
			return pipelineStepStashItems
		case pipelineStepStashItems:
			return pipelineStepCloseStash
		case pipelineStepCloseStash:
			return pipelineStepComplete
		case pipelineStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.phase == RunPhaseRetryReturn {
		switch current {
		case pipelineStepPrecheck:
			return pipelineStepCastTownPortal
		case pipelineStepCastTownPortal:
			return pipelineStepEnterTownPortal
		case pipelineStepEnterTownPortal:
			return pipelineStepWaitOriginTown
		case pipelineStepWaitOriginTown:
			if foreignTownOrigin(c.effectiveDefinition().ReturnOrigin) {
				return pipelineStepPlayTownEgress
			}
			return pipelineStepComplete
		case pipelineStepPlayTownEgress:
			return pipelineStepOpenOriginWaypoint
		case pipelineStepOpenOriginWaypoint:
			return pipelineStepSelectHubWaypoint
		case pipelineStepSelectHubWaypoint:
			return pipelineStepWaitHubArea
		case pipelineStepWaitHubArea:
			return pipelineStepComplete
		case pipelineStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.isTravelPhase() {
		switch current {
		case pipelineStepPrecheck:
			if c.resumeAfterPrecheckSet {
				return c.resumeAfterPrecheck
			}
			return pipelineStepAcquireTownWaypoint
		case pipelineStepAcquireTownWaypoint:
			return pipelineStepOpenWaypoint
		case pipelineStepOpenWaypoint:
			return pipelineStepSelectRunWaypoint
		case pipelineStepSelectRunWaypoint:
			return pipelineStepWaitEntryArea
		case pipelineStepWaitEntryArea:
			if c.phase == RunPhasePlayRoute {
				return pipelineStepPlayRoute
			}
			return ""
		case pipelineStepPlayRoute:
			return ""
		default:
			return ""
		}
	}
	switch current {
	case pipelineStepPrecheck:
		return pipelineStepAcquireTownWaypoint
	case pipelineStepAcquireTownWaypoint:
		return pipelineStepOpenWaypoint
	case pipelineStepOpenWaypoint:
		return pipelineStepSelectRunWaypoint
	case pipelineStepSelectRunWaypoint:
		return pipelineStepWaitEntryArea
	case pipelineStepWaitEntryArea:
		return pipelineStepPlayRoute
	case pipelineStepPlayRoute:
		return pipelineStepAcquireBoss
	case pipelineStepAcquireBoss:
		if c.bossKillEmitted {
			if c.effectiveDefinition().ClearNearbyAfterBoss {
				return pipelineStepClearNearbyHostiles
			}
			return pipelineStepRepositionForLoot
		}
		return pipelineStepEngageBoss
	case pipelineStepEngageBoss:
		if c.effectiveDefinition().ClearNearbyAfterBoss {
			return pipelineStepClearNearbyHostiles
		}
		// Every registered farming run owns a Memory-pinned boss. Reaching its
		// last confirmed position before scanning is therefore a shared loot
		// invariant, not run-specific metadata.
		return pipelineStepRepositionForLoot
	case pipelineStepRepositionForLoot:
		return pipelineStepWaitForDrops
	case pipelineStepClearNearbyHostiles:
		return pipelineStepRepositionForLoot
	case pipelineStepWaitForDrops:
		return pipelineStepScanLoot
	case pipelineStepScanLoot:
		if c.lootScanHasTarget {
			return pipelineStepPickLoot
		}
		return pipelineStepCastTownPortal
	case pipelineStepPickLoot:
		return pipelineStepCastTownPortal
	case pipelineStepCastTownPortal:
		return pipelineStepEnterTownPortal
	case pipelineStepEnterTownPortal:
		return pipelineStepWaitOriginTown
	case pipelineStepWaitOriginTown:
		if foreignTownOrigin(c.effectiveDefinition().ReturnOrigin) {
			return pipelineStepPlayTownEgress
		}
		return pipelineStepOpenStash
	case pipelineStepPlayTownEgress:
		return pipelineStepOpenOriginWaypoint
	case pipelineStepOpenOriginWaypoint:
		return pipelineStepSelectHubWaypoint
	case pipelineStepSelectHubWaypoint:
		return pipelineStepWaitHubArea
	case pipelineStepWaitHubArea:
		return pipelineStepOpenStash
	case pipelineStepOpenStash:
		return pipelineStepStashItems
	case pipelineStepStashItems:
		return pipelineStepCloseStash
	case pipelineStepCloseStash:
		return pipelineStepPrepareTown
	case pipelineStepPrepareTown:
		return pipelineStepComplete
	case pipelineStepComplete:
		return ""
	default:
		return ""
	}
}

func (c *runPipeline) usesTickTimeout(step string) bool {
	return step == pipelineStepPlayRoute
}

func (c *runPipeline) timeoutReason(step string) string {
	if step == pipelineStepWaitEntryArea || step == pipelineStepWaitHubArea {
		return string(RunReasonWaypointDestinationTimeout)
	}
	return "timeout"
}

func (c *runPipeline) allowsNonInputTick(step string) bool {
	if step == pipelineStepWaitOriginTown && (c.phase == "" || c.phase == RunPhaseLootAndReturn || c.phase == RunPhaseRetryReturn) {
		return true
	}
	return (c.isTravelPhase() || c.phase == "") && (step == pipelineStepWaitEntryArea || step == pipelineStepPlayRoute)
}

func (c *runPipeline) onStepEnter(step string) {
	c.navStarted = false
	c.routeStarted = false
	c.resetRouteProgressUnavailable()
	c.egressStarted = false
	if step == pipelineStepClearNearbyHostiles {
		c.resetPostBossCleanup()
	}
	if step == pipelineStepRepositionForLoot {
		c.resetPostKillReposition()
	}
	if step == pipelineStepWaitForDrops {
		c.dropStableTicks = 0
	}
	if step == pipelineStepScanLoot {
		c.lootScanHasTarget = false
		c.lootNoTargetTicks = 0
	}
	if step == pipelineStepPickLoot {
		c.lootPickupActive = false
		c.lootNoTargetTicks = 0
		c.resetLootApproach()
	}
	if step == pipelineStepAcquireBoss {
		c.resetRouteLoot()
		c.resetBossApproach()
		c.chestFallbackStarted = false
		c.targetSeen = false
		c.targetUnitID = 0
		c.targetPosition = world.Position{}
		c.targetPositionSet = false
		c.targetAbsentTicks = 0
		c.encounterActionIndex = 0
		c.encounterActionStarted = false
	}
	if step == pipelineStepPlayRoute {
		c.resetTerminalSafe()
	}
}

func (c *runPipeline) onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult {
	if c.phase == RunPhaseTownReady {
		return c.onTownReadyTick(ctx, deps, step, w, now)
	}
	if c.phase == RunPhaseStashPersonal {
		return c.onStashPersonalTick(ctx, deps, step, w)
	}
	if c.phase == RunPhaseBoss {
		return c.onBossTick(ctx, deps, step, w, now)
	}
	if c.phase == RunPhaseLootAndReturn {
		return c.onLootTick(ctx, deps, step, w, now, stepStartedAt)
	}
	if c.phase == RunPhaseRetryReturn {
		return c.onRetryReturnTick(ctx, deps, step, w, now, stepStartedAt)
	}
	if c.isTravelPhase() {
		return c.onTravelTick(ctx, deps, step, w, now, stepStartedAt)
	}
	if c.phase == "" {
		return c.onRunTick(ctx, deps, step, w, now, stepStartedAt)
	}
	return stepResult{failed: true, reason: "unknown_step"}
}

func (c *runPipeline) onRetryReturnTick(ctx context.Context, deps Deps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	if step == pipelineStepPrecheck {
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if !c.allowsRetryReturnArea(w.Area.ID) {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		return stepResult{complete: true}
	}
	return c.onLootTick(ctx, deps, step, w, now, stepStartedAt)
}

func (c *runPipeline) onTownReadyTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		if !w.Identity.Valid {
			return stepResult{}
		}
		return stepResult{complete: true}
	case pipelineStepApplyTownProfile:
		if deps.Profile == nil {
			return stepResult{failed: true, reason: "profile_not_wired"}
		}
		res := deps.Profile.TickHook(ctx, profile.HookTownReady, w, profile.EncounterTarget{}, now)
		switch res.Status {
		case profile.StatusComplete:
			return stepResult{complete: true}
		case profile.StatusFailed:
			return stepResult{failed: true, reason: res.Reason}
		default:
			return stepResult{}
		}
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) onStashPersonalTick(ctx context.Context, deps Deps, step string, w world.State) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		if deps.Stash == nil {
			return stepResult{failed: true, reason: "stash_actions_not_wired"}
		}
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		return stepResult{complete: true}
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case pipelineStepPrepareTown:
		if deps.Town == nil {
			return stepResult{failed: true, reason: "town_preparation_not_wired"}
		}
		res := deps.Town.Tick(ctx, w)
		if !res.Done {
			return stepResult{}
		}
		if res.Status == "complete" {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: res.Reason}
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func tickPersonalStashWorkflow(ctx context.Context, deps Deps, step string, w world.State) stepResult {
	switch step {
	case pipelineStepOpenStash:
		if deps.Stash == nil {
			return stepResult{failed: true, reason: "stash_actions_not_wired"}
		}
		res := deps.Stash.Tick(ctx, w)
		if !res.Done {
			return stepResult{}
		}
		if res.Status == pathing.PersonalStashOpened {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: string(res.Status)}
	case pipelineStepStashItems:
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		res := deps.Loot.TickStash(w, w.At)
		if !res.Done {
			return stepResult{}
		}
		if res.Status == LootStashSuccess {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: string(res.Status)}
	case pipelineStepCloseStash:
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		res := deps.Loot.TickCloseStash(w, w.At)
		if !res.Done {
			return stepResult{}
		}
		if res.Status == LootStashClosed {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: string(res.Status)}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) onRunTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID == world.RogueEncampment {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: "not_act1_town"}
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint,
		pipelineStepWaitEntryArea, pipelineStepPlayRoute:
		return c.onTravelTick(ctx, deps, step, w, now, stepStartedAt)
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles, pipelineStepRepositionForLoot:
		return c.onBossTick(ctx, deps, step, w, now)
	case pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return c.onLootTick(ctx, deps, step, w, now, stepStartedAt)
	case pipelineStepCastTownPortal:
		return tickRunTownPortal(deps, w)
	case pipelineStepEnterTownPortal:
		return c.tickEnterTownPortal(ctx, deps, w, now)
	case pipelineStepWaitOriginTown:
		return c.tickWaitOriginTown(w)
	case pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return c.onTownNormalizationTick(ctx, deps, step, w, now, stepStartedAt)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case pipelineStepPrepareTown:
		if deps.Town == nil {
			return stepResult{failed: true, reason: "town_preparation_not_wired"}
		}
		res := deps.Town.Tick(ctx, w)
		if !res.Done {
			return stepResult{}
		}
		if res.Status == "complete" {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: res.Reason}
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) onLootTick(ctx context.Context, deps Deps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		return stepResult{complete: true}
	case pipelineStepWaitForDrops:
		if res := c.lootAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			c.dropStableTicks = 0
			return stepResult{}
		}
		c.dropStableTicks++
		if c.dropStableTicks >= dropStableTicks {
			return stepResult{complete: true}
		}
		return stepResult{}
	case pipelineStepScanLoot:
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		if res := c.lootAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		scan := deps.Loot.Scan(w)
		if scan.TelemetryFailed {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		if scan.InventoryFull {
			c.lootScanHasTarget = false
			return stepResult{complete: true}
		}
		if scan.HasTarget {
			c.lootNoTargetTicks = 0
			c.lootScanHasTarget = true
			return stepResult{complete: true}
		}
		c.lootScanHasTarget = false
		c.lootNoTargetTicks++
		if c.lootNoTargetTicks >= lootNoTargetStableTicks {
			return stepResult{complete: true}
		}
		return stepResult{}
	case pipelineStepPickLoot:
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		if res := c.lootAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if !c.lootPickupActive {
			if c.lootRecoveryPending {
				if res := c.tickLootPickupRecovery(deps, w, now); res.failed || res.complete {
					return res
				}
				if !c.lootPickupActive {
					return stepResult{}
				}
			} else {
				targetSelectedThisTick := false
				if !c.lootApproachTargetSet {
					scan := deps.Loot.Scan(w)
					if scan.TelemetryFailed {
						return stepResult{failed: true, reason: "telemetry_failed"}
					}
					if scan.InventoryFull {
						return stepResult{complete: true}
					}
					if !scan.HasTarget {
						c.lootNoTargetTicks++
						if c.lootNoTargetTicks >= lootNoTargetStableTicks {
							return stepResult{complete: true}
						}
						return stepResult{}
					}
					c.lootNoTargetTicks = 0
					c.lootApproachTarget = scan.NextTarget
					c.lootApproachTargetSet = true
					targetSelectedThisTick = true
				}
				target := c.lootApproachTarget
				if !targetSelectedThisTick {
					var found bool
					target, found = currentLootTarget(w, target)
					if !found {
						// The frozen candidate disappeared or changed before input.
						// Rescan on a later tick instead of acting on stale coordinates.
						c.resetLootApproach()
						return stepResult{}
					}
					c.lootApproachTarget = target
				}
				if world.Distance(w.Player.Position, target.Position) > c.effectiveLootPickupDistance() {
					if deps.Combat == nil {
						return stepResult{failed: true, reason: "combat_not_wired"}
					}
					// Far drops are abandoned without chase teleports so the
					// character stays near the boss pile for town portal.
					if world.Distance(w.Player.Position, target.Position) <= lootApproachMaxDistanceTiles &&
						c.lootApproachAttempts < lootRepositionMaxAttempts {
						if !lootRepositionReady(now, w.At, c.lootApproachAt, c.lootApproachSnapshot) {
							return stepResult{}
						}
						sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
						if err != nil {
							return stepResult{failed: true, reason: "loot_reposition_failed"}
						}
						if sent {
							c.lootApproachAttempts++
							c.lootApproachAt = now
							c.lootApproachSnapshot = w.At
						}
						return stepResult{}
					}
					if c.lootApproachAttempts > 0 && !lootRepositionReady(now, w.At, c.lootApproachAt, c.lootApproachSnapshot) {
						return stepResult{}
					}
					// Let the existing pickup executor record `too_far` and skip
					// this exact UnitID after the bounded reposition budget.
				}
				if err := deps.Loot.StartPickup(target); err != nil {
					return stepResult{failed: true, reason: "loot_pickup_start_failed"}
				}
				c.lootPickupActive = true
				c.resetLootApproach()
			}
		}
		res := deps.Loot.TickPickup(w, now)
		if !res.Done {
			return stepResult{}
		}
		switch res.Status {
		case LootPickupHoverNotFound, LootPickupFailed, LootPickupTooFar:
			// Post-kill keep/pick candidates get the same one-shot item teleport as
			// route loot: a single too_far must not permanently soft-skip the UnitID.
			c.lootPickupActive = false
			c.resetLootApproach()
			if c.beginLootPickupRecovery(deps, res.Target, lootApproachMaxDistanceTiles) {
				return stepResult{}
			}
			return stepResult{}
		case LootPickupPickedUp, LootPickupMonsterNearby,
			LootPickupTargetLost, LootPickupTargetUnstable:
			c.lootPickupActive = false
			c.resetLootApproach()
			return stepResult{}
		case LootPickupInputBlocked, LootPickupProjectionFailed, LootPickupInvalidWorld, LootPickupTelemetryFailed:
			return stepResult{failed: true, reason: string(res.Status)}
		default:
			return stepResult{failed: true, reason: "loot_pickup_failed"}
		}
	case pipelineStepCastTownPortal:
		return tickRunTownPortal(deps, w)
	case pipelineStepEnterTownPortal:
		return c.tickEnterTownPortal(ctx, deps, w, now)
	case pipelineStepWaitOriginTown:
		return c.tickWaitOriginTown(w)
	case pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return c.onTownNormalizationTick(ctx, deps, step, w, now, stepStartedAt)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) lootAreaGuard(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func (c *runPipeline) allowsRetryReturnArea(area world.AreaID) bool {
	definition := c.effectiveDefinition()
	contract := definition.Recording
	if definition.RouteSet != nil {
		primary, found := definition.RecordingForRole(definition.RouteSet.PrimaryRole)
		if !found {
			return false
		}
		contract = primary
	}
	for _, allowed := range contract.AllowedRouteAreas {
		if area == allowed {
			return true
		}
	}
	return false
}

func tickRunTownPortal(deps Deps, w world.State) stepResult {
	if !w.Valid {
		return stepResult{failed: true, reason: "invalid_world"}
	}
	if w.Phase != world.GamePhaseInGame {
		return stepResult{failed: true, reason: "not_in_game"}
	}
	if deps.Actions == nil {
		return stepResult{failed: true, reason: "run_actions_not_wired"}
	}
	if err := deps.Actions.CastTownPortal(time.Now(), w.Player); err != nil {
		if errors.Is(err, profile.ErrSkillSelectionPending) {
			return stepResult{}
		}
		return stepResult{failed: true, reason: "town_portal_failed"}
	}
	return stepResult{complete: true}
}

func (c *runPipeline) tickEnterTownPortal(ctx context.Context, deps Deps, w world.State, now time.Time) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID == c.originTownArea() {
		return stepResult{complete: true}
	}
	if w.Area.ID != c.effectiveDefinition().RouteTerminalArea &&
		(c.phase != RunPhaseRetryReturn || !c.allowsRetryReturnArea(w.Area.ID)) {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	if deps.Portal == nil {
		return stepResult{failed: true, reason: "town_portal_actions_not_wired"}
	}
	if c.portalRecoveryPending {
		return c.tickPortalEntryRecovery(deps, w, now)
	}
	res := deps.Portal.Tick(ctx, w, now)
	switch res.Status {
	case pathing.TownPortalActionPending:
		return stepResult{}
	case pathing.TownPortalActionClicked:
		return stepResult{complete: true}
	case pathing.TownPortalActionNotFound:
		return stepResult{failed: true, reason: "town_portal_not_found"}
	case pathing.TownPortalActionTooFar, pathing.TownPortalActionHoverNotFound:
		portal, ok := w.NearestObject(world.ObjectKindTownPortal)
		if ok && c.beginPortalEntryRecovery(portal.UnitID, portal.Position) {
			return stepResult{}
		}
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	default:
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	}
}

func (c *runPipeline) tickWaitOriginTown(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID == c.originTownArea() {
		return stepResult{complete: true}
	}
	if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func (c *runPipeline) onTownNormalizationTick(ctx context.Context, deps Deps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	if c.effectiveDefinition().ReturnOrigin == town.OriginAct1 {
		return stepResult{failed: true, reason: string(RunReasonHubTransferUnsupported)}
	}
	switch step {
	case pipelineStepPlayTownEgress:
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if w.Area.ID != c.originTownArea() {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		if deps.TownEgress == nil {
			return stepResult{failed: true, reason: string(RunReasonTownEgressMissing)}
		}
		if !c.egressStarted {
			if err := deps.TownEgress.Start(c.effectiveDefinition().ReturnOrigin, w); err != nil {
				return stepResult{failed: true, reason: townEgressFailureReason(err)}
			}
			c.egressStarted = true
		}
		done, err := deps.TownEgress.Tick(ctx, w)
		if err != nil {
			return stepResult{failed: true, reason: townEgressFailureReason(err)}
		}
		return stepResult{complete: done}
	case pipelineStepOpenOriginWaypoint:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		res := deps.Waypoint.TickTownWaypoint(ctx, w)
		if res.Status == pathing.WaypointActionPending {
			return stepResult{}
		}
		if res.Status == pathing.WaypointActionClicked {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: waypointFailureReason(res)}
	case pipelineStepSelectHubWaypoint:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		if !stepStartedAt.IsZero() && now.Sub(stepStartedAt) < waypointSelectSettleDelay {
			return stepResult{}
		}
		res := deps.Waypoint.SelectWaypointTarget(ctx, w, pathing.WaypointTargetRogueEncampment, now)
		if res.Status == pathing.WaypointActionPending {
			return stepResult{}
		}
		if res.Status == pathing.WaypointActionClicked {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: waypointFailureReason(res)}
	case pipelineStepWaitHubArea:
		if w.Valid && w.Area.ID == world.RogueEncampment {
			return stepResult{complete: true}
		}
		if w.Valid && w.Phase == world.GamePhaseInGame && w.Area.ID != c.originTownArea() {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func townEgressFailureReason(err error) string {
	if errors.Is(err, pathing.ErrRouteCharacterMismatch) || errors.Is(err, pathing.ErrRouteGameVersionMismatch) || errors.Is(err, pathing.ErrRouteLayoutUnverified) || errors.Is(err, pathing.ErrRouteLayoutMismatch) || errors.Is(err, pathing.ErrRouteStartMismatch) {
		return string(RunReasonTownEgressBindingMismatch)
	}
	if errors.Is(err, pathing.ErrRouteNotFound) {
		return string(RunReasonTownEgressMissing)
	}
	return "town_egress_failed"
}

func (c *runPipeline) originTownArea() world.AreaID {
	switch c.effectiveDefinition().ReturnOrigin {
	case town.OriginAct1:
		return world.RogueEncampment
	case town.OriginAct3:
		return world.KurastDocks
	case town.OriginAct2:
		return world.LutGholein
	case town.OriginAct4:
		return world.ThePandemoniumFortress
	case town.OriginAct5:
		return world.Harrogath
	default:
		return world.None
	}
}

func foreignTownOrigin(act town.OriginAct) bool {
	switch act {
	case town.OriginAct2, town.OriginAct3, town.OriginAct4, town.OriginAct5:
		return true
	default:
		return false
	}
}

func (c *runPipeline) onBossTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		return stepResult{complete: true}
	case pipelineStepAcquireBoss:
		if res := c.killAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if target, ok := c.findBossTarget(w); ok {
			c.storeBossTarget(target)
			return stepResult{complete: true}
		}
		if c.phase == "" && c.effectiveDefinition().ID == RunIDSummoner {
			// The Summoner can die to the mercenary or a piercing route-clear
			// attack before the dedicated boss step starts. Priority enumeration
			// makes his absence authoritative once the recorded route completed.
			// Continue from the player's terminal position instead of waiting for
			// an already dead boss; stronger bosses retain normal acquisition.
			c.targetPosition = w.Player.Position
			c.targetPositionSet = true
			if err := c.emitBossKill(deps); err != nil {
				return stepResult{failed: true, reason: "telemetry_failed"}
			}
			c.bossKillEmitted = true
			return stepResult{complete: true}
		}
		return c.tickBossSearchFallback(ctx, deps, w)
	case pipelineStepEngageBoss:
		if res := c.killAreaGuard(w); res.failed {
			if deps.Combat != nil {
				if err := deps.Combat.StopAttack(); err != nil {
					return stepResult{failed: true, reason: "combat_action_failed"}
				}
			}
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			if deps.Combat != nil {
				if err := deps.Combat.StopAttack(); err != nil {
					return stepResult{failed: true, reason: "combat_action_failed"}
				}
			}
			return stepResult{}
		}
		if !c.targetSeen {
			return stepResult{failed: true, reason: "target_not_set"}
		}
		actions := c.effectiveDefinition().BossEngageSequence
		target, visible := c.findMonsterByUnitID(w, c.targetUnitID)
		if !visible {
			if deps.Combat != nil {
				if err := deps.Combat.StopAttack(); err != nil {
					return stepResult{failed: true, reason: "combat_action_failed"}
				}
			}
			// A boss disappearing or changing UnitID before every indexed
			// encounter action completed is not valid kill evidence. Once the
			// sequence is complete, a different live boss with the configured
			// NPC ID still invalidates the original pin before absence counting.
			if deps.Profile != nil && c.encounterActionIndex < len(actions) {
				return stepResult{failed: true, reason: string(RunReasonBossPinLost)}
			}
			if replacement, found := c.findConfiguredBossTarget(w); found && replacement.UnitID != c.targetUnitID {
				return stepResult{failed: true, reason: string(RunReasonBossPinLost)}
			}
			c.targetAbsentTicks++
			if c.targetAbsentTicks >= c.combat.KillConfirmTicks {
				if !c.bossKillEmitted {
					if err := c.emitBossKill(deps); err != nil {
						return stepResult{failed: true, reason: "telemetry_failed"}
					}
					c.bossKillEmitted = true
				}
				return stepResult{complete: true}
			}
			return stepResult{}
		}
		c.targetPosition = target.Position
		c.targetPositionSet = true
		c.targetAbsentTicks = 0
		if c.encounterActionIndex < len(actions) && deps.Profile != nil {
			action := actions[c.encounterActionIndex]
			if !c.encounterActionStarted {
				if err := c.emitEncounterAction(deps, telemetry.RunEncounterActionStarted, RunOutcomeRunning, "", target.UnitID); err != nil {
					return stepResult{failed: true, reason: "telemetry_failed"}
				}
				c.encounterActionStarted = true
			}
			res := deps.Profile.TickHook(ctx, action.Hook, w, profile.EncounterTarget{UnitID: target.UnitID, Position: target.Position, ActionIndex: c.encounterActionIndex}, now)
			switch res.Status {
			case profile.StatusFailed:
				return stepResult{failed: true, reason: res.Reason}
			case profile.StatusAction, profile.StatusPending:
				return stepResult{}
			case profile.StatusComplete:
				if err := c.emitEncounterAction(deps, telemetry.RunEncounterActionCompleted, RunOutcomeSuccess, "", target.UnitID); err != nil {
					return stepResult{failed: true, reason: "telemetry_failed"}
				}
				c.encounterActionIndex++
				c.encounterActionStarted = false
				if c.encounterActionIndex < len(actions) {
					// Indexed actions are separate input opportunities; one poll
					// can never advance more than one action.
					return stepResult{}
				}
			}
		} else if deps.Profile == nil {
			// Isolated combat diagnostics may intentionally omit the profile;
			// productive app wiring validates it before starting the run.
			c.encounterActionIndex = len(actions)
		}
		return c.tickEngageTarget(deps, w, target, now)
	case pipelineStepClearNearbyHostiles:
		if res := c.killAreaGuard(w); res.failed {
			return c.stopCleanup(deps, res)
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return c.stopCleanup(deps, stepResult{})
		}
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		if c.cleanupCastCount >= c.postBossCleanupMaxCasts() {
			// Cleanup is deliberately best-effort. The configured budget must
			// never trap the run in combat instead of advancing to loot.
			return c.stopCleanup(deps, stepResult{complete: true})
		}
		if c.effectiveDefinition().ID == RunIDNihlathak {
			if c.cleanupLastProgressAt.IsZero() {
				c.cleanupLastProgressAt = now
			} else if now.Sub(c.cleanupLastProgressAt) >= nihlathakCleanupNoProgressTimeout {
				// Cleanup must not park the whole run when Memory still exposes
				// a living hostile that cannot produce another combat input.
				return c.stopCleanup(deps, stepResult{complete: true})
			}
		}
		target, visible := c.findCleanupTarget(w)
		if !visible {
			c.cleanupTargetUnitID = 0
			c.cleanupNoTargetTicks++
			if c.cleanupNoTargetTicks >= postBossCleanupStableTicks {
				return c.stopCleanup(deps, stepResult{complete: true})
			}
			return c.stopCleanup(deps, stepResult{})
		}
		c.cleanupTargetUnitID = target.UnitID
		c.cleanupNoTargetTicks = 0
		if c.effectiveDefinition().ID == RunIDNihlathak {
			if deps.RouteClear == nil {
				return stepResult{failed: true, reason: "combat_not_wired"}
			}
			result := deps.RouteClear.TickRouteClear(ctx, profile.RouteClearRequest{
				RunID:        string(c.effectiveDefinition().ID),
				DefinitionID: c.combat.Profile,
				Player:       w.Player,
				Target:       target,
				Mode:         profile.RouteClearThreat,
				AssessmentAt: w.At,
			}, now)
			switch result.Status {
			case profile.StatusFailed:
				return stepResult{failed: true, reason: "combat_action_failed"}
			case profile.StatusAction:
				c.cleanupCastCount++
				c.cleanupLastProgressAt = now
			case profile.StatusPending:
				if result.Reason == profile.RouteClearReasonTargetUnprojectable {
					if c.cleanupSkippedUnitIDs == nil {
						c.cleanupSkippedUnitIDs = make(map[uint32]bool)
					}
					c.cleanupSkippedUnitIDs[target.UnitID] = true
					c.cleanupTargetUnitID = 0
				}
			}
			return stepResult{}
		}
		cast, err := deps.Combat.CastAttackAtMonster(now, c.combat.AttackSkillID, w.Player, target)
		if err != nil {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}
		if cast.Sent {
			c.cleanupCastCount++
		}
		return stepResult{}
	case pipelineStepRepositionForLoot:
		if res := c.killAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		if !c.targetPositionSet {
			return stepResult{failed: true, reason: "boss_position_missing"}
		}
		if world.Distance(w.Player.Position, c.targetPosition) <= postKillLootDistanceTiles {
			return stepResult{complete: true}
		}
		if c.postKillTeleportAttempts >= lootRepositionMaxAttempts {
			if lootRepositionReady(now, w.At, c.postKillTeleportAt, c.postKillTeleportSnapshot) {
				// Boss repositioning is a best-effort first approach. Candidate-
				// specific repositioning after the drop scan remains authoritative.
				return stepResult{complete: true}
			}
			return stepResult{}
		}
		if !lootRepositionReady(now, w.At, c.postKillTeleportAt, c.postKillTeleportSnapshot) {
			return stepResult{}
		}
		sent, err := deps.Combat.TeleportToward(now, w.Player, c.targetPosition, 0)
		if err != nil {
			return stepResult{failed: true, reason: "post_kill_reposition_failed"}
		}
		if sent {
			c.postKillTeleportAttempts++
			c.postKillTeleportAt = now
			c.postKillTeleportSnapshot = w.At
		}
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) emitEncounterAction(deps Deps, event telemetry.EventName, outcome RunOutcome, reason string, unitID uint32) error {
	if deps.Telemetry == nil {
		return nil
	}
	index := c.encounterActionIndex
	return deps.Telemetry.Emit(telemetry.Event{
		Event: event, DefinitionID: string(c.effectiveDefinition().ID), Step: pipelineStepEngageBoss,
		Stage: telemetry.HistoryStageCombat, ActionIndex: &index, UnitID: unitID, Outcome: string(outcome), Reason: reason,
	})
}

func (c *runPipeline) emitBossKill(deps Deps) error {
	if deps.Telemetry == nil {
		return nil
	}
	definition := c.effectiveDefinition()
	return deps.Telemetry.Emit(telemetry.Event{
		Event: telemetry.BossKillConfirmed, DefinitionID: string(definition.ID), Step: pipelineStepEngageBoss,
		Stage: telemetry.HistoryStageCombat, UnitID: c.targetUnitID, BossID: string(definition.ID), BossName: definition.Boss.Name,
	})
}

func (c *runPipeline) killAreaGuard(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func (c *runPipeline) findBossTarget(w world.State) (world.Monster, bool) {
	definition := c.effectiveDefinition()
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID != definition.RouteTerminalArea {
		return world.Monster{}, false
	}
	if target, ok := c.findConfiguredBossTarget(w); ok {
		return target, true
	}
	if !definition.Boss.AllowAnySuperUniqueFallback {
		return world.Monster{}, false
	}
	return w.FindSuperUnique(0)
}

func (c *runPipeline) findConfiguredBossTarget(w world.State) (world.Monster, bool) {
	definition := c.effectiveDefinition()
	if definition.Boss.RequireSuperUnique {
		// Countess shares a base NPC ID with ordinary Dark Stalkers and must
		// retain the super-unique flag gate before its explicit fallback.
		return w.FindSuperUnique(definition.Boss.NPCID)
	}
	// Act bosses such as Mephisto have an exact generated NPC ID but do not
	// carry d2go's super-unique type flag. Their NPC identity is authoritative.
	return w.FindNPC(definition.Boss.NPCID)
}

func (c *runPipeline) findMonsterByUnitID(w world.State, unitID uint32) (world.Monster, bool) {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return world.Monster{}, false
	}
	for _, m := range w.Monsters {
		if m.UnitID == unitID {
			return m, true
		}
	}
	return world.Monster{}, false
}

func (c *runPipeline) storeBossTarget(target world.Monster) {
	c.targetSeen = true
	c.targetUnitID = target.UnitID
	c.targetPosition = target.Position
	c.targetPositionSet = true
	c.targetAbsentTicks = 0
}

func (c *runPipeline) tickBossSearchFallback(ctx context.Context, deps Deps, w world.State) stepResult {
	if !c.chestFallbackStarted {
		target, ok := c.bossSearchAnchor(w)
		if !ok {
			return stepResult{}
		}
		if deps.Pathing == nil {
			return stepResult{failed: true, reason: "pathing_not_wired"}
		}
		goal := pathing.Goal{Kind: pathing.GoalKindMoveToPosition, TargetPos: target}
		if err := deps.Pathing.Start(goal); err != nil {
			return stepResult{failed: true, reason: pathingStartFailureReason(err)}
		}
		c.chestFallbackStarted = true
	}
	if deps.Pathing == nil {
		return stepResult{failed: true, reason: "pathing_not_wired"}
	}
	if !deps.Pathing.Active() {
		return stepResult{}
	}
	res := deps.Pathing.Tick(ctx, w)
	if !res.Done {
		return stepResult{}
	}
	if res.Status == pathing.NavArrived {
		return stepResult{}
	}
	return stepResult{failed: true, reason: navigatorFailureReason(res)}
}

func (c *runPipeline) bossSearchAnchor(w world.State) (world.Position, bool) {
	boss := c.effectiveDefinition().Boss
	if boss.SearchAnchorObject != world.ObjectKindUnknown {
		if chest, ok := w.NearestObject(boss.SearchAnchorObject); ok {
			return chest.Position, true
		}
	}
	if boss.SearchAnchorEntrance != world.EntranceKindUnknown {
		if down, ok := w.NearestEntrance(boss.SearchAnchorEntrance); ok {
			return down.Position, true
		}
	}
	return world.Position{}, false
}

func (c *runPipeline) tickEngageTarget(deps Deps, w world.State, target world.Monster, now time.Time) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if c.effectiveDefinition().ID == RunIDNihlathak {
		return c.tickNihlathakEngageTarget(deps, w, target, now)
	}
	distance := world.Distance(w.Player.Position, target.Position)
	var err error
	if distance > c.combat.RepositionDistanceTiles {
		_, err = deps.Combat.TeleportToward(now, w.Player, target.Position, c.combat.EngageDistanceTiles)
	} else {
		_, err = deps.Combat.CastAttackAtWorld(now, c.combat.AttackSkillID, w.Player, target.Position)
	}
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	return stepResult{}
}

func (c *runPipeline) tickNihlathakEngageTarget(deps Deps, w world.State, target world.Monster, now time.Time) stepResult {
	if c.bossApproachPending {
		if now.Sub(c.bossApproachAt) < bossApproachSettle || !w.At.After(c.bossApproachSnapshot) {
			return stepResult{}
		}
		c.bossApproachPending = false
	}

	// Bone Spear pierces the pack around Nihlathak. Any living monster already
	// under the cursor is an immediate attack surface; the pinned boss UnitID
	// remains authoritative only for presence and kill confirmation.
	attackTarget := target
	for _, monster := range w.Monsters {
		if monster.IsHovered {
			attackTarget = monster
			break
		}
	}
	canAim := attackTarget.IsHovered || deps.Combat.MonsterAimProjectable(w.Player.Position, target.Position)
	if canAim {
		_, err := deps.Combat.CastAttackAtMonster(now, c.combat.AttackSkillID, w.Player, attackTarget)
		if err == nil {
			return stepResult{}
		}
		if !errors.Is(err, profile.ErrRouteClearTargetUnprojectable) {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}
		// A shove or blocked landing can invalidate aim between ticks. Fall
		// through to the one-shot approach gate or controlled retry-return.
	}

	if c.bossApproachAttempted {
		// The recorded route endpoint is the preferred combat anchor. One
		// projection-driven approach is the entire in-fight fallback; further
		// displacement exits via retry-return instead of a cold queue abort.
		return stepResult{failed: true, reason: "boss_combat_unprojectable"}
	}
	_, desiredDistance, ok := deps.Combat.FarthestProjectableMonsterApproach(w.Player.Position, target.Position)
	if !ok {
		return stepResult{failed: true, reason: "boss_combat_unprojectable"}
	}
	sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, desiredDistance)
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	if sent {
		c.bossApproachPending = true
		c.bossApproachAttempted = true
		c.bossApproachAt = now
		c.bossApproachSnapshot = w.At
	}
	return stepResult{}
}

func (c *runPipeline) resetBossApproach() {
	c.bossApproachPending = false
	c.bossApproachAttempted = false
	c.bossApproachAt = time.Time{}
	c.bossApproachSnapshot = time.Time{}
}

func (c *runPipeline) findCleanupTarget(w world.State) (world.Monster, bool) {
	var nearest world.Monster
	var nearestDistanceSquared float64
	found := false
	radius := c.postBossCleanupRadiusTiles()
	for _, monster := range w.Monsters {
		if monster.UnitID == c.targetUnitID || c.cleanupSkippedUnitIDs[monster.UnitID] || !c.isCleanupHostile(monster) {
			continue
		}
		distanceSquared := positionDistanceSquared(w.Player.Position, monster.Position)
		if c.effectiveDefinition().ID == RunIDNihlathak && monster.IsHovered &&
			distanceSquared <= radius*radius {
			// Nihlathak is already confirmed dead. Accept any living monster
			// currently under the cursor so overlapping sprites cannot restart
			// an aim loop during the post-boss clear.
			return monster, true
		}
		if distanceSquared <= radius*radius &&
			preferLivingTarget(monster, distanceSquared, nearest, nearestDistanceSquared, found) {
			nearest = monster
			nearestDistanceSquared = distanceSquared
			found = true
		}
	}
	return nearest, found
}

func (c *runPipeline) isCleanupHostile(monster world.Monster) bool {
	switch c.effectiveDefinition().ID {
	case RunIDSummoner:
		return c.effectiveDefinition().AllowsRouteHostile(monster.NPCID)
	case RunIDNihlathak:
		// Once Nihlathak is confirmed dead, every remaining living monster near
		// the player blocks safe loot and portal handling.
		return true
	default:
		return false
	}
}

func (c *runPipeline) postBossCleanupRadiusTiles() float64 {
	if c.effectiveDefinition().ID == RunIDNihlathak {
		// Halls of Vaught combines dense melee packs with ranged attackers. Keep
		// Summoner's 18-tile radius unchanged, but clear the full nearby
		// encounter instead of declaring the portal safe too early.
		return nihlathakCleanupRadiusTiles
	}
	return postBossCleanupRadiusTiles
}

func (c *runPipeline) postBossCleanupMaxCasts() int {
	if c.effectiveDefinition().ID == RunIDNihlathak {
		return nihlathakCleanupMaxCasts
	}
	return postBossCleanupMaxCasts
}

func (c *runPipeline) stopCleanup(deps Deps, result stepResult) stepResult {
	if c.effectiveDefinition().ID == RunIDNihlathak && deps.RouteClear != nil {
		deps.RouteClear.ResetRouteClear()
	}
	if deps.Combat != nil {
		if err := deps.Combat.StopAttack(); err != nil {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}
	}
	return result
}

func (c *runPipeline) resetPostBossCleanup() {
	c.cleanupTargetUnitID = 0
	c.cleanupCastCount = 0
	c.cleanupNoTargetTicks = 0
	c.cleanupLastProgressAt = time.Time{}
	c.cleanupSkippedUnitIDs = nil
}

func (c *runPipeline) effectiveLootPickupDistance() float64 {
	if c.lootPickupDistanceTiles > 0 {
		return c.lootPickupDistanceTiles
	}
	return defaultLootPickupDistance
}

func (c *runPipeline) resetPostKillReposition() {
	c.postKillTeleportAttempts = 0
	c.postKillTeleportAt = time.Time{}
	c.postKillTeleportSnapshot = time.Time{}
}

func (c *runPipeline) resetLootApproach() {
	c.lootApproachTarget = LootTarget{}
	c.lootApproachTargetSet = false
	c.lootApproachAttempts = 0
	c.lootApproachAt = time.Time{}
	c.lootApproachSnapshot = time.Time{}
}

func (c *runPipeline) resetLootPickupRecovery() {
	c.lootPickupRecovered = nil
	c.clearLootRecoveryPending()
}

func (c *runPipeline) resetRouteLoot() {
	c.routeLootPointSet = false
	c.routeLootSegmentIndex = 0
	c.routeLootPointIndex = 0
	c.routeLootScanned = false
}

func (c *runPipeline) clearLootRecoveryPending() {
	c.lootRecoveryPending = false
	c.lootRecoveryTarget = LootTarget{}
	c.lootRecoveryTeleportSent = false
	c.lootRecoveryAt = time.Time{}
	c.lootRecoverySnapshot = time.Time{}
	c.lootRecoveryMaxDistance = 0
}

func (c *runPipeline) resetPortalEntryRecovery() {
	c.portalRecovered = nil
	c.clearPortalRecoveryPending()
}

func (c *runPipeline) clearPortalRecoveryPending() {
	c.portalRecoveryPending = false
	c.portalRecoveryUnitID = 0
	c.portalRecoveryPos = world.Position{}
	c.portalRecoveryTeleportSent = false
	c.portalRecoveryAt = time.Time{}
	c.portalRecoverySnapshot = time.Time{}
}

// beginPortalEntryRecovery arms one distance-ignoring portal teleport after too_far
// or hover_not_found, so Bone-Prison and similar blockers can be escaped once per UnitID.
func (c *runPipeline) beginPortalEntryRecovery(unitID uint32, pos world.Position) bool {
	if unitID == 0 || c.portalRecovered[unitID] {
		return false
	}
	if c.portalRecovered == nil {
		c.portalRecovered = make(map[uint32]bool)
	}
	c.portalRecovered[unitID] = true
	c.portalRecoveryPending = true
	c.portalRecoveryUnitID = unitID
	c.portalRecoveryPos = pos
	c.portalRecoveryTeleportSent = false
	c.portalRecoveryAt = time.Time{}
	c.portalRecoverySnapshot = time.Time{}
	return true
}

// tickPortalEntryRecovery teleports onto the failed portal once, settles, then retries entry.
func (c *runPipeline) tickPortalEntryRecovery(deps Deps, w world.State, now time.Time) stepResult {
	portal, found := w.NearestObject(world.ObjectKindTownPortal)
	if !found {
		c.clearPortalRecoveryPending()
		return stepResult{}
	}
	if c.portalRecoveryUnitID != 0 && portal.UnitID != c.portalRecoveryUnitID {
		// Prefer the armed UnitID when still present; otherwise follow the nearest portal.
		if match, ok := findTownPortalByUnitID(w, c.portalRecoveryUnitID); ok {
			portal = match
		}
	}
	c.portalRecoveryUnitID = portal.UnitID
	c.portalRecoveryPos = portal.Position
	if deps.Combat == nil {
		c.clearPortalRecoveryPending()
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if !c.portalRecoveryTeleportSent {
		if !lootRepositionReady(now, w.At, c.portalRecoveryAt, c.portalRecoverySnapshot) {
			return stepResult{}
		}
		sent, err := deps.Combat.TeleportToward(now, w.Player, portal.Position, 0)
		if err != nil {
			c.clearPortalRecoveryPending()
			return stepResult{failed: true, reason: "portal_recovery_teleport_failed"}
		}
		if sent {
			c.portalRecoveryTeleportSent = true
			c.portalRecoveryAt = now
			c.portalRecoverySnapshot = w.At
		}
		return stepResult{}
	}
	if !lootRepositionReady(now, w.At, c.portalRecoveryAt, c.portalRecoverySnapshot) {
		return stepResult{}
	}
	if deps.Portal != nil {
		deps.Portal.Reset()
	}
	c.clearPortalRecoveryPending()
	return stepResult{}
}

func findTownPortalByUnitID(state world.State, unitID uint32) (world.Object, bool) {
	for _, object := range state.Objects {
		if object.Kind == world.ObjectKindTownPortal && object.UnitID == unitID {
			return object, true
		}
	}
	return world.Object{}, false
}

// beginLootPickupRecovery arms one distance-ignoring item teleport after a
// recoverable pickup result. maxDistance keeps ordinary boss loot bounded while
// a threat-free combat-route Hold may recover any item it deliberately scanned.
func (c *runPipeline) beginLootPickupRecovery(deps Deps, target LootTarget, maxDistance float64) bool {
	if target.UnitID == 0 || c.lootPickupRecovered[target.UnitID] {
		return false
	}
	if c.lootPickupRecovered == nil {
		c.lootPickupRecovered = make(map[uint32]bool)
	}
	c.lootPickupRecovered[target.UnitID] = true
	if deps.Loot != nil {
		deps.Loot.ClearSkippedPickup(target.UnitID)
	}
	c.lootRecoveryPending = true
	c.lootRecoveryTarget = target
	c.lootRecoveryTeleportSent = false
	c.lootRecoveryAt = time.Time{}
	c.lootRecoverySnapshot = time.Time{}
	c.lootRecoveryMaxDistance = maxDistance
	return true
}

// tickLootPickupRecovery teleports onto the failed candidate once, settles, then restarts pickup.
func (c *runPipeline) tickLootPickupRecovery(deps Deps, w world.State, now time.Time) stepResult {
	target, found := currentLootTarget(w, c.lootRecoveryTarget)
	if !found {
		c.clearLootRecoveryPending()
		return stepResult{}
	}
	c.lootRecoveryTarget = target
	if deps.Combat == nil {
		c.clearLootRecoveryPending()
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if !c.lootRecoveryTeleportSent {
		maxDistance := c.lootRecoveryMaxDistance
		if maxDistance <= 0 {
			maxDistance = lootApproachMaxDistanceTiles
		}
		if world.Distance(w.Player.Position, target.Position) > maxDistance {
			// The caller-owned scan radius remains authoritative; recovery never
			// turns a rejected candidate into an unbounded multi-screen chase.
			c.clearLootRecoveryPending()
			return stepResult{}
		}
		if !lootRepositionReady(now, w.At, c.lootRecoveryAt, c.lootRecoverySnapshot) {
			return stepResult{}
		}
		sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
		if err != nil {
			c.clearLootRecoveryPending()
			return stepResult{failed: true, reason: "loot_recovery_teleport_failed"}
		}
		if sent {
			c.lootRecoveryTeleportSent = true
			c.lootRecoveryAt = now
			c.lootRecoverySnapshot = w.At
		}
		return stepResult{}
	}
	if !lootRepositionReady(now, w.At, c.lootRecoveryAt, c.lootRecoverySnapshot) {
		return stepResult{}
	}
	if err := deps.Loot.StartPickup(target); err != nil {
		c.clearLootRecoveryPending()
		return stepResult{failed: true, reason: "loot_pickup_start_failed"}
	}
	c.lootPickupActive = true
	c.clearLootRecoveryPending()
	return stepResult{}
}

// lootRepositionReady prevents a retry from reusing the snapshot that caused
// the previous cast. Both a newer Memory sample and the bounded retry interval
// are required before another input opportunity.
func lootRepositionReady(now, snapshotAt, lastAttemptAt, lastSnapshotAt time.Time) bool {
	if lastAttemptAt.IsZero() {
		return true
	}
	return snapshotAt.After(lastSnapshotAt) && now.Sub(lastAttemptAt) >= lootRepositionRetryDelay
}

func currentLootTarget(state world.State, frozen LootTarget) (LootTarget, bool) {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != frozen.AreaID {
		return LootTarget{}, false
	}
	for _, item := range state.Items {
		if item.UnitID != frozen.UnitID {
			continue
		}
		if item.Location != world.ItemLocationGround || item.TxtFileNo != frozen.TxtFileNo || item.Code != frozen.Code {
			return LootTarget{}, false
		}
		frozen.Position = item.Position
		return frozen, true
	}
	return LootTarget{}, false
}

func (c *runPipeline) onTravelTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if c.phase == RunPhasePlayRoute {
			if next, ok := c.runRouteResumeStep(w.Area.ID); ok {
				c.resumeAfterPrecheckSet = true
				c.resumeAfterPrecheck = next
				return stepResult{complete: true}
			}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		c.resumeAfterPrecheckSet = false
		c.resumeAfterPrecheck = ""
		return stepResult{complete: true}
	case pipelineStepAcquireTownWaypoint:
		if deps.Profile != nil {
			res := deps.Profile.TickHook(ctx, profile.HookTownReady, w, profile.EncounterTarget{}, now)
			switch res.Status {
			case profile.StatusFailed:
				return stepResult{failed: true, reason: res.Reason}
			case profile.StatusAction, profile.StatusPending:
				return stepResult{}
			}
		}
		if deps.TownWalk == nil {
			return stepResult{failed: true, reason: "town_walk_not_wired"}
		}
		res := deps.TownWalk.TickAct1Waypoint(ctx, w)
		switch res.Status {
		case pathing.TownWalkPending:
			return stepResult{}
		case pathing.TownWalkWaypointVisible, pathing.TownWalkArrived:
			return stepResult{complete: true}
		default:
			return stepResult{failed: true, reason: townWalkFailureReason(res)}
		}
	case pipelineStepOpenWaypoint:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		res := deps.Waypoint.TickTownWaypoint(ctx, w)
		switch res.Status {
		case pathing.WaypointActionPending:
			return stepResult{}
		case pathing.WaypointActionClicked:
			return stepResult{complete: true}
		default:
			return stepResult{failed: true, reason: waypointFailureReason(res)}
		}
	case pipelineStepSelectRunWaypoint:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		if now.Sub(stepStartedAt) < waypointSelectSettleDelay {
			return stepResult{}
		}
		return c.selectRunWaypoint(ctx, deps, w, now)
	case pipelineStepWaitEntryArea:
		if w.Valid && w.Area.ID == c.effectiveDefinition().EntryArea {
			return stepResult{complete: true}
		}
		// Waypoint travel can expose a short-lived `in_game` snapshot with
		// Area 0 while the destination room is loading. It contains no usable
		// position and must not be classified as a confirmed wrong area.
		if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID == world.None {
			return stepResult{}
		}
		if w.Valid && w.Phase == world.GamePhaseInGame && w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		return stepResult{}
	case pipelineStepPlayRoute:
		if deps.Route == nil {
			return stepResult{failed: true, reason: "route_playback_not_wired"}
		}
		if c.routeID == "" {
			return stepResult{failed: true, reason: "route_id_missing"}
		}
		if !c.routeStarted {
			if err := deps.Route.Start(c.routeID, w); err != nil {
				if errors.Is(err, pathing.ErrGameIdentityUnavailable) {
					return stepResult{}
				}
				return stepResult{failed: true, reason: "route_playback_start_failed"}
			}
			c.routeStarted = true
		}
		if c.routeCombat.Enabled && c.definition.HasCapability(RunCapabilityRouteClear) {
			if !w.Valid || w.Phase != world.GamePhaseInGame {
				return stepResult{}
			}
			progress, ok := deps.Route.Progress(w)
			if !ok {
				if c.routeProgressUnavailableSince.IsZero() {
					c.routeProgressUnavailableSince = now
					c.routeProgressUnavailableSnapshot = w.At
					return stepResult{}
				}
				// Repeated processing of the same snapshot must never consume the
				// grace. Only a newer authoritative read can prove persistence.
				if !w.At.After(c.routeProgressUnavailableSnapshot) {
					return stepResult{}
				}
				c.routeProgressUnavailableSnapshot = w.At
				if now.Sub(c.routeProgressUnavailableSince) < routeProgressUnavailableGrace {
					return stepResult{}
				}
				return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
			}
			c.resetRouteProgressUnavailable()
			assessment := assessThreats(w, progress, c.definition.RouteHostileNPCIDs, c.routeCombat)
			if observer, ok := deps.RouteClear.(routeClearObjectiveObserver); ok && observer.ObserveObjectiveProgress(w) {
				c.routeThreat.observeExternalProgress(now)
				c.cowNoProgressRecoveryStage = cowNoProgressStageNone
				c.cowNoProgressApproachUnitID = 0
			}
			terminalSafe := c.observeTerminalSafe(w, progress, assessment)
			c.routeThreat.SetTelemetry(deps.Telemetry)
			resourceContext := c.routeThreat.ObserveResources(w, assessment, c.routeCombat, now)
			if deps.Profile == nil {
				if resourceContext.MobilityCritical {
					return stepResult{failed: true, reason: string(RouteThreatReasonManaRecoveryFailed)}
				}
			} else {
				resource := deps.Profile.TickResources(w, resourceContext, now)
				switch resource.Status {
				case profile.StatusFailed:
					if resourceContext.MobilityCritical {
						return stepResult{failed: true, reason: string(RouteThreatReasonManaRecoveryFailed)}
					}
					return stepResult{failed: true, reason: resource.Reason}
				case profile.StatusAction:
					if err := c.routeThreat.ObserveResourceResult(w, progress, resourceContext, resource, now); err != nil {
						return stepResult{failed: true, reason: "telemetry_failed"}
					}
					return stepResult{}
				}
				if err := c.routeThreat.ObserveResourceResult(w, progress, resourceContext, resource, now); err != nil {
					return stepResult{failed: true, reason: "telemetry_failed"}
				}
				if resourceContext.MobilityCritical && strings.HasSuffix(resource.Reason, "_potion_unavailable") {
					return stepResult{failed: true, reason: string(RouteThreatReasonManaRecoveryFailed)}
				}
			}
			if deps.Profile != nil {
				maintenance := deps.Profile.TickRouteMaintenance(w, now)
				switch maintenance.Status {
				case profile.StatusFailed:
					return stepResult{failed: true, reason: maintenance.Reason}
				case profile.StatusAction, profile.StatusPending:
					return stepResult{}
				}
			}
			// A projection-driven approach remains authoritative until its fresh
			// snapshot/settle proof runs. A newly projectable cow must not make us
			// discard the pending movement first: the Cow executor uses that proof
			// to release the hold-local projection exclusions accumulated before
			// the teleport.
			if c.routeApproachPending && w.At.After(c.routeApproachSnapshotAt) && now.Sub(c.routeApproachSentAt) >= routeThreatApproachSettle {
				target, found := w.FindMonsterByUnitID(c.routeApproachTargetUnitID)
				if !found {
					target = world.Monster{UnitID: c.routeApproachTargetUnitID, Position: c.routeApproachGoal}
				}
				result := c.tickRouteThreatApproach(deps, w, progress, target, now)
				if result.failed || c.routeApproachPending {
					return result
				}
				// Keep movement and combat mutually exclusive for this snapshot. The
				// next fresh snapshot may select from the restored local Cow group.
				return stepResult{}
			}
			threat := c.routeThreat.Tick(ctx, deps.Route, deps.RouteClear, w, progress, assessment, c.definition, c.routeCombat, c.combat.Profile, now)
			if threat.Failed {
				if c.definition.ID == RunIDCows && threat.Reason == RouteThreatReasonCowNoProgress {
					return c.tickCowNoProgressRecovery(deps, w, progress, assessment, now)
				}
				cowApproach := c.definition.ID == RunIDCows && threat.Reason == RouteThreatReasonOutOfRange
				standardApproach := threat.Reason == RouteThreatReasonOutOfRange &&
					progress.Mode == RouteProgressMovement && progress.TargetAvailable
				if cowApproach || standardApproach {
					if threat.ApproachTarget.UnitID != 0 {
						return c.tickRouteThreatApproach(deps, w, progress, threat.ApproachTarget, now)
					}
					if assessment.RouteTargetFound {
						return c.tickRouteThreatApproach(deps, w, progress, assessment.RouteTarget, now)
					}
					if cowApproach && assessment.DensityTargetFound {
						return c.tickRouteThreatApproach(deps, w, progress, assessment.DensityTarget, now)
					}
				}
				if threat.Reason == RouteThreatReasonOutOfRange {
					// Out-of-range is a local recovery signal, not a run-level
					// emergency. If no bounded approach is permitted or remains,
					// hold without input until objective progress or the shared
					// no-progress watchdog makes the terminal decision.
					return stepResult{}
				}
				return stepResult{failed: true, reason: string(threat.Reason)}
			}
			if !c.routeApproachPending {
				c.resetRouteThreatApproach()
			}
			if c.routeThreat.State() == RouteThreatMoving {
				c.cowNoProgressRecoveryStage = cowNoProgressStageNone
				c.cowNoProgressApproachUnitID = 0
			}
			terminalCompletionReady := terminalSafe &&
				c.terminalSafeSnapshots >= Phase17StableClearSnapshots &&
				c.routeThreat.State() == RouteThreatMoving
			// Transition progress has no positional input. The third proven-safe
			// snapshot may therefore let Route.Tick commit the terminal marker even
			// when the controller deliberately withholds regular movement this tick.
			if !threat.AllowMovement && !terminalCompletionReady {
				// A combat hold may create fresh drops at the current point.
				// Re-evaluate them only after the threat controller releases movement.
				c.routeLootScanned = false
				return stepResult{}
			}
			if terminalSafe && c.terminalSafeSnapshots < Phase17StableClearSnapshots {
				if err := deps.Route.Hold(w); err != nil {
					return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
				}
				return stepResult{}
			}
			if !c.suppressRouteLoot {
				if handled, result := c.tickRouteLoot(deps, w, progress, now); handled {
					return result
				}
			}
		}
		done, err := deps.Route.Tick(ctx, w)
		if err != nil {
			return stepResult{failed: true, reason: routePlaybackFailureReason(err)}
		}
		if done {
			c.routeThreat.Reset(deps.RouteClear)
			c.resetTerminalSafe()
			return stepResult{complete: true}
		}
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) observeTerminalSafe(state world.State, progress RouteProgress, assessment ThreatAssessment) bool {
	if !c.requireTerminalSafe || progress.Mode != RouteProgressTransition {
		c.resetTerminalSafe()
		return false
	}
	safe := state.Valid && state.Phase == world.GamePhaseInGame && assessment.SnapshotAt.Equal(state.At) &&
		assessment.CoverageComplete && !assessment.RouteTargetFound && !assessment.DensityTargetFound
	if !safe {
		c.resetTerminalSafe()
		return true
	}
	if c.terminalSafeSnapshotAt.IsZero() || state.At.After(c.terminalSafeSnapshotAt) {
		c.terminalSafeSnapshots++
		c.terminalSafeSnapshotAt = state.At
	}
	return true
}

func (c *runPipeline) resetTerminalSafe() {
	c.terminalSafeSnapshots = 0
	c.terminalSafeSnapshotAt = time.Time{}
}

func (c *runPipeline) resetRouteThreatApproach() {
	c.routeApproachTargetUnitID = 0
	c.routeApproachOrigin = world.Position{}
	c.routeApproachGoal = world.Position{}
	c.routeApproachSentAt = time.Time{}
	c.routeApproachSnapshotAt = time.Time{}
	c.routeApproachPending = false
	c.routeApproachFailures = 0
	c.routeApproachExhaustedUnitID = 0
}

func (c *runPipeline) resetRouteProgressUnavailable() {
	c.routeProgressUnavailableSince = time.Time{}
	c.routeProgressUnavailableSnapshot = time.Time{}
}

// tickCowNoProgressRecovery stages retarget → approach → soft exit when the Cow
// objective watchdog expires without corpses, kills, or coverage progress.
// Soft exit keeps reason `cow_combat_no_progress` so the queue consumes its
// normal retry-return / restart budget.
func (c *runPipeline) tickCowNoProgressRecovery(
	deps Deps,
	w world.State,
	progress RouteProgress,
	assessment ThreatAssessment,
	now time.Time,
) stepResult {
	switch c.cowNoProgressRecoveryStage {
	case cowNoProgressStageNone:
		currentUnitID := uint32(0)
		if assessment.RouteTargetFound {
			currentUnitID = assessment.RouteTarget.UnitID
		} else if assessment.DensityTargetFound {
			currentUnitID = assessment.DensityTarget.UnitID
		}
		c.cowNoProgressApproachUnitID = currentUnitID
		if observer, ok := deps.RouteClear.(routeClearNoProgressRetargetObserver); ok {
			if selected, found := observer.ObserveNoProgressRetarget(currentUnitID); found {
				c.cowNoProgressApproachUnitID = selected.UnitID
			}
		}
		c.routeApproachExhaustedUnitID = 0
		c.routeApproachFailures = 0
		c.routeApproachPending = false
		c.routeThreat.observeExternalProgress(now)
		c.cowNoProgressRecoveryStage = cowNoProgressStageRetargeted
		return stepResult{}
	case cowNoProgressStageRetargeted:
		target := world.Monster{}
		if c.cowNoProgressApproachUnitID != 0 {
			if selected, found := w.FindMonsterByUnitID(c.cowNoProgressApproachUnitID); found {
				target = selected
			}
		}
		if target.UnitID == 0 && assessment.RouteTargetFound {
			target = assessment.RouteTarget
		}
		if target.UnitID == 0 && assessment.DensityTargetFound {
			target = assessment.DensityTarget
		}
		c.routeApproachExhaustedUnitID = 0
		c.routeApproachFailures = 0
		c.routeThreat.observeExternalProgress(now)
		c.cowNoProgressRecoveryStage = cowNoProgressStageApproached
		if target.UnitID == 0 {
			return stepResult{}
		}
		return c.tickRouteThreatApproach(deps, w, progress, target, now)
	default:
		return stepResult{failed: true, reason: string(RouteThreatReasonCowNoProgress)}
	}
}

// tickRouteThreatApproach keeps Summoner on the already validated next route
// point. Cow instead reuses the bounded projection-driven combat teleport
// toward the executor-pinned group member, so recovery cannot walk past the
// blocked pack. Neither path calls Route.Tick.
func (c *runPipeline) tickRouteThreatApproach(deps Deps, w world.State, progress RouteProgress, target world.Monster, now time.Time) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if c.routeApproachTargetUnitID != target.UnitID {
		c.resetRouteThreatApproach()
		c.routeApproachTargetUnitID = target.UnitID
	}
	if c.routeApproachPending {
		if !w.At.After(c.routeApproachSnapshotAt) || now.Sub(c.routeApproachSentAt) < routeThreatApproachSettle {
			return stepResult{}
		}
		positionProgress := routeApproachDirectionalProgress(c.routeApproachOrigin, c.routeApproachGoal, w.Player.Position)
		if positionProgress > routeThreatApproachProgressEpsilonTiles {
			c.routeApproachFailures = 0
			c.routeApproachExhaustedUnitID = 0
			if err := c.routeThreat.ObserveApproachProgress(w, progress, target, positionProgress, now); err != nil {
				return stepResult{failed: true, reason: "telemetry_failed"}
			}
			if observer, ok := deps.RouteClear.(routeClearApproachObserver); ok {
				observer.ObserveRouteClearApproachProgress()
			}
			c.routeApproachPending = false
			return stepResult{}
		}
		if err := c.routeThreat.ObserveApproachNoProgress(w, progress, target, positionProgress, now); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		c.routeApproachFailures++
		c.routeApproachPending = false
		if c.routeApproachFailures >= routeThreatApproachMaxFailures {
			c.routeApproachExhaustedUnitID = target.UnitID
			return stepResult{}
		}
	}
	if c.routeApproachExhaustedUnitID == target.UnitID {
		return stepResult{}
	}
	goal := progress.MovementTarget
	sent := false
	actionKind := "force_move"
	var err error
	if c.definition.ID == RunIDCows {
		landing, desiredDistance, projectable := deps.Combat.FarthestProjectableMonsterApproach(w.Player.Position, target.Position)
		if !projectable {
			c.routeApproachExhaustedUnitID = target.UnitID
			return stepResult{}
		}
		if !cowApproachLandingSafe(w, landing, c.definition.RouteHostileNPCIDs, c.routeCombat.LandingRadiusTiles) {
			// Projection only proves that the selected monster can be aimed at. A
			// ranged Cow approach additionally needs complete, pack-wide clearance.
			c.routeApproachExhaustedUnitID = target.UnitID
			return stepResult{}
		}
		goal = target.Position
		actionKind = "teleport"
		sent, err = deps.Combat.TeleportToward(now, w.Player, target.Position, desiredDistance)
	} else {
		sent, err = deps.Combat.ForceMoveToward(now, w.Player.Position, progress.MovementTarget)
	}
	if err != nil {
		return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
	}
	if sent {
		c.routeApproachOrigin = w.Player.Position
		c.routeApproachGoal = goal
		c.routeApproachSentAt = now
		c.routeApproachSnapshotAt = w.At
		c.routeApproachPending = true
		if err := c.routeThreat.ObserveApproachInput(w, progress, target, c.routeApproachFailures+1, actionKind, now); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
	}
	return stepResult{}
}

func cowApproachLandingSafe(state world.State, landing world.Position, allowedNPCIDs []uint32, clearanceTiles float64) bool {
	requiredCoverage := world.Distance(state.Player.Position, landing) + clearanceTiles
	if state.MonsterCoverage.MonstersTruncated && state.MonsterCoverage.MonsterCoverageRadiusTiles <= requiredCoverage {
		return false
	}
	clearanceSquared := clearanceTiles * clearanceTiles
	for _, monster := range state.Monsters {
		if monster.UnitID == 0 || !routeHostileAllowed(monster.NPCID, allowedNPCIDs) {
			continue
		}
		if positionDistanceSquared(landing, monster.Position) < clearanceSquared {
			return false
		}
	}
	return true
}

func routeApproachDirectionalProgress(origin, goal, current world.Position) float64 {
	directionX := float64(goal.X) - float64(origin.X)
	directionY := float64(goal.Y) - float64(origin.Y)
	directionLength := math.Hypot(directionX, directionY)
	if directionLength == 0 {
		return 0
	}
	movementX := float64(current.X) - float64(origin.X)
	movementY := float64(current.Y) - float64(origin.Y)
	return (movementX*directionX + movementY*directionY) / directionLength
}

// tickRouteLoot opportunistically consumes every nearby `keep` match before
// the next route input. The caller has already proved a fresh, threat-free
// route snapshot; every input path holds route ownership for the whole tick.
func (c *runPipeline) tickRouteLoot(deps Deps, w world.State, progress RouteProgress, now time.Time) (bool, stepResult) {
	if deps.Loot == nil || progress.Mode == RouteProgressTransition {
		return false, stepResult{}
	}
	if !c.routeLootPointSet ||
		c.routeLootSegmentIndex != progress.SegmentIndex ||
		c.routeLootPointIndex != progress.PointIndex {
		c.routeLootPointSet = true
		c.routeLootSegmentIndex = progress.SegmentIndex
		c.routeLootPointIndex = progress.PointIndex
		c.routeLootScanned = false
		c.lootPickupActive = false
		c.resetLootApproach()
		c.clearLootRecoveryPending()
	}

	if c.lootPickupActive {
		if err := deps.Route.Hold(w); err != nil {
			return true, stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
		}
		return true, c.tickRouteLootPickup(deps, w, now)
	}
	if c.lootRecoveryPending {
		if err := deps.Route.Hold(w); err != nil {
			return true, stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
		}
		result := c.tickLootPickupRecovery(deps, w, now)
		return true, result
	}
	if c.routeLootScanned {
		return false, stepResult{}
	}

	scan := deps.Loot.ScanRouteKeep(w, routeLootRadiusTiles)
	if scan.TelemetryFailed {
		return true, stepResult{failed: true, reason: "telemetry_failed"}
	}
	// A large keep item may not fit while another nearby keep candidate still
	// does. Exhaust every actionable target before treating the point as done.
	if !scan.HasTarget {
		c.routeLootScanned = true
		return false, stepResult{}
	}
	c.lootApproachTarget = scan.NextTarget
	c.lootApproachTargetSet = true

	if err := deps.Route.Hold(w); err != nil {
		return true, stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
	}
	target := c.lootApproachTarget
	if world.Distance(w.Player.Position, target.Position) > c.effectiveLootPickupDistance() {
		if deps.Combat == nil {
			return true, stepResult{failed: true, reason: "combat_not_wired"}
		}
		if world.Distance(w.Player.Position, target.Position) <= lootApproachMaxDistanceTiles &&
			c.lootApproachAttempts < lootRepositionMaxAttempts {
			if !lootRepositionReady(now, w.At, c.lootApproachAt, c.lootApproachSnapshot) {
				return true, stepResult{}
			}
			sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
			if err != nil {
				return true, stepResult{failed: true, reason: "loot_reposition_failed"}
			}
			if sent {
				c.lootApproachAttempts++
				c.lootApproachAt = now
				c.lootApproachSnapshot = w.At
			}
			return true, stepResult{}
		}
		if c.lootApproachAttempts > 0 && !lootRepositionReady(now, w.At, c.lootApproachAt, c.lootApproachSnapshot) {
			return true, stepResult{}
		}
	}
	if err := deps.Loot.StartPickup(target); err != nil {
		return true, stepResult{failed: true, reason: "loot_pickup_start_failed"}
	}
	c.lootPickupActive = true
	c.resetLootApproach()
	return true, c.tickRouteLootPickup(deps, w, now)
}

func (c *runPipeline) tickRouteLootPickup(deps Deps, w world.State, now time.Time) stepResult {
	result := deps.Loot.TickPickup(w, now)
	if !result.Done {
		return stepResult{}
	}
	c.lootPickupActive = false
	c.resetLootApproach()
	c.routeLootScanned = false
	switch result.Status {
	case LootPickupHoverNotFound, LootPickupFailed, LootPickupTooFar:
		// Route loot is selected only after a threat-free Hold and already lies
		// inside routeLootRadiusTiles. Reuse the existing one-shot recovery for a
		// stale distance verdict instead of permanently skipping a wanted drop.
		if c.beginLootPickupRecovery(deps, result.Target, routeLootRadiusTiles) {
			return stepResult{}
		}
		return stepResult{}
	case LootPickupPickedUp, LootPickupMonsterNearby,
		LootPickupTargetLost, LootPickupTargetUnstable:
		return stepResult{}
	case LootPickupInputBlocked, LootPickupProjectionFailed, LootPickupInvalidWorld, LootPickupTelemetryFailed:
		return stepResult{failed: true, reason: string(result.Status)}
	default:
		return stepResult{failed: true, reason: "loot_pickup_failed"}
	}
}

func routePlaybackFailureReason(err error) string {
	switch {
	case errors.Is(err, pathing.ErrRouteHardStuck):
		return "hard_stuck"
	case errors.Is(err, pathing.ErrRouteDriftExceeded):
		return "route_drift_exceeded"
	case errors.Is(err, pathing.ErrRouteTransitionFailed):
		return "route_transition_failed"
	case errors.Is(err, pathing.ErrRouteSegmentTimeout):
		return "route_segment_timeout"
	case errors.Is(err, pathing.ErrRouteUnexpectedArea):
		return "unexpected_area"
	default:
		return "route_playback_failed"
	}
}

func (c *runPipeline) runRouteResumeStep(area world.AreaID) (string, bool) {
	definition := c.effectiveDefinition()
	switch area {
	case definition.EntryArea:
		return pipelineStepPlayRoute, true
	case definition.RouteTerminalArea:
		return "", true
	default:
		return "", false
	}
}

func (c *runPipeline) isTravelPhase() bool {
	return c.phase == RunPhaseTravelEntry || c.phase == RunPhasePlayRoute
}

func (c *runPipeline) selectRunWaypoint(ctx context.Context, deps Deps, state world.State, now time.Time) stepResult {
	res := deps.Waypoint.SelectWaypointTarget(ctx, state, c.effectiveDefinition().WaypointTarget, now)
	switch res.Status {
	case pathing.WaypointActionPending:
		return stepResult{}
	case pathing.WaypointActionClicked:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: waypointFailureReason(res)}
	}
}

func pathingStartFailureReason(err error) string {
	if errors.Is(err, pathing.ErrNavigatorNotWired) {
		return "pathing_not_wired"
	}
	if strings.Contains(err.Error(), pathing.ReasonInvalidGoal) {
		return pathing.ReasonInvalidGoal
	}
	return "pathing_start_failed"
}

func navigatorFailureReason(res pathing.NavTickResult) string {
	if res.Reason != "" {
		return res.Reason
	}
	return string(res.Status)
}

func waypointFailureReason(res pathing.WaypointActionResult) string {
	if res.Reason != "" {
		return res.Reason
	}
	return string(res.Status)
}

func townWalkFailureReason(res pathing.TownWalkResult) string {
	if res.Reason != "" {
		return res.Reason
	}
	return string(res.Status)
}

type stepResult struct {
	complete bool
	failed   bool
	reason   string
}
