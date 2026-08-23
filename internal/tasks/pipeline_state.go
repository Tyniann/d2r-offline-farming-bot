package tasks

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// runPipeline executes one immutable run definition or a thin isolated-phase alias.
// Persistent executor state belongs to this generation and is cleared at the
// runner's central reset barrier before another generation may start.
type runPipeline struct {
	definition RunDefinition
	phase      string

	core   pipelineCoreState
	travel pipelineTravelState
	boss   pipelineBossState
	chest  pipelineChestState
	loot   pipelineLootState
	ret    pipelineReturnState
}

// pipelineCoreState freezes immutable configuration for one runner generation.
type pipelineCoreState struct {
	routeID                 string
	combat                  CombatConfig
	routeCombat             RouteCombatConfig
	lootPickupDistanceTiles float64
	suppressRouteLoot       bool
	requireTerminalSafe     bool
}

// pipelineTravelState owns state that may survive multiple travel or route ticks.
type pipelineTravelState struct {
	routeThreat            RouteThreatController
	navStarted             bool
	resumeAfterPrecheckSet bool
	resumeAfterPrecheck    string
	routeStarted           bool
	fieldReadyComplete     bool
	// routeProgressUnavailableSince guards transient read-side projection loss
	// without letting an unavailable RoutePlayer stall the run indefinitely.
	routeProgressUnavailableSince    time.Time
	routeProgressUnavailableSnapshot time.Time
	routeLootPointSet                bool
	routeLootSegmentIndex            int
	routeLootPointIndex              int
	routeLootScanned                 bool
	routeApproachTargetUnitID        uint32
	routeApproachOrigin              world.Position
	routeApproachGoal                world.Position
	routeApproachSentAt              time.Time
	routeApproachSnapshotAt          time.Time
	routeApproachPending             bool
	routeApproachFailures            int
	// routeApproachHammerdinReposition marks a fallback teleport toward another
	// monster while the route controller keeps the previous attack target pinned.
	routeApproachHammerdinReposition bool
	// routeApproachHammerdinRouteForward distinguishes the bounded next-route
	// fallback from a teleport toward another monster.
	routeApproachHammerdinRouteForward bool
	// routeApproachExhaustedUnitID suppresses further local movement for one
	// blocker after the bounded attempts. The shared no-progress watchdog, not
	// this low-level targeting inconvenience, owns any later run termination.
	routeApproachExhaustedUnitID uint32
	// cowNoProgressRecoveryStage stages Cow soft-exit: retarget → approach → fail.
	// Real objective progress resets it so a later stall may recover again.
	cowNoProgressRecoveryStage  int
	cowNoProgressApproachUnitID uint32
	terminalSafeSnapshots       int
	terminalSafeSnapshotAt      time.Time
	// entryArrive* confirms wait_entry_area against a premature destination ID
	// during the waypoint load fade, not a single matching area snapshot.
	entryArriveAt         time.Time
	entryArriveSnapshots  int
	entryArriveSnapshotAt time.Time
}

type chestOperatePhase string

const (
	chestPhaseIdle         chestOperatePhase = ""
	chestPhaseClick        chestOperatePhase = "click"
	chestPhaseClearBlocker chestOperatePhase = "clear_blocker"
	chestPhaseSettle       chestOperatePhase = "settle"
	chestPhaseWaitDrops    chestOperatePhase = "wait_drops"
	chestPhasePickup       chestOperatePhase = "pickup"
)

// pipelineChestState owns one Lower-Kurast object workflow.
//
// Transitions:
//
//   - idle -> click -> settle -> wait_drops -> pickup -> idle is the success path.
//   - click or pickup -> clear_blocker -> saved phase is the one-shot local recovery.
//   - exhausted approach, hover or settle -> skipped UnitID -> idle is the abandon path.
//
// The pin, cluster anchor, click evidence and clear budgets persist across
// ticks. opened and skipped are terminal UnitID sets; only hover misses receive
// the explicit leftover-sweep retry. A known closed object gets at most two
// clicks, an unknown Mode exactly one. Mode, drop or key-consumption evidence
// starts the success path. Local clear, approach, settle and pickup have finite
// budgets; a clear timeout resumes once, so another blocker miss is abandoned.
type pipelineChestState struct {
	skipped            map[uint32]bool
	hoverMissed        map[uint32]bool
	leftoverHoverRetry bool
	opened             map[uint32]bool
	seenEligible       bool
	openedSuperChests  int
	phase              chestOperatePhase
	pin                world.Object
	clusterChest       world.Object
	clicksOnPin        int
	keysAtClick        int
	groundAtClick      map[uint32]bool
	settleTicks        int
	dropWaitTicks      int
	lootNoTargetTicks  int
	approachAttempts   int
	approachAt         time.Time
	approachSnapshot   time.Time
	clusterActive      bool
	blockerUnitID      uint32
	clearAttempted     map[uint32]bool
	clearResume        chestOperatePhase
	clear              localThreatClear
}

