package tasks

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (c *runPipeline) resetRouteLoot() {
	c.travel.routeLootPointSet = false
	c.travel.routeLootSegmentIndex = 0
	c.travel.routeLootPointIndex = 0
	c.travel.routeLootScanned = false
}

func (c *runPipeline) onTravelTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	return c.tickTravel(ctx, narrowTravelDeps(deps), step, w, now, stepStartedAt)
}

func (c *runPipeline) tickTravel(ctx context.Context, deps pipelineTravelDeps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
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
				c.travel.resumeAfterPrecheckSet = true
				c.travel.resumeAfterPrecheck = next
				return stepResult{complete: true}
			}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		c.travel.resumeAfterPrecheckSet = false
		c.travel.resumeAfterPrecheck = ""
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
		return c.tickWaitEntryArea(w, now)
	case pipelineStepPlayRoute:
		if deps.Profile != nil {
			if !w.Valid || w.Phase != world.GamePhaseInGame {
				return stepResult{}
			}
			alreadyReady := c.travel.fieldReadyComplete
			res := deps.Profile.TickHook(ctx, profile.HookFieldReady, w, profile.EncounterTarget{}, now)
			switch res.Status {
			case profile.StatusFailed:
				return stepResult{failed: true, reason: res.Reason}
			case profile.StatusComplete:
				c.travel.fieldReadyComplete = true
			default:
				// Keep ticking field-ready after the first complete so Hammerdin
				// CTA/Holy Shield can recast during a long route. Release any
				// held attack before the weapon-swap sequence starts.
				if alreadyReady && deps.Combat != nil {
					if err := deps.Combat.StopAttack(); err != nil {
						return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
					}
				}
				return stepResult{}
			}
		}
		if deps.Route == nil {
			return stepResult{failed: true, reason: "route_playback_not_wired"}
		}
		if c.core.routeID == "" {
			return stepResult{failed: true, reason: "route_id_missing"}
		}
		if !c.travel.routeStarted {
			if err := deps.Route.Start(c.core.routeID, w); err != nil {
				if errors.Is(err, pathing.ErrGameIdentityUnavailable) {
					return stepResult{}
				}
				return stepResult{failed: true, reason: "route_playback_start_failed"}
			}
			c.travel.routeStarted = true
		}
		if c.core.routeCombat.Enabled && c.definition.HasCapability(RunCapabilityRouteClear) {
			if !w.Valid || w.Phase != world.GamePhaseInGame {
				return stepResult{}
			}
			progress, ok := deps.Route.Progress(w)
			if !ok {
				if c.travel.routeProgressUnavailableSince.IsZero() {
					c.travel.routeProgressUnavailableSince = now
					c.travel.routeProgressUnavailableSnapshot = w.At
					return stepResult{}
				}

				if !w.At.After(c.travel.routeProgressUnavailableSnapshot) {
					return stepResult{}
				}
				c.travel.routeProgressUnavailableSnapshot = w.At
				if now.Sub(c.travel.routeProgressUnavailableSince) < routeProgressUnavailableGrace {
					return stepResult{}
				}
				return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
			}
			c.resetRouteProgressUnavailable()
			assessment := assessThreats(w, progress, c.definition.RouteHostileNPCIDs, c.core.routeCombat)
			if observer, ok := deps.RouteClear.(routeClearObjectiveObserver); ok && observer.ObserveObjectiveProgress(w) {
				c.travel.routeThreat.observeExternalProgress(now)
				c.travel.cowNoProgressRecoveryStage = cowNoProgressStageNone
				c.travel.cowNoProgressApproachUnitID = 0
			}
			terminalSafe := c.observeTerminalSafe(w, progress, assessment)
			c.travel.routeThreat.SetTelemetry(deps.Telemetry)
			resourceContext := c.travel.routeThreat.ObserveResources(w, assessment, c.core.routeCombat, now)
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
					if err := c.travel.routeThreat.ObserveResourceResult(w, progress, resourceContext, resource, now); err != nil {
						return stepResult{failed: true, reason: "telemetry_failed"}
					}
					return stepResult{}
				}
				if err := c.travel.routeThreat.ObserveResourceResult(w, progress, resourceContext, resource, now); err != nil {
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

			if c.travel.routeApproachPending && w.At.After(c.travel.routeApproachSnapshotAt) {
				target, found := w.FindMonsterByUnitID(c.travel.routeApproachTargetUnitID)
				if !found {
					target = world.Monster{UnitID: c.travel.routeApproachTargetUnitID, Position: c.travel.routeApproachGoal}
				}
				result := c.tickRouteThreatApproach(deps, w, progress, target, now)
				if result.failed || c.travel.routeApproachPending {
					return result
				}

				return stepResult{}
			}
			threat := c.travel.routeThreat.Tick(ctx, deps.Route, deps.RouteClear, w, progress, assessment, c.definition, c.core.routeCombat, c.core.combat, now)
			if threat.StopAttack {
				if deps.Combat == nil {
					return stepResult{failed: true, reason: "combat_not_wired"}
				}
				if err := deps.Combat.StopAttack(); err != nil {
					return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
				}
				if !threat.Failed {
					return stepResult{}
				}
			}
			if threat.Failed {
				if c.definition.ID == RunIDCows && threat.Reason == RouteThreatReasonCowNoProgress {
					return c.tickCowNoProgressRecovery(deps, w, progress, assessment, now)
				}
				cowApproach := c.definition.ID == RunIDCows && threat.Reason == RouteThreatReasonOutOfRange
				hammerdinApproach := c.hammerdinBossCombat() && threat.Reason == RouteThreatReasonOutOfRange
				standardApproach := threat.Reason == RouteThreatReasonOutOfRange &&
					progress.Mode == RouteProgressMovement && progress.TargetAvailable
				if cowApproach || hammerdinApproach || standardApproach {
					if threat.ApproachTarget.UnitID != 0 {
						if threat.HammerdinReposition {
							return c.tickRouteThreatHammerdinReposition(deps, w, progress, threat.ApproachTarget, threat.HammerdinRouteForward, now)
						}
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

					return stepResult{}
				}
				return stepResult{failed: true, reason: string(threat.Reason)}
			}
			if !c.travel.routeApproachPending {
				c.resetRouteThreatApproach()
			}
			if c.travel.routeThreat.State() == RouteThreatMoving {
				c.travel.cowNoProgressRecoveryStage = cowNoProgressStageNone
				c.travel.cowNoProgressApproachUnitID = 0
			}
			terminalCompletionReady := terminalSafe &&
				c.travel.terminalSafeSnapshots >= Phase17StableClearSnapshots &&
				c.travel.routeThreat.State() == RouteThreatMoving

			if !threat.AllowMovement && !terminalCompletionReady {

				c.travel.routeLootScanned = false
				return stepResult{}
			}
			if terminalSafe && c.travel.terminalSafeSnapshots < Phase17StableClearSnapshots {
				if err := deps.Route.Hold(w); err != nil {
					return stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
				}
				return stepResult{}
			}
			if !c.core.suppressRouteLoot {
				if handled, result := c.tickRouteLoot(deps, w, progress, now); handled {
					return result
				}
			}
		}
		// Operate-on-sight is independent of route-clear. Lower Kurast has
		// chest_sweep without route_clear; Summoner and Cows skip via capability.
		if handled, result := c.tickRouteChestOperate(ctx, deps, w, now); handled {
			return result
		}
		done, err := deps.Route.Tick(ctx, w)
		if err != nil {
			return stepResult{failed: true, reason: routePlaybackFailureReason(err)}
		}
		if done {
			c.travel.routeThreat.Reset(deps.RouteClear)
			c.resetTerminalSafe()
			return stepResult{complete: true}
		}
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) observeTerminalSafe(state world.State, progress RouteProgress, assessment ThreatAssessment) bool {
	if !c.core.requireTerminalSafe || progress.Mode != RouteProgressTransition {
		c.resetTerminalSafe()
		return false
	}
	safe := state.Valid && state.Phase == world.GamePhaseInGame && assessment.SnapshotAt.Equal(state.At) &&
		assessment.CoverageComplete && !assessment.RouteTargetFound && !assessment.DensityTargetFound
	if !safe {
		c.resetTerminalSafe()
		return true
	}
	if c.travel.terminalSafeSnapshotAt.IsZero() || state.At.After(c.travel.terminalSafeSnapshotAt) {
		c.travel.terminalSafeSnapshots++
		c.travel.terminalSafeSnapshotAt = state.At
	}
	return true
}

