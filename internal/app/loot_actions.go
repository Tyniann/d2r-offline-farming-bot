package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type lootActionsAdapter struct {
	log       *slog.Logger
	filter    *loot.Filter
	cfg       loot.PickupConfig
	clicker   loot.PickupClicker
	active    *loot.PickupExecutor
	skipped   map[uint32]bool
	lastStart tasks.LootTarget
}

func newLootActionsAdapter(log *slog.Logger, filter *loot.Filter, pickupCfg config.LootPickupConfig, driver pathing.InputDriver, pathingCfg pathing.Config) *lootActionsAdapter {
	clicker := pathing.NewEntityClicker(log, driver, pathingCfg.Projector(), pathingCfg.Click)
	return &lootActionsAdapter{
		log:     log.With("component", "loot_actions"),
		filter:  filter,
		cfg:     mapLootPickupConfig(pickupCfg),
		clicker: &pickupClickerAdapter{input: driver, clicker: clicker},
		skipped: make(map[uint32]bool),
	}
}

func (a *lootActionsAdapter) Scan(state world.State) tasks.LootScanResult {
	if a == nil || a.filter == nil {
		return tasks.LootScanResult{}
	}
	report := a.filter.Decide(state)
	target, found := loot.SelectPickupCandidateExcluding(state, report, a.skipped)
	result := tasks.LootScanResult{
		GroundItemCount: report.GroundItemCount,
		CandidateCount:  countPickupCandidates(report, a.skipped),
		HasTarget:       found,
	}
	if found {
		result.NextTarget = mapTaskLootTarget(target)
	}
	a.log.Info("countess loot scan",
		"ground_item_count", result.GroundItemCount,
		"candidate_count", result.CandidateCount,
		"blocked_candidate_count", countBlockedPickupCandidates(report),
		"has_target", result.HasTarget,
	)
	return result
}

func (a *lootActionsAdapter) StartPickup(target tasks.LootTarget) error {
	if a == nil || a.filter == nil || a.clicker == nil {
		return fmt.Errorf("loot actions not wired")
	}
	if a.active != nil {
		return fmt.Errorf("loot pickup already active")
	}
	if target.UnitID == 0 || target.AreaID == 0 {
		return fmt.Errorf("invalid loot target")
	}
	if a.skipped[target.UnitID] {
		return fmt.Errorf("loot target already skipped: unit_id=%d", target.UnitID)
	}
	a.lastStart = target
	a.active = loot.NewPickupExecutor(a.log, a.cfg, a.clicker, mapLootPickupTarget(target))
	return nil
}

func (a *lootActionsAdapter) TickPickup(state world.State, now time.Time) tasks.LootPickupResult {
	if a == nil || a.active == nil {
		return tasks.LootPickupResult{Status: tasks.LootPickupInvalidWorld, Done: true}
	}
	res := a.active.Tick(state, now)
	out := mapTaskLootPickupResult(res)
	if out.Done {
		a.active = nil
		if isSkippedPickupStatus(out.Status) {
			a.skipped[out.Target.UnitID] = true
		}
	}
	return out
}

func (a *lootActionsAdapter) Reset() {
	if a == nil {
		return
	}
	a.active = nil
	a.lastStart = tasks.LootTarget{}
	a.skipped = make(map[uint32]bool)
	if a.clicker != nil {
		a.clicker.Reset()
	}
}

func countPickupCandidates(report loot.DecisionReport, skipped map[uint32]bool) int {
	count := 0
	for _, decision := range report.Decisions {
		if decision.Stage != loot.DecisionStagePickCandidate ||
			decision.Kind != loot.DecisionKindPickCandidate ||
			!decision.CanFit ||
			skipped[decision.UnitID] {
			continue
		}
		count++
	}
	return count
}

func countBlockedPickupCandidates(report loot.DecisionReport) int {
	count := 0
	for _, decision := range report.Decisions {
		if decision.Stage == loot.DecisionStageFail && decision.Kind == loot.DecisionKindFail {
			count++
		}
	}
	return count
}

func mapTaskLootTarget(target loot.PickupTarget) tasks.LootTarget {
	return tasks.LootTarget{
		UnitID:    target.UnitID,
		TxtFileNo: target.TxtFileNo,
		Code:      target.Code,
		Name:      target.Name,
		Position:  target.Position,
		AreaID:    target.AreaID,
	}
}

func mapLootPickupTarget(target tasks.LootTarget) loot.PickupTarget {
	return loot.PickupTarget{
		UnitID:    target.UnitID,
		TxtFileNo: target.TxtFileNo,
		Code:      target.Code,
		Name:      target.Name,
		Position:  target.Position,
		AreaID:    target.AreaID,
	}
}

func mapTaskLootPickupResult(res loot.PickupResult) tasks.LootPickupResult {
	return tasks.LootPickupResult{
		Status:       mapTaskLootPickupStatus(res.Status),
		Done:         res.Done,
		Target:       mapTaskLootTarget(res.Target),
		Retry:        res.Retry,
		HoverAttempt: res.HoverAttempt,
	}
}

func mapTaskLootPickupStatus(status loot.PickupStatus) tasks.LootPickupStatus {
	switch status {
	case loot.PickupPending:
		return tasks.LootPickupPending
	case loot.PickupPickedUp:
		return tasks.LootPickupPickedUp
	case loot.PickupTargetUnstable:
		return tasks.LootPickupTargetUnstable
	case loot.PickupTargetLost:
		return tasks.LootPickupTargetLost
	case loot.PickupTooFar:
		return tasks.LootPickupTooFar
	case loot.PickupHoverNotFound:
		return tasks.LootPickupHoverNotFound
	case loot.PickupProjectionFailed:
		return tasks.LootPickupProjectionFailed
	case loot.PickupFailed:
		return tasks.LootPickupFailed
	case loot.PickupMonsterNearby:
		return tasks.LootPickupMonsterNearby
	case loot.PickupInvalidWorld:
		return tasks.LootPickupInvalidWorld
	case loot.PickupInputBlocked:
		return tasks.LootPickupInputBlocked
	default:
		return tasks.LootPickupFailed
	}
}

func isSkippedPickupStatus(status tasks.LootPickupStatus) bool {
	switch status {
	case tasks.LootPickupMonsterNearby, tasks.LootPickupHoverNotFound,
		tasks.LootPickupTargetLost, tasks.LootPickupTargetUnstable,
		tasks.LootPickupTooFar, tasks.LootPickupFailed:
		return true
	default:
		return false
	}
}
