package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// Chest blocker recovery is object-local and one-shot. These limits keep a
	// crowded hut from turning chest_sweep into unrestricted route combat.
	chestBlockerRadiusTiles          float64 = 12
	chestBlockerMaxActions                   = 12
	chestBlockerStableClearSnapshots         = 3
	chestBlockerTimeout                      = 6 * time.Second
	chestBlockerNoProgressTimeout            = 3 * time.Second
)

func (c *runPipeline) resetChestWork() {
	c.chest = pipelineChestState{}
}

func (c *runPipeline) rememberChestDone(unitID uint32) {
	if unitID == 0 {
		return
	}
	if c.chest.skipped == nil {
		c.chest.skipped = make(map[uint32]bool)
	}
	c.chest.skipped[unitID] = true
}

func (c *runPipeline) rememberChestHoverMiss(unitID uint32) {
	if unitID == 0 {
		return
	}
	if c.chest.hoverMissed == nil {
		c.chest.hoverMissed = make(map[uint32]bool)
	}
	c.chest.hoverMissed[unitID] = true
}

// releaseHoverMissesForLeftoverSweep gives objects that failed hover during
// route playback exactly one more try from the campfire. The leftover sweep
// stands somewhere else; the same 15-pixel spiral is not retried in a loop.
func (c *runPipeline) releaseHoverMissesForLeftoverSweep() {
	if c.chest.leftoverHoverRetry {
		return
	}
	c.chest.leftoverHoverRetry = true
	for id := range c.chest.hoverMissed {
		delete(c.chest.skipped, id)
	}
}

func (c *runPipeline) markChestSkipped(deps pipelineChestDeps, object world.Object, reason string) error {
	if object.UnitID == 0 || c.chest.skipped[object.UnitID] {
		return nil
	}
	c.rememberChestDone(object.UnitID)
	event := telemetry.ChestSkipped
	if object.Kind == world.ObjectKindRack {
		event = telemetry.RackSkipped
	}
	return c.emitChestOutcome(deps, event, object, reason)
}

func (c *runPipeline) markChestOpened(deps pipelineChestDeps, object world.Object) error {
	if object.UnitID == 0 {
		return nil
	}
	if c.chest.opened == nil {
		c.chest.opened = make(map[uint32]bool)
	}
	already := c.chest.opened[object.UnitID]
	if !already && object.Kind == world.ObjectKindSuperChest {
		c.chest.openedSuperChests++
	}
	c.chest.opened[object.UnitID] = true
	c.rememberChestDone(object.UnitID)
	if already {
		return nil
	}
	event := telemetry.ChestOpened
	if object.Kind == world.ObjectKindRack {
		event = telemetry.RackOperated
	}
	return c.emitChestOutcome(deps, event, object, "")
}

func (c *runPipeline) emitChestOutcome(deps pipelineChestDeps, event telemetry.EventName, object world.Object, reason string) error {
	if deps.Telemetry == nil {
		return nil
	}
	return deps.Telemetry.Emit(telemetry.Event{
		Event: event, DefinitionID: string(c.effectiveDefinition().ID), Step: pipelineStepChestSweep,
		Stage: telemetry.HistoryStageCombat, UnitID: object.UnitID, TxtFileNo: object.ID,
		Name: object.Name, Code: object.Kind.String(), Reason: reason,
		TargetX: object.Position.X, TargetY: object.Position.Y,
	})
}

func chestTelemetryFailed(err error) stepResult {
	if err == nil {
		return stepResult{}
	}
	return stepResult{failed: true, reason: "telemetry_failed"}
}

func (c *runPipeline) abandonChest(deps pipelineChestDeps, object world.Object, reason string) stepResult {
	if reason == "chest_hover_not_found" {
		c.rememberChestHoverMiss(object.UnitID)
	}
	if err := c.markChestSkipped(deps, object, reason); err != nil {
		return chestTelemetryFailed(err)
	}
	c.clearChestPin()
	c.chest.phase = chestPhaseIdle
	if deps.Chest != nil {
		deps.Chest.Reset()
	}
	return stepResult{}
}

