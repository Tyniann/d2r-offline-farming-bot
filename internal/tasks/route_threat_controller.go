package tasks

import (
	"context"
	"math"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RouteThreatTickResult reports the exclusive route action selected for one tick.
type RouteThreatTickResult struct {
	State         RouteThreatState
	AllowMovement bool
	// StopAttack requests a fail-checked release before a mana hold suppresses
	// the Hammerdin teleport that would otherwise follow a lost-distance check.
	StopAttack bool
	Failed     bool
	Reason     RouteThreatReason
	// ApproachTarget identifies the executor-selected living target that could not be projected.
	ApproachTarget world.Monster
	// HammerdinReposition requests a teleport toward another living monster
	// while the controller keeps the previous attack target pinned.
	HammerdinReposition bool
	// HammerdinRouteForward identifies the bounded route-direction fallback
	// used when no nearby alternate monster exists.
	HammerdinRouteForward bool
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
	// stickyTargetUnitID pins a Hammerdin target only after the LMB hold
	// actually starts. Aim-only ticks may still accept a different living
	// monster that Memory confirms under the cursor.
	stickyTargetUnitID  uint32
	stickyAttackHeld    bool
	stickyHoldSnapshots int
	stickyHoldStartedAt time.Time
	// A pending reposition target survives Teleport skill selection. The last
	// target remains excluded so repeated fallbacks keep advancing through the
	// nearby pack instead of teleporting to the same landing again.
	stickyRepositionPending      bool
	stickyRepositionReady        bool
	stickyRepositionTargetUnitID uint32
	stickyRepositionPosition     world.Position
	stickyRepositionRouteForward bool
	lastEligible                 int
	lastRelevant                 int
	lastCoverage                 float64
	lastComplete                 bool

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
		// AllowMercenary only while route clear would attack a threat or density target.
		AllowMercenary: assessment.RouteTargetFound ||
			(assessment.DensityTargetFound && !assessment.CoverageComplete) ||
			c.stickyTargetUnitID != 0,
		FailOnUnavailable: true,
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
	combat CombatConfig,
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

	stickyTarget, stickyTargetFound := c.hammerdinStickyTarget(state, combat.Profile)
	needsBlock := assessment.RouteTargetFound || !assessment.CoverageComplete || c.manaRecovery || stickyTargetFound ||
		definition.ID == RunIDCows && assessment.DensityTargetFound
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
		if err := c.emitClearStarted(state, progress, definition, cfg, combat.Profile, now); err != nil {
			return c.fail(RouteThreatReasonStateInvalid)
		}
	}
	previousEligible, previousRelevant := c.lastEligible, c.lastRelevant
	progressTargetUnitID := c.lastTargetUnitID
	if c.observeObjectiveProgress(state, progress, assessment, definition.RouteHostileNPCIDs, cfg, definition.ID == RunIDCows, combat.Profile, now) {
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

	// The encounter-wide objective watchdog is authoritative over every local
	// targeting recovery. Otherwise a repeated out-of-range signal can return
	// early forever after the movement budget is exhausted and prevent the
	// intended emergency terminal from ever being reached.
	if !c.lastProgressAt.IsZero() && now.Sub(c.lastProgressAt) >= cfg.NoProgressTimeout {
		if definition.ID == RunIDCows {
			return c.fail(RouteThreatReasonCowNoProgress)
		}
		return c.fail(RouteThreatReasonClearNoProgress)
	}

	target, mode, targetFound := selectRouteClearTarget(state, assessment, cfg)
	if stickyTargetFound {
		target = stickyTarget
		mode = profile.RouteClearThreat
		targetFound = true
	}
	if assessment.RouteTargetFound && !routeTargetWithinAttack(state, assessment.RouteTarget, cfg) && !assessment.DensityTargetFound {
		if c.observeOutOfRange(assessment.RouteTarget.UnitID) {
			return c.fail(RouteThreatReasonOutOfRange)
		}
	} else if !targetFound {
		c.resetOutOfRange()
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
	hammerdinApproachRequired := false
	if combat.Profile == hammerdinCombatProfileID {
		if stickyTargetFound && c.stickyRepositionPending {
			if c.manaRecovery {
				c.state = RouteThreatManaRecovery
				return RouteThreatTickResult{State: c.state, StopAttack: true}
			}
			approachTarget := stickyTarget
			if c.stickyRepositionRouteForward {
				approachTarget.Position = c.stickyRepositionPosition
			} else {
				var found bool
				approachTarget, found = state.FindMonsterByUnitID(c.stickyRepositionTargetUnitID)
				if !found {
					c.completeHammerdinReposition(false)
					c.stickyHoldStartedAt = now
					return RouteThreatTickResult{State: c.State()}
				}
			}
			failed := c.fail(RouteThreatReasonOutOfRange)
			failed.ApproachTarget = approachTarget
			failed.HammerdinReposition = true
			failed.HammerdinRouteForward = c.stickyRepositionRouteForward
			return failed
		}
		distance := world.Distance(state.Player.Position, target.Position)
		if stickyTargetFound && c.stickyAttackHeld {
			if !c.stickyHoldStartedAt.IsZero() && now.Sub(c.stickyHoldStartedAt) >= hammerdinAttackWindow {
				return c.beginHammerdinReposition(state, progress, stickyTarget, definition.RouteHostileNPCIDs)
			}
			c.stickyHoldSnapshots++
			if c.stickyHoldSnapshots >= hammerdinHoldRecheckSnapshots {
				c.stickyHoldSnapshots = 0
				hammerdinApproachRequired = distance > hammerdinLostDistanceTiles
			}
		} else if !c.stickyRepositionReady {
			hammerdinApproachRequired = distance > combat.RepositionDistanceTiles
		}
	}
	if hammerdinApproachRequired {
		c.state = RouteThreatClearing
		c.recordBlockBaseline(state, assessment)
		c.stickyAttackHeld = false
		c.stickyHoldSnapshots = 0
		if c.manaRecovery {
			c.state = RouteThreatManaRecovery
			return RouteThreatTickResult{State: c.state, StopAttack: true}
		}
		failed := c.fail(RouteThreatReasonOutOfRange)
		failed.ApproachTarget = target
		return failed
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
		RunID: string(definition.ID), DefinitionID: combat.Profile, Player: state.Player,
		Target: target, Mode: mode, AssessmentAt: assessment.SnapshotAt,
	}, now)
	if result.Status == profile.StatusPending && result.Reason == profile.RouteClearReasonTargetUnprojectable {
		approachTarget := target
		if result.TargetUnitID != 0 {
			if selected, found := state.FindMonsterByUnitID(result.TargetUnitID); found {
				approachTarget = selected
			}
		}
		// A valid target can be within the configured tile radius while still
		// lying outside the directional client viewport. Keep holding without
		// moving the cursor and require the same fresh-snapshot proof as range.
		if c.observeOutOfRange(approachTarget.UnitID) {
			c.stickyRepositionReady = false
			failed := c.fail(RouteThreatReasonOutOfRange)
			failed.ApproachTarget = approachTarget
			return failed
		}
		c.recordBlockBaseline(state, assessment)
		return RouteThreatTickResult{State: c.state}
	}
	if result.Status == profile.StatusFailed {
		return c.fail(RouteThreatReasonStateInvalid)
	}
	c.resetOutOfRange()
	trackingTarget := target
	if result.TargetUnitID != 0 && result.ActionKind != profile.RouteClearActionCorpseExplosion {
		trackingTarget.UnitID = result.TargetUnitID
	}
	if result.Status == profile.StatusAction {
		hoverConfirmed := result.TargetingMode != profile.MonsterTargetingWorldProjected
		actionTarget := target
		if result.TargetUnitID != 0 {
			actionTarget.UnitID = result.TargetUnitID
			actionTarget.NPCID = result.TargetNPCID
		}
		if result.ActionKind == profile.RouteClearActionCorpseExplosion {
			hoverConfirmed = false
		}
		if err := c.emitClearAction(state, progress, actionTarget, mode, combat.Profile, result, hoverConfirmed, now); err != nil {
			return c.fail(RouteThreatReasonStateInvalid)
		}
		if combat.Profile == hammerdinCombatProfileID && result.ActionKind == profile.RouteClearActionAttack {
			if c.stickyTargetUnitID != actionTarget.UnitID {
				c.stickyRepositionPending = false
				c.stickyRepositionReady = false
				c.stickyRepositionTargetUnitID = 0
				c.stickyRepositionPosition = world.Position{}
				c.stickyRepositionRouteForward = false
			}
			c.stickyTargetUnitID = actionTarget.UnitID
			c.stickyAttackHeld = true
			c.stickyRepositionReady = false
			c.stickyHoldSnapshots = 0
			c.stickyHoldStartedAt = now
		}
	}
	c.lastTargetUnitID = trackingTarget.UnitID
	c.lastTargetMode = mode
	c.recordBlockBaseline(state, assessment)
	return RouteThreatTickResult{State: c.state}
}

func (c *RouteThreatController) observeExternalProgress(now time.Time) {
	if !now.IsZero() {
		c.lastProgressAt = now
	}
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
	actionKind string,
	now time.Time,
) error {
	return c.emitApproachInput(state, progress, target, attempt, actionKind, now)
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
	if err := c.emitApproachProgress(state, progress, target, positionProgress, "approach", now); err != nil {
		return err
	}
	c.lastProgressAt = now
	c.resetOutOfRange()
	return nil
}

// ObserveApproachNoProgress records a settled approach whose measured player
// movement did not advance along the held command vector. It deliberately
// leaves controller progress and range proof untouched so the caller's bounded
// retry remains authoritative.
func (c *RouteThreatController) ObserveApproachNoProgress(
	state world.State,
	progress RouteProgress,
	target world.Monster,
	positionProgress float64,
	now time.Time,
) error {
	return c.emitApproachProgress(state, progress, target, positionProgress, "approach_no_progress", now)
}

func (c *RouteThreatController) completeHammerdinReposition(moved bool) {
	c.stickyRepositionPending = false
	c.stickyRepositionReady = moved
	c.stickyRepositionPosition = world.Position{}
	c.stickyRepositionRouteForward = false
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
	c.stickyTargetUnitID = 0
	c.stickyAttackHeld = false
	c.stickyHoldSnapshots = 0
	c.stickyHoldStartedAt = time.Time{}
	c.stickyRepositionPending = false
	c.stickyRepositionReady = false
	c.stickyRepositionTargetUnitID = 0
	c.stickyRepositionPosition = world.Position{}
	c.stickyRepositionRouteForward = false
	c.recordBlockBaseline(state, assessment)
}

func (c *RouteThreatController) observeObjectiveProgress(state world.State, progress RouteProgress, assessment ThreatAssessment, allowed []uint32, cfg RouteCombatConfig, cowHold bool, profileID string, now time.Time) bool {
	targetProgressed := c.lastTargetUnitID != 0 &&
		!routeClearTargetStillRelevant(state, progress, c.lastTargetUnitID, c.lastTargetMode, allowed, cfg, assessment.CoverageComplete, cowHold)
	progressed := targetProgressed
	if profileID == hammerdinCombatProfileID && c.stickyTargetUnitID != 0 {
		_, found := state.FindMonsterByUnitID(c.stickyTargetUnitID)
		targetProgressed = !found
		progressed = targetProgressed
	} else if assessment.RelevantThreatCount < c.lastRelevant ||
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
		c.stickyTargetUnitID = 0
		c.stickyAttackHeld = false
		c.stickyHoldSnapshots = 0
		c.stickyHoldStartedAt = time.Time{}
		c.stickyRepositionPending = false
		c.stickyRepositionReady = false
		c.stickyRepositionTargetUnitID = 0
		c.stickyRepositionPosition = world.Position{}
		c.stickyRepositionRouteForward = false
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
	c.stickyTargetUnitID = 0
	c.stickyAttackHeld = false
	c.stickyHoldSnapshots = 0
	c.stickyHoldStartedAt = time.Time{}
	c.stickyRepositionPending = false
	c.stickyRepositionReady = false
	c.stickyRepositionTargetUnitID = 0
	c.stickyRepositionPosition = world.Position{}
	c.stickyRepositionRouteForward = false
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

func (c *RouteThreatController) hammerdinStickyTarget(state world.State, profileID string) (world.Monster, bool) {
	if profileID != hammerdinCombatProfileID || c.stickyTargetUnitID == 0 {
		return world.Monster{}, false
	}
	return state.FindMonsterByUnitID(c.stickyTargetUnitID)
}

func (c *RouteThreatController) beginHammerdinReposition(state world.State, progress RouteProgress, target world.Monster, allowedNPCIDs []uint32) RouteThreatTickResult {
	repositionTarget, found := selectHammerdinRepositionTarget(state, target, c.stickyRepositionTargetUnitID, allowedNPCIDs)
	if !found {
		position, ok := hammerdinRouteForwardPosition(state.Player.Position, progress)
		if !ok {
			return RouteThreatTickResult{State: c.State()}
		}
		c.stickyAttackHeld = false
		c.stickyHoldSnapshots = 0
		c.stickyHoldStartedAt = time.Time{}
		c.stickyRepositionPending = true
		c.stickyRepositionReady = false
		c.stickyRepositionPosition = position
		c.stickyRepositionRouteForward = true
		approachTarget := target
		approachTarget.Position = position
		failed := c.fail(RouteThreatReasonOutOfRange)
		failed.ApproachTarget = approachTarget
		failed.HammerdinReposition = true
		failed.HammerdinRouteForward = true
		return failed
	}
	c.stickyAttackHeld = false
	c.stickyHoldSnapshots = 0
	c.stickyHoldStartedAt = time.Time{}
	c.stickyRepositionPending = true
	c.stickyRepositionReady = false
	c.stickyRepositionTargetUnitID = repositionTarget.UnitID
	c.stickyRepositionPosition = repositionTarget.Position
	c.stickyRepositionRouteForward = false
	failed := c.fail(RouteThreatReasonOutOfRange)
	failed.ApproachTarget = repositionTarget
	failed.HammerdinReposition = true
	return failed
}

func selectHammerdinRepositionTarget(state world.State, pinned world.Monster, excludedUnitID uint32, allowedNPCIDs []uint32) (world.Monster, bool) {
	var selected world.Monster
	var selectedDistance float64
	for _, candidate := range state.Monsters {
		if candidate.UnitID == 0 || candidate.UnitID == pinned.UnitID || candidate.UnitID == excludedUnitID ||
			!routeHostileAllowed(candidate.NPCID, allowedNPCIDs) {
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

func hammerdinRouteForwardPosition(player world.Position, progress RouteProgress) (world.Position, bool) {
	if !progress.TargetAvailable {
		return world.Position{}, false
	}
	distance := world.Distance(player, progress.MovementTarget)
	if distance <= routeThreatApproachProgressEpsilonTiles {
		return world.Position{}, false
	}
	step := math.Min(distance, hammerdinRouteForwardDistanceTiles)
	scale := step / distance
	return world.Position{
		X: uint32(math.Round(float64(player.X) + (float64(progress.MovementTarget.X)-float64(player.X))*scale)),
		Y: uint32(math.Round(float64(player.Y) + (float64(progress.MovementTarget.Y)-float64(player.Y))*scale)),
	}, true
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

func routeClearTargetStillRelevant(state world.State, progress RouteProgress, unitID uint32, mode profile.RouteClearMode, allowed []uint32, cfg RouteCombatConfig, coverageComplete, cowHold bool) bool {
	for _, monster := range state.Monsters {
		if monster.UnitID != unitID || !routeHostileAllowed(monster.NPCID, allowed) {
			continue
		}
		if cowHold {
			return routeTargetWithinAttack(state, monster, cfg)
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
