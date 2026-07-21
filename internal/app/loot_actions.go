package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type lootActionsAdapter struct {
	log          *slog.Logger
	filter       *loot.Filter
	cfg          loot.PickupConfig
	clicker      loot.PickupClicker
	active       *loot.PickupExecutor
	skipped      map[uint32]bool
	lastStart    tasks.LootTarget
	stash        *loot.StashExecutor
	telemetry    telemetryEmitter
	telemetryErr error
}

func (a *lootActionsAdapter) setTelemetry(trace *telemetry.Recorder) { a.telemetry = trace }

type telemetryEmitter interface {
	Emit(telemetry.Event) error
}

func newLootActionsAdapter(log *slog.Logger, filter *loot.Filter, pickupCfg config.LootPickupConfig, driver pathing.InputDriver, pathingCfg pathing.Config, stash *loot.StashExecutor, runTelemetry telemetryEmitter) *lootActionsAdapter {
	clicker := pathing.NewEntityClicker(log, driver, pathingCfg.Projector(), pathingCfg.Click)
	return &lootActionsAdapter{
		log:       log.With("component", "loot_actions"),
		filter:    filter,
		cfg:       mapLootPickupConfig(pickupCfg),
		clicker:   &pickupClickerAdapter{input: driver, clicker: clicker},
		skipped:   make(map[uint32]bool),
		stash:     stash,
		telemetry: runTelemetry,
	}
}

func (a *lootActionsAdapter) TickStash(state world.State, now time.Time) tasks.LootStashResult {
	if a == nil {
		return tasks.LootStashResult{Status: tasks.LootStashFailed, Done: true}
	}
	if a.telemetryErr != nil {
		return tasks.LootStashResult{Status: tasks.LootStashTelemetryFailed, Done: true}
	}
	if a.stash == nil {
		return tasks.LootStashResult{Status: tasks.LootStashFailed, Done: true}
	}
	res := a.stash.Tick(state, now)
	if res.Attempted {
		event := telemetry.Event{Timestamp: now, Event: telemetry.StashAttempt, AreaID: uint32(state.Area.ID), UnitID: res.UnitID, Code: res.Code, Name: res.Name, Attempt: res.Attempt, GridX: intPointer(res.GridX), GridY: intPointer(res.GridY)}
		applyPickitTelemetry(&event, res.Pickit)
		if err := a.emit(event); err != nil {
			return tasks.LootStashResult{Status: tasks.LootStashTelemetryFailed, Done: true}
		}
	}
	if res.Transferred {
		event := telemetry.Event{Timestamp: now, Event: telemetry.StashSuccess, AreaID: uint32(state.Area.ID), UnitID: res.UnitID, Code: res.Code, Name: res.Name, Attempt: res.Attempt, GridX: intPointer(res.GridX), GridY: intPointer(res.GridY)}
		applyPickitTelemetry(&event, res.Pickit)
		if err := a.emit(event); err != nil {
			return tasks.LootStashResult{Status: tasks.LootStashTelemetryFailed, Done: true}
		}
	}
	if res.Status == loot.StashFull {
		if err := a.emit(telemetry.Event{Timestamp: now, Event: telemetry.StashFull, AreaID: uint32(state.Area.ID), Reason: string(res.Status)}); err != nil {
			return tasks.LootStashResult{Status: tasks.LootStashTelemetryFailed, Done: true}
		}
	}
	return mapTaskLootStashResult(res)
}

func (a *lootActionsAdapter) TickCloseStash(state world.State, now time.Time) tasks.LootStashResult {
	if a == nil {
		return tasks.LootStashResult{Status: tasks.LootStashCloseFailed, Done: true}
	}
	if a.telemetryErr != nil {
		return tasks.LootStashResult{Status: tasks.LootStashTelemetryFailed, Done: true}
	}
	if a.stash == nil {
		return tasks.LootStashResult{Status: tasks.LootStashCloseFailed, Done: true}
	}
	return mapTaskLootStashResult(a.stash.TickClose(state, now))
}

func (a *lootActionsAdapter) Scan(state world.State) tasks.LootScanResult {
	if a == nil || a.filter == nil {
		return tasks.LootScanResult{}
	}
	report := a.filter.Decide(state)
	if a.telemetryErr != nil {
		return tasks.LootScanResult{TelemetryFailed: true}
	}
	for _, item := range state.GroundItems() {
		if err := a.emit(telemetry.Event{Timestamp: state.At, Event: telemetry.DropSeen, AreaID: uint32(state.Area.ID), UnitID: item.UnitID, TxtFileNo: item.TxtFileNo, Code: item.Code, Name: item.Name}); err != nil {
			return tasks.LootScanResult{TelemetryFailed: true}
		}
	}
	for _, decision := range report.Decisions {
		if decision.Stage != loot.DecisionStageClassify || decision.Kind != loot.DecisionKindClassifyMatch {
			continue
		}
		event := telemetry.Event{Timestamp: state.At, Event: telemetry.PickitMatch, AreaID: uint32(state.Area.ID), UnitID: decision.UnitID, TxtFileNo: decision.TxtFileNo, Code: decision.Code, Name: decision.Name}
		applyPickitTelemetry(&event, decision.Pickit)
		a.log.Info("pickit decision", "unit_id", decision.UnitID, "profile_id", decision.Pickit.ProfileID, "rule_id", decision.Pickit.RuleID, "action", decision.Pickit.Action, "profile_revision", decision.Pickit.ProfileRevision, "assignment_revision", decision.Pickit.AssignmentRevision)
		if err := a.emit(event); err != nil {
			return tasks.LootScanResult{TelemetryFailed: true}
		}
	}
	target, found := loot.SelectPickupCandidateExcluding(state, report, a.skipped)
	result := tasks.LootScanResult{
		GroundItemCount:             report.GroundItemCount,
		CandidateCount:              countPickupCandidates(report, a.skipped),
		InventoryFullCandidateCount: countInventoryFullCandidates(report),
		HasTarget:                   found,
	}
	result.InventoryFull = result.InventoryFullCandidateCount > 0
	if found {
		result.NextTarget = mapTaskLootTarget(target)
	}
	a.log.Info("run loot scan",
		"ground_item_count", result.GroundItemCount,
		"candidate_count", result.CandidateCount,
		"blocked_candidate_count", countBlockedPickupCandidates(report),
		"inventory_full_candidate_count", result.InventoryFullCandidateCount,
		"inventory_full", result.InventoryFull,
		"has_target", result.HasTarget,
	)
	if result.InventoryFull {
		if err := a.emit(telemetry.Event{Timestamp: state.At, Event: telemetry.InventoryFull, AreaID: uint32(state.Area.ID), CandidateCount: result.InventoryFullCandidateCount, Reason: "no_unlocked_space"}); err != nil {
			return tasks.LootScanResult{TelemetryFailed: true}
		}
		a.log.Warn("inventory_full",
			"ground_item_count", result.GroundItemCount,
			"inventory_full_candidate_count", result.InventoryFullCandidateCount,
			"recovery", "town",
		)
	}
	return result
}

