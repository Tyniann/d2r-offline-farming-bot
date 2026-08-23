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
		if c.definition.HasCapability(RunCapabilityChestSweep) {
			return c.onChestSweepPhaseTick(ctx, deps, step, w, now)
		}
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

func (c *runPipeline) onChestSweepPhaseTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
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
		return stepResult{complete: true}
	case pipelineStepChestSweep:
		return c.tickChestSweep(ctx, narrowChestDeps(deps), w, now)
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
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
	if step == pipelineStepWaitRecoveryArea {
		return c.tickWaitRecoveryArea(w)
	}
	return c.tickReturn(ctx, narrowReturnDeps(deps), step, w, now, stepStartedAt)
}

func (c *runPipeline) tickWaitRecoveryArea(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID == world.None {
		return stepResult{}
	}
	if !c.allowsRetryReturnArea(w.Area.ID) {
		return stepResult{failed: true, reason: string(RunReasonUnexpectedArea)}
	}
	if c.ret.recoveryAreaID != w.Area.ID {
		c.resetRecoveryAreaArrival()
		c.ret.recoveryAreaID = w.Area.ID
		c.ret.recoveryAreaStartedAt = w.At
	}
	if w.Generation != 0 {
		if w.Generation != c.ret.recoveryAreaGeneration {
			c.ret.recoveryAreaSnapshots++
			c.ret.recoveryAreaGeneration = w.Generation
			c.ret.recoveryAreaSnapshotAt = w.At
		}
	} else if !w.At.IsZero() && w.At != c.ret.recoveryAreaSnapshotAt {
		c.ret.recoveryAreaSnapshots++
		c.ret.recoveryAreaSnapshotAt = w.At
	}
	if c.ret.recoveryAreaSnapshots < entryAreaArriveSnapshots || c.ret.recoveryAreaStartedAt.IsZero() ||
		w.At.Sub(c.ret.recoveryAreaStartedAt) < entryAreaArriveSettle {
		return stepResult{}
	}
	return stepResult{complete: true}
}

func (c *runPipeline) resetRecoveryAreaArrival() {
	c.ret.recoveryAreaID = world.None
	c.ret.recoveryAreaStartedAt = time.Time{}
	c.ret.recoveryAreaSnapshots = 0
	c.ret.recoveryAreaGeneration = 0
	c.ret.recoveryAreaSnapshotAt = time.Time{}
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
	case pipelineStepChestSweep:
		return c.tickChestSweep(ctx, narrowChestDeps(deps), w, now)
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
