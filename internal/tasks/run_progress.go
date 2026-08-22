package tasks

import "github.com/Tyniann/d2r-offline-farming-bot/internal/world"

// RunProgress describes one stable, user-facing stage of an active run.
type RunProgress struct {
	StageCode string
	Params    map[string]any
	Current   int
	Total     int
}

// ProjectRunProgress maps an internal task step to the product-facing stage
// owned by the selected run. Route points deliberately remain below this seam;
// only Countess floor transitions use the semantic current area.
func ProjectRunProgress(runID, step string, areaID world.AreaID) (RunProgress, bool) {
	switch RunID(runID) {
	case RunIDCountess:
		return countessRunProgress(step, areaID)
	case RunIDCows:
		return cowRunProgress(step)
	case RunIDMephisto:
		return standardBossRunProgress(step, "waypoint_durance_of_hate_level_2", "travel_mephisto")
	case RunIDSummoner:
		return standardBossRunProgress(step, "waypoint_arcane_sanctuary", "travel_summoner")
	case RunIDNihlathak:
		return standardBossRunProgress(step, "waypoint_halls_of_pain", "travel_nihlathak")
	case RunIDLowerKurast:
		return lowerKurastRunProgress(step)
	default:
		return RunProgress{}, false
	}
}

// Progress returns the current user-facing stage only while this runner owns
// an active, non-terminal run. Invalid projections are never published.
func (r *Runner) Progress(areaID world.AreaID) (RunProgress, bool) {
	if r == nil || !r.Result().Active {
		return RunProgress{}, false
	}
	projected, ok := ProjectRunProgress(r.selection.Run, r.tracker.name, areaID)
	if !ok {
		return RunProgress{}, false
	}
	// Loading snapshots can temporarily hide the semantic area while the task
	// remains in `play_bound_route`. Keep the last visible boundary so the UI
	// never jumps backwards between two Countess cellar floors.
	if r.progress.Total == projected.Total && r.progress.Current > projected.Current {
		return r.progress, true
	}
	r.progress = projected
	return projected, true
}

func countessRunProgress(step string, areaID world.AreaID) (RunProgress, bool) {
	const total = 13
	switch step {
	case pipelineStepPrecheck, pipelineStepApplyTownProfile:
		return validRunProgress("town_preparation", nil, 1, total)
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return validRunProgress("waypoint_black_marsh", nil, 2, total)
	case pipelineStepWaitEntryArea:
		return validRunProgress("travel_tower", nil, 3, total)
	case pipelineStepPlayRoute:
		switch areaID {
		case world.TowerCellarLevel1:
			return validRunProgress("cellar_floor", map[string]any{"floor": 1, "floors": 5}, 4, total)
		case world.TowerCellarLevel2:
			return validRunProgress("cellar_floor", map[string]any{"floor": 2, "floors": 5}, 5, total)
		case world.TowerCellarLevel3:
			return validRunProgress("cellar_floor", map[string]any{"floor": 3, "floors": 5}, 6, total)
		case world.TowerCellarLevel4:
			return validRunProgress("cellar_floor", map[string]any{"floor": 4, "floors": 5}, 7, total)
		case world.TowerCellarLevel5:
			return validRunProgress("cellar_floor", map[string]any{"floor": 5, "floors": 5}, 8, total)
		default:
			return validRunProgress("travel_tower", nil, 3, total)
		}
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles:
		return validRunProgress("boss_combat", nil, 9, total)
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return validRunProgress("loot", nil, 10, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return validRunProgress("return_town", nil, 11, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("stash", nil, 12, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("complete", nil, 13, total)
	default:
		return RunProgress{}, false
	}
}

func standardBossRunProgress(step, waypointCode, travelCode string) (RunProgress, bool) {
	const total = 8
	switch step {
	case pipelineStepPrecheck, pipelineStepApplyTownProfile:
		return validRunProgress("town_preparation", nil, 1, total)
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return validRunProgress(waypointCode, nil, 2, total)
	case pipelineStepWaitEntryArea, pipelineStepPlayRoute:
		return validRunProgress(travelCode, nil, 3, total)
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles:
		return validRunProgress("boss_combat", nil, 4, total)
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return validRunProgress("loot", nil, 5, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return validRunProgress("return_town", nil, 6, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("stash", nil, 7, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("complete", nil, 8, total)
	default:
		return RunProgress{}, false
	}
}

func lowerKurastRunProgress(step string) (RunProgress, bool) {
	const total = 8
	switch step {
	case pipelineStepPrecheck, pipelineStepApplyTownProfile:
		return validRunProgress("town_preparation", nil, 1, total)
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return validRunProgress("waypoint_lower_kurast", nil, 2, total)
	case pipelineStepWaitEntryArea, pipelineStepPlayRoute:
		return validRunProgress("travel_huts", nil, 3, total)
	case pipelineStepChestSweep:
		return validRunProgress("superchests", nil, 4, total)
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return validRunProgress("loot", nil, 5, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return validRunProgress("return_town", nil, 6, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("stash", nil, 7, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("complete", nil, 8, total)
	default:
		return RunProgress{}, false
	}
}

func cowRunProgress(step string) (RunProgress, bool) {
	const total = 12
	switch step {
	case cowStepPreflight, cowStepTownReady:
		return validRunProgress("town_preparation", nil, 1, total)
	case cowStepAcquireWaypoint, cowStepOpenWaypoint, cowStepSelectStony, cowStepWaitStony:
		return validRunProgress("waypoint_stony_field", nil, 2, total)
	case cowStepPlayLegRoute:
		return validRunProgress("travel_tristram", nil, 3, total)
	case cowStepOpenWirt:
		return validRunProgress("wirts_body", nil, 4, total)
	case cowStepPickupLeg:
		return validRunProgress("wirts_leg", nil, 5, total)
	case cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue, cowStepSafeFailure:
		return validRunProgress("return_village", nil, 6, total)
	case cowStepBuyTome:
		return validRunProgress("buy_tome", nil, 7, total)
	case cowStepSetupComplete, cowStepPortalRecipe, cowStepRecipeComplete:
		return validRunProgress("cow_portal", nil, 8, total)
	case cowStepSweep, cowStepSweepComplete:
		return validRunProgress("cow_sweep", nil, 9, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown:
		return validRunProgress("return_town", nil, 10, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("stash", nil, 11, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("complete", nil, 12, total)
	default:
		return RunProgress{}, false
	}
}

func validRunProgress(stageCode string, params map[string]any, current, total int) (RunProgress, bool) {
	if stageCode == "" || current < 1 || total < 1 || current > total {
		return RunProgress{}, false
	}
	return RunProgress{StageCode: stageCode, Params: params, Current: current, Total: total}, true
}
