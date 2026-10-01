package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/crafting"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type storageCompactionAdapter struct {
	executor   *crafting.Executor
	controller loot.CollectionInput
	loot       *lootActionsAdapter
	request    crafting.Request
	terminal   crafting.Result
	started    bool
}

func newStorageCompactionAdapter(log *slog.Logger, controller loot.CollectionInput, looting *lootActionsAdapter) (*storageCompactionAdapter, error) {
	actions := loot.NewCollectionActions(log, controller, loot.DefaultCollectionLayout())
	executor, err := crafting.NewExecutor(log, actions)
	if err != nil {
		return nil, fmt.Errorf("storage compaction executor: %w", err)
	}
	return &storageCompactionAdapter{executor: executor, controller: controller, loot: looting}, nil
}

func (a *storageCompactionAdapter) Reset() {
	a.executor.Reset()
	a.request = crafting.Request{}
	a.terminal = crafting.Result{}
	a.started = false
}
func (a *storageCompactionAdapter) Tick(s world.State, now time.Time, r crafting.Request) crafting.Result {
	if a.terminal.Done {
		return a.terminal
	}
	status := a.controller.Status()
	if status.Paused || status.Stopped {
		return a.executor.Tick(s, now, r, status.Paused, status.Stopped)
	}
	// The original Keep/lock policy is checked again on every actionable tick;
	// collection counts need no longer be full after the first withdrawal.
	bound := a.loot.compactionTrigger
	if a.loot.telemetryErr != nil {
		return a.telemetryFailed(r, a.loot.telemetryErr)
	}
	if !status.Enabled || bound == nil || bound.UnitID != r.UnitID || bound.Code != r.Code || bound.GridX != r.GridX || bound.GridY != r.GridY || !s.Collection.ScopeKnown || s.Collection.ScopeID != a.loot.compactionScope || !a.loot.stash.AuthorizeCompaction(s, *bound) {
		// Freeze the stash-time scope before the executor's own first snapshot.
		// Rejected policy/context results remain terminal even if a caller reticks.
		a.terminal = crafting.Result{Done: true, Failure: &crafting.Failure{Reason: crafting.ReasonUnconfirmed, TriggerCode: r.Code, MaterialCode: r.Code}}
		if err := a.emit(s, now, r, telemetry.StorageCompactionFailed, a.terminal); err != nil {
			return a.telemetryFailed(r, err)
		}
		return a.terminal
	}
	a.request = r
	if !a.started {
		if err := a.emit(s, now, r, telemetry.StorageCompactionStarted, crafting.Result{}); err != nil {
			return a.telemetryFailed(r, err)
		}
		a.started = true
	}
	result := a.executor.Tick(s, now, r, false, false)
	// Emit only confirmed progress. A failed write latches before another input
	// tick, even if the executor already accepted the stored output.
	if result.Progress != nil {
		if err := a.emit(s, now, r, telemetry.StorageCompactionRecipe, result); err != nil {
			return a.telemetryFailed(r, err)
		}
	}
	if result.Done {
		event := telemetry.StorageCompactionCompleted
		if !result.Success {
			event = telemetry.StorageCompactionFailed
		}
		if err := a.emit(s, now, r, event, result); err != nil {
			return a.telemetryFailed(r, err)
		}
		a.terminal = result
		a.terminal.Progress = nil
	}
	return result
}

func (a *storageCompactionAdapter) telemetryFailed(r crafting.Request, err error) crafting.Result {
	a.terminal = crafting.Result{Done: true, Failure: &crafting.Failure{Reason: "telemetry_failed", TriggerCode: r.Code, MaterialCode: r.Code, Cause: err}}
	return a.terminal
}

func (a *storageCompactionAdapter) emit(s world.State, now time.Time, r crafting.Request, name telemetry.EventName, result crafting.Result) error {
	event := telemetry.Event{Timestamp: now, Event: name, Stage: telemetry.HistoryStageReturnTown, Step: "compact_storage", AreaID: uint32(s.Area.ID), UnitID: r.UnitID, Code: r.Code}
	if result.Failure != nil {
		event.Reason = result.Failure.Reason
		event.ReasonParams = map[string]string{"item_code": result.Failure.TriggerCode, "material_code": result.Failure.MaterialCode}
	}
	if p := result.Progress; p != nil {
		recipe, _ := crafting.LookupRecipe(p.InputCode)
		event.Compaction = &telemetry.CompactionProgress{RecipeKey: recipe.SourceKey, RecipeIndex: p.RecipeIndex, SourceCode: p.InputCode, OutputCode: p.OutputCode, SourceBefore: p.InputBefore, SourceAfter: p.InputAfter, OutputBefore: p.OutputBefore, OutputAfter: p.OutputAfter}
	}
	return a.loot.emit(event)
}

// observePause is called by the runtime even when Tasks are input-gated. It
// freezes the executor's active budgets without advancing an input stage.
func (a *storageCompactionAdapter) observePause(s world.State, now time.Time) {
	if a == nil || a.request.UnitID == 0 {
		return
	}
	status := a.controller.Status()
	if status.Paused || status.Stopped {
		a.executor.Tick(s, now, a.request, status.Paused, status.Stopped)
	}
}

func isStorageCompactionFailure(reason string) bool {
	switch reason {
	case crafting.ReasonGemFull, crafting.ReasonRuneFull, crafting.ReasonUnavailable, crafting.ReasonCubeNotEmpty, crafting.ReasonUnconfirmed, crafting.ReasonTimeout:
		return true
	default:
		return false
	}
}
