package tasks

import (
	"context"
	"errors"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (c *runPipeline) onBossTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time) stepResult {
	return c.tickBoss(ctx, narrowBossDeps(deps), step, w, now)
}

func (c *runPipeline) tickBoss(ctx context.Context, deps pipelineBossDeps, step string, w world.State, now time.Time) stepResult {
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
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		return stepResult{complete: true}
	case pipelineStepAcquireBoss:
		if res := c.killAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if target, ok := c.findBossTarget(w); ok {
			c.storeBossTarget(target)
			return stepResult{complete: true}
		}
		if c.phase == "" && c.effectiveDefinition().ID == RunIDSummoner {

			c.boss.targetPosition = w.Player.Position
			c.boss.targetPositionSet = true
			if err := c.emitBossKill(deps); err != nil {
				return stepResult{failed: true, reason: "telemetry_failed"}
			}
			c.boss.bossKillEmitted = true
			return stepResult{complete: true}
		}
		return c.tickBossSearchFallback(ctx, deps, w)
	case pipelineStepEngageBoss:
		if res := c.killAreaGuard(w); res.failed {
			if stop := c.stopCombatAttack(deps); stop.failed {
				return stop
			}
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			if stop := c.stopCombatAttack(deps); stop.failed {
				return stop
			}
			return stepResult{}
		}
		if !c.boss.targetSeen {
			return stepResult{failed: true, reason: "target_not_set"}
		}
		actions := c.effectiveDefinition().BossEngageSequence
		target, visible := c.findMonsterByUnitID(w, c.boss.targetUnitID)
		if !visible {
			if stop := c.stopCombatAttack(deps); stop.failed {
				return stop
			}

			if deps.Profile != nil && c.boss.encounterActionIndex < len(actions) {
				return stepResult{failed: true, reason: string(RunReasonBossPinLost)}
			}
			if replacement, found := c.findConfiguredBossTarget(w); found && replacement.UnitID != c.boss.targetUnitID {
				return stepResult{failed: true, reason: string(RunReasonBossPinLost)}
			}
			c.boss.targetAbsentTicks++
			if c.boss.targetAbsentTicks >= c.core.combat.KillConfirmTicks {
				if !c.boss.bossKillEmitted {
					if err := c.emitBossKill(deps); err != nil {
						return stepResult{failed: true, reason: "telemetry_failed"}
					}
					c.boss.bossKillEmitted = true
				}
				return stepResult{complete: true}
			}
			return stepResult{}
		}
		c.boss.targetPosition = target.Position
		c.boss.targetPositionSet = true
		c.boss.targetAbsentTicks = 0
		if c.boss.encounterActionIndex < len(actions) && deps.Profile != nil {
			action := actions[c.boss.encounterActionIndex]
			if !c.boss.encounterActionStarted {
				if err := c.emitEncounterAction(deps, telemetry.RunEncounterActionStarted, RunOutcomeRunning, "", target.UnitID); err != nil {
					return stepResult{failed: true, reason: "telemetry_failed"}
				}
				c.boss.encounterActionStarted = true
			}
			res := deps.Profile.TickHook(ctx, action.Hook, w, profile.EncounterTarget{UnitID: target.UnitID, Position: target.Position, ActionIndex: c.boss.encounterActionIndex}, now)
			switch res.Status {
			case profile.StatusFailed:
				return stepResult{failed: true, reason: res.Reason}
			case profile.StatusAction, profile.StatusPending:
				return stepResult{}
			case profile.StatusComplete:
				if err := c.emitEncounterAction(deps, telemetry.RunEncounterActionCompleted, RunOutcomeSuccess, "", target.UnitID); err != nil {
					return stepResult{failed: true, reason: "telemetry_failed"}
				}
				c.boss.encounterActionIndex++
				c.boss.encounterActionStarted = false
				if c.boss.encounterActionIndex < len(actions) {

					return stepResult{}
				}
			}
		} else if deps.Profile == nil {

			c.boss.encounterActionIndex = len(actions)
		}
		return c.tickEngageTarget(deps, w, target, now)
	case pipelineStepClearNearbyHostiles:
		if res := c.killAreaGuard(w); res.failed {
			return c.stopCleanup(deps, res)
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return c.stopCleanup(deps, stepResult{})
		}
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		if c.boss.cleanupCastCount >= c.postBossCleanupMaxCasts() {

			return c.stopCleanup(deps, stepResult{complete: true})
		}
		if c.effectiveDefinition().ID == RunIDNihlathak {
			if c.boss.cleanupLastProgressAt.IsZero() {
				c.boss.cleanupLastProgressAt = now
			} else if now.Sub(c.boss.cleanupLastProgressAt) >= nihlathakCleanupNoProgressTimeout {

				return c.stopCleanup(deps, stepResult{complete: true})
			}
		}
		target, visible := c.findCleanupTarget(w)
		if !visible {
			c.boss.cleanupTargetUnitID = 0
			c.boss.cleanupNoTargetTicks++
			if c.boss.cleanupNoTargetTicks >= postBossCleanupStableTicks {
				return c.stopCleanup(deps, stepResult{complete: true})
			}
			return c.stopCleanup(deps, stepResult{})
		}
		c.boss.cleanupTargetUnitID = target.UnitID
		c.boss.cleanupNoTargetTicks = 0
		if c.effectiveDefinition().ID == RunIDNihlathak {
			if deps.RouteClear == nil {
				return stepResult{failed: true, reason: "combat_not_wired"}
			}
			result := deps.RouteClear.TickRouteClear(ctx, profile.RouteClearRequest{
				RunID:        string(c.effectiveDefinition().ID),
				DefinitionID: c.core.combat.Profile,
				Player:       w.Player,
				Target:       target,
				Mode:         profile.RouteClearThreat,
				AssessmentAt: w.At,
			}, now)
			switch result.Status {
			case profile.StatusFailed:
				return stepResult{failed: true, reason: "combat_action_failed"}
			case profile.StatusAction:
				c.boss.cleanupCastCount++
				c.boss.cleanupLastProgressAt = now
			case profile.StatusPending:
				if result.Reason == profile.RouteClearReasonTargetUnprojectable {
					if c.boss.cleanupSkippedUnitIDs == nil {
						c.boss.cleanupSkippedUnitIDs = make(map[uint32]bool)
					}
					c.boss.cleanupSkippedUnitIDs[target.UnitID] = true
					c.boss.cleanupTargetUnitID = 0
				}
			}
			return stepResult{}
		}
		cast, err := deps.Combat.CastAttackAtMonster(now, c.core.combat.AttackSkillID, w.Player, target)
		if err != nil {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}
		if cast.Sent {
			c.boss.cleanupCastCount++
		}
		return stepResult{}
	case pipelineStepRepositionForLoot:
		if res := c.killAreaGuard(w); res.failed {
			return res
		}
		if !w.Valid || w.Phase != world.GamePhaseInGame {
			return stepResult{}
		}
		if deps.Combat == nil {
			return stepResult{failed: true, reason: "combat_not_wired"}
		}
		if !c.boss.targetPositionSet {
			return stepResult{failed: true, reason: "boss_position_missing"}
		}
		if world.Distance(w.Player.Position, c.boss.targetPosition) <= postKillLootDistanceTiles {
			return stepResult{complete: true}
		}
		if c.loot.postKillTeleportAttempts >= lootRepositionMaxAttempts {
			if lootRepositionReady(now, w.At, c.loot.postKillTeleportAt, c.loot.postKillTeleportSnapshot) {

				return stepResult{complete: true}
			}
			return stepResult{}
		}
		if !lootRepositionReady(now, w.At, c.loot.postKillTeleportAt, c.loot.postKillTeleportSnapshot) {
			return stepResult{}
		}
		sent, err := deps.Combat.TeleportToward(now, w.Player, c.boss.targetPosition, 0)
		if err != nil {
			return stepResult{failed: true, reason: "post_kill_reposition_failed"}
		}
		if sent {
			c.loot.postKillTeleportAttempts++
			c.loot.postKillTeleportAt = now
			c.loot.postKillTeleportSnapshot = w.At
		}
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *runPipeline) emitEncounterAction(deps pipelineBossDeps, event telemetry.EventName, outcome RunOutcome, reason string, unitID uint32) error {
	if deps.Telemetry == nil {
		return nil
	}
	index := c.boss.encounterActionIndex
	return deps.Telemetry.Emit(telemetry.Event{
		Event: event, DefinitionID: string(c.effectiveDefinition().ID), Step: pipelineStepEngageBoss,
		Stage: telemetry.HistoryStageCombat, ActionIndex: &index, UnitID: unitID, Outcome: string(outcome), Reason: reason,
	})
}