func (c *runPipeline) completeChestOpen(deps pipelineChestDeps, object world.Object) stepResult {
	if err := c.markChestOpened(deps, object); err != nil {
		return chestTelemetryFailed(err)
	}
	if deps.Chest != nil {
		deps.Chest.Reset()
	}
	c.beginChestObjectLoot()
	return stepResult{}
}

func (c *runPipeline) completeUnknownModeAttempt(deps pipelineChestDeps, object world.Object) stepResult {
	if err := c.markChestSkipped(deps, object, "chest_mode_unknown_unconfirmed"); err != nil {
		return chestTelemetryFailed(err)
	}
	if deps.Chest != nil {
		deps.Chest.Reset()
	}
	// Mode cannot confirm the operate. The terminal UnitID set prevents a
	// second click while the normal drop window still catches delayed evidence.
	c.beginChestObjectLoot()
	return stepResult{}
}

func (c *runPipeline) observeEligibleChests(objects []world.Object) {
	if len(hutEligibleSuperChests(objects)) > 0 {
		c.chest.seenEligible = true
	}
}

func (c *runPipeline) clearChestPin() {
	c.chest.pin = world.Object{}
	c.chest.clicksOnPin = 0
	c.chest.keysAtClick = 0
	c.chest.groundAtClick = nil
	c.chest.settleTicks = 0
	c.chest.approachAttempts = 0
	c.chest.approachAt = time.Time{}
	c.chest.approachSnapshot = time.Time{}
	c.resetChestBlockerState()
}

func (c *runPipeline) resetChestBlockerState() {
	c.chest.blockerUnitID = 0
	c.chest.clearResume = ""
	c.chest.clearActions = 0
	c.chest.clearNoTargetTicks = 0
	c.chest.clearStartedAt = time.Time{}
	c.chest.clearLastActionAt = time.Time{}
}

func (c *runPipeline) beginChestObjectLoot() {
	c.clearChestPin()
	c.chest.phase = chestPhaseWaitDrops
	c.chest.dropWaitTicks = 0
	c.chest.lootNoTargetTicks = 0
	c.loot.lootPickupActive = false
	c.resetLootApproach()
}

func (c *runPipeline) finishChestObjectLoot() {
	c.clearChestPin()
	c.chest.phase = chestPhaseIdle
	c.chest.dropWaitTicks = 0
	c.chest.lootNoTargetTicks = 0
}

func (c *runPipeline) finishChestCluster() {
	c.clearChestPin()
	c.chest.clusterChest = world.Object{}
	c.chest.clusterActive = false
	c.chest.phase = chestPhaseIdle
	c.chest.dropWaitTicks = 0
	c.chest.lootNoTargetTicks = 0
}

func (c *runPipeline) tickChestSweep(ctx context.Context, deps pipelineChestDeps, w world.State, now time.Time) stepResult {
	c.releaseHoverMissesForLeftoverSweep()
	if !w.Valid {
		return stepResult{failed: true, reason: "invalid_world"}
	}
	if w.Phase != world.GamePhaseInGame {
		return stepResult{failed: true, reason: "not_in_game"}
	}
	if c.effectiveDefinition().RouteTerminalArea != 0 && w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
	}
	handled, result := c.tickChestWork(ctx, deps, w, now, chestSelectSweep)
	if result.failed {
		return result
	}
	if handled {
		return result
	}
	if !c.chest.seenEligible && c.chest.openedSuperChests == 0 {
		return stepResult{failed: true, reason: string(RunReasonChestSweepEmpty)}
	}
	return stepResult{complete: true}
}

func (c *runPipeline) tickRouteChestOperate(ctx context.Context, deps pipelineTravelDeps, w world.State, now time.Time) (bool, stepResult) {
	if !c.definition.HasCapability(RunCapabilityChestSweep) {
		return false, stepResult{}
	}
	if deps.Route != nil {
		if _, ok := deps.Route.Progress(w); !ok {
			return false, stepResult{}
		}
	}
	return c.tickChestWork(ctx, narrowChestDepsFromTravel(deps), w, now, chestSelectRoute)
}

