package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (c *runPipeline) tickReturn(ctx context.Context, deps pipelineReturnDeps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	switch step {
	case pipelineStepCastTownPortal:
		return tickRunTownPortal(deps, w)
	case pipelineStepEnterTownPortal:
		return c.tickEnterTownPortalWithDeps(ctx, deps, w, now)
	case pipelineStepWaitOriginTown:
		return c.tickWaitOriginTown(w)
	case pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return c.tickTownNormalization(ctx, deps, step, w, now, stepStartedAt)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case pipelineStepPrepareTown:
		if deps.Town == nil {
			return stepResult{failed: true, reason: "town_preparation_not_wired"}
		}
		result := deps.Town.Tick(ctx, w)
		if !result.Done {
			return stepResult{}
		}
		if result.Status == "complete" {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: result.Reason}
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) tickStashPersonal(ctx context.Context, deps pipelineReturnDeps, step string, w world.State) stepResult {
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

func tickPersonalStashWorkflow(ctx context.Context, deps pipelineReturnDeps, step string, w world.State) stepResult {
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

func tickRunTownPortal(deps pipelineReturnDeps, w world.State) stepResult {
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
	return c.tickEnterTownPortalWithDeps(ctx, narrowReturnDeps(deps), w, now)
}

func (c *runPipeline) tickEnterTownPortalWithDeps(ctx context.Context, deps pipelineReturnDeps, w world.State, now time.Time) stepResult {
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
	if c.ret.portalRecoveryPending {
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
	return c.tickTownNormalization(ctx, narrowReturnDeps(deps), step, w, now, stepStartedAt)
}

func (c *runPipeline) tickTownNormalization(ctx context.Context, deps pipelineReturnDeps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
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
		if !c.ret.egressStarted {
			if err := deps.TownEgress.Start(c.effectiveDefinition().ReturnOrigin, w); err != nil {
				return stepResult{failed: true, reason: townEgressFailureReason(err)}
			}
			c.ret.egressStarted = true
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
		return c.tickWaitHubArea(w)
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) tickWaitHubArea(w world.State) stepResult {
	// Area 0, Loading, and a confirmed identity with seed 0 are the waypoint
	// fade. The Paladin Mephisto return failed on that single InGame tick
	// while Necro runs already showed Rogue Encampment in the same 100 ms window.
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID == world.None {
		return stepResult{}
	}
	if w.Identity.Valid && w.Identity.MapSeed == 0 {
		return stepResult{}
	}
	if w.Area.ID == world.RogueEncampment {
		return stepResult{complete: true}
	}
	if w.Area.ID == c.originTownArea() {
		return stepResult{}
	}
	return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
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

func (c *runPipeline) resetPortalEntryRecovery() {
	c.ret.portalRecovered = nil
	c.clearPortalRecoveryPending()
}

func (c *runPipeline) clearPortalRecoveryPending() {
	c.ret.portalRecoveryPending = false
	c.ret.portalRecoveryUnitID = 0
	c.ret.portalRecoveryPos = world.Position{}
	c.ret.portalRecoveryTeleportSent = false
	c.ret.portalRecoveryAt = time.Time{}
	c.ret.portalRecoverySnapshot = time.Time{}
}

// beginPortalEntryRecovery arms one distance-ignoring portal teleport after too_far
// or hover_not_found, so Bone-Prison and similar blockers can be escaped once per UnitID.
func (c *runPipeline) beginPortalEntryRecovery(unitID uint32, pos world.Position) bool {
	if unitID == 0 || c.ret.portalRecovered[unitID] {
		return false
	}
	if c.ret.portalRecovered == nil {
		c.ret.portalRecovered = make(map[uint32]bool)
	}
	c.ret.portalRecovered[unitID] = true
	c.ret.portalRecoveryPending = true
	c.ret.portalRecoveryUnitID = unitID
	c.ret.portalRecoveryPos = pos
	c.ret.portalRecoveryTeleportSent = false
	c.ret.portalRecoveryAt = time.Time{}
	c.ret.portalRecoverySnapshot = time.Time{}
	return true
}

// tickPortalEntryRecovery teleports onto the failed portal once, settles, then retries entry.
func (c *runPipeline) tickPortalEntryRecovery(deps pipelineReturnDeps, w world.State, now time.Time) stepResult {
	portal, found := w.NearestObject(world.ObjectKindTownPortal)
	if !found {
		c.clearPortalRecoveryPending()
		return stepResult{}
	}
	if c.ret.portalRecoveryUnitID != 0 && portal.UnitID != c.ret.portalRecoveryUnitID {

		if match, ok := findTownPortalByUnitID(w, c.ret.portalRecoveryUnitID); ok {
			portal = match
		}
	}
	c.ret.portalRecoveryUnitID = portal.UnitID
	c.ret.portalRecoveryPos = portal.Position
	if deps.Combat == nil {
		c.clearPortalRecoveryPending()
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if !c.ret.portalRecoveryTeleportSent {
		if !lootRepositionReady(now, w.At, c.ret.portalRecoveryAt, c.ret.portalRecoverySnapshot) {
			return stepResult{}
		}
		sent, err := deps.Combat.TeleportToward(now, w.Player, portal.Position, 0)
		if err != nil {
			c.clearPortalRecoveryPending()
			return stepResult{failed: true, reason: "portal_recovery_teleport_failed"}
		}
		if sent {
			c.ret.portalRecoveryTeleportSent = true
			c.ret.portalRecoveryAt = now
			c.ret.portalRecoverySnapshot = w.At
		}
		return stepResult{}
	}
	if !lootRepositionReady(now, w.At, c.ret.portalRecoveryAt, c.ret.portalRecoverySnapshot) {
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
