package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	portalDestinationStableSnapshots = 3
	portalDestinationGrace           = time.Second
)

type portalDestinationPhase string

// Destination recovery remains inside stepReturnWaitOriginTown, so all phases
// share the original return timeout: observe -> clear -> teleport -> settle ->
// retry click -> passive wait. Town arrival succeeds from every phase;
// invalid/loading snapshots are input-free, and any other known area aborts.
const (
	portalDestinationObserve    portalDestinationPhase = ""
	portalDestinationClear      portalDestinationPhase = "clear"
	portalDestinationTeleport   portalDestinationPhase = "teleport"
	portalDestinationSettle     portalDestinationPhase = "settle"
	portalDestinationRetryClick portalDestinationPhase = "retry_click"
	portalDestinationRetryWait  portalDestinationPhase = "retry_wait"
)

func (c *runPipeline) tickWaitOriginTown(ctx context.Context, deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		if c.ret.destinationPhase == portalDestinationObserve {
			c.resetPortalDestinationObservation()
		}
		return stepResult{}
	}
	if state.Area.ID == c.originTownArea() {
		return c.completePortalDestinationRecovery(deps)
	}
	// A portal transition can briefly expose either Area 0 or the last source
	// snapshot after the click. Retry-return already stabilized that source area
	// before allowing input, so keep it authoritative until town arrives.
	if state.Area.ID == world.None {
		return stepResult{}
	}
	if !c.isPortalSourceArea(state.Area.ID) {
		if reason := c.stopPortalDestinationCombat(deps); reason != "" {
			return stepResult{failed: true, reason: reason}
		}
		return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
	}

	switch c.ret.destinationPhase {
	case portalDestinationObserve:
		return c.tickPortalDestinationObservation(deps, state, now)
	case portalDestinationClear:
		return c.tickPortalDestinationClear(ctx, deps, state, now)
	case portalDestinationTeleport:
		return c.tickPortalDestinationTeleport(deps, state, now)
	case portalDestinationSettle:
		return c.tickPortalDestinationSettle(deps, state, now)
	case portalDestinationRetryClick:
		return c.tickPortalDestinationRetryClick(ctx, deps, state, now, false)
	case portalDestinationRetryWait:
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "portal_destination_state_invalid"}
	}
}

func (c *runPipeline) isPortalSourceArea(areaID world.AreaID) bool {
	if c.phase == RunPhaseRetryReturn && c.ret.recoveryAreaID != world.None {
		return areaID == c.ret.recoveryAreaID
	}
	return areaID == c.effectiveDefinition().RouteTerminalArea
}

func (c *runPipeline) tickPortalDestinationObservation(deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
	if c.ret.destinationRecoveryUsed {
		c.ret.destinationPhase = portalDestinationRetryWait
		return stepResult{}
	}
	snapshotAt := state.At
	if snapshotAt.IsZero() {
		snapshotAt = now
	}
	if snapshotAt != c.ret.destinationLastSnapshotAt {
		c.ret.destinationLastSnapshotAt = snapshotAt
		c.ret.destinationSnapshots++
		if c.ret.destinationObservedAt.IsZero() {
			c.ret.destinationObservedAt = now
		}
	}
	if c.ret.destinationSnapshots < portalDestinationStableSnapshots || now.Sub(c.ret.destinationObservedAt) < portalDestinationGrace {
		return stepResult{}
	}
	portal, found := c.pinnedDestinationPortal(state)
	if !found {
		return stepResult{}
	}
	preferredUnitID := uint32(0)
	if state.Hover.IsHovered && state.Hover.UnitType == world.HoverUnitTypeMonster {
		preferredUnitID = state.Hover.UnitID
	}
	return c.beginPortalDestinationRecovery(deps, state, portal, preferredUnitID, now)
}