func (c *runPipeline) tickChestWork(ctx context.Context, deps pipelineChestDeps, w world.State, now time.Time, mode chestSelectMode) (bool, stepResult) {
	c.observeEligibleChests(w.Objects)
	switch c.chest.phase {
	case chestPhaseClearBlocker:
		if err := c.holdChestRoute(deps, w); err != nil {
			return true, stepResult{failed: true, reason: err.Error()}
		}
		return true, c.tickChestBlockerClear(ctx, deps, w, now)
	case chestPhasePickup:
		if err := c.holdChestRoute(deps, w); err != nil {
			return true, stepResult{failed: true, reason: err.Error()}
		}
		return true, c.tickChestPickup(deps, w, now)
	case chestPhaseWaitDrops:
		if err := c.holdChestRoute(deps, w); err != nil {
			return true, stepResult{failed: true, reason: err.Error()}
		}
		return true, c.tickChestWaitDrops(deps, w, now)
	case chestPhaseSettle:
		if err := c.holdChestRoute(deps, w); err != nil {
			return true, stepResult{failed: true, reason: err.Error()}
		}
		return true, c.tickChestSettle(deps, w)
	case chestPhaseClick:
		if err := c.holdChestRoute(deps, w); err != nil {
			return true, stepResult{failed: true, reason: err.Error()}
		}
		return true, c.tickChestClick(deps, w, now)
	}

	target, ok := selectChestOperateTarget(w.Player.Position, w.Objects, c.chest.skipped, c.chest.clusterChest, mode)
	if !ok {
		if c.chest.clusterActive {
			if err := c.holdChestRoute(deps, w); err != nil {
				return true, stepResult{failed: true, reason: err.Error()}
			}
			c.finishChestCluster()
			return true, stepResult{}
		}
		return false, stepResult{}
	}
	if err := c.holdChestRoute(deps, w); err != nil {
		return true, stepResult{failed: true, reason: err.Error()}
	}
	return true, c.beginChestClick(deps, w, now, target)
}

func (c *runPipeline) holdChestRoute(deps pipelineChestDeps, w world.State) error {
	if deps.Route == nil {
		return nil
	}
	if err := deps.Route.Hold(w); err != nil {
		return errRouteHoldFailed{}
	}
	return nil
}

type errRouteHoldFailed struct{}

func (errRouteHoldFailed) Error() string { return string(RouteThreatReasonStateInvalid) }

func (c *runPipeline) beginChestClick(deps pipelineChestDeps, w world.State, now time.Time, target world.Object) stepResult {
	if deps.Chest == nil {
		return stepResult{failed: true, reason: "chest_not_wired"}
	}
	c.chest.pin = target
	c.chest.clusterActive = true
	if target.Kind == world.ObjectKindSuperChest {
		c.chest.clusterChest = target
	}
	c.chest.phase = chestPhaseClick
	c.chest.settleTicks = 0
	distance := world.Distance(w.Player.Position, target.Position)
	if distance > defaultLootPickupDistance {
		return c.tickChestApproach(deps, w, now, target)
	}
	c.chest.approachAttempts = 0
	return c.tickChestClick(deps, w, now)
}

func (c *runPipeline) tickChestApproach(deps pipelineChestDeps, w world.State, now time.Time, target world.Object) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	distance := world.Distance(w.Player.Position, target.Position)
	if distance <= defaultLootPickupDistance {
		c.chest.approachAttempts = 0
		return c.tickChestClick(deps, w, now)
	}
	if c.chest.approachAttempts >= chestApproachMaxAttempts && distance > defaultLootPickupDistance {
		return c.abandonChest(deps, target, "chest_approach_exhausted")
	}
	if !lootRepositionReady(now, w.At, c.chest.approachAt, c.chest.approachSnapshot) {
		return stepResult{}
	}
	sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
	if err != nil {
		return stepResult{failed: true, reason: "chest_approach_failed"}
	}
	if sent {
		c.chest.approachAttempts++
		c.chest.approachAt = now
		c.chest.approachSnapshot = w.At
	}
	return stepResult{}
}