func (c *runPipeline) emitBossKill(deps pipelineBossDeps) error {
	if deps.Telemetry == nil {
		return nil
	}
	definition := c.effectiveDefinition()
	return deps.Telemetry.Emit(telemetry.Event{
		Event: telemetry.BossKillConfirmed, DefinitionID: string(definition.ID), Step: pipelineStepEngageBoss,
		Stage: telemetry.HistoryStageCombat, UnitID: c.boss.targetUnitID, BossID: string(definition.ID), BossName: definition.Boss.Name,
	})
}

func (c *runPipeline) killAreaGuard(w world.State) stepResult {
	if !w.Valid || w.Phase != world.GamePhaseInGame {
		return stepResult{}
	}
	if w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return stepResult{failed: true, reason: "unexpected_area"}
	}
	return stepResult{}
}

func (c *runPipeline) findBossTarget(w world.State) (world.Monster, bool) {
	definition := c.effectiveDefinition()
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID != definition.RouteTerminalArea {
		return world.Monster{}, false
	}
	if target, ok := c.findConfiguredBossTarget(w); ok {
		return target, true
	}
	if !definition.Boss.AllowAnySuperUniqueFallback {
		return world.Monster{}, false
	}
	return w.FindSuperUnique(0)
}

func (c *runPipeline) findConfiguredBossTarget(w world.State) (world.Monster, bool) {
	definition := c.effectiveDefinition()
	if definition.Boss.RequireSuperUnique {

		return w.FindSuperUnique(definition.Boss.NPCID)
	}

	return w.FindNPC(definition.Boss.NPCID)
}

