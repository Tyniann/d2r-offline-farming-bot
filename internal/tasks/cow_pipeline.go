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
	cowStepPreflight       = "cow_preflight"
	cowStepTownReady       = "cow_town_ready_profile"
	cowStepAcquireWaypoint = "cow_acquire_town_waypoint"
	cowStepOpenWaypoint    = "cow_open_waypoint"
	cowStepSelectStony     = "cow_select_stony_field"
	cowStepWaitStony       = "cow_wait_stony_field"
	cowStepPlayLegRoute    = "cow_play_leg_acquisition"
	cowStepOpenWirt        = "cow_open_wirt"
	cowStepPickupLeg       = "cow_pickup_leg"
	cowStepCastReturnTP    = "cow_cast_return_portal"
	cowStepEnterReturnTP   = "cow_enter_return_portal"
	cowStepWaitRogue       = "cow_wait_rogue_encampment"
	cowStepBuyTome         = "cow_buy_recipe_tome"
	cowStepSafeFailure     = "cow_safe_setup_failure"
	cowStepSetupComplete   = "cow_setup_gate_complete"
	cowStepPortalRecipe    = "cow_portal_recipe"
	cowStepRecipeComplete  = "cow_recipe_gate_complete"
	cowStepSweep           = "cow_play_cow_sweep"
	cowStepSweepComplete   = "cow_sweep_gate_complete"
)

// cowPipeline is intentionally separate from runPipeline. Cow setup and the
// one irreversible portal recipe remain explicit before later combat states.
type cowPipeline struct {
	definition      RunDefinition
	config          RunConfig
	preflight       cowPreflight
	legRoute        runPipeline
	cowSweep        runPipeline
	cowHold         cowHoldExecutor
	legUnitID       uint32
	tomeUnitID      uint32
	cubeUnitID      uint32
	pickupStarted   bool
	pickupFinished  bool
	legMissingTicks int
	pendingFailure  string
}

func newCowPipeline(definition RunDefinition, cfg RunConfig) *cowPipeline {
	legRouteCombat := cfg.RouteCombat
	// Stony-/Tristram-Hausgeometrie besitzt noch kein autoritatives LOS-Modell.
	// Nur die private Setup-Route läuft deshalb combatfrei; die eingefrorene
	// Cow-Config bleibt für den späteren Cow-Sweep unverändert aktiv.
	legRouteCombat.Enabled = false
	return &cowPipeline{
		definition: definition, config: cfg, preflight: cowPreflight{config: cfg.Cow},
		legRoute: runPipeline{definition: definition, core: pipelineCoreState{routeID: cfg.SetupRouteID, combat: cfg.Combat, routeCombat: legRouteCombat, suppressRouteLoot: true}},
		cowSweep: runPipeline{definition: definition, core: pipelineCoreState{routeID: cfg.RouteID, combat: cfg.Combat, routeCombat: cfg.RouteCombat, requireTerminalSafe: true}},
		cowHold:  newCowHoldExecutor(cfg.RouteCombat),
	}
}

func (c *cowPipeline) firstStep() string { return cowStepPreflight }

func (c *cowPipeline) nextStep(step string) string {
	switch step {
	case cowStepPreflight:
		return cowStepTownReady
	case cowStepTownReady:
		return cowStepAcquireWaypoint
	case cowStepAcquireWaypoint:
		return cowStepOpenWaypoint
	case cowStepOpenWaypoint:
		return cowStepSelectStony
	case cowStepSelectStony:
		return cowStepWaitStony
	case cowStepWaitStony:
		return cowStepPlayLegRoute
	case cowStepPlayLegRoute:
		return cowStepOpenWirt
	case cowStepOpenWirt:
		if c.pendingFailure != "" {
			return cowStepCastReturnTP
		}
		return cowStepPickupLeg
	case cowStepPickupLeg:
		return cowStepCastReturnTP
	case cowStepCastReturnTP:
		return cowStepEnterReturnTP
	case cowStepEnterReturnTP:
		return cowStepWaitRogue
	case cowStepWaitRogue:
		if c.pendingFailure != "" {
			return cowStepSafeFailure
		}
		return cowStepBuyTome
	case cowStepBuyTome:
		return cowStepSetupComplete
	case cowStepSetupComplete:
		return cowStepPortalRecipe
	case cowStepPortalRecipe:
		return cowStepRecipeComplete
	case cowStepRecipeComplete:
		return cowStepSweep
	case cowStepSweep:
		return cowStepSweepComplete
	case cowStepSweepComplete:
		return pipelineStepCastTownPortal
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash,
		pipelineStepPrepareTown, pipelineStepComplete:
		return c.cowSweep.nextStep(step)
	case cowStepSafeFailure:
		return ""
	default:
		return ""
	}
}