func applyPickitTelemetry(event *telemetry.Event, result loot.PickitResult) {
	if event == nil {
		return
	}
	event.PickitProfileID, event.PickitRuleID, event.PickitAction = result.ProfileID, result.RuleID, string(result.Action)
	event.PickitProfileRevision, event.PickitAssignmentRevision = result.ProfileRevision, result.AssignmentRevision
}

func (a *lootActionsAdapter) StartPickup(target tasks.LootTarget) error {
	if a == nil {
		return fmt.Errorf("loot actions not wired")
	}
	if a.telemetryErr != nil {
		return fmt.Errorf("telemetry_failed: %w", a.telemetryErr)
	}
	if a.filter == nil || a.clicker == nil {
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
	if a == nil {
		return tasks.LootPickupResult{Status: tasks.LootPickupInvalidWorld, Done: true}
	}
	if a.telemetryErr != nil {
		return tasks.LootPickupResult{Status: tasks.LootPickupTelemetryFailed, Done: true}
	}
	if a.active == nil {
		return tasks.LootPickupResult{Status: tasks.LootPickupInvalidWorld, Done: true}
	}
	res := a.active.Tick(state, now)
	out := mapTaskLootPickupResult(res)
	if res.Attempted {
		if err := a.emit(telemetry.Event{Timestamp: now, Event: telemetry.PickupAttempt, AreaID: uint32(state.Area.ID), UnitID: res.Target.UnitID, TxtFileNo: res.Target.TxtFileNo, Code: res.Target.Code, Name: res.Target.Name, Attempt: res.Retry, HoverAttempt: res.HoverAttempt}); err != nil {
			a.active = nil
			return tasks.LootPickupResult{Status: tasks.LootPickupTelemetryFailed, Done: true, Target: out.Target}
		}
	}
	if res.Done {
		event := telemetry.PickupFailed
		reason := string(res.Status)
		if res.Status == loot.PickupPickedUp {
			event = telemetry.PickupSuccess
			reason = ""
		}
		if err := a.emit(telemetry.Event{Timestamp: now, Event: event, AreaID: uint32(state.Area.ID), UnitID: res.Target.UnitID, TxtFileNo: res.Target.TxtFileNo, Code: res.Target.Code, Name: res.Target.Name, Reason: reason, Attempt: res.Retry, HoverAttempt: res.HoverAttempt}); err != nil {
			a.active = nil
			return tasks.LootPickupResult{Status: tasks.LootPickupTelemetryFailed, Done: true, Target: out.Target}
		}
	}
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
	if a.stash != nil {
		a.stash.Reset()
	}
}

func (a *lootActionsAdapter) emit(event telemetry.Event) error {
	if a == nil || a.telemetry == nil {
		return nil
	}
	if a.telemetryErr != nil {
		return a.telemetryErr
	}
	if err := a.telemetry.Emit(event); err != nil {
		a.telemetryErr = err
		a.log.Error("run telemetry failed", "error", err)
		return err
	}
	return nil
}

func intPointer(value int) *int { return &value }

func mapLootStashConfig(cfg config.LootStashConfig) loot.StashConfig {
	return loot.StashConfig{
		MaxRetries: cfg.MaxRetries, VerifyTimeout: time.Duration(cfg.VerifyTimeoutMs) * time.Millisecond,
		CloseTimeout:  time.Duration(cfg.CloseTimeoutMs) * time.Millisecond,
		InventoryLeft: cfg.InventoryLeft, InventoryTop: cfg.InventoryTop,
		InventoryCellW: cfg.InventoryCellW, InventoryCellH: cfg.InventoryCellH,
	}
}

func mapTaskLootStashResult(res loot.StashResult) tasks.LootStashResult {
	return tasks.LootStashResult{Status: tasks.LootStashStatus(res.Status), Done: res.Done, Attempted: res.Attempted, Transferred: res.Transferred, UnitID: res.UnitID, Code: res.Code, Name: res.Name, Attempt: res.Attempt}
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

func countInventoryFullCandidates(report loot.DecisionReport) int {
	count := 0
	for _, decision := range report.Decisions {
		if decision.Stage == loot.DecisionStageFail &&
			decision.Kind == loot.DecisionKindFail &&
			decision.Reason == loot.DecisionReasonInventoryFull {
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
		Attempted:    res.Attempted,
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
