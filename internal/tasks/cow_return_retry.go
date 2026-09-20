package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const cowReturnTeleportMinTiles = 1

type cowReturnPhase string

const (
	cowReturnObserve    cowReturnPhase = ""
	cowReturnTeleport   cowReturnPhase = "teleport"
	cowReturnSettle     cowReturnPhase = "settle"
	cowReturnRetryClick cowReturnPhase = "retry_click"
	cowReturnRetryWait  cowReturnPhase = "retry_wait"
)

type cowReturnRetry struct {
	phase            cowReturnPhase
	portalUnitID     uint32
	portalPos        world.Position
	observedAt       time.Time
	lastSnapshotAt   time.Time
	snapshots        int
	used             bool
	teleportAt       time.Time
	teleportSnapshot time.Time
}

func (c *cowPipeline) pinReturnPortal(state world.State) {
	if c.returnRetry.portalUnitID != 0 {
		if portal, ok := findTownPortalByUnitID(state, c.returnRetry.portalUnitID); ok {
			c.returnRetry.portalPos = portal.Position
		}
		return
	}
	portal, ok := state.NearestObject(world.ObjectKindTownPortal)
	if !ok {
		return
	}
	c.returnRetry.portalUnitID = portal.UnitID
	c.returnRetry.portalPos = portal.Position
}

func (c *cowPipeline) tickWaitRogue(ctx context.Context, deps Deps, state world.State, now time.Time) stepResult {
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		if c.returnRetry.phase == cowReturnObserve {
			c.resetReturnObservation()
		}
		return stepResult{}
	}
	if state.Area.ID == world.RogueEncampment {
		return stepResult{complete: true}
	}
	if state.Area.ID == world.None {
		return stepResult{}
	}
	if state.Area.ID != world.Tristram {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}

	switch c.returnRetry.phase {
	case cowReturnObserve:
		return c.tickReturnObservation(deps, state, now)
	case cowReturnTeleport:
		return c.tickReturnTeleport(deps, state, now)
	case cowReturnSettle:
		return c.tickReturnSettle(deps, state, now)
	case cowReturnRetryClick:
		return c.tickReturnRetryClick(ctx, deps, state, now)
	case cowReturnRetryWait:
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
}

func (c *cowPipeline) tickReturnObservation(deps Deps, state world.State, now time.Time) stepResult {
	if c.returnRetry.used {
		c.returnRetry.phase = cowReturnRetryWait
		return stepResult{}
	}
	snapshotAt := state.At
	if snapshotAt.IsZero() {
		snapshotAt = now
	}
	if snapshotAt != c.returnRetry.lastSnapshotAt {
		c.returnRetry.lastSnapshotAt = snapshotAt
		c.returnRetry.snapshots++
		if c.returnRetry.observedAt.IsZero() {
			c.returnRetry.observedAt = now
		}
	}
	if c.returnRetry.snapshots < portalDestinationStableSnapshots || now.Sub(c.returnRetry.observedAt) < portalDestinationGrace {
		return stepResult{}
	}
	portal, found := c.pinnedReturnPortal(state)
	if !found {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
	c.returnRetry.used = true
	if err := c.emitReturnRetryEvent(deps, telemetry.TownPortalEntryUnconfirmed, state, "destination_unconfirmed", ""); err != nil {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	if world.Distance(state.Player.Position, portal.Position) >= cowReturnTeleportMinTiles {
		c.returnRetry.phase = cowReturnTeleport
		return stepResult{}
	}
	c.returnRetry.phase = cowReturnSettle
	return stepResult{}
}

func (c *cowPipeline) tickReturnTeleport(deps Deps, state world.State, now time.Time) stepResult {
	if deps.Combat == nil {
		c.returnRetry.phase = cowReturnSettle
		return stepResult{}
	}
	portal, found := c.pinnedReturnPortal(state)
	if !found {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
	sent, err := deps.Combat.TeleportToward(now, state.Player, portal.Position, 0)
	if err != nil {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
	if !sent {
		return stepResult{}
	}
	c.returnRetry.teleportAt = now
	c.returnRetry.teleportSnapshot = state.At
	c.returnRetry.phase = cowReturnSettle
	return stepResult{}
}

func (c *cowPipeline) tickReturnSettle(deps Deps, state world.State, now time.Time) stepResult {
	if !c.returnRetry.teleportAt.IsZero() && !lootRepositionReady(now, state.At, c.returnRetry.teleportAt, c.returnRetry.teleportSnapshot) {
		return stepResult{}
	}
	if deps.Portal == nil {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
	deps.Portal.Reset()
	c.returnRetry.phase = cowReturnRetryClick
	return stepResult{}
}

func (c *cowPipeline) tickReturnRetryClick(ctx context.Context, deps Deps, state world.State, now time.Time) stepResult {
	if deps.Portal == nil {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
	portal, found := c.pinnedReturnPortal(state)
	if !found {
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
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
		c.returnRetry.phase = cowReturnRetryWait
		if err := c.emitReturnRetryEvent(deps, telemetry.ReturnPortalRetry, state, "", "success"); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{}
	default:
		if err := c.emitReturnRetryEvent(deps, telemetry.ReturnPortalRetry, state, "cow_return_portal_failed", "failed"); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{failed: true, reason: "cow_return_portal_failed"}
	}
}

func (c *cowPipeline) pinnedReturnPortal(state world.State) (world.Object, bool) {
	c.pinReturnPortal(state)
	if c.returnRetry.portalUnitID == 0 {
		return world.Object{}, false
	}
	return findTownPortalByUnitID(state, c.returnRetry.portalUnitID)
}

func (c *cowPipeline) emitReturnRetryEvent(deps Deps, name telemetry.EventName, state world.State, reason, outcome string) error {
	if deps.Telemetry == nil {
		return nil
	}
	return deps.Telemetry.Emit(telemetry.Event{
		Event: name, DefinitionID: string(c.definition.ID), Step: cowStepWaitRogue,
		Stage: telemetry.HistoryStageReturnTown, AreaID: uint32(state.Area.ID),
		UnitID: c.returnRetry.portalUnitID, Attempt: 1, Reason: reason, Outcome: outcome,
		TargetX: c.returnRetry.portalPos.X, TargetY: c.returnRetry.portalPos.Y,
	})
}

func (c *cowPipeline) resetReturnObservation() {
	c.returnRetry.observedAt = time.Time{}
	c.returnRetry.lastSnapshotAt = time.Time{}
	c.returnRetry.snapshots = 0
}
