package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (c *runPipeline) onLootTick(ctx context.Context, deps Deps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	return c.tickLootAndReturn(ctx, deps, step, w, now, stepStartedAt)
}

func (c *runPipeline) tickLootAndReturn(ctx context.Context, deps Deps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	switch step {
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea,
		pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return c.tickReturn(ctx, narrowReturnDeps(deps), step, w, now, stepStartedAt)
	default:
		return c.tickLoot(narrowLootDeps(deps), step, w, now)
	}
}

func (c *runPipeline) tickLoot(deps pipelineLootDeps, step string, w world.State, now time.Time) stepResult {
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
			c.loot.dropStableTicks = 0
			return stepResult{}
		}
		c.loot.dropStableTicks++
		if c.loot.dropStableTicks >= dropStableTicks {
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
			c.loot.lootScanHasTarget = false
			return stepResult{complete: true}
		}
		if scan.HasTarget {
			c.loot.lootNoTargetTicks = 0
			c.loot.lootScanHasTarget = true
			return stepResult{complete: true}
		}
		c.loot.lootScanHasTarget = false
		c.loot.lootNoTargetTicks++
		if c.loot.lootNoTargetTicks >= lootNoTargetStableTicks {
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
		if !c.loot.lootPickupActive {
			if c.loot.lootRecoveryPending {
				if res := c.tickLootPickupRecovery(deps, w, now); res.failed || res.complete {
					return res
				}
				if !c.loot.lootPickupActive {
					return stepResult{}
				}
			} else {
				targetSelectedThisTick := false
				if !c.loot.lootApproachTargetSet {
					scan := deps.Loot.Scan(w)
					if scan.TelemetryFailed {
						return stepResult{failed: true, reason: "telemetry_failed"}
					}
					if scan.InventoryFull {
						return stepResult{complete: true}
					}
					if !scan.HasTarget {
						c.loot.lootNoTargetTicks++
						if c.loot.lootNoTargetTicks >= lootNoTargetStableTicks {
							return stepResult{complete: true}
						}
						return stepResult{}
					}
					c.loot.lootNoTargetTicks = 0
					c.loot.lootApproachTarget = scan.NextTarget
					c.loot.lootApproachTargetSet = true
					targetSelectedThisTick = true
				}
				target := c.loot.lootApproachTarget
				if !targetSelectedThisTick {
					var found bool
					target, found = currentLootTarget(w, target)
					if !found {

						c.resetLootApproach()
						return stepResult{}
					}
					c.loot.lootApproachTarget = target
				}
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
					if c.loot.lootApproachAttempts > 0 && !lootRepositionReady(now, w.At, c.loot.lootApproachAt, c.loot.lootApproachSnapshot) {
						return stepResult{}
					}

				}
				if err := deps.Loot.StartPickup(target); err != nil {
					return stepResult{failed: true, reason: "loot_pickup_start_failed"}
				}
				c.loot.lootPickupActive = true
				c.resetLootApproach()
			}
		}
		res := deps.Loot.TickPickup(w, now)
		if !res.Done {
			return stepResult{}
		}
		switch res.Status {
		case LootPickupHoverNotFound, LootPickupFailed, LootPickupTooFar:

			c.loot.lootPickupActive = false
			c.resetLootApproach()
			if c.beginLootPickupRecovery(deps, res.Target, lootApproachMaxDistanceTiles) {
				return stepResult{}
			}
			return stepResult{}
		case LootPickupPickedUp, LootPickupMonsterNearby,
			LootPickupTargetLost, LootPickupTargetUnstable:
			c.loot.lootPickupActive = false
			c.resetLootApproach()
			return stepResult{}
		case LootPickupInputBlocked, LootPickupProjectionFailed, LootPickupInvalidWorld, LootPickupTelemetryFailed:
			return stepResult{failed: true, reason: string(res.Status)}
		default:
			return stepResult{failed: true, reason: "loot_pickup_failed"}
		}
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

func (c *runPipeline) effectiveLootPickupDistance() float64 {
	if c.core.lootPickupDistanceTiles > 0 {
		return c.core.lootPickupDistanceTiles
	}
	return defaultLootPickupDistance
}