func (c *runPipeline) resetTerminalSafe() {
	c.travel.terminalSafeSnapshots = 0
	c.travel.terminalSafeSnapshotAt = time.Time{}
}

func (c *runPipeline) resetRouteThreatApproach() {
	c.travel.routeApproachTargetUnitID = 0
	c.travel.routeApproachOrigin = world.Position{}
	c.travel.routeApproachGoal = world.Position{}
	c.travel.routeApproachSentAt = time.Time{}
	c.travel.routeApproachSnapshotAt = time.Time{}
	c.travel.routeApproachPending = false
	c.travel.routeApproachFailures = 0
	c.travel.routeApproachHammerdinReposition = false
	c.travel.routeApproachHammerdinRouteForward = false
	c.travel.routeApproachExhaustedUnitID = 0
}

func (c *runPipeline) resetRouteProgressUnavailable() {
	c.travel.routeProgressUnavailableSince = time.Time{}
	c.travel.routeProgressUnavailableSnapshot = time.Time{}
}

// tickCowNoProgressRecovery stages retarget → approach → soft exit when the Cow
// objective watchdog expires without corpses, kills, or coverage progress.
// Soft exit keeps reason `cow_combat_no_progress` so the queue consumes its
// normal retry-return / restart budget.
func (c *runPipeline) tickCowNoProgressRecovery(
	deps pipelineTravelDeps,
	w world.State,
	progress RouteProgress,
	assessment ThreatAssessment,
	now time.Time,
) stepResult {
	switch c.travel.cowNoProgressRecoveryStage {
	case cowNoProgressStageNone:
		currentUnitID := uint32(0)
		if assessment.RouteTargetFound {
			currentUnitID = assessment.RouteTarget.UnitID
		} else if assessment.DensityTargetFound {
			currentUnitID = assessment.DensityTarget.UnitID
		}
		c.travel.cowNoProgressApproachUnitID = currentUnitID
		if observer, ok := deps.RouteClear.(routeClearNoProgressRetargetObserver); ok {
			if selected, found := observer.ObserveNoProgressRetarget(currentUnitID); found {
				c.travel.cowNoProgressApproachUnitID = selected.UnitID
			}
		}
		c.travel.routeApproachExhaustedUnitID = 0
		c.travel.routeApproachFailures = 0
		c.travel.routeApproachPending = false
		c.travel.routeThreat.observeExternalProgress(now)
		c.travel.cowNoProgressRecoveryStage = cowNoProgressStageRetargeted
		return stepResult{}
	case cowNoProgressStageRetargeted:
		target := world.Monster{}
		if c.travel.cowNoProgressApproachUnitID != 0 {
			if selected, found := w.FindMonsterByUnitID(c.travel.cowNoProgressApproachUnitID); found {
				target = selected
			}
		}
		if target.UnitID == 0 && assessment.RouteTargetFound {
			target = assessment.RouteTarget
		}
		if target.UnitID == 0 && assessment.DensityTargetFound {
			target = assessment.DensityTarget
		}
		c.travel.routeApproachExhaustedUnitID = 0
		c.travel.routeApproachFailures = 0
		c.travel.routeThreat.observeExternalProgress(now)
		c.travel.cowNoProgressRecoveryStage = cowNoProgressStageApproached
		if target.UnitID == 0 {
			return stepResult{}
		}
		return c.tickRouteThreatApproach(deps, w, progress, target, now)
	default:
		return stepResult{failed: true, reason: string(RouteThreatReasonCowNoProgress)}
	}
}

