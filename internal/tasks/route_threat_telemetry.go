package tasks

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (c *RouteThreatController) observeSnapshotTelemetry(
	state world.State,
	progress RouteProgress,
	assessment ThreatAssessment,
	definition RunDefinition,
	now time.Time,
) error {
	if c.telemetry == nil {
		return nil
	}
	if assessment.RouteTargetFound &&
		(assessment.RouteTarget.UnitID != c.telemetryTargetUnitID || assessment.RouteZone != c.telemetryTargetZone) {
		target := assessment.RouteTarget
		if err := c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteThreatDetected, state, progress, now, telemetry.Event{
			Run: string(definition.ID), Zone: string(assessment.RouteZone), UnitID: target.UnitID, NPCID: target.NPCID,
			TargetX: target.Position.X, TargetY: target.Position.Y,
			PlayerX: state.Player.Position.X, PlayerY: state.Player.Position.Y,
			DistanceTiles: world.Distance(state.Player.Position, target.Position),
		})); err != nil {
			return err
		}
		c.telemetryTargetUnitID = target.UnitID
		c.telemetryTargetZone = assessment.RouteZone
	}
	if !assessment.RouteTargetFound {
		c.telemetryTargetUnitID = 0
		c.telemetryTargetZone = ThreatZoneNone
	}

	truncated := state.MonsterCoverage.MonstersTruncated
	coverageComplete := assessment.CoverageComplete
	if !c.telemetrySaturationKnown || truncated != c.telemetryTruncated ||
		truncated && coverageComplete != c.telemetryCoverageComplete {
		if truncated || c.telemetryTruncated {
			if err := c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteMonsterSnapshotSaturated, state, progress, now, telemetry.Event{
				Run: string(definition.ID), MonstersTruncated: routeTelemetryBool(truncated),
				EligibleMonsterCount: state.MonsterCoverage.EligibleMonsterCount,
				RetainedMonsterCount: len(state.Monsters),
				CoverageRadiusTiles:  state.MonsterCoverage.MonsterCoverageRadiusTiles,
				RequiredRadiusTiles:  assessment.RequiredCoverageTiles,
				CoverageComplete:     routeTelemetryBool(coverageComplete),
			})); err != nil {
				return err
			}
		}
		c.telemetrySaturationKnown = true
		c.telemetryTruncated = truncated
		c.telemetryCoverageComplete = coverageComplete
	}
	return nil
}

func (c *RouteThreatController) emitClearStarted(
	state world.State,
	progress RouteProgress,
	definition RunDefinition,
	cfg RouteCombatConfig,
	profileID string,
	now time.Time,
) error {
	c.telemetryBlockStartedAt = now
	c.telemetryActions = 0
	c.telemetryDensityActions = 0
	c.telemetryTargets = make(map[uint32]struct{})
	if c.telemetry == nil {
		return nil
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteClearStarted, state, progress, now, telemetry.Event{
		Run: string(definition.ID), Profile: profileID, Strategy: string(profile.RouteClearSingleTarget),
		HPPercent: state.Player.HPPercent(), ManaPercent: state.Player.ManaPercent(),
		NoProgressTimeoutMs: cfg.NoProgressTimeout.Milliseconds(),
	}))
}

func (c *RouteThreatController) emitClearAction(
	state world.State,
	progress RouteProgress,
	target world.Monster,
	mode profile.RouteClearMode,
	profileID string,
	result profile.Result,
	hoverConfirmed bool,
	now time.Time,
) error {
	c.telemetryActions++
	if mode == profile.RouteClearDensityRelief {
		c.telemetryDensityActions++
	}
	if c.telemetryTargets == nil {
		c.telemetryTargets = make(map[uint32]struct{})
	}
	c.telemetryTargets[target.UnitID] = struct{}{}
	if c.telemetry == nil {
		return nil
	}
	actionIndex := c.telemetryActions
	actionKind := result.ActionKind
	if actionKind == "" {
		actionKind = profile.RouteClearActionAttack
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteClearAction, state, progress, now, telemetry.Event{
		ModeName: string(mode), Profile: profileID, Strategy: string(profile.RouteClearSingleTarget),
		SkillID: result.SkillID, ActionKind: string(actionKind), TargetingMode: string(result.TargetingMode), UnitID: target.UnitID, NPCID: target.NPCID,
		CowGroupAnchorUnitID: result.CowGroupAnchorUnitID, CowGroupLivingCount: result.CowGroupLivingCount,
		CowCorpseAnchorDistanceTiles: result.CowCorpseAnchorDistanceTiles, CowCorpseCoverageCount: result.CowCorpseCoverageCount,
		ActionIndex: &actionIndex, HoverConfirmed: routeTelemetryBool(hoverConfirmed),
	}))
}

func (c *RouteThreatController) emitApproachInput(
	state world.State,
	progress RouteProgress,
	target world.Monster,
	attempt int,
	actionKind string,
	now time.Time,
) error {
	if c.telemetry == nil {
		return nil
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteClearAction, state, progress, now, telemetry.Event{
		ModeName: "approach", Strategy: actionKind, ActionKind: actionKind,
		UnitID: target.UnitID, NPCID: target.NPCID, Attempt: attempt,
		PlayerX: state.Player.Position.X, PlayerY: state.Player.Position.Y,
		DistanceTiles:  world.Distance(state.Player.Position, target.Position),
		HoverConfirmed: routeTelemetryBool(false),
	}))
}

