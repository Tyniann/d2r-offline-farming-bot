package tasks

import "github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"

// RunStageForStep ordnet jeden bekannten Pipeline-Step genau einer stabilen Historien-Stage zu.
func RunStageForStep(step string) (telemetry.HistoryStage, bool) {
	switch step {
	case cowStepPreflight, cowStepTownReady, cowStepAcquireWaypoint, cowStepOpenWaypoint, cowStepSelectStony,
		cowStepWaitStony, cowStepPlayLegRoute, cowStepOpenWirt, cowStepPortalRecipe, cowStepRecipeComplete:
		return telemetry.HistoryStageTravel, true
	case cowStepSweep, cowStepSweepComplete:
		return telemetry.HistoryStageCombat, true
	case cowStepPickupLeg:
		return telemetry.HistoryStageLoot, true
	case cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue, cowStepBuyTome,
		cowStepSafeFailure, cowStepSetupComplete:
		return telemetry.HistoryStageReturnTown, true
	case pipelineStepPrecheck, pipelineStepApplyTownProfile, pipelineStepAcquireTownWaypoint,
		pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint, pipelineStepWaitEntryArea,
		pipelineStepPlayRoute:
		return telemetry.HistoryStageTravel, true
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles, pipelineStepChestSweep:
		return telemetry.HistoryStageCombat, true
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return telemetry.HistoryStageLoot, true
	case pipelineStepWaitRecoveryArea, pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint,
		pipelineStepWaitHubArea, pipelineStepOpenStash, pipelineStepStashItems,
		pipelineStepCloseStash, pipelineStepPrepareTown, pipelineStepComplete:
		return telemetry.HistoryStageReturnTown, true
	default:
		return "", false
	}
}
