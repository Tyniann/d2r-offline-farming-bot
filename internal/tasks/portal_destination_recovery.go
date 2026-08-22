package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
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
// invalid/loading snapshots are input-free, and any other valid area aborts.
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
		return c.completePortalDestinationRecovery(deps, state)
	}
	if state.Area.ID != c.effectiveDefinition().RouteTerminalArea {
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
		return c.tickPortalDestinationRetryClick(ctx, deps, state, now)
	case portalDestinationRetryWait:
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "portal_destination_state_invalid"}
	}
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
	if deps.RouteClear == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	preferredUnitID := uint32(0)
	if state.Hover.IsHovered && state.Hover.UnitType == world.HoverUnitTypeMonster {
		preferredUnitID = state.Hover.UnitID
	}
	c.ret.destinationRecoveryUsed = true
	c.ret.destinationPhase = portalDestinationClear
	c.ret.destinationPortalUnitID = portal.UnitID
	c.ret.destinationPortalPos = portal.Position
	c.ret.destinationClear.start(portal.Position, preferredUnitID, now)
	if err := c.emitPortalDestinationEvent(deps, telemetry.TownPortalEntryUnconfirmed, state, "destination_unconfirmed"); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	if err := c.emitPortalDestinationEvent(deps, telemetry.TownPortalRecoveryStarted, state, "destination_unconfirmed"); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	return stepResult{}
}

func (c *runPipeline) tickPortalDestinationClear(ctx context.Context, deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
	result := c.ret.destinationClear.tick(ctx, deps.RouteClear, state, now, string(c.effectiveDefinition().ID), c.core.combat.Profile)
	if result.failed {
		c.ret.destinationClear.reset(deps.RouteClear)
		return stepResult{failed: true, reason: result.reason}
	}
	if !result.done {
		return stepResult{}
	}
	c.ret.destinationClearActions = c.ret.destinationClear.actions
	c.ret.destinationClear.reset(deps.RouteClear)
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if err := deps.Combat.StopAttack(); err != nil {
		return stepResult{failed: true, reason: "portal_recovery_combat_stop_failed"}
	}
	deps.Combat.Reset()
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

func (c *runPipeline) tickPortalDestinationRetryClick(ctx context.Context, deps pipelineReturnDeps, state world.State, now time.Time) stepResult {
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
		if err := c.emitPortalDestinationEvent(deps, telemetry.TownPortalRetryClicked, state, ""); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{}
	case pathing.TownPortalActionInputError:
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	default:
		// The single retry budget is exhausted. Keep observing the destination
		// without further input until the original wait_origin_town deadline.
		c.ret.destinationPhase = portalDestinationRetryWait
		return stepResult{}
	}
}

func (c *runPipeline) completePortalDestinationRecovery(deps pipelineReturnDeps, state world.State) stepResult {
	if !c.ret.destinationRecoveryUsed {
		return stepResult{complete: true}
	}
	if reason := c.stopPortalDestinationCombat(deps); reason != "" {
		return stepResult{failed: true, reason: reason}
	}
	if err := c.emitPortalDestinationEvent(deps, telemetry.TownPortalRecoveryCompleted, state, ""); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
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

func (c *runPipeline) emitPortalDestinationEvent(deps pipelineReturnDeps, name telemetry.EventName, state world.State, reason string) error {
	if deps.Telemetry == nil {
		return nil
	}
	return deps.Telemetry.Emit(telemetry.Event{
		Event: name, DefinitionID: string(c.effectiveDefinition().ID), Step: pipelineStepWaitOriginTown,
		Stage: telemetry.HistoryStageReturnTown, AreaID: uint32(state.Area.ID),
		UnitID: c.ret.destinationPortalUnitID, Attempt: 1, Reason: reason,
		TargetX: c.ret.destinationPortalPos.X, TargetY: c.ret.destinationPortalPos.Y,
		CombatActionsSent: c.ret.destinationClearActions,
	})
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
