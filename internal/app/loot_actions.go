package app

import (
	"fmt"
	"log/slog"
	"strings"
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
	profile      config.ProfileResourcesConfig
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

func newLootActionsAdapter(log *slog.Logger, filter *loot.Filter, profile config.ProfileResourcesConfig, pickupCfg config.LootPickupConfig, driver pathing.InputDriver, pathingCfg pathing.Config, stash *loot.StashExecutor, runTelemetry telemetryEmitter) *lootActionsAdapter {
	clicker := pathing.NewEntityClicker(log, driver, pathingCfg.Projector(), pathingCfg.Click)
	return &lootActionsAdapter{
		log:       log.With("component", "loot_actions"),
		filter:    filter,
		profile:   profile,
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
		event := telemetry.Event{Timestamp: now, Event: telemetry.StashAttempt, Stage: telemetry.HistoryStageReturnTown, AreaID: uint32(state.Area.ID), UnitID: res.UnitID, Code: res.Code, Name: res.Name, Attempt: res.Attempt, GridX: intPointer(res.GridX), GridY: intPointer(res.GridY)}
		applyItemTelemetry(&event, res.Code, res.Name, res.Quality, res.IdentityKind, res.IdentityKey, res.IdentityValid)
		applyPickitTelemetry(&event, res.Pickit)
		if err := a.emit(event); err != nil {
			return tasks.LootStashResult{Status: tasks.LootStashTelemetryFailed, Done: true}
		}
	}
	if res.Transferred {
		event := telemetry.Event{Timestamp: now, Event: telemetry.StashSuccess, Stage: telemetry.HistoryStageReturnTown, AreaID: uint32(state.Area.ID), UnitID: res.UnitID, Code: res.Code, Name: res.Name, Attempt: res.Attempt, GridX: intPointer(res.GridX), GridY: intPointer(res.GridY)}
		applyItemTelemetry(&event, res.Code, res.Name, res.Quality, res.IdentityKind, res.IdentityKey, res.IdentityValid)
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
	return a.scan(state, false, 0)
}

// ScanRouteKeep applies the run's immutable user-selected Pickit chain and
// exposes nearby `keep` targets to combat-route orchestration. Only when no
// keep target exists does it offer an exact profile-belt supply potion.
func (a *lootActionsAdapter) ScanRouteKeep(state world.State, maxDistanceTiles float64) tasks.LootScanResult {
	return a.scan(state, true, maxDistanceTiles)
}

func (a *lootActionsAdapter) scan(state world.State, routeKeepOnly bool, maxDistanceTiles float64) tasks.LootScanResult {
	if a == nil || a.filter == nil {
		return tasks.LootScanResult{}
	}
	report := a.filter.Decide(state)
	if a.telemetryErr != nil {
		return tasks.LootScanResult{TelemetryFailed: true}
	}
	matchedPolicies := make(map[uint32]loot.PickitResult)
	for _, decision := range report.Decisions {
		if decision.Stage == loot.DecisionStageClassify && decision.Kind == loot.DecisionKindClassifyMatch {
			matchedPolicies[decision.UnitID] = decision.Pickit
		}
	}
	for _, item := range state.GroundItems() {
		event := telemetry.Event{Timestamp: state.At, Event: telemetry.DropSeen, Stage: telemetry.HistoryStageLoot, AreaID: uint32(state.Area.ID), UnitID: item.UnitID, TxtFileNo: item.TxtFileNo, Code: item.Code, Name: item.Name}
		applyItemTelemetry(&event, item.Code, item.Name, item.Quality, item.IdentityKind, item.IdentityKey, item.IdentityValid)
		applyPickitTelemetry(&event, matchedPolicies[item.UnitID])
		if err := a.emit(event); err != nil {
			return tasks.LootScanResult{TelemetryFailed: true}
		}
	}
	for _, decision := range report.Decisions {
		if decision.Stage != loot.DecisionStageClassify || decision.Kind != loot.DecisionKindClassifyMatch {
			continue
		}
		event := telemetry.Event{Timestamp: state.At, Event: telemetry.PickitMatch, Stage: telemetry.HistoryStageLoot, AreaID: uint32(state.Area.ID), UnitID: decision.UnitID, TxtFileNo: decision.TxtFileNo, Code: decision.Code, Name: decision.Name}
		applyItemTelemetry(&event, decision.Code, decision.Name, decision.Quality, decision.IdentityKind, decision.IdentityKey, decision.IdentityValid)
		applyPickitTelemetry(&event, decision.Pickit)
		a.log.Info("pickit decision", "unit_id", decision.UnitID, "profile_id", decision.Pickit.ProfileID, "rule_id", decision.Pickit.RuleID, "action", decision.Pickit.Action, "profile_revision", decision.Pickit.ProfileRevision, "assignment_revision", decision.Pickit.AssignmentRevision)
		if err := a.emit(event); err != nil {
			return tasks.LootScanResult{TelemetryFailed: true}
		}
	}
	var target loot.PickupTarget
	var found bool
	var supplyCandidateCount int
	if routeKeepOnly {
		target, found = loot.SelectKeepPickupCandidateExcludingWithin(state, report, a.skipped, maxDistanceTiles)
		if !found {
			target, found, supplyCandidateCount = selectRouteSupplyTarget(state, a.profile, a.skipped, maxDistanceTiles)
			if found {
				a.log.Info("route supply pickup selected", "unit_id", target.UnitID, "code", target.Code, "candidate_count", supplyCandidateCount)
			}
		}
	} else {
		target, found = loot.SelectPickupCandidateExcluding(state, report, a.skipped)
	}
	result := tasks.LootScanResult{
		GroundItemCount:             report.GroundItemCount,
		CandidateCount:              countPickupCandidatesForMode(state, report, a.skipped, routeKeepOnly, maxDistanceTiles) + supplyCandidateCount,
		InventoryFullCandidateCount: countInventoryFullCandidatesForMode(state, report, routeKeepOnly, maxDistanceTiles),
		HasTarget:                   found,
	}
	result.InventoryFull = result.InventoryFullCandidateCount > 0
	if found {
		result.NextTarget = mapTaskLootTarget(target)
	}
	a.log.Info("run loot scan",
		"route_keep_only", routeKeepOnly,
		"maximum_distance_tiles", maxDistanceTiles,
		"ground_item_count", result.GroundItemCount,
		"candidate_count", result.CandidateCount,
		"route_supply_candidate_count", supplyCandidateCount,
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

func selectRouteSupplyTarget(state world.State, profile config.ProfileResourcesConfig, skipped map[uint32]bool, maxDistanceTiles float64) (loot.PickupTarget, bool, int) {
	healing, mana, rejuvenation := countProfilePotionSupplies(state, profile)
	needed := map[string]bool{
		"hp5": healing < len(slotSet(profile.Healing.BeltSlots))*4,
		"mp5": mana < len(slotSet(profile.Mana.BeltSlots))*4,
		"rvl": rejuvenation < len(slotSet(profile.Rejuvenation.BeltSlots))*4,
	}

	var best loot.PickupTarget
	bestDistance := 0.0
	found, candidates := false, 0
	for _, item := range state.GroundItems() {
		if !needed[item.Code] || skipped[item.UnitID] || !isExactRouteSupplyPotion(item) {
			continue
		}
		distance := world.Distance(state.Player.Position, item.Position)
		if maxDistanceTiles > 0 && distance > maxDistanceTiles {
			continue
		}
		candidates++
		if found && (distance > bestDistance || distance == bestDistance && item.UnitID >= best.UnitID) {
			continue
		}
		found, bestDistance = true, distance
		best = loot.PickupTarget{
			UnitID: item.UnitID, TxtFileNo: item.TxtFileNo, Code: item.Code, Name: item.Name,
			Quality: item.Quality, IdentityKind: item.IdentityKind, IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid,
			Position: item.Position, AreaID: state.Area.ID,
		}
	}
	return best, found, candidates
}

func isExactRouteSupplyPotion(item world.Item) bool {
	switch item.Code {
	case "hp5":
		return item.Type == "hpot"
	case "mp5":
		return item.Type == "mpot"
	case "rvl":
		return item.Type == "rpot"
	default:
		return false
	}
}

func applyPickitTelemetry(event *telemetry.Event, result loot.PickitResult) {
	if event == nil {
		return
	}
	event.PickitProfileID, event.PickitRuleID, event.PickitAction = result.ProfileID, result.RuleID, string(result.Action)
	event.PickitProfileRevision, event.PickitAssignmentRevision = result.ProfileRevision, result.AssignmentRevision
}

func (a *lootActionsAdapter) StartPickup(target tasks.LootTarget) error {
	return a.startPickup(target, false)
}

// StartCowLegPickup starts only a bound Wirt's Leg target with the narrow
// monster-tolerant quest pickup executor.
func (a *lootActionsAdapter) StartCowLegPickup(target tasks.LootTarget) error {
	return a.startPickup(target, true)
}

func (a *lootActionsAdapter) startPickup(target tasks.LootTarget, cowLeg bool) error {
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
	mapped := mapLootPickupTarget(target)
	if cowLeg {
		executor, err := loot.NewWirtsLegPickupExecutor(a.log, a.cfg, a.clicker, mapped)
		if err != nil {
			return err
		}
		a.active = executor
	} else {
		a.active = loot.NewPickupExecutor(a.log, a.cfg, a.clicker, mapped)
	}
	return nil
}

// ClearSkippedPickup removes unitID from the in-step skip set so one recovery retry may StartPickup again.
func (a *lootActionsAdapter) ClearSkippedPickup(unitID uint32) {
	if a == nil || unitID == 0 || a.skipped == nil {
		return
	}
	delete(a.skipped, unitID)
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
		event := telemetry.Event{Timestamp: now, Event: telemetry.PickupAttempt, Stage: telemetry.HistoryStageLoot, AreaID: uint32(state.Area.ID), UnitID: res.Target.UnitID, TxtFileNo: res.Target.TxtFileNo, Code: res.Target.Code, Name: res.Target.Name, Attempt: res.Retry, HoverAttempt: res.HoverAttempt}
		applyItemTelemetry(&event, res.Target.Code, res.Target.Name, res.Target.Quality, res.Target.IdentityKind, res.Target.IdentityKey, res.Target.IdentityValid)
		applyPickitTelemetry(&event, res.Target.Pickit)
		if err := a.emit(event); err != nil {
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
		record := telemetry.Event{Timestamp: now, Event: event, Stage: telemetry.HistoryStageLoot, AreaID: uint32(state.Area.ID), UnitID: res.Target.UnitID, TxtFileNo: res.Target.TxtFileNo, Code: res.Target.Code, Name: res.Target.Name, Reason: reason, Attempt: res.Retry, HoverAttempt: res.HoverAttempt}
		applyItemTelemetry(&record, res.Target.Code, res.Target.Name, res.Target.Quality, res.Target.IdentityKind, res.Target.IdentityKey, res.Target.IdentityValid)
		applyPickitTelemetry(&record, res.Target.Pickit)
		if err := a.emit(record); err != nil {
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

func applyItemTelemetry(event *telemetry.Event, code, name string, quality world.ItemQuality, identityKind world.ItemIdentityKind, identityKey string, identityValid bool) {
	if event == nil {
		return
	}
	event.ItemName, event.BaseCode, event.Quality = name, code, quality.String()
	event.ItemIdentityKind = string(identityKind)
	if identityValid {
		event.ItemIdentityKey = identityKey
	}
	event.ItemKey = itemTelemetryKey(code, quality, identityKind, identityKey, identityValid)
}

func itemTelemetryKey(code string, quality world.ItemQuality, identityKind world.ItemIdentityKind, identityKey string, identityValid bool) string {
	if identityValid && (identityKind == world.ItemIdentitySet || identityKind == world.ItemIdentityUnique) && strings.TrimSpace(identityKey) != "" {
		return fmt.Sprintf("%s:%s", identityKind, identityKey)
	}
	if strings.TrimSpace(code) != "" && quality != world.ItemQualityUnknown {
		return fmt.Sprintf("base:%s:%s", code, quality.String())
	}
	return ""
}

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

func countPickupCandidatesForMode(state world.State, report loot.DecisionReport, skipped map[uint32]bool, keepOnly bool, maxDistanceTiles float64) int {
	count := 0
	for _, decision := range report.Decisions {
		if decision.Stage != loot.DecisionStagePickCandidate ||
			decision.Kind != loot.DecisionKindPickCandidate ||
			!decision.CanFit ||
			skipped[decision.UnitID] {
			continue
		}
		if keepOnly && (decision.Pickit.Action != loot.ActionKeep || !lootDecisionWithin(state, decision.UnitID, maxDistanceTiles)) {
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
	return countInventoryFullCandidatesForMode(world.State{}, report, false, 0)
}

func countInventoryFullCandidatesForMode(state world.State, report loot.DecisionReport, keepOnly bool, maxDistanceTiles float64) int {
	count := 0
	for _, decision := range report.Decisions {
		if decision.Stage == loot.DecisionStageFail &&
			decision.Kind == loot.DecisionKindFail &&
			decision.Reason == loot.DecisionReasonInventoryFull {
			if keepOnly && (decision.Pickit.Action != loot.ActionKeep || !lootDecisionWithin(state, decision.UnitID, maxDistanceTiles)) {
				continue
			}
			count++
		}
	}
	return count
}

func lootDecisionWithin(state world.State, unitID uint32, maxDistanceTiles float64) bool {
	if maxDistanceTiles <= 0 {
		return true
	}
	for _, item := range state.GroundItems() {
		if item.UnitID == unitID {
			return world.Distance(state.Player.Position, item.Position) <= maxDistanceTiles
		}
	}
	return false
}

func mapTaskLootTarget(target loot.PickupTarget) tasks.LootTarget {
	return tasks.LootTarget{
		UnitID:    target.UnitID,
		TxtFileNo: target.TxtFileNo,
		Code:      target.Code,
		Name:      target.Name,
		Quality:   target.Quality, IdentityKind: target.IdentityKind, IdentityKey: target.IdentityKey, IdentityValid: target.IdentityValid,
		PickitProfileID: target.Pickit.ProfileID, PickitRuleID: target.Pickit.RuleID, PickitAction: string(target.Pickit.Action), PickitProfileRevision: target.Pickit.ProfileRevision, PickitAssignmentRevision: target.Pickit.AssignmentRevision,
		Position: target.Position,
		AreaID:   target.AreaID,
	}
}

func mapLootPickupTarget(target tasks.LootTarget) loot.PickupTarget {
	return loot.PickupTarget{
		UnitID:    target.UnitID,
		TxtFileNo: target.TxtFileNo,
		Code:      target.Code,
		Name:      target.Name,
		Quality:   target.Quality, IdentityKind: target.IdentityKind, IdentityKey: target.IdentityKey, IdentityValid: target.IdentityValid,
		Pickit:   loot.PickitResult{ProfileID: target.PickitProfileID, RuleID: target.PickitRuleID, Action: loot.Action(target.PickitAction), ProfileRevision: target.PickitProfileRevision, AssignmentRevision: target.PickitAssignmentRevision},
		Position: target.Position,
		AreaID:   target.AreaID,
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