func (c *runPipeline) findMonsterByUnitID(w world.State, unitID uint32) (world.Monster, bool) {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Area.ID != c.effectiveDefinition().RouteTerminalArea {
		return world.Monster{}, false
	}
	for _, m := range w.Monsters {
		if m.UnitID == unitID {
			return m, true
		}
	}
	return world.Monster{}, false
}

func (c *runPipeline) storeBossTarget(target world.Monster) {
	c.boss.targetSeen = true
	c.boss.targetUnitID = target.UnitID
	c.boss.targetPosition = target.Position
	c.boss.targetPositionSet = true
	c.boss.targetAbsentTicks = 0
}

func (c *runPipeline) tickBossSearchFallback(ctx context.Context, deps pipelineBossDeps, w world.State) stepResult {
	if !c.boss.chestFallbackStarted {
		target, ok := c.bossSearchAnchor(w)
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
		c.boss.chestFallbackStarted = true
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

func (c *runPipeline) bossSearchAnchor(w world.State) (world.Position, bool) {
	boss := c.effectiveDefinition().Boss
	if boss.SearchAnchorObject != world.ObjectKindUnknown {
		if chest, ok := w.NearestObject(boss.SearchAnchorObject); ok {
			return chest.Position, true
		}
	}
	if boss.SearchAnchorEntrance != world.EntranceKindUnknown {
		if down, ok := w.NearestEntrance(boss.SearchAnchorEntrance); ok {
			return down.Position, true
		}
	}
	return world.Position{}, false
}

func (c *runPipeline) tickEngageTarget(deps pipelineBossDeps, w world.State, target world.Monster, now time.Time) stepResult {
	if deps.Combat == nil {
		return stepResult{failed: true, reason: "combat_not_wired"}
	}
	// Hammerdin uses the shared Mephisto standard-attack hold for every
	// registered boss, including Nihlathak. The Necro projection/aim path
	// stays Nihlathak-only for Bone Spear.
	if c.hammerdinBossCombat() {
		return c.tickHammerdinEngageTarget(deps, w, target, now)
	}
	if c.effectiveDefinition().ID == RunIDNihlathak {
		return c.tickNihlathakEngageTarget(deps, w, target, now)
	}
	distance := world.Distance(w.Player.Position, target.Position)
	var err error
	if distance > c.core.combat.RepositionDistanceTiles {
		_, err = deps.Combat.TeleportToward(now, w.Player, target.Position, c.core.combat.EngageDistanceTiles)
	} else {
		_, err = deps.Combat.CastAttackAtWorld(now, c.core.combat.AttackSkillID, w.Player, target.Position)
	}
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	return stepResult{}
}

const (
	hammerdinEngageNoProgressTimeout = 25 * time.Second
	hammerdinEngageMaxTeleports      = 12
	// hammerdinAttackWindow bounds one stationary hold before the pinned target
	// is attacked again from another living monster in the pack.
	hammerdinAttackWindow = 2 * time.Second
	// hammerdinHoldRecheckSnapshots is how many World snapshots stay ignored
	// while LMB remains down before the next distance check.
	hammerdinHoldRecheckSnapshots = 3
	// hammerdinLostDistanceTiles releases an active hold only after the
	// Paladin has actually left melee, not after one boss step.
	hammerdinLostDistanceTiles                = 5
	hammerdinRepositionMaxTargetDistanceTiles = 18
	hammerdinRouteForwardDistanceTiles        = 8
	hammerdinCombatProfileID                  = "paladin_hammerdin"
)

func (c *runPipeline) hammerdinBossCombat() bool {
	return c.core.combat.Profile == hammerdinCombatProfileID
}

func (c *runPipeline) tickHammerdinEngageTarget(deps pipelineBossDeps, w world.State, target world.Monster, now time.Time) stepResult {
	if c.boss.engageStartedAt.IsZero() {
		c.boss.engageStartedAt = now
		c.boss.engageLastProgressAt = now
	}
	if now.Sub(c.boss.engageLastProgressAt) >= hammerdinEngageNoProgressTimeout {
		if stop := c.stopCombatAttack(deps); stop.failed {
			return stop
		}
		return stepResult{failed: true, reason: string(RunReasonBossCombatNoProgress)}
	}
	if c.boss.hammerdinRepositionPending {
		return c.tickHammerdinBossReposition(deps, w, now)
	}
	distance := world.Distance(w.Player.Position, target.Position)
	if c.boss.hammerdinAttackHeld {
		if !c.boss.hammerdinHoldStartedAt.IsZero() && now.Sub(c.boss.hammerdinHoldStartedAt) >= hammerdinAttackWindow {
			repositionTarget, found := selectHammerdinBossRepositionTarget(w, target, c.boss.hammerdinRepositionTargetUnitID)
			if !found {
				c.boss.hammerdinHoldStartedAt = now
				return stepResult{}
			}
			if stop := c.stopCombatAttack(deps); stop.failed {
				return stop
			}
			c.boss.hammerdinRepositionPending = true
			c.boss.hammerdinRepositionTargetUnitID = repositionTarget.UnitID
			return stepResult{}
		}
		c.boss.hammerdinHoldSnapshots++
		if c.boss.hammerdinHoldSnapshots < hammerdinHoldRecheckSnapshots {
			return stepResult{}
		}
		c.boss.hammerdinHoldSnapshots = 0
		if distance > hammerdinLostDistanceTiles {
			if stop := c.stopCombatAttack(deps); stop.failed {
				return stop
			}
			// Wait one snapshot before teleporting. The boss may already be
			// dying; a same-tick teleport looks like a walk off the corpse.
			return stepResult{}
		}
		return stepResult{}
	}
	if distance > c.core.combat.RepositionDistanceTiles && !c.boss.hammerdinRepositionReady {
		return c.teleportHammerdinToTarget(deps, w, target, now)
	}
	attackTarget := target
	if !target.IsHovered && c.nihlathakAimAuthorizesHover(w, target) {
		if hovered, ok := hoveredLivingMonster(w); ok {
			attackTarget = hovered
		}
	}
	result, err := deps.Combat.HoldStandardAttack(now, c.core.combat.AttackSkillID, w.Player, attackTarget)
	if err != nil {
		if c.boss.hammerdinRepositionReady && errors.Is(err, profile.ErrRouteClearTargetUnprojectable) {
			c.boss.hammerdinRepositionReady = false
			return stepResult{}
		}
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	if result.AimRequested {
		c.storeNihlathakAim(w, target)
	}
	if result.Sent {
		c.boss.hammerdinAttackHeld = true
		c.boss.hammerdinRepositionReady = false
		c.boss.hammerdinHoldSnapshots = 0
		c.boss.hammerdinHoldStartedAt = now
	}
	return stepResult{}
}

func hoveredLivingMonster(w world.State) (world.Monster, bool) {
	for _, monster := range w.Monsters {
		if monster.IsHovered {
			return monster, true
		}
	}
	return world.Monster{}, false
}

func (c *runPipeline) teleportHammerdinToTarget(deps pipelineBossDeps, w world.State, target world.Monster, now time.Time) stepResult {
	sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, c.core.combat.EngageDistanceTiles)
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	if sent {
		c.boss.engageTeleportCount++
		if c.boss.engageTeleportCount >= hammerdinEngageMaxTeleports {
			return stepResult{failed: true, reason: string(RunReasonBossCombatNoProgress)}
		}
	}
	return stepResult{}
}

func (c *runPipeline) tickHammerdinBossReposition(deps pipelineBossDeps, w world.State, now time.Time) stepResult {
	repositionTarget, found := w.FindMonsterByUnitID(c.boss.hammerdinRepositionTargetUnitID)
	if !found {
		c.clearHammerdinBossReposition(false)
		return stepResult{}
	}
	if c.boss.hammerdinRepositionSent {
		if !w.At.After(c.boss.hammerdinRepositionSnapshot) {
			return stepResult{}
		}
		if world.Distance(c.boss.hammerdinRepositionOrigin, w.Player.Position) > routeThreatApproachProgressEpsilonTiles {
			c.clearHammerdinBossReposition(true)
			return stepResult{}
		}
		if now.Sub(c.boss.hammerdinRepositionAt) < routeThreatApproachSettle {
			return stepResult{}
		}
		// A blocked landing does not terminate the fight. The next attack
		// window may choose another monster while the boss stays pinned.
		c.clearHammerdinBossReposition(false)
		return stepResult{}
	}
	sent, err := deps.Combat.TeleportToward(now, w.Player, repositionTarget.Position, c.core.combat.EngageDistanceTiles)
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	if sent {
		c.boss.engageTeleportCount++
		if c.boss.engageTeleportCount >= hammerdinEngageMaxTeleports {
			return stepResult{failed: true, reason: string(RunReasonBossCombatNoProgress)}
		}
		c.boss.hammerdinRepositionSent = true
		c.boss.hammerdinRepositionOrigin = w.Player.Position
		c.boss.hammerdinRepositionAt = now
		c.boss.hammerdinRepositionSnapshot = w.At
	}
	return stepResult{}
}

func selectHammerdinBossRepositionTarget(state world.State, pinned world.Monster, excludedUnitID uint32) (world.Monster, bool) {
	var selected world.Monster
	var selectedDistance float64
	for _, candidate := range state.Monsters {
		if candidate.UnitID == 0 || candidate.UnitID == pinned.UnitID || candidate.UnitID == excludedUnitID {
			continue
		}
		if world.Distance(pinned.Position, candidate.Position) > hammerdinRepositionMaxTargetDistanceTiles {
			continue
		}
		distance := positionDistanceSquared(state.Player.Position, candidate.Position)
		if distance <= 1 {
			continue
		}
		if selected.UnitID == 0 || distance < selectedDistance ||
			(distance == selectedDistance && candidate.UnitID < selected.UnitID) {
			selected = candidate
			selectedDistance = distance
		}
	}
	return selected, selected.UnitID != 0
}

func (c *runPipeline) clearHammerdinBossReposition(moved bool) {
	c.boss.hammerdinRepositionPending = false
	c.boss.hammerdinRepositionSent = false
	c.boss.hammerdinRepositionReady = moved
	c.boss.hammerdinRepositionOrigin = world.Position{}
	c.boss.hammerdinRepositionAt = time.Time{}
	c.boss.hammerdinRepositionSnapshot = time.Time{}
}

func (c *runPipeline) stopCombatAttack(deps pipelineBossDeps) stepResult {
	if deps.Combat != nil {
		if err := deps.Combat.StopAttack(); err != nil {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}
	}
	c.boss.hammerdinAttackHeld = false
	c.boss.hammerdinHoldSnapshots = 0
	c.boss.hammerdinHoldStartedAt = time.Time{}
	return stepResult{}
}

func (c *runPipeline) resetHammerdinEngage() {
	c.boss.engageStartedAt = time.Time{}
	c.boss.engageLastProgressAt = time.Time{}
	c.boss.engageTeleportCount = 0
	c.boss.hammerdinAttackHeld = false
	c.boss.hammerdinHoldSnapshots = 0
	c.boss.hammerdinHoldStartedAt = time.Time{}
	c.boss.hammerdinRepositionPending = false
	c.boss.hammerdinRepositionSent = false
	c.boss.hammerdinRepositionReady = false
	c.boss.hammerdinRepositionTargetUnitID = 0
	c.boss.hammerdinRepositionOrigin = world.Position{}
	c.boss.hammerdinRepositionAt = time.Time{}
	c.boss.hammerdinRepositionSnapshot = time.Time{}
}

func (c *runPipeline) tickNihlathakEngageTarget(deps pipelineBossDeps, w world.State, target world.Monster, now time.Time) stepResult {
	if c.boss.bossApproachPending {
		if now.Sub(c.boss.bossApproachAt) < bossApproachSettle || !w.At.After(c.boss.bossApproachSnapshot) {
			return stepResult{}
		}
		c.boss.bossApproachPending = false
	}

	attackTarget := target
	if !target.IsHovered && c.nihlathakAimAuthorizesHover(w, target) {
		if hovered, ok := hoveredLivingMonster(w); ok {
			attackTarget = hovered
		}
	}
	canAim := attackTarget.IsHovered || deps.Combat.MonsterAimProjectable(w.Player.Position, target.Position)
	if canAim {
		cast, err := deps.Combat.CastAttackAtMonster(now, c.core.combat.AttackSkillID, w.Player, attackTarget)
		if err == nil {
			if cast.AimRequested {
				c.storeNihlathakAim(w, target)
			}
			return stepResult{}
		}
		if !errors.Is(err, profile.ErrRouteClearTargetUnprojectable) {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}

	}

	if c.boss.bossApproachAttempted {

		return stepResult{failed: true, reason: "boss_combat_unprojectable"}
	}
	_, desiredDistance, ok := deps.Combat.FarthestProjectableMonsterApproach(w.Player.Position, target.Position)
	if !ok {
		return stepResult{failed: true, reason: "boss_combat_unprojectable"}
	}
	sent, err := deps.Combat.TeleportToward(now, w.Player, target.Position, desiredDistance)
	if err != nil {
		return stepResult{failed: true, reason: "combat_action_failed"}
	}
	if sent {
		c.resetNihlathakAim()
		c.boss.bossApproachPending = true
		c.boss.bossApproachAttempted = true
		c.boss.bossApproachAt = now
		c.boss.bossApproachSnapshot = w.At
	}
	return stepResult{}
}

func (c *runPipeline) resetBossApproach() {
	c.boss.bossApproachPending = false
	c.boss.bossApproachAttempted = false
	c.boss.bossApproachAt = time.Time{}
	c.boss.bossApproachSnapshot = time.Time{}
	c.resetNihlathakAim()
}

func (c *runPipeline) storeNihlathakAim(w world.State, target world.Monster) {
	c.boss.nihlathakAimUnitID = target.UnitID
	c.boss.nihlathakAimPlayerPosition = w.Player.Position
	c.boss.nihlathakAimTargetPosition = target.Position
	c.boss.nihlathakAimSnapshot = w.At
}

func (c *runPipeline) nihlathakAimAuthorizesHover(w world.State, target world.Monster) bool {
	return c.boss.nihlathakAimUnitID == target.UnitID &&
		c.boss.nihlathakAimPlayerPosition == w.Player.Position &&
		c.boss.nihlathakAimTargetPosition == target.Position &&
		w.At.After(c.boss.nihlathakAimSnapshot)
}

func (c *runPipeline) resetNihlathakAim() {
	c.boss.nihlathakAimUnitID = 0
	c.boss.nihlathakAimPlayerPosition = world.Position{}
	c.boss.nihlathakAimTargetPosition = world.Position{}
	c.boss.nihlathakAimSnapshot = time.Time{}
}

func (c *runPipeline) findCleanupTarget(w world.State) (world.Monster, bool) {
	var nearest world.Monster
	var nearestDistanceSquared float64
	found := false
	radius := c.postBossCleanupRadiusTiles()
	for _, monster := range w.Monsters {
		if monster.UnitID == c.boss.targetUnitID || c.boss.cleanupSkippedUnitIDs[monster.UnitID] || !c.isCleanupHostile(monster) {
			continue
		}
		distanceSquared := positionDistanceSquared(w.Player.Position, monster.Position)
		if c.effectiveDefinition().ID == RunIDNihlathak && monster.IsHovered &&
			distanceSquared <= radius*radius {

			return monster, true
		}
		if distanceSquared <= radius*radius &&
			preferLivingTarget(monster, distanceSquared, nearest, nearestDistanceSquared, found) {
			nearest = monster
			nearestDistanceSquared = distanceSquared
			found = true
		}
	}
	return nearest, found
}

func (c *runPipeline) isCleanupHostile(monster world.Monster) bool {
	switch c.effectiveDefinition().ID {
	case RunIDSummoner:
		return c.effectiveDefinition().AllowsRouteHostile(monster.NPCID)
	case RunIDNihlathak:

		return true
	default:
		return false
	}
}

func (c *runPipeline) postBossCleanupRadiusTiles() float64 {
	if c.effectiveDefinition().ID == RunIDNihlathak {

		return nihlathakCleanupRadiusTiles
	}
	return postBossCleanupRadiusTiles
}

func (c *runPipeline) postBossCleanupMaxCasts() int {
	if c.effectiveDefinition().ID == RunIDNihlathak {
		return nihlathakCleanupMaxCasts
	}
	return postBossCleanupMaxCasts
}

func (c *runPipeline) stopCleanup(deps pipelineBossDeps, result stepResult) stepResult {
	if c.effectiveDefinition().ID == RunIDNihlathak && deps.RouteClear != nil {
		deps.RouteClear.ResetRouteClear()
	}
	if deps.Combat != nil {
		if err := deps.Combat.StopAttack(); err != nil {
			return stepResult{failed: true, reason: "combat_action_failed"}
		}
	}
	return result
}

func (c *runPipeline) resetPostBossCleanup() {
	c.boss.cleanupTargetUnitID = 0
	c.boss.cleanupCastCount = 0
	c.boss.cleanupNoTargetTicks = 0
	c.boss.cleanupLastProgressAt = time.Time{}
	c.boss.cleanupSkippedUnitIDs = nil
}
