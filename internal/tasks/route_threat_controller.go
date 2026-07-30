package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RouteThreatTickResult reports the exclusive route action selected for one tick.
type RouteThreatTickResult struct {
	State         RouteThreatState
	AllowMovement bool
	Failed        bool
	Reason        RouteThreatReason
}

// RouteThreatController owns generation-scoped route-clear state and watchdogs.
// It never receives a movement or teleport action surface.
type RouteThreatController struct {
	state RouteThreatState

	blocked          bool
	lastAssessmentAt time.Time
	movementAfter    time.Time
	stableClear      int
	lastProgressAt   time.Time

	lastTargetUnitID uint32
	lastTargetMode   profile.RouteClearMode
	lastEligible     int
	lastRelevant     int
	lastCoverage     float64
	lastComplete     bool

	outOfRangeUnitID uint32
	outOfRangeTicks  int

	manaRecovery          bool
	manaRecoveryStartedAt time.Time

	telemetry                 RunTelemetry
	telemetryTargetUnitID     uint32
	telemetryTargetZone       ThreatZone
	telemetrySaturationKnown  bool
	telemetryTruncated        bool
	telemetryCoverageComplete bool
	telemetryBlockStartedAt   time.Time
	telemetryActions          int
	telemetryDensityActions   int
	telemetryTargets          map[uint32]struct{}
	telemetryManaKnown        bool
	telemetryManaActive       bool
	telemetryManaThreatened   bool
	telemetryManaResource     profile.ResourceKind
}

// State returns the current generation-scoped threat state.
func (c *RouteThreatController) State() RouteThreatState {
	if c.state == "" {
		return RouteThreatMoving
	}
	return c.state
}

// Reset clears all pins, counters, deadlines, and resume guards.
func (c *RouteThreatController) Reset(clear RouteClearExecutor) {
	if clear != nil {
		clear.ResetRouteClear()
	}
	*c = RouteThreatController{state: RouteThreatMoving}
}

// SetTelemetry binds the current run recorder without changing controller state.
func (c *RouteThreatController) SetTelemetry(sink RunTelemetry) {
	c.telemetry = sink
}

// ObserveResources updates the route-only mana hysteresis and returns the
// immutable context consumed by the profile resource policy.
func (c *RouteThreatController) ObserveResources(state world.State, assessment ThreatAssessment, cfg RouteCombatConfig, now time.Time) profile.ResourceContext {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Player.MaxMana == 0 || assessment.SnapshotAt != state.At {
		return profile.ResourceContext{}
	}
	if !c.lastAssessmentAt.IsZero() && state.At.Before(c.lastAssessmentAt) {
		return profile.ResourceContext{}
	}
	if now.IsZero() {
		now = state.At
	}
	manaPercent := int(state.Player.ManaPercent())
	if c.manaRecovery {
		if manaPercent >= cfg.ResumeManaPercent {
			c.manaRecovery = false
			c.manaRecoveryStartedAt = time.Time{}
		}
	} else if manaPercent < cfg.TeleportManaReservePercent {
		c.manaRecovery = true
		c.manaRecoveryStartedAt = now
	}
	immediateThreat := assessment.RouteTargetFound && assessment.RouteZone == ThreatZoneImmediate
	return profile.ResourceContext{
		MobilityCritical: c.manaRecovery,
		Threatened:       assessment.RouteTargetFound,
		EmergencyMana:    immediateThreat && manaPercent <= cfg.EmergencyManaPercent,
	}
}