func (c *cowPipeline) usesTickTimeout(step string) bool {
	return step == cowStepPlayLegRoute || step == cowStepSweep
}

func (c *cowPipeline) timeoutReason(step string) string {
	switch step {
	case cowStepPreflight:
		return CowReasonCapabilityMissing
	case cowStepOpenWirt:
		return "cow_wirt_unavailable"
	case cowStepPickupLeg:
		return "cow_leg_pickup_failed"
	case cowStepBuyTome:
		return "cow_tome_purchase_failed"
	case cowStepPortalRecipe:
		return "cow_portal_recipe_timeout"
	case cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue:
		return "cow_return_portal_failed"
	default:
		return "timeout"
	}
}

func (c *cowPipeline) allowsNonInputTick(step string) bool {
	return step == cowStepWaitStony || step == cowStepPlayLegRoute || step == cowStepEnterReturnTP || step == cowStepWaitRogue ||
		step == cowStepPortalRecipe || step == cowStepSweep || c.cowSweep.allowsNonInputTick(step)
}

func (c *cowPipeline) blocksAutomaticInput(string) bool { return true }

func (c *cowPipeline) handlesResources(string) bool { return false }

func (c *cowPipeline) onStepEnter(step string) {
	if step == cowStepPlayLegRoute {
		c.legRoute.onStepEnter(pipelineStepPlayRoute)
	}
	if step == cowStepSweep {
		c.cowSweep.onStepEnter(pipelineStepPlayRoute)
	}
	if step == cowStepPickupLeg {
		c.pickupStarted, c.pickupFinished = false, false
		c.legMissingTicks = 0
	}
	if step == pipelineStepCastTownPortal || step == pipelineStepEnterTownPortal || step == pipelineStepWaitOriginTown ||
		step == pipelineStepOpenStash || step == pipelineStepStashItems || step == pipelineStepCloseStash ||
		step == pipelineStepPrepareTown || step == pipelineStepComplete {
		c.cowSweep.onStepEnter(step)
	}
}

func (c *cowPipeline) resetGeneration() {
	c.preflight.reset()
	c.legRoute.resetGeneration()
	c.cowSweep.resetGeneration()
	c.cowHold.ResetRouteClear()
	c.legUnitID = 0
	c.tomeUnitID = 0
	c.cubeUnitID = 0
	c.pickupStarted = false
	c.pickupFinished = false
	c.legMissingTicks = 0
	c.pendingFailure = ""
}

