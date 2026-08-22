package app

import (
	"log/slog"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// chestOperateAdapter wraps the shared hover-confirmed entity clicker for hut
// Supertruhen and nearby racks. Approach and Mode settle stay in the task pipeline.
type chestOperateAdapter struct {
	log           *slog.Logger
	clicker       *pathing.EntityClicker
	targetUnitID  uint32
	probeActive   bool
	blockerUnitID uint32
}

func newChestOperateAdapter(log *slog.Logger, driver pathing.InputDriver, pathCfg pathing.Config) *chestOperateAdapter {
	// Supertruhen sitzen über ihrer Bodenkachel. Der normale Objekt-Anker
	// (Default 2 Kacheln) legt die Hover-Spirale auf den Sprite, nicht auf den
	// Boden; Offset 0 traf den Deckel erst bei Versuch 11–12 von 15.
	return &chestOperateAdapter{log: log, clicker: pathing.NewEntityClicker(log, driver, pathCfg.Projector(), pathCfg.Click)}
}

// Tick advances one hover-confirmed object click toward target.
func (a *chestOperateAdapter) Tick(state world.State, target world.Object, maxDistance float64) tasks.ChestOperateResult {
	if a == nil || a.clicker == nil {
		return tasks.ChestOperateResult{Status: tasks.ChestOperateFailed, Done: true, Reason: "chest_not_wired"}
	}
	if a.targetUnitID != target.UnitID {
		a.Reset()
		a.targetUnitID = target.UnitID
	}
	if a.probeActive && state.Hover.IsHovered && state.Hover.UnitType == world.HoverUnitTypeMonster {
		a.blockerUnitID = state.Hover.UnitID
	}
	result, err := a.clicker.Tick(state, pathing.ClickTarget{
		UnitID:   target.UnitID,
		UnitType: world.HoverUnitTypeObject,
		Position: target.Position,
		Name:     target.Name,
	}, maxDistance)
	if err != nil {
		if a.log != nil {
			a.log.Error("chest interaction failed", "unit_id", target.UnitID, "error", err)
		}
		return tasks.ChestOperateResult{Status: tasks.ChestOperateFailed, Done: true, Reason: "chest_operate_failed"}
	}
	status := tasks.ChestOperatePending
	switch result.Status {
	case pathing.ClickHit:
		status = tasks.ChestOperateClicked
	case pathing.ClickTooFar:
		status = tasks.ChestOperateTooFar
	case pathing.ClickHoverNotFound:
		status = tasks.ChestOperateHoverNotFound
	case pathing.ClickProjectionFailed:
		status = tasks.ChestOperateFailed
	}
	a.probeActive = !result.Done
	return tasks.ChestOperateResult{
		Status: status, Done: result.Done, Attempt: result.Attempt, BlockerUnitID: a.blockerUnitID,
	}
}

// Reset clears hover-search state between UnitIDs.
func (a *chestOperateAdapter) Reset() {
	if a != nil && a.clicker != nil {
		a.clicker.Reset()
	}
	if a != nil {
		a.targetUnitID = 0
		a.probeActive = false
		a.blockerUnitID = 0
	}
}
