package tasks

import (
	"math"

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