func (c *cowPipeline) onTick(ctx context.Context, deps Deps, step string, state world.State, now, stepStartedAt time.Time, _ int) stepResult {
	switch step {
	case cowStepPreflight:
		width, height := 0, 0
		if deps.Input != nil {
			if win, ok := deps.Input.Window(); ok {
				width, height = win.ClientWidth, win.ClientHeight
			}
		}
		done, reason := c.preflight.tick(state, c.config.SetupRouteID, c.config.RouteID, width, height)
		if !done {
			return stepResult{}
		}
		if reason != "" {
			return stepResult{failed: true, reason: reason}
		}
		for _, item := range state.InventoryItems() {
			if item.Code == "box" {
				c.cubeUnitID = item.UnitID
				break
			}
		}
		if c.cubeUnitID == 0 {
			return stepResult{failed: true, reason: CowReasonCubeMissing}
		}
		return stepResult{complete: true}
	case cowStepTownReady:
		if deps.Profile == nil {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		result := deps.Profile.TickHook(ctx, profile.HookTownReady, state, profile.EncounterTarget{}, now)
		switch result.Status {
		case profile.StatusComplete:
			return stepResult{complete: true}
		case profile.StatusFailed:
			return stepResult{failed: true, reason: result.Reason}
		default:
			return stepResult{}
		}
	case cowStepAcquireWaypoint:
		if deps.TownWalk == nil {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		result := deps.TownWalk.TickAct1Waypoint(ctx, state)
		if !result.Done {
			return stepResult{}
		}
		if result.Status != pathing.TownWalkWaypointVisible {
			return stepResult{failed: true, reason: result.Reason}
		}
		return stepResult{complete: true}
	case cowStepOpenWaypoint:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		result := deps.Waypoint.TickTownWaypoint(ctx, state)
		if result.Status == pathing.WaypointActionPending {
			return stepResult{}
		}
		if result.Status != pathing.WaypointActionClicked {
			return stepResult{failed: true, reason: waypointFailureReason(result)}
		}
		return stepResult{complete: true}
	case cowStepSelectStony:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		if !stepStartedAt.IsZero() && now.Sub(stepStartedAt) < waypointSelectSettleDelay {
			return stepResult{}
		}
		result := deps.Waypoint.SelectWaypointTarget(ctx, state, pathing.WaypointTargetStonyField, now)
		if result.Status == pathing.WaypointActionPending {
			return stepResult{}
		}
		if result.Status != pathing.WaypointActionClicked {
			return stepResult{failed: true, reason: waypointFailureReason(result)}
		}
		return stepResult{complete: true}
	case cowStepWaitStony:
		if state.Valid && state.Area.ID == world.StonyField {
			return stepResult{complete: true}
		}
		return stepResult{}
	case cowStepPlayLegRoute:
		return c.legRoute.tickTravel(ctx, narrowTravelDeps(deps), pipelineStepPlayRoute, state, now, stepStartedAt)
	case cowStepOpenWirt:
		if deps.Cow == nil {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		result := deps.Cow.TickWirt(ctx, state)
		if !result.Done {
			return stepResult{}
		}
		if result.Reason != "" {
			c.pendingFailure = result.Reason
			return stepResult{complete: true}
		}
		c.legUnitID = result.UnitID
		if err := emitCowRecipeProgress(deps, now, cowStepOpenWirt, "leg_acquired", c.legUnitID, "leg"); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{complete: true}
	case cowStepPickupLeg:
		return c.tickLegPickup(deps, state, now)
	case cowStepCastReturnTP:
		result := tickRunTownPortal(narrowReturnDeps(deps), state)
		if result.failed {
			return stepResult{failed: true, reason: "cow_return_portal_failed"}
		}
		return result
	case cowStepEnterReturnTP:
		if state.Valid && state.Area.ID == world.RogueEncampment {
			return stepResult{complete: true}
		}
		if !state.Valid || state.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if deps.Portal == nil {
			return stepResult{failed: true, reason: CowReasonReturnPortalUnavailable}
		}
		result := deps.Portal.Tick(ctx, state, now)
		if result.Status == pathing.TownPortalActionPending {
			return stepResult{}
		}
		if result.Status != pathing.TownPortalActionClicked {
			return stepResult{failed: true, reason: "cow_return_portal_failed"}
		}
		return stepResult{complete: true}
	case cowStepWaitRogue:
		if state.Valid && state.Area.ID == world.RogueEncampment {
			return stepResult{complete: true}
		}
		return stepResult{}
	case cowStepBuyTome:
		if deps.Cow == nil {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		result := deps.Cow.TickTome(ctx, state)
		if !result.Done {
			return stepResult{}
		}
		if result.Reason != "" || result.UnitID == 0 {
			return stepResult{failed: true, reason: result.Reason}
		}
		c.tomeUnitID = result.UnitID
		if err := emitCowRecipeProgress(deps, now, cowStepBuyTome, "tome_purchased", c.tomeUnitID, "tbk"); err != nil {
			return stepResult{failed: true, reason: "telemetry_failed"}
		}
		return stepResult{complete: true}
	case cowStepSafeFailure:
		return stepResult{failed: true, reason: c.pendingFailure}
	case cowStepSetupComplete:
		return stepResult{complete: true}
	case cowStepPortalRecipe:
		if deps.CowRecipe == nil || c.legUnitID == 0 || c.tomeUnitID == 0 || c.cubeUnitID == 0 {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		result := deps.CowRecipe.Tick(state, now, c.legUnitID, c.tomeUnitID, c.cubeUnitID)
		if result.ProgressKind != "" {
			if err := c.emitRecipeResultProgress(deps, now, result); err != nil {
				return stepResult{failed: true, reason: "telemetry_failed"}
			}
		}
		if !result.Done {
			return stepResult{}
		}
		if result.Reason != "" {
			return stepResult{failed: true, reason: result.Reason}
		}
		return stepResult{complete: true}
	case cowStepRecipeComplete:
		return stepResult{complete: true}
	case cowStepSweep:
		clear, ok := deps.RouteClear.(CowRouteClearExecutor)
		if !ok {
			return stepResult{failed: true, reason: CowReasonCapabilityMissing}
		}
		c.cowHold.bind(clear)
		sweepDeps := deps
		sweepDeps.RouteClear = &c.cowHold
		return c.cowSweep.tickTravel(ctx, narrowTravelDeps(sweepDeps), pipelineStepPlayRoute, state, now, stepStartedAt)
	case cowStepSweepComplete:
		return stepResult{complete: true}
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash,
		pipelineStepPrepareTown, pipelineStepComplete:
		// The Cow-specific setup and sweep end here. Reuse the proven Act-1
		// return, personal-stash and Town-handoff implementation so the two
		// route roles still finish inside one Runner generation.
		return c.cowSweep.onRunTick(ctx, deps, step, state, now, stepStartedAt)
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *cowPipeline) emitRecipeResultProgress(deps Deps, now time.Time, result CowSetupActionResult) error {
	switch result.ProgressKind {
	case "ingredients_confirmed", "transmute_sent", "ingredients_consumed":
		if err := emitCowRecipeProgress(deps, now, cowStepPortalRecipe, result.ProgressKind, c.legUnitID, "leg"); err != nil {
			return err
		}
		return emitCowRecipeProgress(deps, now, cowStepPortalRecipe, result.ProgressKind, c.tomeUnitID, "tbk")
	case "portal_confirmed", "area_39_confirmed":
		return emitCowRecipeProgress(deps, now, cowStepPortalRecipe, result.ProgressKind, result.UnitID, "")
	default:
		return nil
	}
}

func emitCowRecipeProgress(deps Deps, now time.Time, step, progress string, unitID uint32, code string) error {
	if deps.Telemetry == nil {
		return nil
	}
	return deps.Telemetry.Emit(telemetry.Event{
		Timestamp: now, Event: telemetry.CowRecipeProgress, Step: step,
		Stage: telemetry.HistoryStageTravel, ProgressKind: progress, UnitID: unitID, Code: code,
	})
}

func (c *cowPipeline) tickLegPickup(deps Deps, state world.State, now time.Time) stepResult {
	if deps.Loot == nil || c.legUnitID == 0 {
		c.pendingFailure = "cow_leg_pickup_failed"
		return stepResult{complete: true}
	}
	// Once a leg pickup started, its executor owns the final inventory
	// verification. Checking the inventory first would complete this step while
	// leaving that executor active, which blocks the first regular Cow-Sweep
	// pickup later in the same run.
	if c.pickupStarted {
		result := deps.Loot.TickPickup(state, now)
		if !result.Done {
			return stepResult{}
		}
		if result.Status != LootPickupPickedUp {
			c.pendingFailure = "cow_leg_pickup_failed"
			return stepResult{complete: true}
		}
		c.pickupFinished = true
		return stepResult{complete: true}
	}
	if item, ok := state.FindItemByUnitID(c.legUnitID); ok && item.Code == "leg" && item.Location == world.ItemLocationInventory && item.PlayerOwned && item.Page == 0 {
		return stepResult{complete: true}
	}
	if c.pickupFinished {
		c.legMissingTicks++
		if c.legMissingTicks >= 3 {
			c.pendingFailure = "cow_leg_pickup_unconfirmed"
			return stepResult{complete: true}
		}
		return stepResult{}
	}
	item, ok := state.FindItemByUnitID(c.legUnitID)
	if !ok || item.Code != "leg" || item.Location != world.ItemLocationGround {
		c.legMissingTicks++
		if c.legMissingTicks >= 3 {
			c.pendingFailure = "cow_leg_pickup_failed"
			return stepResult{complete: true}
		}
		return stepResult{}
	}
	c.legMissingTicks = 0
	target := LootTarget{UnitID: item.UnitID, TxtFileNo: item.TxtFileNo, Code: item.Code, Name: item.Name, Quality: item.Quality, Position: item.Position, AreaID: state.Area.ID}
	if err := deps.Loot.StartCowLegPickup(target); err != nil {
		c.pendingFailure = "cow_leg_pickup_failed"
		return stepResult{complete: true}
	}
	c.pickupStarted = true
	return stepResult{}
}