// pipelineBossState owns boss identity, encounter, approach, and cleanup state.
type pipelineBossState struct {
	chestFallbackStarted       bool
	targetSeen                 bool
	targetUnitID               uint32
	targetPosition             world.Position
	targetPositionSet          bool
	targetAbsentTicks          int
	encounterActionIndex       int
	encounterActionStarted     bool
	bossKillEmitted            bool
	bossApproachPending        bool
	bossApproachAttempted      bool
	bossApproachAt             time.Time
	bossApproachSnapshot       time.Time
	nihlathakAimUnitID         uint32
	nihlathakAimPlayerPosition world.Position
	nihlathakAimTargetPosition world.Position
	nihlathakAimSnapshot       time.Time
	cleanupTargetUnitID        uint32
	cleanupCastCount           int
	cleanupNoTargetTicks       int
	cleanupLastProgressAt      time.Time
	// cleanupSkippedUnitIDs prevents an unprojectable hostile from pinning the
	// best-effort Nihlathak cleanup while another nearby target remains usable.
	cleanupSkippedUnitIDs map[uint32]bool
	// engageStartedAt and engageLastProgressAt bound Hammerdin boss combat so
	// a missed teleport or unconfirmed hold cannot stall forever.
	engageStartedAt      time.Time
	engageLastProgressAt time.Time
	engageTeleportCount  int
	// hammerdinAttackHeld is true while LMB stays down on the boss.
	hammerdinAttackHeld bool
	// hammerdinHoldSnapshots counts World snapshots since the last distance
	// re-check while the hold is down.
	hammerdinHoldSnapshots int
	hammerdinHoldStartedAt time.Time
	// hammerdinRepositionPending survives Teleport skill selection and waits
	// for a fresh snapshot after a sent teleport toward another monster.
	hammerdinRepositionPending      bool
	hammerdinRepositionSent         bool
	hammerdinRepositionReady        bool
	hammerdinRepositionTargetUnitID uint32
	hammerdinRepositionOrigin       world.Position
	hammerdinRepositionAt           time.Time
	hammerdinRepositionSnapshot     time.Time
}

// pipelineLootState owns drop, pickup, reposition, and bounded recovery state.
type pipelineLootState struct {
	dropStableTicks          int
	lootScanHasTarget        bool
	lootPickupActive         bool
	lootNoTargetTicks        int
	postKillTeleportAttempts int
	postKillTeleportAt       time.Time
	postKillTeleportSnapshot time.Time
	lootApproachTarget       LootTarget
	lootApproachTargetSet    bool
	lootApproachAttempts     int
	lootApproachAt           time.Time
	lootApproachSnapshot     time.Time
	// lootPickupRecovered bounds post-fail item teleports to one attempt per UnitID.
	lootPickupRecovered      map[uint32]bool
	lootRecoveryPending      bool
	lootRecoveryTarget       LootTarget
	lootRecoveryTeleportSent bool
	lootRecoveryAt           time.Time
	lootRecoverySnapshot     time.Time
	lootRecoveryMaxDistance  float64
}