func (c *runPipeline) tickChestClick(deps pipelineChestDeps, w world.State, now time.Time) stepResult {
	if deps.Chest == nil {
		return stepResult{failed: true, reason: "chest_not_wired"}
	}
	fresh, ok := objectByUnitID(w.Objects, c.chest.pin.UnitID)
	if !ok {
		return c.abandonChest(deps, c.chest.pin, "chest_unit_lost")
	}
	c.chest.pin = fresh
	distance := world.Distance(w.Player.Position, fresh.Position)
	if distance > defaultLootPickupDistance {
		return c.tickChestApproach(deps, w, now, fresh)
	}
	result := deps.Chest.Tick(w, fresh, defaultLootPickupDistance)
	if result.BlockerUnitID != 0 {
		c.chest.blockerUnitID = result.BlockerUnitID
	}
	switch result.Status {
	case ChestOperatePending:
		return stepResult{}
	case ChestOperateClicked:
		c.chest.clicksOnPin++
		c.chest.keysAtClick = inventoryKeyCount(w)
		c.chest.groundAtClick = groundItemIDs(w)
		c.chest.settleTicks = 0
		c.chest.phase = chestPhaseSettle
		deps.Chest.Reset()
		return stepResult{}
	case ChestOperateTooFar:
		return c.tickChestApproach(deps, w, now, fresh)
	case ChestOperateHoverNotFound:
		if c.startChestBlockerClear(deps, w, now, fresh) {
			return stepResult{}
		}
		return c.abandonChest(deps, fresh, "chest_hover_not_found")
	case ChestOperateFailed:
		skipped := c.abandonChest(deps, fresh, "chest_hover_not_found")
		if result.Reason != "" {
			return stepResult{failed: true, reason: result.Reason}
		}
		return skipped
	default:
		return stepResult{failed: true, reason: "chest_operate_failed"}
	}
}

func (c *runPipeline) startChestBlockerClear(deps pipelineChestDeps, w world.State, now time.Time, object world.Object) bool {
	if c.chest.blockerUnitID == 0 || deps.RouteClear == nil || object.UnitID == 0 || c.chest.clearAttempted[object.UnitID] {
		return false
	}
	c.chest.pin = object
	if _, found := c.findChestBlockerTarget(w); !found {
		return false
	}
	if c.chest.clearAttempted == nil {
		c.chest.clearAttempted = make(map[uint32]bool)
	}
	c.chest.clearAttempted[object.UnitID] = true
	if c.chest.clearResume == "" {
		c.chest.clearResume = chestPhaseClick
	}
	c.chest.phase = chestPhaseClearBlocker
	c.chest.clearActions = 0
	c.chest.clearNoTargetTicks = 0
	c.chest.clearStartedAt = now
	c.chest.clearLastActionAt = now
	if deps.Chest != nil {
		deps.Chest.Reset()
	}
	return true
}

func (c *runPipeline) startChestLootBlockerClear(deps pipelineChestDeps, w world.State, now time.Time, target LootTarget) bool {
	// Require a current monster hover so a leftover chest blocker ID cannot
	// start combat after an unrelated pickup miss.
	if target.UnitID == 0 || !w.Hover.IsHovered || w.Hover.UnitType != world.HoverUnitTypeMonster {
		return false
	}
	c.chest.blockerUnitID = w.Hover.UnitID
	c.chest.clearResume = chestPhasePickup
	object := world.Object{UnitID: target.UnitID, Position: target.Position, Name: target.Name}
	if c.startChestBlockerClear(deps, w, now, object) {
		return true
	}
	c.chest.clearResume = ""
	return false
}