func (c *runPipeline) beginPortalDestinationRecovery(deps pipelineReturnDeps, state world.State, portal world.Object, preferredUnitID uint32, now time.Time) stepResult {
	if c.ret.destinationRecoveryUsed {
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	}
	if !c.effectiveDefinition().HasCapability(RunCapabilityLocalRecoveryClear) || deps.RouteClear == nil || deps.Profile == nil || deps.Combat == nil {
		return stepResult{failed: true, reason: "local_recovery_clear_unavailable"}
	}
	c.ret.destinationRecoveryUsed = true
	c.ret.destinationPhase = portalDestinationClear
	c.ret.destinationPortalUnitID = portal.UnitID
	c.ret.destinationPortalPos = portal.Position
	c.ret.destinationClear.start(portal.Position, preferredUnitID, now)
	if err := c.emitPortalDestinationEvent(deps, c.portalDestinationEvent(telemetry.TownPortalEntryUnconfirmed, state, "destination_unconfirmed")); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	started := c.portalDestinationEvent(telemetry.LocalRecoveryClearStarted, state, "destination_unconfirmed")
	started.BlockerUnitID = preferredUnitID
	started.RequiredRadiusTiles = localThreatClearRadiusTiles
	started.ActionBudget = localThreatClearMaxActions
	started.TimeoutMs = localThreatClearTimeout.Milliseconds()
	started.NoProgressTimeoutMs = localThreatClearNoProgressTimeout.Milliseconds()
	if err := c.emitPortalDestinationEvent(deps, started); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	return stepResult{}
}

func (c *runPipeline) tickPortalDestinationClear(ctx context.Context, deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
	if deps.Profile == nil {
		return c.finishPortalDestinationClear(deps, state, now, localThreatClearFailed, "local_recovery_clear_unavailable")
	}
	resource := deps.Profile.TickResources(state, profile.ResourceContext{
		Threatened:        hasLocalThreat(state, c.ret.destinationClear.anchor, localThreatClearRadiusTiles),
		AllowMercenary:    hasLocalThreat(state, c.ret.destinationClear.anchor, localThreatClearRadiusTiles),
		FailOnUnavailable: true,
	}, now)
	switch resource.Status {
	case profile.StatusFailed:
		reason := resource.Reason
		if reason == "" {
			reason = "recovery_resource_failed"
		}
		return c.finishPortalDestinationClear(deps, state, now, localThreatClearFailed, reason)
	case profile.StatusAction, profile.StatusPending:
		return stepResult{}
	case profile.StatusComplete:
	default:
		return c.finishPortalDestinationClear(deps, state, now, localThreatClearFailed, "recovery_resource_state_invalid")
	}
	result := c.ret.destinationClear.tick(ctx, deps.RouteClear, state, now, string(c.effectiveDefinition().ID), c.core.combat.Profile)
	if result.outcome == localThreatClearFailed {
		return c.finishPortalDestinationClear(deps, state, now, result.outcome, result.reason)
	}
	if result.outcome != localThreatClearCleared && result.outcome != localThreatClearExhausted {
		return stepResult{}
	}
	c.ret.destinationClearActions = c.ret.destinationClear.actions
	return c.finishPortalDestinationClear(deps, state, now, result.outcome, result.reason)
}

