package tasks

import (
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// assessThreats evaluates one immutable World snapshot in one allocation-free
// pass. It neither reads process memory nor authorizes input.
func assessThreats(state world.State, progress RouteProgress, allowedNPCIDs []uint32, cfg RouteCombatConfig) ThreatAssessment {
	requiredCoverage := cfg.ImmediateRadiusTiles
	if progress.TargetAvailable {
		requiredCoverage = math.Max(requiredCoverage, world.Distance(state.Player.Position, progress.MovementTarget)+cfg.LandingRadiusTiles)
	}
	assessment := ThreatAssessment{
		SnapshotAt:            state.At,
		RequiredCoverageTiles: requiredCoverage,
		CoverageComplete:      !state.MonsterCoverage.MonstersTruncated || state.MonsterCoverage.MonsterCoverageRadiusTiles > requiredCoverage,
	}

	var routeDistanceSquared, densityDistanceSquared float64
	for _, monster := range state.Monsters {
		if monster.IsHovered {
			// Route clear is already active because an allowlisted blocker holds
			// movement. Any living monster currently confirmed under the cursor
			// is a better immediate target than another aim-only poll.
			assessment.HoveredRouteTarget = monster
			assessment.HoveredRouteTargetFound = true
		}
		if !routeHostileAllowed(monster.NPCID, allowedNPCIDs) {
			continue
		}
		playerDistanceSquared := positionDistanceSquared(state.Player.Position, monster.Position)
		zone := ThreatZoneNone
		switch {
		case playerDistanceSquared <= cfg.ImmediateRadiusTiles*cfg.ImmediateRadiusTiles:
			zone = ThreatZoneImmediate
		case progress.TargetAvailable && positionDistanceSquared(progress.MovementTarget, monster.Position) <= cfg.LandingRadiusTiles*cfg.LandingRadiusTiles:
			zone = ThreatZoneLanding
		case progress.TargetAvailable && pointWithinCorridor(monster.Position, state.Player.Position, progress.MovementTarget, cfg.CorridorWidthTiles):
			zone = ThreatZoneCorridor
		}
		if zone != ThreatZoneNone {
			assessment.RelevantThreatCount++
			if !assessment.RouteTargetFound ||
				routeZonePriority(zone) < routeZonePriority(assessment.RouteZone) ||
				(routeZonePriority(zone) == routeZonePriority(assessment.RouteZone) &&
					preferLivingTarget(monster, playerDistanceSquared, assessment.RouteTarget, routeDistanceSquared, assessment.RouteTargetFound)) {
				assessment.RouteTarget = monster
				assessment.RouteTargetFound = true
				assessment.RouteZone = zone
				routeDistanceSquared = playerDistanceSquared
			}
		}
		if playerDistanceSquared <= cfg.AttackDistanceTiles*cfg.AttackDistanceTiles &&
			preferLivingTarget(monster, playerDistanceSquared, assessment.DensityTarget, densityDistanceSquared, assessment.DensityTargetFound) {
			assessment.DensityTarget = monster
			assessment.DensityTargetFound = true
			densityDistanceSquared = playerDistanceSquared
		}
	}
	return assessment
}

func preferLivingTarget(candidate world.Monster, candidateDistanceSquared float64, current world.Monster, currentDistanceSquared float64, found bool) bool {
	return !found || candidateDistanceSquared < currentDistanceSquared ||
		(candidateDistanceSquared == currentDistanceSquared && candidate.UnitID < current.UnitID)
}

func routeHostileAllowed(npcID uint32, allowedNPCIDs []uint32) bool {
	for _, allowed := range allowedNPCIDs {
		if npcID == allowed {
			return true
		}
	}
	return false
}

func routeZonePriority(zone ThreatZone) int {
	switch zone {
	case ThreatZoneImmediate:
		return 0
	case ThreatZoneLanding:
		return 1
	case ThreatZoneCorridor:
		return 2
	default:
		return 3
	}
}

func pointWithinCorridor(point, start, end world.Position, width float64) bool {
	dx := float64(end.X) - float64(start.X)
	dy := float64(end.Y) - float64(start.Y)
	if dx == 0 && dy == 0 {
		return positionDistanceSquared(point, start) <= width*width
	}
	px := float64(point.X) - float64(start.X)
	py := float64(point.Y) - float64(start.Y)
	projection := (px*dx + py*dy) / (dx*dx + dy*dy)
	if projection < 0 || projection > 1 {
		return false
	}
	nearestX := float64(start.X) + projection*dx
	nearestY := float64(start.Y) + projection*dy
	offsetX := float64(point.X) - nearestX
	offsetY := float64(point.Y) - nearestY
	return offsetX*offsetX+offsetY*offsetY <= width*width
}

func positionDistanceSquared(a, b world.Position) float64 {
	dx := float64(a.X) - float64(b.X)
	dy := float64(a.Y) - float64(b.Y)
	return dx*dx + dy*dy
}

// selectAimableRouteTarget only changes attack preference. Off-screen monsters
// remain in the authoritative threat assessment and cannot be marked cleared.
// Zone priority precedes temporary exclusion, hover confirmation and distance.
func selectAimableRouteTarget(state world.State, progress RouteProgress, assessment ThreatAssessment, allowed []uint32, cfg RouteCombatConfig, combat CombatActions, avoid uint32) (world.Monster, profile.RouteClearMode, bool) {
	var selected world.Monster
	selectedZone := ThreatZoneNone
	selectedDistance := 0.0
	for _, candidate := range state.Monsters {
		if candidate.UnitID == 0 || !routeHostileAllowed(candidate.NPCID, allowed) || !routeTargetWithinAttack(state, candidate, cfg) {
			continue
		}
		zone := threatZoneForMonster(state.Player.Position, progress, candidate.Position, cfg)
		if zone == ThreatZoneNone && !assessment.DensityTargetFound {
			continue
		}
		if !candidate.IsHovered && !combat.MonsterAimProjectable(state.Player.Position, candidate.Position) {
			continue
		}
		distance := positionDistanceSquared(state.Player.Position, candidate.Position)
		prefer := selected.UnitID == 0 || routeZonePriority(zone) < routeZonePriority(selectedZone)
		if selected.UnitID != 0 && zone == selectedZone {
			switch {
			case (candidate.UnitID == avoid) != (selected.UnitID == avoid):
				prefer = selected.UnitID == avoid
			case candidate.IsHovered != selected.IsHovered:
				prefer = candidate.IsHovered
			default:
				prefer = preferLivingTarget(candidate, distance, selected, selectedDistance, true)
			}
		}
		if prefer {
			selected, selectedZone, selectedDistance = candidate, zone, distance
		}
	}
	mode := profile.RouteClearThreat
	if selectedZone == ThreatZoneNone {
		mode = profile.RouteClearDensityRelief
	}
	return selected, mode, selected.UnitID != 0
}