func (c *runPipeline) tickChestBlockerClear(ctx context.Context, deps pipelineChestDeps, w world.State, now time.Time) stepResult {
	if deps.RouteClear == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		c.stopChestBlockerClear(deps)
		return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
	}
	// A loot pin is a ground item UnitID; looking it up in Objects would
	// abort the clear before Blessed Hammer runs.
	if c.chest.clearResume != chestPhasePickup {
		fresh, found := objectByUnitID(w.Objects, c.chest.pin.UnitID)
		if !found {
			c.stopChestBlockerClear(deps)
			return c.abandonChest(deps, c.chest.pin, "chest_unit_lost")
		}
		c.chest.pin = fresh
	}
	if c.chest.clearActions >= chestBlockerMaxActions ||
		now.Sub(c.chest.clearStartedAt) >= chestBlockerTimeout ||
		now.Sub(c.chest.clearLastActionAt) >= chestBlockerNoProgressTimeout {
		c.finishChestBlockerClear(deps)
		return stepResult{}
	}
	target, found := c.findChestBlockerTarget(w)
	if !found {
		c.chest.clearNoTargetTicks++
		deps.RouteClear.ResetRouteClear()
		if c.chest.clearNoTargetTicks >= chestBlockerStableClearSnapshots {
			c.finishChestBlockerClear(deps)
		}
		return stepResult{}
	}
	c.chest.clearNoTargetTicks = 0
	result := deps.RouteClear.TickRouteClear(ctx, profile.RouteClearRequest{
		RunID:        string(c.effectiveDefinition().ID),
		DefinitionID: c.core.combat.Profile,
		Player:       w.Player,
		Target:       target,
		Mode:         profile.RouteClearThreat,
		AssessmentAt: w.At,
	}, now)
	switch result.Status {
	case profile.StatusFailed:
		c.stopChestBlockerClear(deps)
		return stepResult{failed: true, reason: "combat_action_failed"}
	case profile.StatusAction:
		c.chest.clearActions++
		c.chest.clearLastActionAt = now
	}
	return stepResult{}
}

func (c *runPipeline) findChestBlockerTarget(w world.State) (world.Monster, bool) {
	// Hover may name a merc or corpse that is not in the hostile list. Prefer
	// that UnitID when it is a living hostile in range; otherwise the nearest
	// living monster within [chestBlockerRadiusTiles] of the pinned object.
	var nearest world.Monster
	nearestDistance := 0.0
	found := false
	for _, monster := range w.Monsters {
		distance := world.Distance(monster.Position, c.chest.pin.Position)
		if distance > chestBlockerRadiusTiles {
			continue
		}
		if monster.UnitID == c.chest.blockerUnitID {
			return monster, true
		}
		if !found || distance < nearestDistance {
			nearest = monster
			nearestDistance = distance
			found = true
		}
	}
	return nearest, found
}

func (c *runPipeline) finishChestBlockerClear(deps pipelineChestDeps) {
	resume := c.chest.clearResume
	itemID := c.chest.pin.UnitID
	c.stopChestBlockerClear(deps)
	if resume == chestPhasePickup {
		c.chest.phase = chestPhasePickup
		c.loot.lootPickupActive = false
		c.clearLootRecoveryPending()
		c.resetLootApproach()
		if deps.Loot != nil && itemID != 0 {
			deps.Loot.ClearSkippedPickup(itemID)
		}
	} else {
		c.chest.phase = chestPhaseClick
	}
	if deps.Chest != nil {
		deps.Chest.Reset()
	}
}

func (c *runPipeline) stopChestBlockerClear(deps pipelineChestDeps) {
	if deps.RouteClear != nil {
		deps.RouteClear.ResetRouteClear()
	}
	c.resetChestBlockerState()
}

func (c *runPipeline) tickChestSettle(deps pipelineChestDeps, w world.State) stepResult {
	c.chest.settleTicks++
	if c.chest.settleTicks < dropStableTicks {
		return stepResult{}
	}
	fresh, ok := objectByUnitID(w.Objects, c.chest.pin.UnitID)
	if ok {
		c.chest.pin = fresh
	}
	opened := objectIsOpened(c.chest.pin) || dropEvidenceNear(w, c.chest.pin, c.chest.groundAtClick)
	if opened {
		return c.completeChestOpen(deps, c.chest.pin)
	}
	if !c.chest.pin.ModeKnown {
		return c.completeUnknownModeAttempt(deps, c.chest.pin)
	}
	keysNow := inventoryKeyCount(w)
	if c.chest.pin.Kind == world.ObjectKindSuperChest && c.chest.keysAtClick == 0 {
		return c.abandonChest(deps, c.chest.pin, "chest_locked_no_key")
	}
	if keysNow < c.chest.keysAtClick {
		return c.completeChestOpen(deps, c.chest.pin)
	}
	if c.chest.clicksOnPin < 2 && c.chest.pin.ModeKnown {
		c.chest.phase = chestPhaseClick
		c.chest.settleTicks = 0
		if deps.Chest != nil {
			deps.Chest.Reset()
		}
		return stepResult{}
	}
	return c.abandonChest(deps, c.chest.pin, "chest_open_unconfirmed")
}