func (c *runPipeline) finishPortalDestinationClear(deps pipelineReturnDeps, state world.State, now time.Time, outcome localThreatClearOutcome, outcomeReason string) stepResult {
	actions := c.ret.destinationClear.actions
	startedAt := c.ret.destinationClear.startedAt
	anchor := c.ret.destinationClear.anchor
	c.ret.destinationClear.reset(deps.RouteClear)
	if deps.Combat == nil {
		outcome, outcomeReason = localThreatClearFailed, "combat_not_wired"
	} else if err := deps.Combat.StopAttack(); err != nil {
		outcome, outcomeReason = localThreatClearFailed, "portal_recovery_combat_stop_failed"
	} else {
		deps.Combat.Reset()
	}
	finished := c.portalDestinationEvent(telemetry.LocalRecoveryClearFinished, state, outcomeReason)
	finished.Outcome = string(outcome)
	finished.CombatActionsSent = actions
	finished.RequiredRadiusTiles = localThreatClearRadiusTiles
	finished.CoverageRadiusTiles = state.MonsterCoverage.MonsterCoverageRadiusTiles
	coverageComplete := !state.MonsterCoverage.MonstersTruncated || state.MonsterCoverage.MonsterCoverageRadiusTiles >= localThreatClearRadiusTiles
	finished.CoverageComplete = &coverageComplete
	finished.RelevantThreatCount = countLocalThreats(state, anchor, localThreatClearRadiusTiles)
	if !startedAt.IsZero() {
		finished.ElapsedMs = now.Sub(startedAt).Milliseconds()
	}
	if err := c.emitPortalDestinationEvent(deps, finished); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	if outcome == localThreatClearFailed {
		return stepResult{failed: true, reason: outcomeReason}
	}
	c.ret.destinationPhase = portalDestinationTeleport
	return stepResult{}
}

func (c *runPipeline) tickPortalDestinationTeleport(deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	portal, found := c.pinnedDestinationPortal(state)
	if !found {
		return stepResult{}
	}
	c.ret.destinationPortalPos = portal.Position
	sent, err := deps.Combat.TeleportToward(now, state.Player, portal.Position, 0)
	if err != nil {
		return stepResult{failed: true, reason: "portal_recovery_teleport_failed"}
	}
	if !sent {
		return stepResult{}
	}
	c.ret.destinationTeleportAt = now
	c.ret.destinationTeleportSnap = state.At
	c.ret.destinationPhase = portalDestinationSettle
	return stepResult{}
}

func (c *runPipeline) tickPortalDestinationSettle(deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
	if !lootRepositionReady(now, state.At, c.ret.destinationTeleportAt, c.ret.destinationTeleportSnap) {
		return stepResult{}
	}
	if deps.Portal == nil {
		return stepResult{failed: true, reason: "town_portal_actions_not_wired"}
	}
	deps.Portal.Reset()
	c.ret.destinationPhase = portalDestinationRetryClick
	return stepResult{}
}

func (c *runPipeline) tickPortalDestinationRetryClick(ctx context.Context, deps pipelineReturnDeps, state world.State, now time.Time, completeOnClick bool) stepResult {
	if deps.Portal == nil {
		return stepResult{failed: true, reason: "town_portal_actions_not_wired"}
	}
	portal, found := c.pinnedDestinationPortal(state)
	if !found {
		return stepResult{}
	}
	nearest, nearestFound := state.NearestObject(world.ObjectKindTownPortal)
	if !nearestFound || nearest.UnitID != portal.UnitID {
		return stepResult{}
	}
	result := deps.Portal.Tick(ctx, state, now)
	switch result.Status {
	case pathing.TownPortalActionPending:
		return stepResult{}
	case pathing.TownPortalActionClicked:
		c.ret.destinationPhase = portalDestinationRetryWait
		event := c.portalDestinationEvent(telemetry.ReturnPortalRetry, state, "")
		event.Outcome = "success"
		if err := c.emitPortalDestinationEvent(deps, event); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{complete: completeOnClick}
	case pathing.TownPortalActionInputError:
		if err := c.emitPortalRetryFailure(deps, state, "town_portal_enter_failed"); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	default:
		if err := c.emitPortalRetryFailure(deps, state, "town_portal_enter_failed"); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	}
}

func (c *runPipeline) tickPortalDestinationRecovery(ctx context.Context, deps pipelineReturnDeps, state world.State, now time.Time, completeOnClick bool) stepResult {
	switch c.ret.destinationPhase {
	case portalDestinationClear:
		return c.tickPortalDestinationClear(ctx, deps, state, now)
	case portalDestinationTeleport:
		return c.tickPortalDestinationTeleport(deps, state, now)
	case portalDestinationSettle:
		return c.tickPortalDestinationSettle(deps, state, now)
	case portalDestinationRetryClick:
		return c.tickPortalDestinationRetryClick(ctx, deps, state, now, completeOnClick)
	case portalDestinationRetryWait:
		return stepResult{complete: completeOnClick}
	default:
		return stepResult{failed: true, reason: "portal_destination_state_invalid"}
	}
}

