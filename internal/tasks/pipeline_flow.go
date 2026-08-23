package tasks

import (
	"time"
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
	pipelineStepChestSweep          = "chest_sweep"
	pipelineStepAcquireBoss         = "acquire_boss"
	pipelineStepEngageBoss          = "engage_boss"
	pipelineStepClearNearbyHostiles = "clear_nearby_hostiles"
	pipelineStepRepositionForLoot   = "reposition_for_loot"
	pipelineStepWaitForDrops        = "wait_for_drops"
	pipelineStepScanLoot            = "scan_loot"
	pipelineStepPickLoot            = "pick_loot"
	pipelineStepWaitRecoveryArea    = "wait_recovery_area"
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
	// Memory can report the destination area, and even InGame, while the
	// waypoint load fade is still up. Require a settled InGame arrival before
	// field-ready or route input.
	entryAreaArriveSettle     = 3 * time.Second
	entryAreaArriveSnapshots  = 3
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

func (c *runPipeline) effectiveDefinition() RunDefinition {
	return c.definition
}

func (c *runPipeline) handlesResources(step string) bool {
	return step == pipelineStepPlayRoute &&
		c.core.routeCombat.Enabled &&
		c.definition.HasCapability(RunCapabilityRouteClear)
}

func (c *runPipeline) firstStep() string {
	return pipelineStepPrecheck
}

// nextSharedPortalReturn returns the successor inside the shared town-portal
// subsequence. Branch points after wait_origin_town stay in nextStep.
func nextSharedPortalReturn(current string) string {
	switch current {
	case pipelineStepCastTownPortal:
		return pipelineStepEnterTownPortal
	case pipelineStepEnterTownPortal:
		return pipelineStepWaitOriginTown
	default:
		return ""
	}
}

// nextSharedForeignEgress returns the successor inside the shared foreign-town
// egress subsequence. wait_hub_area stays a phase-specific branch.
func nextSharedForeignEgress(current string) string {
	switch current {
	case pipelineStepPlayTownEgress:
		return pipelineStepOpenOriginWaypoint
	case pipelineStepOpenOriginWaypoint:
		return pipelineStepSelectHubWaypoint
	case pipelineStepSelectHubWaypoint:
		return pipelineStepWaitHubArea
	default:
		return ""
	}
}

// nextSharedWaypointTravel returns the successor inside the shared Act-1
// waypoint subsequence. precheck and wait_entry_area stay phase-specific.
func nextSharedWaypointTravel(current string) string {
	switch current {
	case pipelineStepAcquireTownWaypoint:
		return pipelineStepOpenWaypoint
	case pipelineStepOpenWaypoint:
		return pipelineStepSelectRunWaypoint
	case pipelineStepSelectRunWaypoint:
		return pipelineStepWaitEntryArea
	default:
		return ""
	}
}

// nextSharedStash returns the successor inside the shared personal-stash
// subsequence. close_personal_stash stays a phase-specific branch.
func nextSharedStash(current string) string {
	switch current {
	case pipelineStepOpenStash:
		return pipelineStepStashItems
	case pipelineStepStashItems:
		return pipelineStepCloseStash
	default:
		return ""
	}
}

// shouldClearNearbyAfterBoss reports whether the current profile should run
// the definition's post-boss cleanup. Hammerdin skips it: Blessed Hammer is
// an AOE and already clears most nearby hostiles during the engage.
func (c *runPipeline) shouldClearNearbyAfterBoss() bool {
	return c.effectiveDefinition().ClearNearbyAfterBoss && !c.hammerdinBossCombat()
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
		case pipelineStepOpenStash, pipelineStepStashItems:
			return nextSharedStash(current)
		case pipelineStepCloseStash:
			return pipelineStepComplete
		case pipelineStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.phase == RunPhaseBoss {
		if c.definition.HasCapability(RunCapabilityChestSweep) {
			switch current {
			case pipelineStepPrecheck:
				return pipelineStepChestSweep
			case pipelineStepChestSweep, pipelineStepComplete:
				return ""
			default:
				return ""
			}
		}
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
			if c.loot.lootScanHasTarget {
				return pipelineStepPickLoot
			}
			return pipelineStepCastTownPortal
		case pipelineStepPickLoot:
			return pipelineStepCastTownPortal
		case pipelineStepCastTownPortal, pipelineStepEnterTownPortal:
			return nextSharedPortalReturn(current)
		case pipelineStepWaitOriginTown:
			if foreignTownOrigin(c.effectiveDefinition().ReturnOrigin) {
				return pipelineStepPlayTownEgress
			}
			return pipelineStepOpenStash
		case pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint:
			return nextSharedForeignEgress(current)
		case pipelineStepWaitHubArea:
			return pipelineStepOpenStash
		case pipelineStepOpenStash, pipelineStepStashItems:
			return nextSharedStash(current)
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
			return pipelineStepWaitRecoveryArea
		case pipelineStepWaitRecoveryArea:
			return pipelineStepCastTownPortal
		case pipelineStepCastTownPortal, pipelineStepEnterTownPortal:
			return nextSharedPortalReturn(current)
		case pipelineStepWaitOriginTown:
			if foreignTownOrigin(c.effectiveDefinition().ReturnOrigin) {
				return pipelineStepPlayTownEgress
			}
			return pipelineStepComplete
		case pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint:
			return nextSharedForeignEgress(current)
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
			if c.travel.resumeAfterPrecheckSet {
				return c.travel.resumeAfterPrecheck
			}
			return pipelineStepAcquireTownWaypoint
		case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
			return nextSharedWaypointTravel(current)
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
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return nextSharedWaypointTravel(current)
	case pipelineStepWaitEntryArea:
		return pipelineStepPlayRoute
	case pipelineStepPlayRoute:
		if c.definition.HasCapability(RunCapabilityChestSweep) {
			return pipelineStepChestSweep
		}
		return pipelineStepAcquireBoss
	case pipelineStepChestSweep:
		return pipelineStepWaitForDrops
	case pipelineStepAcquireBoss:
		if c.boss.bossKillEmitted {
			if c.shouldClearNearbyAfterBoss() {
				return pipelineStepClearNearbyHostiles
			}
			return pipelineStepRepositionForLoot
		}
		return pipelineStepEngageBoss
	case pipelineStepEngageBoss:
		if c.shouldClearNearbyAfterBoss() {
			return pipelineStepClearNearbyHostiles
		}

		return pipelineStepRepositionForLoot
	case pipelineStepRepositionForLoot:
		return pipelineStepWaitForDrops
	case pipelineStepClearNearbyHostiles:
		return pipelineStepRepositionForLoot
	case pipelineStepWaitForDrops:
		return pipelineStepScanLoot
	case pipelineStepScanLoot:
		if c.loot.lootScanHasTarget {
			return pipelineStepPickLoot
		}
		return pipelineStepCastTownPortal
	case pipelineStepPickLoot:
		return pipelineStepCastTownPortal
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal:
		return nextSharedPortalReturn(current)
	case pipelineStepWaitOriginTown:
		if foreignTownOrigin(c.effectiveDefinition().ReturnOrigin) {
			return pipelineStepPlayTownEgress
		}
		return pipelineStepOpenStash
	case pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint:
		return nextSharedForeignEgress(current)
	case pipelineStepWaitHubArea:
		return pipelineStepOpenStash
	case pipelineStepOpenStash, pipelineStepStashItems:
		return nextSharedStash(current)
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
	if step == pipelineStepWaitRecoveryArea {
		return "retry_return_area_unstable"
	}
	if step == pipelineStepWaitEntryArea || step == pipelineStepWaitHubArea {
		return string(RunReasonWaypointDestinationTimeout)
	}
	if step == pipelineStepWaitOriginTown {
		return string(RunReasonTownPortalDestinationTimeout)
	}
	return "timeout"
}

func (c *runPipeline) allowsNonInputTick(step string) bool {
	if c.phase == RunPhaseRetryReturn && step == pipelineStepWaitRecoveryArea {
		return true
	}
	if step == pipelineStepWaitOriginTown && (c.phase == "" || c.phase == RunPhaseLootAndReturn || c.phase == RunPhaseRetryReturn) {
		return true
	}
	return (c.isTravelPhase() || c.phase == "") && (step == pipelineStepWaitEntryArea || step == pipelineStepPlayRoute)
}

func (c *runPipeline) isTravelPhase() bool {
	return c.phase == RunPhaseTravelEntry || c.phase == RunPhasePlayRoute
}
