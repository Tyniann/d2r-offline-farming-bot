package tasks

import (
	"context"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// These fixtures preserve coverage for the retired exploratory Cellar walker.
// Production runs now use only the layout-bound route player.
const (
	pipelineStepFindTower    = "find_tower"
	pipelineStepEnterCellar1 = "enter_cellar_1"
	pipelineStepEnterCellar2 = "enter_cellar_2"
	pipelineStepEnterCellar3 = "enter_cellar_3"
	pipelineStepEnterCellar4 = "enter_cellar_4"
	pipelineStepEnterCellar5 = "enter_cellar_5"
)

func countessNavigationGoal(step string) (pathing.Goal, bool) {
	switch step {
	case pipelineStepFindTower:
		return pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: world.ForgottenTower, ViaEntrance: world.EntranceKindWildernessToTower}, true
	case pipelineStepEnterCellar1:
		return pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: world.TowerCellarLevel1}, true
	case pipelineStepEnterCellar2:
		return countessCellarGoal(world.TowerCellarLevel2), true
	case pipelineStepEnterCellar3:
		return countessCellarGoal(world.TowerCellarLevel3), true
	case pipelineStepEnterCellar4:
		return countessCellarGoal(world.TowerCellarLevel4), true
	case pipelineStepEnterCellar5:
		return countessCellarGoal(world.TowerCellarLevel5), true
	default:
		return pathing.Goal{}, false
	}
}

func countessCellarGoal(target world.AreaID) pathing.Goal {
	return pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: target, ViaEntrance: world.EntranceKindTowerCellarDown}
}

func countessNavigationSourceGuard(step string, w world.State, goal pathing.Goal) stepResult {
	if !w.Valid || w.Area.ID == goal.TargetArea {
		return stepResult{}
	}
	source, ok := countessNavigationSource(step)
	if ok && w.Area.ID != source {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func countessNavigationSource(step string) (world.AreaID, bool) {
	switch step {
	case pipelineStepFindTower:
		return world.BlackMarsh, true
	case pipelineStepEnterCellar1:
		return world.ForgottenTower, true
	case pipelineStepEnterCellar2:
		return world.TowerCellarLevel1, true
	case pipelineStepEnterCellar3:
		return world.TowerCellarLevel2, true
	case pipelineStepEnterCellar4:
		return world.TowerCellarLevel3, true
	case pipelineStepEnterCellar5:
		return world.TowerCellarLevel4, true
	default:
		return world.None, false
	}
}

func (c *runPipeline) tickNavigateArea(ctx context.Context, deps Deps, w world.State, goal pathing.Goal) stepResult {
	if w.Valid && w.Area.ID == goal.TargetArea {
		return stepResult{complete: true}
	}
	if deps.Pathing == nil {
		return stepResult{failed: true, reason: "pathing_not_wired"}
	}
	if !c.travel.navStarted {
		if err := deps.Pathing.Start(goal); err != nil {
			return stepResult{failed: true, reason: pathingStartFailureReason(err)}
		}
		c.travel.navStarted = true
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
