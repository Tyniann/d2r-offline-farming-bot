package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (c *runPipeline) onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult {
	if c.phase == RunPhaseTownReady {
		return c.onTownReadyTick(ctx, deps, step, w, now)
	}
	if c.phase == RunPhaseStashPersonal {
		return c.tickStashPersonal(ctx, narrowReturnDeps(deps), step, w)
	}
	if c.phase == RunPhaseBoss {
		return c.tickBoss(ctx, narrowBossDeps(deps), step, w, now)
	}
	if c.phase == RunPhaseLootAndReturn {
		return c.tickLootAndReturn(ctx, deps, step, w, now, stepStartedAt)
	}
	if c.phase == RunPhaseRetryReturn {
		return c.onRetryReturnTick(ctx, deps, step, w, now, stepStartedAt)
	}
	if c.isTravelPhase() {
		return c.tickTravel(ctx, narrowTravelDeps(deps), step, w, now, stepStartedAt)
	}
	if c.phase == "" {
		return c.onRunTick(ctx, deps, step, w, now, stepStartedAt)
	}
	return stepResult{failed: true, reason: "unknown_step"}
}

func (c *runPipeline) onRetryReturnTick(ctx context.Context, deps Deps, step string, w world.State, now, stepStartedAt time.Time) stepResult {
	if step == pipelineStepPrecheck {
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if !c.allowsRetryReturnArea(w.Area.ID) {
			return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
		}
		return stepResult{complete: true}
	}
	return c.tickReturn(ctx, narrowReturnDeps(deps), step, w, now, stepStartedAt)
}

func (c *runPipeline) onTownReadyTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		if !w.Identity.Valid {
			return stepResult{}
		}
		return stepResult{complete: true}
	case pipelineStepApplyTownProfile:
		if deps.Profile == nil {
			return stepResult{failed: true, reason: "profile_not_wired"}
		}
		res := deps.Profile.TickHook(ctx, profile.HookTownReady, w, profile.EncounterTarget{}, now)
		switch res.Status {
		case profile.StatusComplete:
			return stepResult{complete: true}
		case profile.StatusFailed:
			return stepResult{failed: true, reason: res.Reason}
		default:
			return stepResult{}
		}
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) onRunTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	switch step {
	case pipelineStepPrecheck:
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
	case pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint,
		pipelineStepWaitEntryArea, pipelineStepPlayRoute:
		return c.tickTravel(ctx, narrowTravelDeps(deps), step, w, now, stepStartedAt)
	case pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles, pipelineStepRepositionForLoot:
		return c.tickBoss(ctx, narrowBossDeps(deps), step, w, now)
	case pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot:
		return c.tickLoot(narrowLootDeps(deps), step, w, now)
	case pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea,
		pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash, pipelineStepPrepareTown:
		return c.tickReturn(ctx, narrowReturnDeps(deps), step, w, now, stepStartedAt)
	case pipelineStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}
