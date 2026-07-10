package tasks

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// CountessPhaseTravelMarsh selects the Town -> Black Marsh travel phase.
	CountessPhaseTravelMarsh = "travel-marsh"
	// CountessPhaseTravelCellar5 selects best-effort travel to Tower Cellar Level 5.
	CountessPhaseTravelCellar5 = "travel-cellar5"
	// CountessPhaseKillCountess selects the Cellar-5 Countess kill phase.
	CountessPhaseKillCountess = "kill-countess"
	// CountessPhaseLootCountess selects the Cellar-5 Countess loot phase.
	CountessPhaseLootCountess = "loot-countess"
	// CountessPhaseStashPersonal selects transfer-free Act-1 personal-stash navigation and opening.
	CountessPhaseStashPersonal = "stash-personal"

	countessStepPrecheck        = "precheck"
	countessStepAcquireTownWP   = "acquire_town_waypoint"
	countessStepOpenWaypoint    = "open_waypoint"
	countessStepSelectMarsh     = "select_black_marsh"
	countessStepWaitBlackMarsh  = "wait_black_marsh"
	countessStepFindTower       = "find_tower"
	countessStepEnterCellar1    = "enter_cellar_1"
	countessStepEnterCellar2    = "enter_cellar_2"
	countessStepEnterCellar3    = "enter_cellar_3"
	countessStepEnterCellar4    = "enter_cellar_4"
	countessStepEnterCellar5    = "enter_cellar_5"
	countessStepLocateCountess  = "locate_countess"
	countessStepEngageCountess  = "engage_countess"
	countessStepWaitForDrops    = "wait_for_drops"
	countessStepScanLoot        = "scan_loot"
	countessStepPickLoot        = "pick_loot"
	countessStepCastTownPortal  = "cast_town_portal"
	countessStepEnterTownPortal = "enter_town_portal"
	countessStepWaitAct1Town    = "wait_act1_town"
	countessStepOpenStash       = "open_personal_stash"
	countessStepStashItems      = "stash_items"
	countessStepCloseStash      = "close_personal_stash"
	countessStepComplete        = "complete"

	selectMarshSettleDelay  = 500 * time.Millisecond
	dropStableTicks         = 3
	lootNoTargetStableTicks = 3
)

// countessRun executes the Countess stub or a selected Countess phase.
type countessRun struct {
	phase                  string
	combat                 CountessCombatConfig
	selectedOnce           bool
	navStarted             bool
	resumeAfterPrecheckSet bool
	resumeAfterPrecheck    string
	chestFallbackStarted   bool
	targetSeen             bool
	targetUnitID           uint32
	targetAbsentTicks      int
	dropStableTicks        int
	lootScanHasTarget      bool
	lootPickupActive       bool
	lootNoTargetTicks      int
}

func (c *countessRun) firstStep() string {
	return countessStepPrecheck
}