func (c *runPipeline) tickChestWaitDrops(deps pipelineChestDeps, w world.State, now time.Time) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		c.chest.dropWaitTicks = 0
		return stepResult{}
	}
	c.chest.dropWaitTicks++
	if c.chest.dropWaitTicks < dropStableTicks {
		return stepResult{}
	}
	c.chest.phase = chestPhasePickup
	c.chest.lootNoTargetTicks = 0
	return c.tickChestPickup(deps, w, now)
}

func (c *runPipeline) tickChestPickup(deps pipelineChestDeps, w world.State, now time.Time) stepResult {
	if deps.Loot == nil {
		c.finishChestObjectLoot()
		return stepResult{}
	}
	if c.loot.lootPickupActive {
		return c.tickChestLootPickup(deps, w, now)
	}
	if c.loot.lootRecoveryPending {
		return c.tickLootPickupRecovery(pipelineLootDeps{Combat: deps.Combat, Loot: deps.Loot}, w, now)
	}
	scan := deps.Loot.Scan(w)
	if scan.TelemetryFailed {
		return stepResult{failed: true, reason: "telemetry_failed"}
	}
	if scan.InventoryFull || !scan.HasTarget {
		c.chest.lootNoTargetTicks++
		if scan.InventoryFull || c.chest.lootNoTargetTicks >= lootNoTargetStableTicks {
			c.finishChestObjectLoot()
		}
		return stepResult{}
	}
	c.chest.lootNoTargetTicks = 0
	c.loot.lootApproachTarget = scan.NextTarget
	c.loot.lootApproachTargetSet = true
	target := scan.NextTarget
	if world.Distance(w.Player.Position, target.Position) > c.effectiveLootPickupDistance() {
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		if world.Distance(w.Player.Position, target.Position) <= lootApproachMaxDistanceTiles &&
			c.loot.lootApproachAttempts < lootRepositionMaxAttempts {
			if !lootRepositionReady(now, w.At, c.loot.lootApproachAt, c.loot.lootApproachSnapshot) {
				return stepResult{}
			}
			sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
			if err != nil {
				return stepResult{failed: true, reason: "loot_reposition_failed"}
			}
			if sent {
				c.loot.lootApproachAttempts++
				c.loot.lootApproachAt = now
				c.loot.lootApproachSnapshot = w.At
			}
			return stepResult{}
		}
	}
	if err := deps.Loot.StartPickup(target); err != nil {
		return stepResult{failed: true, reason: "loot_pickup_start_failed"}
	}
	c.loot.lootPickupActive = true
	c.resetLootApproach()
	return c.tickChestLootPickup(deps, w, now)
}

func (c *runPipeline) tickChestLootPickup(deps pipelineChestDeps, w world.State, now time.Time) stepResult {
	if deps.Loot == nil {
		c.loot.lootPickupActive = false
		return stepResult{}
	}
	res := deps.Loot.TickPickup(w, now)
	if !res.Done {
		return stepResult{}
	}
	switch res.Status {
	case LootPickupHoverNotFound, LootPickupFailed:
		// Monster hover on a gem is combat, not distance recovery. A teleport
		// retry would ClearSkippedPickup and burn another 15-probe spiral.
		c.loot.lootPickupActive = false
		c.resetLootApproach()
		if c.startChestLootBlockerClear(deps, w, now, res.Target) {
			return stepResult{}
		}
		return stepResult{}
	case LootPickupTooFar:
		c.loot.lootPickupActive = false
		c.resetLootApproach()
		if c.beginLootPickupRecovery(pipelineLootDeps{Combat: deps.Combat, Loot: deps.Loot}, res.Target, lootApproachMaxDistanceTiles) {
			return stepResult{}
		}
		return stepResult{}
	case LootPickupPickedUp, LootPickupMonsterNearby, LootPickupTargetLost, LootPickupTargetUnstable:
		c.loot.lootPickupActive = false
		c.resetLootApproach()
		return stepResult{}
	case LootPickupInputBlocked, LootPickupProjectionFailed, LootPickupInvalidWorld, LootPickupTelemetryFailed:
		return stepResult{failed: true, reason: string(res.Status)}
	default:
		return stepResult{failed: true, reason: "loot_pickup_failed"}
	}
}