func (c *RouteThreatController) emitApproachProgress(
	state world.State,
	progress RouteProgress,
	target world.Monster,
	positionProgress float64,
	progressKind string,
	now time.Time,
) error {
	if c.telemetry == nil {
		return nil
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteClearProgress, state, progress, now, telemetry.Event{
		ProgressKind: progressKind, UnitID: target.UnitID, NPCID: target.NPCID,
		PlayerX: state.Player.Position.X, PlayerY: state.Player.Position.Y,
		DistanceTiles:         world.Distance(state.Player.Position, target.Position),
		PositionProgressTiles: positionProgress,
	}))
}

func (c *RouteThreatController) emitClearProgress(
	state world.State,
	progress RouteProgress,
	assessment ThreatAssessment,
	targetUnitID uint32,
	previousEligible int,
	previousRelevant int,
	now time.Time,
) error {
	if c.telemetry == nil {
		return nil
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteClearProgress, state, progress, now, telemetry.Event{
		ProgressKind: "objective", UnitID: targetUnitID,
		PreviousEligibleCount: previousEligible, EligibleMonsterCount: state.MonsterCoverage.EligibleMonsterCount,
		PreviousRelevantCount: previousRelevant, RelevantThreatCount: assessment.RelevantThreatCount,
		MonstersTruncated: routeTelemetryBool(state.MonsterCoverage.MonstersTruncated),
	}))
}

func (c *RouteThreatController) emitClearCompleted(state world.State, progress RouteProgress, now time.Time) error {
	if c.telemetry == nil {
		return nil
	}
	hold := time.Duration(0)
	if !c.telemetryBlockStartedAt.IsZero() && now.After(c.telemetryBlockStartedAt) {
		hold = now.Sub(c.telemetryBlockStartedAt)
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteClearCompleted, state, progress, now, telemetry.Event{
		ElapsedMs:         hold.Milliseconds(),
		CombatActionsSent: c.telemetryActions, TargetsSeen: len(c.telemetryTargets),
		DensityReliefActions: c.telemetryDensityActions, HoldMs: hold.Milliseconds(),
	}))
}

func (c *RouteThreatController) emitRecoverySuppressed(
	state world.State,
	progress RouteProgress,
	reason RouteThreatReason,
	now time.Time,
) error {
	if c.telemetry == nil {
		return nil
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteRecoverySuppressed, state, progress, now, telemetry.Event{
		Reason: string(reason), Attempt: progress.LocalRecoveryAttempts,
		PositionProgressTiles: world.Distance(progress.RecoveryInputOrigin, state.Player.Position),
	}))
}

// ObserveResourceResult emits only mana-hold start, material change, and end.
func (c *RouteThreatController) ObserveResourceResult(
	state world.State,
	progress RouteProgress,
	resourceContext profile.ResourceContext,
	result profile.Result,
	now time.Time,
) error {
	active := resourceContext.MobilityCritical
	if !c.telemetryManaKnown && !active {
		return nil
	}
	materialChange := !c.telemetryManaKnown ||
		active != c.telemetryManaActive ||
		resourceContext.Threatened != c.telemetryManaThreatened ||
		result.Resource != c.telemetryManaResource
	if !materialChange {
		return nil
	}
	c.telemetryManaKnown = true
	c.telemetryManaActive = active
	c.telemetryManaThreatened = resourceContext.Threatened
	c.telemetryManaResource = result.Resource
	if c.telemetry == nil {
		return nil
	}
	demand := "end"
	if active {
		demand = "hold"
	}
	return c.telemetry.Emit(routeTelemetryEvent(telemetry.RouteManaHold, state, progress, now, telemetry.Event{
		ManaPercent: state.Player.ManaPercent(), ManaDemand: demand,
		Threatened: routeTelemetryBool(resourceContext.Threatened), Resource: string(result.Resource),
	}))
}

func routeTelemetryEvent(name telemetry.EventName, state world.State, progress RouteProgress, now time.Time, event telemetry.Event) telemetry.Event {
	segmentIndex, pointIndex := progress.SegmentIndex, progress.PointIndex
	event.Event = name
	event.Timestamp = now
	event.AreaID = uint32(state.Area.ID)
	if progress.RouteRole == "" {
		event.RouteID = progress.RouteID
	} else {
		// One multi-route run keeps its primary route immutable in history. The
		// recorder supplies that route ID while this event identifies the active
		// setup member by its fixed role.
		event.RouteID = ""
		event.RouteRole = string(progress.RouteRole)
	}
	event.SegmentID = progress.SegmentID
	event.SegmentIndex = &segmentIndex
	event.PointIndex = &pointIndex
	event.TargetX = progress.MovementTarget.X
	event.TargetY = progress.MovementTarget.Y
	event.DriftTiles = progress.DriftTiles
	event.LocalRecoveryAttempts = progress.LocalRecoveryAttempts
	return event
}

func routeTelemetryBool(value bool) *bool {
	return &value
}