func hasLocalThreat(state world.State, anchor world.Position, radius float64) bool {
	_, found := selectLocalThreat(state, anchor, 0, radius)
	return found
}

func (c *runPipeline) completePortalDestinationRecovery(deps pipelineReturnDeps) stepResult {
	if !c.ret.destinationRecoveryUsed {
		return stepResult{complete: true}
	}
	if reason := c.stopPortalDestinationCombat(deps); reason != "" {
		return stepResult{failed: true, reason: reason}
	}
	c.resetPortalDestinationRecovery()
	return stepResult{complete: true}
}

func (c *runPipeline) stopPortalDestinationCombat(deps pipelineReturnDeps) string {
	c.ret.destinationClear.reset(deps.RouteClear)
	if !c.ret.destinationRecoveryUsed {
		return ""
	}
	if deps.Combat == nil {
		return "combat_not_wired"
	}
	if err := deps.Combat.StopAttack(); err != nil {
		return "portal_recovery_combat_stop_failed"
	}
	deps.Combat.Reset()
	return ""
}

func (c *runPipeline) pinnedDestinationPortal(state world.State) (world.Object, bool) {
	if c.ret.destinationPortalUnitID != 0 {
		return findTownPortalByUnitID(state, c.ret.destinationPortalUnitID)
	}
	portal, found := state.NearestObject(world.ObjectKindTownPortal)
	if found {
		c.ret.destinationPortalUnitID = portal.UnitID
		c.ret.destinationPortalPos = portal.Position
	}
	return portal, found
}

func (c *runPipeline) portalDestinationEvent(name telemetry.EventName, state world.State, reason string) telemetry.Event {
	return telemetry.Event{
		Event: name, DefinitionID: string(c.effectiveDefinition().ID), Step: pipelineStepWaitOriginTown,
		Stage: telemetry.HistoryStageReturnTown, AreaID: uint32(state.Area.ID),
		UnitID: c.ret.destinationPortalUnitID, Attempt: 1, Reason: reason,
		TargetX: c.ret.destinationPortalPos.X, TargetY: c.ret.destinationPortalPos.Y,
	}
}

func (c *runPipeline) emitPortalDestinationEvent(deps pipelineReturnDeps, event telemetry.Event) error {
	if deps.Telemetry == nil {
		return nil
	}
	return deps.Telemetry.Emit(event)
}

func (c *runPipeline) emitPortalRetryFailure(deps pipelineReturnDeps, state world.State, reason string) error {
	event := c.portalDestinationEvent(telemetry.ReturnPortalRetry, state, reason)
	event.Outcome = "failed"
	return c.emitPortalDestinationEvent(deps, event)
}

func countLocalThreats(state world.State, anchor world.Position, radius float64) int {
	count := 0
	for _, monster := range state.Monsters {
		if world.Distance(monster.Position, anchor) <= radius {
			count++
		}
	}
	return count
}

func (c *runPipeline) resetPortalDestinationObservation() {
	c.ret.destinationObservedAt = time.Time{}
	c.ret.destinationLastSnapshotAt = time.Time{}
	c.ret.destinationSnapshots = 0
}

func (c *runPipeline) resetPortalDestinationRecovery() {
	c.ret.destinationPhase = portalDestinationObserve
	c.ret.destinationPortalUnitID = 0
	c.ret.destinationPortalPos = world.Position{}
	c.resetPortalDestinationObservation()
	c.ret.destinationRecoveryUsed = false
	c.ret.destinationClear = localThreatClear{}
	c.ret.destinationTeleportAt = time.Time{}
	c.ret.destinationTeleportSnap = time.Time{}
	c.ret.destinationClearActions = 0
}