func (c *countessRun) nextStep(current string) string {
	if c.phase == CountessPhaseStashPersonal {
		switch current {
		case countessStepPrecheck:
			return countessStepOpenStash
		case countessStepOpenStash:
			return countessStepStashItems
		case countessStepStashItems:
			return countessStepCloseStash
		case countessStepCloseStash:
			return countessStepComplete
		case countessStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.phase == CountessPhaseKillCountess {
		switch current {
		case countessStepPrecheck:
			return countessStepLocateCountess
		case countessStepLocateCountess:
			return countessStepEngageCountess
		case countessStepEngageCountess:
			return ""
		default:
			return ""
		}
	}
	if c.phase == CountessPhaseLootCountess {
		switch current {
		case countessStepPrecheck:
			return countessStepWaitForDrops
		case countessStepWaitForDrops:
			return countessStepScanLoot
		case countessStepScanLoot:
			if c.lootScanHasTarget {
				return countessStepPickLoot
			}
			return countessStepCastTownPortal
		case countessStepPickLoot:
			return countessStepCastTownPortal
		case countessStepCastTownPortal:
			return countessStepEnterTownPortal
		case countessStepEnterTownPortal:
			return countessStepWaitAct1Town
		case countessStepWaitAct1Town:
			return countessStepOpenStash
		case countessStepOpenStash:
			return countessStepStashItems
		case countessStepStashItems:
			return countessStepCloseStash
		case countessStepCloseStash:
			return countessStepComplete
		case countessStepComplete:
			return ""
		default:
			return ""
		}
	}
	if c.isTravelPhase() {
		switch current {
		case countessStepPrecheck:
			if c.resumeAfterPrecheckSet {
				return c.resumeAfterPrecheck
			}
			return countessStepAcquireTownWP
		case countessStepAcquireTownWP:
			return countessStepOpenWaypoint
		case countessStepOpenWaypoint:
			return countessStepSelectMarsh
		case countessStepSelectMarsh:
			return countessStepWaitBlackMarsh
		case countessStepWaitBlackMarsh:
			if c.phase == CountessPhaseTravelCellar5 {
				return countessStepFindTower
			}
			return ""
		case countessStepFindTower:
			return countessStepEnterCellar1
		case countessStepEnterCellar1:
			return countessStepEnterCellar2
		case countessStepEnterCellar2:
			return countessStepEnterCellar3
		case countessStepEnterCellar3:
			return countessStepEnterCellar4
		case countessStepEnterCellar4:
			return countessStepEnterCellar5
		case countessStepEnterCellar5:
			return ""
		default:
			return ""
		}
	}
	switch current {
	case countessStepPrecheck:
		return countessStepAcquireTownWP
	case countessStepAcquireTownWP:
		return countessStepOpenWaypoint
	case countessStepOpenWaypoint:
		return countessStepSelectMarsh
	case countessStepSelectMarsh:
		return countessStepWaitBlackMarsh
	case countessStepWaitBlackMarsh:
		return countessStepFindTower
	case countessStepFindTower:
		return countessStepEnterCellar1
	case countessStepEnterCellar1:
		return countessStepEnterCellar2
	case countessStepEnterCellar2:
		return countessStepEnterCellar3
	case countessStepEnterCellar3:
		return countessStepEnterCellar4
	case countessStepEnterCellar4:
		return countessStepEnterCellar5
	case countessStepEnterCellar5:
		return countessStepLocateCountess
	case countessStepLocateCountess:
		return countessStepEngageCountess
	case countessStepEngageCountess:
		return countessStepWaitForDrops
	case countessStepWaitForDrops:
		return countessStepScanLoot
	case countessStepScanLoot:
		if c.lootScanHasTarget {
			return countessStepPickLoot
		}
		return countessStepCastTownPortal
	case countessStepPickLoot:
		return countessStepCastTownPortal
	case countessStepCastTownPortal:
		return countessStepEnterTownPortal
	case countessStepEnterTownPortal:
		return countessStepWaitAct1Town
	case countessStepWaitAct1Town:
		return countessStepOpenStash
	case countessStepOpenStash:
		return countessStepStashItems
	case countessStepStashItems:
		return countessStepCloseStash
	case countessStepCloseStash:
		return countessStepComplete
	case countessStepComplete:
		return ""
	default:
		return ""
	}
}

func (c *countessRun) usesTickTimeout(step string) bool {
	return false
}

func (c *countessRun) allowsNonInputTick(step string) bool {
	if step == countessStepWaitAct1Town && (c.phase == "" || c.phase == CountessPhaseLootCountess) {
		return true
	}
	return (c.isTravelPhase() || c.phase == "") && step == countessStepWaitBlackMarsh
}

func (c *countessRun) onStepEnter(step string) {
	c.selectedOnce = false
	c.navStarted = false
	if step == countessStepWaitForDrops {
		c.dropStableTicks = 0
	}
	if step == countessStepScanLoot {
		c.lootScanHasTarget = false
		c.lootNoTargetTicks = 0
	}
	if step == countessStepPickLoot {
		c.lootPickupActive = false
		c.lootNoTargetTicks = 0
	}
	if step == countessStepLocateCountess {
		c.chestFallbackStarted = false
		c.targetSeen = false
		c.targetUnitID = 0
		c.targetAbsentTicks = 0
	}
}

func (c *countessRun) onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult {
	if c.phase == CountessPhaseStashPersonal {
		return c.onStashPersonalTick(ctx, deps, step, w)
	}
	if c.phase == CountessPhaseKillCountess {
		return c.onKillCountessTick(ctx, deps, step, w, now)
	}
	if c.phase == CountessPhaseLootCountess {
		return c.onLootCountessTick(ctx, deps, step, w, now)
	}
	if c.isTravelPhase() {
		return c.onTravelMarshTick(ctx, deps, step, w, now, stepStartedAt)
	}
	if c.phase == "" {
		return c.onFullRunTick(ctx, deps, step, w, now, stepStartedAt)
	}
	return stepResult{failed: true, reason: "unknown_step"}
}

func (c *countessRun) onStashPersonalTick(ctx context.Context, deps Deps, step string, w world.State) stepResult {
	switch step {
	case countessStepPrecheck:
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
	case countessStepOpenStash, countessStepStashItems, countessStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case countessStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func tickPersonalStashWorkflow(ctx context.Context, deps Deps, step string, w world.State) stepResult {
	switch step {
	case countessStepOpenStash:
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
	case countessStepStashItems:
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
	case countessStepCloseStash:
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

func (c *countessRun) onFullRunTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID == world.RogueEncampment {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: "not_act1_town"}
	case countessStepAcquireTownWP, countessStepOpenWaypoint, countessStepSelectMarsh,
		countessStepWaitBlackMarsh, countessStepFindTower, countessStepEnterCellar1,
		countessStepEnterCellar2, countessStepEnterCellar3, countessStepEnterCellar4,
		countessStepEnterCellar5:
		return c.onTravelMarshTick(ctx, deps, step, w, now, stepStartedAt)
	case countessStepLocateCountess, countessStepEngageCountess:
		return c.onKillCountessTick(ctx, deps, step, w, now)
	case countessStepWaitForDrops, countessStepScanLoot, countessStepPickLoot:
		return c.onLootCountessTick(ctx, deps, step, w, now)
	case countessStepCastTownPortal:
		return tickCountessTownPortal(deps, w)
	case countessStepEnterTownPortal:
		return tickCountessEnterTownPortal(ctx, deps, w, now)
	case countessStepWaitAct1Town:
		return tickCountessWaitAct1Town(w)
	case countessStepOpenStash, countessStepStashItems, countessStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case countessStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *countessRun) onLootCountessTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID != world.TowerCellarLevel5 {
			return stepResult{failed: true, reason: "not_cellar_5"}
		}
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		return stepResult{complete: true}
	case countessStepWaitForDrops:
		if res := countessLootAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			c.dropStableTicks = 0
			return stepResult{}
		}
		c.dropStableTicks++
		if c.dropStableTicks >= dropStableTicks {
			return stepResult{complete: true}
		}
		return stepResult{}
	case countessStepScanLoot:
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		if res := countessLootAreaGuard(w); res.failed {
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
			c.lootScanHasTarget = false
			return stepResult{complete: true}
		}
		if scan.HasTarget {
			c.lootNoTargetTicks = 0
			c.lootScanHasTarget = true
			return stepResult{complete: true}
		}
		c.lootScanHasTarget = false
		c.lootNoTargetTicks++
		if c.lootNoTargetTicks >= lootNoTargetStableTicks {
			return stepResult{complete: true}
		}
		return stepResult{}
	case countessStepPickLoot:
		if deps.Loot == nil {
			return stepResult{failed: true, reason: "loot_actions_not_wired"}
		}
		if res := countessLootAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if !c.lootPickupActive {
			scan := deps.Loot.Scan(w)
			if scan.TelemetryFailed {
				return stepResult{failed: true, reason: "telemetry_failed"}
			}
			if scan.InventoryFull {
				return stepResult{complete: true}
			}
			if !scan.HasTarget {
				c.lootNoTargetTicks++
				if c.lootNoTargetTicks >= lootNoTargetStableTicks {
					return stepResult{complete: true}
				}
				return stepResult{}
			}
			c.lootNoTargetTicks = 0
			if err := deps.Loot.StartPickup(scan.NextTarget); err != nil {
				return stepResult{failed: true, reason: "loot_pickup_start_failed"}
			}
			c.lootPickupActive = true
		}
		res := deps.Loot.TickPickup(w, now)
		if !res.Done {
			return stepResult{}
		}
		switch res.Status {
		case LootPickupPickedUp, LootPickupMonsterNearby, LootPickupHoverNotFound,
			LootPickupTargetLost, LootPickupTargetUnstable, LootPickupTooFar, LootPickupFailed:
			c.lootPickupActive = false
			return stepResult{}
		case LootPickupInputBlocked, LootPickupProjectionFailed, LootPickupInvalidWorld, LootPickupTelemetryFailed:
			return stepResult{failed: true, reason: string(res.Status)}
		default:
			return stepResult{failed: true, reason: "loot_pickup_failed"}
		}
	case countessStepCastTownPortal:
		return tickCountessTownPortal(deps, w)
	case countessStepEnterTownPortal:
		return tickCountessEnterTownPortal(ctx, deps, w, now)
	case countessStepWaitAct1Town:
		return tickCountessWaitAct1Town(w)
	case countessStepOpenStash, countessStepStashItems, countessStepCloseStash:
		return tickPersonalStashWorkflow(ctx, deps, step, w)
	case countessStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func countessLootAreaGuard(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID != world.TowerCellarLevel5 {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func tickCountessTownPortal(deps Deps, w world.State) stepResult {
	if !w.Valid {
		return stepResult{failed: true, reason: "invalid_world"}
	}
	if w.Phase != world.GamePhaseInGame {
		return stepResult{failed: true, reason: "not_in_game"}
	}
	if deps.Actions == nil {
		return stepResult{failed: true, reason: "run_actions_not_wired"}
	}
	if err := deps.Actions.CastTownPortal(); err != nil {
		return stepResult{failed: true, reason: "town_portal_failed"}
	}
	return stepResult{complete: true}
}

func tickCountessEnterTownPortal(ctx context.Context, deps Deps, w world.State, now time.Time) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID == world.RogueEncampment {
		return stepResult{complete: true}
	}
	if w.Area.ID != world.TowerCellarLevel5 {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	if deps.Portal == nil {
		return stepResult{failed: true, reason: "town_portal_actions_not_wired"}
	}
	res := deps.Portal.Tick(ctx, w, now)
	switch res.Status {
	case pathing.TownPortalActionPending:
		return stepResult{}
	case pathing.TownPortalActionClicked:
		return stepResult{complete: true}
	case pathing.TownPortalActionNotFound:
		return stepResult{failed: true, reason: "town_portal_not_found"}
	default:
		return stepResult{failed: true, reason: "town_portal_enter_failed"}
	}
}

func tickCountessWaitAct1Town(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID == world.RogueEncampment {
		return stepResult{complete: true}
	}
	if w.Area.ID != world.TowerCellarLevel5 {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func (c *countessRun) onKillCountessTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID != world.TowerCellarLevel5 {
			return stepResult{failed: true, reason: "not_cellar_5"}
		}
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		return stepResult{complete: true}
	case countessStepLocateCountess:
		if res := countessKillAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if target, ok := findCountessTarget(w); ok {
			c.storeCountessTarget(target)
			return stepResult{complete: true}
		}
		return c.tickCountessSearchFallback(ctx, deps, w)
	case countessStepEngageCountess:
		if res := countessKillAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if !c.targetSeen {
			return stepResult{failed: true, reason: "target_not_set"}
		}
		target, visible := findMonsterByUnitID(w, c.targetUnitID)
		if !visible {
			c.targetAbsentTicks++
			if c.targetAbsentTicks >= c.combat.KillConfirmTicks {
				return stepResult{complete: true}
			}
			return stepResult{}
		}
		c.targetAbsentTicks = 0
		return c.tickEngageTarget(deps, w, target, now)
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func countessKillAreaGuard(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID != world.TowerCellarLevel5 {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func findCountessTarget(w world.State) (world.Monster, bool) {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID != world.TowerCellarLevel5 {
		return world.Monster{}, false
	}
	if target, ok := w.FindSuperUnique(world.DarkStalker); ok {
		return target, true
	}
	return w.FindSuperUnique(0)
}

func findMonsterByUnitID(w world.State, unitID uint32) (world.Monster, bool) {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID != world.TowerCellarLevel5 {
		return world.Monster{}, false
	}
	for _, m := range w.Monsters {
		if m.UnitID == unitID {
			return m, true
		}
	}
	return world.Monster{}, false
}

func (c *countessRun) storeCountessTarget(target world.Monster) {
	c.targetSeen = true
	c.targetUnitID = target.UnitID
	c.targetAbsentTicks = 0
}

func (c *countessRun) tickCountessSearchFallback(ctx context.Context, deps Deps, w world.State) stepResult {
	if !c.chestFallbackStarted {
		target, ok := countessSearchAnchor(w)
		if !ok {
			return stepResult{}
		}
		if deps.Pathing == nil {
			return stepResult{failed: true, reason: "pathing_not_wired"}
		}
		goal := pathing.Goal{Kind: pathing.GoalKindMoveToPosition, TargetPos: target}
		if err := deps.Pathing.Start(goal); err != nil {
			return stepResult{failed: true, reason: pathingStartFailureReason(err)}
		}
		c.chestFallbackStarted = true
	}
	if deps.Pathing == nil {
		return stepResult{failed: true, reason: "pathing_not_wired"}
	}
	if !deps.Pathing.Active() {
		return stepResult{}
	}
	res := deps.Pathing.Tick(ctx, w)
	if !res.Done {
		return stepResult{}
	}
	if res.Status == pathing.NavArrived {
		return stepResult{}
	}
	return stepResult{failed: true, reason: navigatorFailureReason(res)}
}

func countessSearchAnchor(w world.State) (world.Position, bool) {
	if chest, ok := w.NearestObject(world.ObjectKindGoodChest); ok {
		return chest.Position, true
	}
	if down, ok := w.NearestEntrance(world.EntranceKindTowerCellarDown); ok {
		return down.Position, true
	}
	return world.Position{}, false
}

func (c *countessRun) tickEngageTarget(deps Deps, w world.State, target world.Monster, now time.Time) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	distance := world.Distance(w.Player.Position, target.Position)
	var err error
	if distance > c.combat.RepositionDistanceTiles {
		err = deps.Combat.TeleportToward(now, w.Player.Position, target.Position, c.combat.EngageDistanceTiles)
	} else {
		err = deps.Combat.CastSkillAtWorld(now, c.combat.AttackSkillID, w.Player.Position, target.Position)
	}
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	return stepResult{}
}

func (c *countessRun) onTravelMarshTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if c.phase == CountessPhaseTravelCellar5 {
			if next, ok := countessTravelCellar5ResumeStep(w.Area.ID); ok {
				c.resumeAfterPrecheckSet = true
				c.resumeAfterPrecheck = next
				return stepResult{complete: true}
			}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		c.resumeAfterPrecheckSet = false
		c.resumeAfterPrecheck = ""
		return stepResult{complete: true}
	case countessStepAcquireTownWP:
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
	case countessStepOpenWaypoint:
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
	case countessStepSelectMarsh:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		if now.Sub(stepStartedAt) < selectMarshSettleDelay {
			return stepResult{}
		}
		return c.selectBlackMarsh(ctx, deps, now)
	case countessStepWaitBlackMarsh:
		if w.Valid && w.Area.ID == world.BlackMarsh {
			return stepResult{complete: true}
		}
		return stepResult{}
	case countessStepFindTower, countessStepEnterCellar1, countessStepEnterCellar2, countessStepEnterCellar3, countessStepEnterCellar4, countessStepEnterCellar5:
		goal, ok := countessNavigationGoal(step)
		if !ok {
			return stepResult{failed: true, reason: "unknown_step"}
		}
		if res := countessNavigationSourceGuard(step, w, goal); res.failed || res.complete {
			return res
		}
		return c.tickNavigateArea(ctx, deps, w, goal)
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func countessTravelCellar5ResumeStep(area world.AreaID) (string, bool) {
	switch area {
	case world.BlackMarsh:
		return countessStepFindTower, true
	case world.ForgottenTower:
		return countessStepEnterCellar1, true
	case world.TowerCellarLevel1:
		return countessStepEnterCellar2, true
	case world.TowerCellarLevel2:
		return countessStepEnterCellar3, true
	case world.TowerCellarLevel3:
		return countessStepEnterCellar4, true
	case world.TowerCellarLevel4:
		return countessStepEnterCellar5, true
	case world.TowerCellarLevel5:
		return "", true
	default:
		return "", false
	}
}

func (c *countessRun) isTravelPhase() bool {
	return c.phase == CountessPhaseTravelMarsh || c.phase == CountessPhaseTravelCellar5
}

func (c *countessRun) selectBlackMarsh(ctx context.Context, deps Deps, _ time.Time) stepResult {
	if !c.selectedOnce {
		res := deps.Waypoint.SelectBlackMarsh(ctx)
		c.selectedOnce = true
		if res.Status != pathing.WaypointActionClicked {
			return stepResult{failed: true, reason: waypointFailureReason(res)}
		}
		return stepResult{complete: true}
	}
	return stepResult{complete: true}
}

func countessNavigationGoal(step string) (pathing.Goal, bool) {
	switch step {
	case countessStepFindTower:
		return pathing.Goal{
			Kind:        pathing.GoalKindMoveToArea,
			TargetArea:  world.ForgottenTower,
			ViaEntrance: world.EntranceKindWildernessToTower,
		}, true
	case countessStepEnterCellar1:
		return pathing.Goal{
			Kind:       pathing.GoalKindMoveToArea,
			TargetArea: world.TowerCellarLevel1,
		}, true
	case countessStepEnterCellar2:
		return countessCellarGoal(world.TowerCellarLevel2), true
	case countessStepEnterCellar3:
		return countessCellarGoal(world.TowerCellarLevel3), true
	case countessStepEnterCellar4:
		return countessCellarGoal(world.TowerCellarLevel4), true
	case countessStepEnterCellar5:
		return countessCellarGoal(world.TowerCellarLevel5), true
	default:
		return pathing.Goal{}, false
	}
}

func countessCellarGoal(target world.AreaID) pathing.Goal {
	return pathing.Goal{
		Kind:        pathing.GoalKindMoveToArea,
		TargetArea:  target,
		ViaEntrance: world.EntranceKindTowerCellarDown,
	}
}

func countessNavigationSourceGuard(step string, w world.State, goal pathing.Goal) stepResult {
	if !w.Valid || w.Area.ID == goal.TargetArea {
		return stepResult{}
	}
	source, ok := countessNavigationSource(step)
	if !ok {
		return stepResult{}
	}
	if w.Area.ID != source {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func countessNavigationSource(step string) (world.AreaID, bool) {
	switch step {
	case countessStepFindTower:
		return world.BlackMarsh, true
	case countessStepEnterCellar1:
		return world.ForgottenTower, true
	case countessStepEnterCellar2:
		return world.TowerCellarLevel1, true
	case countessStepEnterCellar3:
		return world.TowerCellarLevel2, true
	case countessStepEnterCellar4:
		return world.TowerCellarLevel3, true
	case countessStepEnterCellar5:
		return world.TowerCellarLevel4, true
	default:
		return 0, false
	}
}

func (c *countessRun) tickNavigateArea(ctx context.Context, deps Deps, w world.State, goal pathing.Goal) stepResult {
	if w.Valid && w.Area.ID == goal.TargetArea {
		return stepResult{complete: true}
	}
	if deps.Pathing == nil {
		return stepResult{failed: true, reason: "pathing_not_wired"}
	}
	if !c.navStarted {
		if err := deps.Pathing.Start(goal); err != nil {
			return stepResult{failed: true, reason: pathingStartFailureReason(err)}
		}
		c.navStarted = true
	}
	res := deps.Pathing.Tick(ctx, w)
	if !res.Done {
		return stepResult{}
	}
	if res.Status == pathing.NavArrived {
		return stepResult{complete: true}
	}
	return stepResult{failed: true, reason: navigatorFailureReason(res)}
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

type stepResult struct {
	complete bool
	failed   bool
	reason   string
}
