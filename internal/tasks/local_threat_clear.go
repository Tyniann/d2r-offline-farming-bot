package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// Local clears are anchor-bound and one-shot. These limits keep recovery
	// from turning an object interaction into unrestricted route combat.
	localThreatClearRadiusTiles       float64 = 12
	localThreatClearMaxActions                = 12
	localThreatClearStableSnapshots           = 3
	localThreatClearTimeout                   = 6 * time.Second
	localThreatClearNoProgressTimeout         = 3 * time.Second
)

type localThreatClear struct {
	anchor            world.Position
	preferredUnitID   uint32
	actions           int
	noTargetSnapshots int
	startedAt         time.Time
	lastActionAt      time.Time
	lastSnapshotAt    time.Time
}

type localThreatClearResult struct {
	done   bool
	failed bool
	reason string
}

func (c *localThreatClear) start(anchor world.Position, preferredUnitID uint32, now time.Time) {
	*c = localThreatClear{
		anchor:          anchor,
		preferredUnitID: preferredUnitID,
		startedAt:       now,
		lastActionAt:    now,
	}
}

func (c *localThreatClear) tick(ctx context.Context, clear RouteClearExecutor, state world.State, now time.Time, runID, profileID string) localThreatClearResult {
	if clear == nil {
		return localThreatClearResult{failed: true, reason: "combat_not_wired"}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return localThreatClearResult{}
	}
	if c.actions >= localThreatClearMaxActions ||
		now.Sub(c.startedAt) >= localThreatClearTimeout ||
		now.Sub(c.lastActionAt) >= localThreatClearNoProgressTimeout {
		return localThreatClearResult{done: true}
	}
	target, found := selectLocalThreat(state, c.anchor, c.preferredUnitID, localThreatClearRadiusTiles)
	if !found {
		clear.ResetRouteClear()
		snapshotAt := state.At
		if snapshotAt.IsZero() {
			snapshotAt = now
		}
		if snapshotAt != c.lastSnapshotAt {
			c.lastSnapshotAt = snapshotAt
			c.noTargetSnapshots++
		}
		return localThreatClearResult{done: c.noTargetSnapshots >= localThreatClearStableSnapshots}
	}
	c.noTargetSnapshots = 0
	c.lastSnapshotAt = state.At
	result := clear.TickRouteClear(ctx, profile.RouteClearRequest{
		RunID: runID, DefinitionID: profileID, Player: state.Player, Target: target,
		Mode: profile.RouteClearThreat, AssessmentAt: state.At,
	}, now)
	switch result.Status {
	case profile.StatusFailed:
		return localThreatClearResult{failed: true, reason: "combat_action_failed"}
	case profile.StatusAction:
		c.actions++
		c.lastActionAt = now
	}
	return localThreatClearResult{}
}

func (c *localThreatClear) reset(clear RouteClearExecutor) {
	if clear != nil {
		clear.ResetRouteClear()
	}
	*c = localThreatClear{}
}

func selectLocalThreat(state world.State, anchor world.Position, preferredUnitID uint32, radius float64) (world.Monster, bool) {
	var nearest world.Monster
	nearestDistance := 0.0
	found := false
	for _, monster := range state.Monsters {
		distance := world.Distance(monster.Position, anchor)
		if distance > radius {
			continue
		}
		if monster.UnitID == preferredUnitID {
			return monster, true
		}
		if !found || distance < nearestDistance {
			nearest = monster
			nearestDistance = distance
			found = true
		}
	}
	return nearest, found
}
