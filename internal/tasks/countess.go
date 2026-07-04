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

	countessStepPrecheck       = "precheck"
	countessStepArmed          = "armed"
	countessStepAcquireTownWP  = "acquire_town_waypoint"
	countessStepOpenWaypoint   = "open_waypoint"
	countessStepSelectMarsh    = "select_black_marsh"
	countessStepWaitBlackMarsh = "wait_black_marsh"
	countessStepFindTower      = "find_tower"
	countessStepEnterCellar1   = "enter_cellar_1"
	countessStepEnterCellar2   = "enter_cellar_2"
	countessStepEnterCellar3   = "enter_cellar_3"
	countessStepEnterCellar4   = "enter_cellar_4"
	countessStepEnterCellar5   = "enter_cellar_5"
	countessStepLocateCountess = "locate_countess"
	countessStepEngageCountess = "engage_countess"
	countessStepComplete       = "complete"

	selectMarshSettleDelay = 500 * time.Millisecond
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
}

func (c *countessRun) firstStep() string {
	return countessStepPrecheck
}

func (c *countessRun) nextStep(current string) string {
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
		return countessStepArmed
	case countessStepArmed:
		return countessStepComplete
	default:
		return ""
	}
}

func (c *countessRun) usesTickTimeout(step string) bool {
	return c.phase == "" && step == countessStepArmed
}

func (c *countessRun) allowsNonInputTick(step string) bool {
	return c.isTravelPhase() && step == countessStepWaitBlackMarsh
}

func (c *countessRun) onStepEnter(step string) {
	c.selectedOnce = false
	c.navStarted = false
	if step == countessStepLocateCountess {
		c.chestFallbackStarted = false
		c.targetSeen = false
		c.targetUnitID = 0
		c.targetAbsentTicks = 0
	}
}

func (c *countessRun) onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult {
	if c.phase == CountessPhaseKillCountess {
		return c.onKillCountessTick(ctx, deps, step, w, now)
	}
	if c.isTravelPhase() {
		return c.onTravelMarshTick(ctx, deps, step, w, now, stepStartedAt)
	}
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Area.IsTown() {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: "not_in_town"}
	case countessStepArmed:
		if ticksInStep >= 2 {
			return stepResult{complete: true}
		}
		return stepResult{}
	case countessStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
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
