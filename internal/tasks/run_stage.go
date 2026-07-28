package tasks

import "github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"

// RunStageForStep ordnet jeden bekannten Pipeline-Step genau einer stabilen Historien-Stage zu.
func RunStageForStep(step string) (telemetry.HistoryStage, bool) {
	switch step {
	case pipelineStepPrecheck, pipelineStepApplyTownProfile, pipelineStepAcquireTownWaypoint,
		pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint, pipelineStepWaitEntryArea,
		pipelineStepPlayRoute:
		return telemetry.HistoryStageTravel, true
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles:
		return telemetry.HistoryStageCombat, true
	case pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return telemetry.HistoryStageLoot, true
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint,
		pipelineStepWaitHubArea, pipelineStepOpenStash, pipelineStepStashItems,
		pipelineStepCloseStash, pipelineStepPrepareTown, pipelineStepComplete:
		return telemetry.HistoryStageReturnTown, true
	default:
		return "", false
	}
}