// tickRouteThreatApproach keeps Summoner on the already validated next route
// point. Hammerdin always teleports to its profile-owned EngageDistanceTiles,
// including a future Cow strategy. Other Cow profiles use the bounded,
// projection-driven combat teleport toward the executor-pinned group member,
// so recovery cannot walk past the blocked pack. Neither path calls Route.Tick.
func (c *runPipeline) tickRouteThreatApproach(deps pipelineTravelDeps, w world.State, progress RouteProgress, target world.Monster, now time.Time) stepResult {
	return c.tickRouteThreatApproachMode(deps, w, progress, target, false, false, now)
}

func (c *runPipeline) tickRouteThreatHammerdinReposition(deps pipelineTravelDeps, w world.State, progress RouteProgress, target world.Monster, routeForward bool, now time.Time) stepResult {
	return c.tickRouteThreatApproachMode(deps, w, progress, target, true, routeForward, now)
}

func (c *runPipeline) tickRouteThreatApproachMode(
	deps pipelineTravelDeps,
	w world.State,
	progress RouteProgress,
	target world.Monster,
	hammerdinReposition bool,
	hammerdinRouteForward bool,
	now time.Time,
) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if c.travel.routeApproachTargetUnitID != target.UnitID {
		c.resetRouteThreatApproach()
		c.travel.routeApproachTargetUnitID = target.UnitID
	}
	if c.travel.routeApproachPending {
		reposition := c.travel.routeApproachHammerdinReposition
		if !w.At.After(c.travel.routeApproachSnapshotAt) {
			return stepResult{}
		}
		positionProgress := routeApproachDirectionalProgress(c.travel.routeApproachOrigin, c.travel.routeApproachGoal, w.Player.Position)
		if positionProgress > routeThreatApproachProgressEpsilonTiles {
			c.travel.routeApproachFailures = 0
			c.travel.routeApproachExhaustedUnitID = 0
			if err := c.travel.routeThreat.ObserveApproachProgress(w, progress, target, positionProgress, now); err != nil {
				return stepResult{failed: true, reason: "telemetry_failed"}
			}
			if observer, ok := deps.RouteClear.(routeClearApproachObserver); ok {
				observer.ObserveRouteClearApproachProgress()
			}
			c.travel.routeApproachPending = false
			c.travel.routeApproachHammerdinReposition = false
			c.travel.routeApproachHammerdinRouteForward = false
			if reposition {
				c.travel.routeThreat.completeHammerdinReposition(true)
			}
			return stepResult{}
		}
		// Positive movement proves a completed teleport immediately. The settle
		// duration remains only the deadline for declaring a blocked landing.
		if now.Sub(c.travel.routeApproachSentAt) < routeThreatApproachSettle {
			return stepResult{}
		}
		if err := c.travel.routeThreat.ObserveApproachNoProgress(w, progress, target, positionProgress, now); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		c.travel.routeApproachFailures++
		c.travel.routeApproachPending = false
		c.travel.routeApproachHammerdinReposition = false
		c.travel.routeApproachHammerdinRouteForward = false
		if reposition {
			c.travel.routeApproachFailures = 0
			c.travel.routeApproachExhaustedUnitID = 0
			c.travel.routeThreat.completeHammerdinReposition(false)
			return stepResult{}
		}
		if c.travel.routeApproachFailures >= routeThreatApproachMaxFailures {
			c.travel.routeApproachExhaustedUnitID = target.UnitID
			return stepResult{}
		}
	}
	if c.travel.routeApproachExhaustedUnitID == target.UnitID {
		return stepResult{}
	}
	goal := progress.MovementTarget
	sent := false
	actionKind := "force_move"
	var err error
	if hammerdinReposition {
		goal = target.Position
		desiredDistance := c.core.combat.EngageDistanceTiles
		actionKind = "hammerdin_reposition"
		if hammerdinRouteForward {
			desiredDistance = 0
			actionKind = "hammerdin_route_forward"
		}
		sent, err = deps.Combat.TeleportToward(now, w.Player, target.Position, desiredDistance)
	} else if c.hammerdinBossCombat() {
		goal = target.Position
		actionKind = "teleport"
		sent, err = deps.Combat.TeleportToward(now, w.Player, target.Position, c.core.combat.EngageDistanceTiles)
	} else if c.definition.ID == RunIDCows {
		landing, desiredDistance, projectable := deps.Combat.FarthestProjectableMonsterApproach(w.Player.Position, target.Position)
		if !projectable {
			c.travel.routeApproachExhaustedUnitID = target.UnitID
			return stepResult{}
		}
		if !cowApproachLandingSafe(w, landing, c.definition.RouteHostileNPCIDs, c.core.routeCombat.LandingRadiusTiles) {

			c.travel.routeApproachExhaustedUnitID = target.UnitID
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
		c.travel.routeApproachOrigin = w.Player.Position
		c.travel.routeApproachGoal = goal
		c.travel.routeApproachSentAt = now
		c.travel.routeApproachSnapshotAt = w.At
		c.travel.routeApproachPending = true
		c.travel.routeApproachHammerdinReposition = hammerdinReposition
		c.travel.routeApproachHammerdinRouteForward = hammerdinRouteForward
		if err := c.travel.routeThreat.ObserveApproachInput(w, progress, target, c.travel.routeApproachFailures+1, actionKind, now); err != nil {
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
func (c *runPipeline) tickRouteLoot(deps pipelineTravelDeps, w world.State, progress RouteProgress, now time.Time) (bool, stepResult) {
	if deps.Loot == nil || progress.Mode == RouteProgressTransition {
		return false, stepResult{}
	}
	if !c.travel.routeLootPointSet ||
		c.travel.routeLootSegmentIndex != progress.SegmentIndex ||
		c.travel.routeLootPointIndex != progress.PointIndex {
		c.travel.routeLootPointSet = true
		c.travel.routeLootSegmentIndex = progress.SegmentIndex
		c.travel.routeLootPointIndex = progress.PointIndex
		c.travel.routeLootScanned = false
		c.loot.lootPickupActive = false
		c.resetLootApproach()
		c.clearLootRecoveryPending()
	}

	if c.loot.lootPickupActive {
		if err := deps.Route.Hold(w); err != nil {
			return true, stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
		}
		return true, c.tickRouteLootPickup(deps, w, now)
	}
	if c.loot.lootRecoveryPending {
		if err := deps.Route.Hold(w); err != nil {
			return true, stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
		}
		result := c.tickLootPickupRecovery(deps.lootDeps(), w, now)
		return true, result
	}
	if c.travel.routeLootScanned {
		return false, stepResult{}
	}

	scan := deps.Loot.ScanRouteKeep(w, routeLootRadiusTiles)
	if scan.TelemetryFailed {
		return true, stepResult{failed: true, reason: "telemetry_failed"}
	}

	if !scan.HasTarget {
		c.travel.routeLootScanned = true
		return false, stepResult{}
	}
	c.loot.lootApproachTarget = scan.NextTarget
	c.loot.lootApproachTargetSet = true

	if err := deps.Route.Hold(w); err != nil {
		return true, stepResult{failed: true, reason: string(RouteThreatReasonStateInvalid)}
	}
	target := c.loot.lootApproachTarget
	if world.Distance(w.Player.Position, target.Position) > c.effectiveLootPickupDistance() {
		if deps.Combat == nil {
			return true, stepResult{failed: true, reason: "combat_not_wired"}
		}
		if world.Distance(w.Player.Position, target.Position) <= lootApproachMaxDistanceTiles &&
			c.loot.lootApproachAttempts < lootRepositionMaxAttempts {
			if !lootRepositionReady(now, w.At, c.loot.lootApproachAt, c.loot.lootApproachSnapshot) {
				return true, stepResult{}
			}
			sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
			if err != nil {
				return true, stepResult{failed: true, reason: "loot_reposition_failed"}
			}
			if sent {
				c.loot.lootApproachAttempts++
				c.loot.lootApproachAt = now
				c.loot.lootApproachSnapshot = w.At
			}
			return true, stepResult{}
		}
		if c.loot.lootApproachAttempts > 0 && !lootRepositionReady(now, w.At, c.loot.lootApproachAt, c.loot.lootApproachSnapshot) {
			return true, stepResult{}
		}
	}
	if err := deps.Loot.StartPickup(target); err != nil {
		return true, stepResult{failed: true, reason: "loot_pickup_start_failed"}
	}
	c.loot.lootPickupActive = true
	c.resetLootApproach()
	return true, c.tickRouteLootPickup(deps, w, now)
}

func (c *runPipeline) tickRouteLootPickup(deps pipelineTravelDeps, w world.State, now time.Time) stepResult {
	result := deps.Loot.TickPickup(w, now)
	if !result.Done {
		return stepResult{}
	}
	c.loot.lootPickupActive = false
	c.resetLootApproach()
	c.travel.routeLootScanned = false
	switch result.Status {
	case LootPickupHoverNotFound, LootPickupFailed, LootPickupTooFar:

		if c.beginLootPickupRecovery(deps.lootDeps(), result.Target, routeLootRadiusTiles) {
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

func (c *runPipeline) tickWaitEntryArea(w world.State, now time.Time) stepResult {
	// WaypointOpen is sticky on this build and must not reset a real InGame
	// arrival; pathing already refuses to trust the bit without our open click.
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID == world.None {
		c.resetEntryArrival()
		return stepResult{}
	}
	if w.Area.ID == c.effectiveDefinition().EntryArea {
		if c.observeEntryArrival(w, now) {
			return stepResult{complete: true}
		}
		return stepResult{}
	}
	c.resetEntryArrival()
	if w.Area.ID != world.RogueEncampment {
		return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
	}
	return stepResult{}
}

func (c *runPipeline) tickWaitSettledArea(w world.State, now time.Time, want world.AreaID) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID == world.None || w.Area.ID != want {
		c.resetEntryArrival()
		return stepResult{}
	}
	if c.observeEntryArrival(w, now) {
		return stepResult{complete: true}
	}
	return stepResult{}
}

func (c *runPipeline) observeEntryArrival(w world.State, now time.Time) bool {
	if c.travel.entryArriveAt.IsZero() {
		c.travel.entryArriveAt = now
	}
	snapshotAt := w.At
	if snapshotAt.IsZero() {
		snapshotAt = now
	}
	if c.travel.entryArriveSnapshotAt.IsZero() || snapshotAt.After(c.travel.entryArriveSnapshotAt) {
		c.travel.entryArriveSnapshots++
		c.travel.entryArriveSnapshotAt = snapshotAt
	}
	return c.travel.entryArriveSnapshots >= entryAreaArriveSnapshots &&
		!c.travel.entryArriveAt.IsZero() &&
		now.Sub(c.travel.entryArriveAt) >= entryAreaArriveSettle
}

func (c *runPipeline) resetEntryArrival() {
	c.travel.entryArriveAt = time.Time{}
	c.travel.entryArriveSnapshots = 0
	c.travel.entryArriveSnapshotAt = time.Time{}
}

func (c *runPipeline) selectRunWaypoint(ctx context.Context, deps pipelineTravelDeps, state world.State, now time.Time) stepResult {
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
