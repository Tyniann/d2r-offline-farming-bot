package tasks

import "github.com/Tyniann/d2r-offline-farming-bot/internal/world"

// RunProgress describes one stable, user-facing stage of an active run.
type RunProgress struct {
	Label   string
	Current int
	Total   int
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
		return standardBossRunProgress(step, "Wegpunkt Kerker des Hasses", "Reise zu Mephisto")
	case RunIDSummoner:
		return standardBossRunProgress(step, "Wegpunkt Geheime Zuflucht", "Reise zum Beschwörer")
	case RunIDNihlathak:
		return standardBossRunProgress(step, "Wegpunkt Hallen der Schmerzen", "Reise zu Nihlathak")
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
		return validRunProgress("Vorbereitung im Dorf", 1, total)
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return validRunProgress("Wegpunkt Schwarzmarsch", 2, total)
	case pipelineStepWaitEntryArea:
		return validRunProgress("Reise zum Turm", 3, total)
	case pipelineStepPlayRoute:
		switch areaID {
		case world.TowerCellarLevel1:
			return validRunProgress("Kellergeschoss 1 von 5", 4, total)
		case world.TowerCellarLevel2:
			return validRunProgress("Kellergeschoss 2 von 5", 5, total)
		case world.TowerCellarLevel3:
			return validRunProgress("Kellergeschoss 3 von 5", 6, total)
		case world.TowerCellarLevel4:
			return validRunProgress("Kellergeschoss 4 von 5", 7, total)
		case world.TowerCellarLevel5:
			return validRunProgress("Kellergeschoss 5 von 5", 8, total)
		default:
			return validRunProgress("Reise zum Turm", 3, total)
		}
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles:
		return validRunProgress("Bosskampf", 9, total)
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return validRunProgress("Beute", 10, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return validRunProgress("Rückkehr in die Stadt", 11, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("Truhe", 12, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("Abschluss", 13, total)
	default:
		return RunProgress{}, false
	}
}

func standardBossRunProgress(step, waypointLabel, travelLabel string) (RunProgress, bool) {
	const total = 8
	switch step {
	case pipelineStepPrecheck, pipelineStepApplyTownProfile:
		return validRunProgress("Vorbereitung im Dorf", 1, total)
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return validRunProgress(waypointLabel, 2, total)
	case pipelineStepWaitEntryArea, pipelineStepPlayRoute:
		return validRunProgress(travelLabel, 3, total)
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles:
		return validRunProgress("Bosskampf", 4, total)
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return validRunProgress("Beute", 5, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return validRunProgress("Rückkehr in die Stadt", 6, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("Truhe", 7, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("Abschluss", 8, total)
	default:
		return RunProgress{}, false
	}
}

func lowerKurastRunProgress(step string) (RunProgress, bool) {
	const total = 8
	switch step {
	case pipelineStepPrecheck, pipelineStepApplyTownProfile:
		return validRunProgress("Vorbereitung im Dorf", 1, total)
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint:
		return validRunProgress("Wegpunkt Unteres Kurast", 2, total)
	case pipelineStepWaitEntryArea, pipelineStepPlayRoute:
		return validRunProgress("Reise zu den Hütten", 3, total)
	case pipelineStepChestSweep:
		return validRunProgress("Supertruhen", 4, total)
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return validRunProgress("Beute", 5, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea:
		return validRunProgress("Rückkehr in die Stadt", 6, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("Truhe", 7, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("Abschluss", 8, total)
	default:
		return RunProgress{}, false
	}
}

func cowRunProgress(step string) (RunProgress, bool) {
	const total = 12
	switch step {
	case cowStepPreflight, cowStepTownReady:
		return validRunProgress("Vorbereitung im Dorf", 1, total)
	case cowStepAcquireWaypoint, cowStepOpenWaypoint, cowStepSelectStony, cowStepWaitStony:
		return validRunProgress("Wegpunkt Steinfeld", 2, total)
	case cowStepPlayLegRoute:
		return validRunProgress("Reise nach Tristram", 3, total)
	case cowStepOpenWirt:
		return validRunProgress("Wirts Leiche", 4, total)
	case cowStepPickupLeg:
		return validRunProgress("Wirts Bein", 5, total)
	case cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue, cowStepSafeFailure:
		return validRunProgress("Rückkehr ins Dorf", 6, total)
	case cowStepBuyTome:
		return validRunProgress("Foliant besorgen", 7, total)
	case cowStepSetupComplete, cowStepPortalRecipe, cowStepRecipeComplete:
		return validRunProgress("Portal ins Kuh-Level", 8, total)
	case cowStepSweep, cowStepSweepComplete:
		return validRunProgress("Kuh-Level räumen", 9, total)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown:
		return validRunProgress("Rückkehr in die Stadt", 10, total)
	case pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash:
		return validRunProgress("Truhe", 11, total)
	case pipelineStepPrepareTown, pipelineStepComplete:
		return validRunProgress("Abschluss", 12, total)
	default:
		return RunProgress{}, false
	}
}

func validRunProgress(label string, current, total int) (RunProgress, bool) {
	if label == "" || current < 1 || total < 1 || current > total {
		return RunProgress{}, false
	}
	return RunProgress{Label: label, Current: current, Total: total}, true
}