func (c *runPipeline) resetPostKillReposition() {
	c.loot.postKillTeleportAttempts = 0
	c.loot.postKillTeleportAt = time.Time{}
	c.loot.postKillTeleportSnapshot = time.Time{}
}

func (c *runPipeline) resetLootApproach() {
	c.loot.lootApproachTarget = LootTarget{}
	c.loot.lootApproachTargetSet = false
	c.loot.lootApproachAttempts = 0
	c.loot.lootApproachAt = time.Time{}
	c.loot.lootApproachSnapshot = time.Time{}
}

func (c *runPipeline) resetLootPickupRecovery() {
	c.loot.lootPickupRecovered = nil
	c.clearLootRecoveryPending()
}

func (c *runPipeline) clearLootRecoveryPending() {
	c.loot.lootRecoveryPending = false
	c.loot.lootRecoveryTarget = LootTarget{}
	c.loot.lootRecoveryTeleportSent = false
	c.loot.lootRecoveryAt = time.Time{}
	c.loot.lootRecoverySnapshot = time.Time{}
	c.loot.lootRecoveryMaxDistance = 0
}

// beginLootPickupRecovery arms one distance-ignoring item teleport after a
// recoverable pickup result. maxDistance keeps ordinary boss loot bounded while
// a threat-free combat-route Hold may recover any item it deliberately scanned.
func (c *runPipeline) beginLootPickupRecovery(deps pipelineLootDeps, target LootTarget, maxDistance float64) bool {
	if target.UnitID == 0 || c.loot.lootPickupRecovered[target.UnitID] {
		return false
	}
	if c.loot.lootPickupRecovered == nil {
		c.loot.lootPickupRecovered = make(map[uint32]bool)
	}
	c.loot.lootPickupRecovered[target.UnitID] = true
	if deps.Loot != nil {
		deps.Loot.ClearSkippedPickup(target.UnitID)
	}
	c.loot.lootRecoveryPending = true
	c.loot.lootRecoveryTarget = target
	c.loot.lootRecoveryTeleportSent = false
	c.loot.lootRecoveryAt = time.Time{}
	c.loot.lootRecoverySnapshot = time.Time{}
	c.loot.lootRecoveryMaxDistance = maxDistance
	return true
}

// tickLootPickupRecovery teleports onto the failed candidate once, settles, then restarts pickup.
func (c *runPipeline) tickLootPickupRecovery(deps pipelineLootDeps, w world.State, now time.Time) stepResult {
	target, found := currentLootTarget(w, c.loot.lootRecoveryTarget)
	if !found {
		c.clearLootRecoveryPending()
		return stepResult{}
	}
	c.loot.lootRecoveryTarget = target
	if deps.Combat == nil {
		c.clearLootRecoveryPending()
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	if !c.loot.lootRecoveryTeleportSent {
		maxDistance := c.loot.lootRecoveryMaxDistance
		if maxDistance <= 0 {
			maxDistance = lootApproachMaxDistanceTiles
		}
		if world.Distance(w.Player.Position, target.Position) > maxDistance {

			c.clearLootRecoveryPending()
			return stepResult{}
		}
		if !lootRepositionReady(now, w.At, c.loot.lootRecoveryAt, c.loot.lootRecoverySnapshot) {
			return stepResult{}
		}
		sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, 0)
		if err != nil {
			c.clearLootRecoveryPending()
			return stepResult{failed: true, reason: "loot_recovery_teleport_failed"}
		}
		if sent {
			c.loot.lootRecoveryTeleportSent = true
			c.loot.lootRecoveryAt = now
			c.loot.lootRecoverySnapshot = w.At
		}
		return stepResult{}
	}
	if !lootRepositionReady(now, w.At, c.loot.lootRecoveryAt, c.loot.lootRecoverySnapshot) {
		return stepResult{}
	}
	if err := deps.Loot.StartPickup(target); err != nil {
		c.clearLootRecoveryPending()
		return stepResult{failed: true, reason: "loot_pickup_start_failed"}
	}
	c.loot.lootPickupActive = true
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