// pipelineReturnState owns foreign-town egress and bounded portal recovery.
type pipelineReturnState struct {
	egressStarted bool
	// recoveryArea* prevents retry-return input during a waypoint/load transition.
	// The destination must remain the same across fresh snapshots for the full
	// load-fade settle before a Town Portal request is allowed.
	recoveryAreaID         world.AreaID
	recoveryAreaStartedAt  time.Time
	recoveryAreaSnapshots  int
	recoveryAreaGeneration uint64
	recoveryAreaSnapshotAt time.Time
	// portalRecovered bounds post-fail portal teleports to one attempt per portal UnitID.
	portalRecovered            map[uint32]bool
	portalRecoveryPending      bool
	portalRecoveryUnitID       uint32
	portalRecoveryPos          world.Position
	portalRecoveryTeleportSent bool
	portalRecoveryAt           time.Time
	portalRecoverySnapshot     time.Time
	// destination* verifies that a sent portal click actually changed Area.
	// One failed confirmation may clear local threats, teleport to the pinned
	// portal, and retry the hover-confirmed click without extending the step.
	destinationPhase          portalDestinationPhase
	destinationPortalUnitID   uint32
	destinationPortalPos      world.Position
	destinationObservedAt     time.Time
	destinationLastSnapshotAt time.Time
	destinationSnapshots      int
	destinationRecoveryUsed   bool
	destinationClear          localThreatClear
	destinationTeleportAt     time.Time
	destinationTeleportSnap   time.Time
	destinationClearActions   int
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

func (c *runPipeline) resetGeneration() {
	c.travel.navStarted = false
	c.travel.resumeAfterPrecheckSet = false
	c.travel.resumeAfterPrecheck = ""
	c.boss.chestFallbackStarted = false
	c.boss.targetSeen = false
	c.boss.targetUnitID = 0
	c.boss.targetPosition = world.Position{}
	c.boss.targetPositionSet = false
	c.boss.targetAbsentTicks = 0
	c.loot.dropStableTicks = 0
	c.loot.lootScanHasTarget = false
	c.loot.lootPickupActive = false
	c.loot.lootNoTargetTicks = 0
	c.travel.routeStarted = false
	c.travel.fieldReadyComplete = false
	c.resetRouteProgressUnavailable()
	c.resetEntryArrival()
	c.travel.routeThreat.Reset(nil)
	c.ret.egressStarted = false
	c.resetRecoveryAreaArrival()
	c.boss.encounterActionIndex = 0
	c.boss.encounterActionStarted = false
	c.boss.bossKillEmitted = false
	c.resetBossApproach()
	c.resetHammerdinEngage()
	c.resetPostBossCleanup()
	c.resetPostKillReposition()
	c.resetLootApproach()
	c.resetLootPickupRecovery()
	c.resetRouteLoot()
	c.resetRouteThreatApproach()
	c.travel.cowNoProgressRecoveryStage = cowNoProgressStageNone
	c.travel.cowNoProgressApproachUnitID = 0
	c.resetPortalEntryRecovery()
	c.resetPortalDestinationRecovery()
	c.resetTerminalSafe()
	c.resetChestWork()
}

func (c *runPipeline) onStepEnter(step string) {
	c.travel.navStarted = false
	c.travel.routeStarted = false
	c.resetRouteProgressUnavailable()
	if step == pipelineStepWaitEntryArea {
		c.resetEntryArrival()
	}
	if step == pipelineStepWaitRecoveryArea {
		c.resetRecoveryAreaArrival()
	}
	if step == pipelineStepCastTownPortal {
		c.resetPortalDestinationRecovery()
	}
	c.ret.egressStarted = false
	if step == pipelineStepClearNearbyHostiles {
		c.resetPostBossCleanup()
	}
	if step == pipelineStepRepositionForLoot {
		c.resetPostKillReposition()
	}
	if step == pipelineStepWaitForDrops {
		c.loot.dropStableTicks = 0
	}
	if step == pipelineStepScanLoot {
		c.loot.lootScanHasTarget = false
		c.loot.lootNoTargetTicks = 0
	}
	if step == pipelineStepPickLoot {
		c.loot.lootPickupActive = false
		c.loot.lootNoTargetTicks = 0
		c.resetLootApproach()
	}
	if step == pipelineStepAcquireBoss {
		c.resetRouteLoot()
		c.resetBossApproach()
		c.resetHammerdinEngage()
		c.boss.chestFallbackStarted = false
		c.boss.targetSeen = false
		c.boss.targetUnitID = 0
		c.boss.targetPosition = world.Position{}
		c.boss.targetPositionSet = false
		c.boss.targetAbsentTicks = 0
		c.boss.encounterActionIndex = 0
		c.boss.encounterActionStarted = false
	}
	if step == pipelineStepPlayRoute {
		c.resetTerminalSafe()
	}
}

type stepResult struct {
	complete bool
	failed   bool
	reason   string
}