// Tick chooses exactly one of Hold/Clear or Movement for the current assessment.
func (c *RouteThreatController) Tick(
	ctx context.Context,
	route RoutePlayback,
	clear RouteClearExecutor,
	state world.State,
	progress RouteProgress,
	assessment ThreatAssessment,
	definition RunDefinition,
	cfg RouteCombatConfig,
	profileID string,
	now time.Time,
) RouteThreatTickResult {
	if ctx.Err() != nil {
		return c.fail(RouteThreatReasonStateInvalid)
	}
	if now.IsZero() {
		now = state.At
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.At.IsZero() || assessment.SnapshotAt != state.At {
		c.stableClear = 0
		return RouteThreatTickResult{State: c.State()}
	}
	if !c.lastAssessmentAt.IsZero() && state.At.Before(c.lastAssessmentAt) {
		return c.fail(RouteThreatReasonStateInvalid)
	}
	if err := c.observeSnapshotTelemetry(state, progress, assessment, definition, now); err != nil {
		return c.fail(RouteThreatReasonStateInvalid)
	}

	needsBlock := assessment.RouteTargetFound || !assessment.CoverageComplete || c.manaRecovery
	if !c.blocked && !needsBlock {
		if !c.movementAfter.IsZero() && !state.At.After(c.movementAfter) {
			if err := route.Hold(state); err != nil {
				return c.fail(RouteThreatReasonStateInvalid)
			}
			return RouteThreatTickResult{State: RouteThreatMoving}
		}
		if progress.Mode == RouteProgressRecovery {
			if reason := guardRecoveryInput(state, progress); reason != "" {
				c.state = RouteThreatRecoveryGuard
				if emitErr := c.emitRecoverySuppressed(state, progress, reason, now); emitErr != nil {
					return c.fail(RouteThreatReasonStateInvalid)
				}
				return c.fail(reason)
			}
			c.state = RouteThreatRecoveryGuard
			c.lastAssessmentAt = state.At
			return RouteThreatTickResult{State: c.state, AllowMovement: true}
		}
		c.state = RouteThreatMoving
		c.lastAssessmentAt = state.At
		return RouteThreatTickResult{State: c.state, AllowMovement: true}
	}
	if route == nil || clear == nil {
		return c.fail(RouteThreatReasonStateInvalid)
	}
	if err := route.Hold(state); err != nil {
		return c.fail(RouteThreatReasonStateInvalid)
	}
	if state.At.Equal(c.lastAssessmentAt) {
		return RouteThreatTickResult{State: c.State()}
	}

	if !c.blocked {
		c.beginBlock(state, assessment, now)
		if err := c.emitClearStarted(state, progress, definition, cfg, profileID, now); err != nil {
			return c.fail(RouteThreatReasonStateInvalid)
		}
	}
	previousEligible, previousRelevant := c.lastEligible, c.lastRelevant
	progressTargetUnitID := c.lastTargetUnitID
	if c.observeObjectiveProgress(state, progress, assessment, definition.RouteHostileNPCIDs, cfg, now) {
		if err := c.emitClearProgress(state, progress, assessment, progressTargetUnitID, previousEligible, previousRelevant, now); err != nil {
			return c.fail(RouteThreatReasonStateInvalid)
		}
	}
	c.lastAssessmentAt = state.At
	if c.manaRecovery && !c.manaRecoveryStartedAt.IsZero() && now.Sub(c.manaRecoveryStartedAt) >= cfg.ManaRecoveryTimeout {
		return c.fail(RouteThreatReasonManaRecoveryFailed)
	}

	if !needsBlock {
		c.recordBlockBaseline(state, assessment)
		c.state = RouteThreatClearing
		c.stableClear++
		if c.stableClear >= Phase17StableClearSnapshots {
			if err := c.emitClearCompleted(state, progress, now); err != nil {
				return c.fail(RouteThreatReasonStateInvalid)
			}
			clear.ResetRouteClear()
			c.blocked = false
			c.state = RouteThreatMoving
			c.movementAfter = state.At
			c.clearBlockTracking()
		}
		return RouteThreatTickResult{State: c.state}
	}
	c.stableClear = 0

	target, mode, targetFound := selectRouteClearTarget(state, assessment, cfg)
	if assessment.RouteTargetFound && !routeTargetWithinAttack(state, assessment.RouteTarget, cfg) && !assessment.DensityTargetFound {
		if c.observeOutOfRange(assessment.RouteTarget.UnitID) {
			return c.fail(RouteThreatReasonOutOfRange)
		}
	} else if !targetFound {
		c.resetOutOfRange()
	}

	if !c.lastProgressAt.IsZero() && now.Sub(c.lastProgressAt) >= cfg.NoProgressTimeout {
		return c.fail(RouteThreatReasonClearNoProgress)
	}
	if !targetFound {
		if c.manaRecovery {
			c.state = RouteThreatManaRecovery
		} else {
			c.state = RouteThreatDensityRelief
		}
		c.recordBlockBaseline(state, assessment)
		return RouteThreatTickResult{State: c.state}
	}

	if mode == profile.RouteClearThreat {
		c.state = RouteThreatClearing
	} else {
		c.state = RouteThreatDensityRelief
	}
	if c.manaRecovery {
		c.state = RouteThreatManaRecovery
	}
	if c.telemetryTargets == nil {
		c.telemetryTargets = make(map[uint32]struct{})
	}
	c.telemetryTargets[target.UnitID] = struct{}{}
	result := clear.TickRouteClear(ctx, profile.RouteClearRequest{
		RunID: string(definition.ID), DefinitionID: profileID, Player: state.Player,
		Target: target, Mode: mode, AssessmentAt: assessment.SnapshotAt,
	}, now)
	if result.Status == profile.StatusPending && result.Reason == profile.RouteClearReasonTargetUnprojectable {
		// A valid target can be within the configured tile radius while still
		// lying outside the directional client viewport. Keep holding without
		// moving the cursor and require the same fresh-snapshot proof as range.
		if c.observeOutOfRange(target.UnitID) {
			return c.fail(RouteThreatReasonOutOfRange)
		}
		c.recordBlockBaseline(state, assessment)
		return RouteThreatTickResult{State: c.state}
	}
	if result.Status == profile.StatusFailed {
		return c.fail(RouteThreatReasonStateInvalid)
	}
	c.resetOutOfRange()
	if result.Status == profile.StatusAction {
		if err := c.emitClearAction(state, progress, target, mode, profileID, result.SkillID, result.ActionKind, now); err != nil {
			return c.fail(RouteThreatReasonStateInvalid)
		}
	}
	c.lastTargetUnitID = target.UnitID
	c.lastTargetMode = mode
	c.recordBlockBaseline(state, assessment)
	return RouteThreatTickResult{State: c.state}
}

func (c *RouteThreatController) observeOutOfRange(unitID uint32) bool {
	if c.outOfRangeUnitID == unitID {
		c.outOfRangeTicks++
	} else {
		c.outOfRangeUnitID = unitID
		c.outOfRangeTicks = 1
	}
	return c.outOfRangeTicks >= Phase17StableClearSnapshots
}

func (c *RouteThreatController) resetOutOfRange() {
	c.outOfRangeUnitID = 0
	c.outOfRangeTicks = 0
}

// ObserveApproachInput records one sent Force-Move approach without counting
// it as a stationary combat action.
func (c *RouteThreatController) ObserveApproachInput(
	state world.State,
	progress RouteProgress,
	target world.Monster,
	attempt int,
	now time.Time,
) error {
	return c.emitApproachInput(state, progress, target, attempt, now)
}

// ObserveApproachProgress accepts Memory-confirmed player movement toward the
// blocked target as objective progress and requires a fresh projection proof
// before another approach may be requested.
func (c *RouteThreatController) ObserveApproachProgress(
	state world.State,
	progress RouteProgress,
	target world.Monster,
	positionProgress float64,
	now time.Time,
) error {
	if err := c.emitApproachProgress(state, progress, target, positionProgress, now); err != nil {
		return err
	}
	c.lastProgressAt = now
	c.resetOutOfRange()
	return nil
}

func guardRecoveryInput(state world.State, progress RouteProgress) RouteThreatReason {
	if !progress.TargetAvailable {
		return RouteThreatReasonStateInvalid
	}
	if !progress.RecoveryInputSent {
		return ""
	}
	if progress.RecoveryInputAt.IsZero() ||
		progress.RecoveryNextInputAt.IsZero() ||
		progress.RecoveryOutcomeAt.IsZero() ||
		progress.RecoveryProgressTiles <= 0 ||
		state.At.Before(progress.RecoveryInputAt) {
		return RouteThreatReasonStateInvalid
	}
	if world.Distance(progress.RecoveryInputOrigin, state.Player.Position) >= progress.RecoveryProgressTiles {
		return ""
	}
	if !state.At.Before(progress.RecoveryOutcomeAt) {
		return RouteThreatReasonRecoveryUnsafe
	}
	return ""
}

func (c *RouteThreatController) beginBlock(state world.State, assessment ThreatAssessment, now time.Time) {
	c.blocked = true
	c.state = RouteThreatClearing
	c.lastProgressAt = now
	c.lastAssessmentAt = time.Time{}
	c.lastTargetUnitID = 0
	c.lastTargetMode = ""
	c.recordBlockBaseline(state, assessment)
}

func (c *RouteThreatController) observeObjectiveProgress(state world.State, progress RouteProgress, assessment ThreatAssessment, allowed []uint32, cfg RouteCombatConfig, now time.Time) bool {
	targetProgressed := c.lastTargetUnitID != 0 &&
		!routeClearTargetStillRelevant(state, progress, c.lastTargetUnitID, c.lastTargetMode, allowed, cfg, assessment.CoverageComplete)
	progressed := targetProgressed
	if assessment.RelevantThreatCount < c.lastRelevant ||
		state.MonsterCoverage.EligibleMonsterCount < c.lastEligible ||
		assessment.CoverageComplete && !c.lastComplete ||
		state.MonsterCoverage.MonsterCoverageRadiusTiles > c.lastCoverage {
		progressed = true
	}
	if progressed {
		c.lastProgressAt = now
	}
	if targetProgressed {
		c.lastTargetUnitID = 0
		c.lastTargetMode = ""
	}
	return progressed
}

func (c *RouteThreatController) recordBlockBaseline(state world.State, assessment ThreatAssessment) {
	c.lastEligible = state.MonsterCoverage.EligibleMonsterCount
	c.lastRelevant = assessment.RelevantThreatCount
	c.lastCoverage = state.MonsterCoverage.MonsterCoverageRadiusTiles
	c.lastComplete = assessment.CoverageComplete
}

func (c *RouteThreatController) clearBlockTracking() {
	c.stableClear = 0
	c.lastProgressAt = time.Time{}
	c.lastTargetUnitID = 0
	c.lastTargetMode = ""
	c.lastEligible = 0
	c.lastRelevant = 0
	c.lastCoverage = 0
	c.lastComplete = false
	c.outOfRangeUnitID = 0
	c.outOfRangeTicks = 0
	c.telemetryBlockStartedAt = time.Time{}
	c.telemetryActions = 0
	c.telemetryDensityActions = 0
	c.telemetryTargets = nil
}

func (c *RouteThreatController) fail(reason RouteThreatReason) RouteThreatTickResult {
	return RouteThreatTickResult{State: c.State(), Failed: true, Reason: reason}
}

func selectRouteClearTarget(state world.State, assessment ThreatAssessment, cfg RouteCombatConfig) (world.Monster, profile.RouteClearMode, bool) {
	if assessment.HoveredRouteTargetFound {
		return assessment.HoveredRouteTarget, profile.RouteClearThreat, true
	}
	if assessment.RouteTargetFound && routeTargetWithinAttack(state, assessment.RouteTarget, cfg) {
		return assessment.RouteTarget, profile.RouteClearThreat, true
	}
	if assessment.DensityTargetFound {
		return assessment.DensityTarget, profile.RouteClearDensityRelief, true
	}
	return world.Monster{}, "", false
}

func routeTargetWithinAttack(state world.State, target world.Monster, cfg RouteCombatConfig) bool {
	return positionDistanceSquared(state.Player.Position, target.Position) <= cfg.AttackDistanceTiles*cfg.AttackDistanceTiles
}

func routeClearTargetStillRelevant(state world.State, progress RouteProgress, unitID uint32, mode profile.RouteClearMode, allowed []uint32, cfg RouteCombatConfig, coverageComplete bool) bool {
	for _, monster := range state.Monsters {
		if monster.UnitID != unitID || !routeHostileAllowed(monster.NPCID, allowed) {
			continue
		}
		if mode == profile.RouteClearDensityRelief {
			return !coverageComplete && routeTargetWithinAttack(state, monster, cfg)
		}
		return threatZoneForMonster(state.Player.Position, progress, monster.Position, cfg) != ThreatZoneNone
	}
	return false
}

func threatZoneForMonster(player world.Position, progress RouteProgress, monster world.Position, cfg RouteCombatConfig) ThreatZone {
	if positionDistanceSquared(player, monster) <= cfg.ImmediateRadiusTiles*cfg.ImmediateRadiusTiles {
		return ThreatZoneImmediate
	}
	if !progress.TargetAvailable {
		return ThreatZoneNone
	}
	if positionDistanceSquared(progress.MovementTarget, monster) <= cfg.LandingRadiusTiles*cfg.LandingRadiusTiles {
		return ThreatZoneLanding
	}
	if pointWithinCorridor(monster, player, progress.MovementTarget, cfg.CorridorWidthTiles) {
		return ThreatZoneCorridor
	}
	return ThreatZoneNone
}
