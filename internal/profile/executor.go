package profile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Actions is the narrow authorized-input surface consumed by [Executor].
type Actions interface {
	CastSkillAtWorld(time.Time, uint16, world.Position, world.Position) error
	CastBelt(int) error
}

type pendingResource struct {
	kind      ResourceKind
	slot      int
	unitID    uint32
	startedAt time.Time
}

// Executor evaluates one resolved profile with finite hook and resource state.
type Executor struct {
	log         *slog.Logger
	definition  Definition
	actions     Actions
	telemetry   Telemetry
	hookIndex   map[Hook]int
	hookReadyAt map[Hook]time.Time
	hookAction  map[Hook]int
	onceGame    map[string]bool
	onceBoss    map[string]bool
	waitUntil   time.Time
	pending     *pendingResource
	lastPotion  time.Time
	lastByKind  map[ResourceKind]time.Time
	unavailable map[ResourceKind]bool
}

// NewExecutor creates a resettable executor for one validated definition.
func NewExecutor(log *slog.Logger, definition Definition, actions Actions) (*Executor, error) {
	if log == nil || definition.ID == "" || actions == nil {
		return nil, fmt.Errorf("profile executor requires logger, definition, class, and actions")
	}
	e := &Executor{log: log.With("component", "profile", "profile", definition.ID), definition: definition, actions: actions}
	e.Reset()
	return e, nil
}

// SetTelemetry replaces the run-scoped telemetry sink; nil disables emission.
func (e *Executor) SetTelemetry(sink Telemetry) {
	if e != nil {
		e.telemetry = sink
	}
}

// Ready reports whether the executor has a definition and authorized action surface.
func (e *Executor) Ready() bool { return e != nil && e.definition.ID != "" && e.actions != nil }

// Reset clears game-, encounter-, throttle-, and verification-scoped state.
func (e *Executor) Reset() {
	e.hookIndex = map[Hook]int{}
	e.hookReadyAt = map[Hook]time.Time{}
	e.hookAction = map[Hook]int{}
	e.onceGame = map[string]bool{}
	e.onceBoss = map[string]bool{}
	e.waitUntil = time.Time{}
	e.pending = nil
	e.lastPotion = time.Time{}
	e.lastByKind = map[ResourceKind]time.Time{}
	e.unavailable = map[ResourceKind]bool{}
}

// TickHook advances an ordered semantic hook by at most one input action.
func (e *Executor) TickHook(ctx context.Context, hook Hook, state world.State, target EncounterTarget, now time.Time) Result {
	if ctx.Err() != nil {
		return Result{Status: StatusFailed, Hook: hook, Reason: "profile_cancelled"}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Identity.Valid {
		return Result{Status: StatusPending, Hook: hook}
	}
	if state.Identity.Class != e.definition.CharacterClass {
		return Result{Status: StatusFailed, Hook: hook, Reason: "profile_class_mismatch"}
	}
	if now.IsZero() {
		now = state.At
	}
	if target.UnitID != 0 {
		if previous, seen := e.hookAction[hook]; !seen {
			e.hookAction[hook] = target.ActionIndex
		} else if previous != target.ActionIndex {
			// A repeated semantic hook is a new definition-owned action only
			// when its stable index advances. Poll retries retain the index and
			// therefore cannot duplicate input.
			e.hookAction[hook] = target.ActionIndex
			e.hookIndex[hook] = 0
			delete(e.hookReadyAt, hook)
		}
	}
	if !e.waitUntil.IsZero() && now.Before(e.waitUntil) {
		return Result{Status: StatusPending, Hook: hook}
	}
	e.waitUntil = time.Time{}
	actions, ok := e.definition.Hooks[hook]
	if !ok {
		return Result{Status: StatusComplete, Hook: hook}
	}
	for e.hookIndex[hook] < len(actions) {
		index := e.hookIndex[hook]
		action := actions[index]
		if action.Delay > 0 {
			readyAt, started := e.hookReadyAt[hook]
			if !started {
				e.hookReadyAt[hook] = now.Add(action.Delay)
				return Result{Status: StatusPending, Hook: hook}
			}
			if now.Before(readyAt) {
				return Result{Status: StatusPending, Hook: hook}
			}
		}
		gameKey := fmt.Sprintf("%s:%d", hook, index)
		bossKey := fmt.Sprintf("%s:%d:%d:%d", hook, index, target.UnitID, target.ActionIndex)
		if action.OncePerGame && e.onceGame[gameKey] || action.OncePerEncounter && target.UnitID != 0 && e.onceBoss[bossKey] {
			e.hookIndex[hook]++
			continue
		}
		targetPos := state.Player.Position
		if action.Target == TargetBoss {
			if target.UnitID == 0 || target.Position.X == 0 || target.Position.Y == 0 {
				return Result{Status: StatusFailed, Hook: hook, Reason: "profile_target_missing"}
			}
			targetPos = target.Position
		}
		if err := e.actions.CastSkillAtWorld(now, action.SkillID, state.Player.Position, targetPos); err != nil {
			e.emitFailure(Event{Hook: hook, SkillID: action.SkillID, Target: action.Target, TargetUnitID: target.UnitID}, "profile_skill_failed")
			return Result{Status: StatusFailed, Hook: hook, SkillID: action.SkillID, Reason: "profile_skill_failed"}
		}
		if err := e.emit(Event{Name: EventHookAction, Hook: hook, SkillID: action.SkillID, Target: action.Target, TargetUnitID: target.UnitID}); err != nil {
			return Result{Status: StatusFailed, Hook: hook, SkillID: action.SkillID, Reason: "profile_telemetry_failed"}
		}
		if action.OncePerGame {
			e.onceGame[gameKey] = true
		}
		if action.OncePerEncounter {
			e.onceBoss[bossKey] = true
		}
		e.hookIndex[hook]++
		delete(e.hookReadyAt, hook)
		settleStartedAt := time.Now()
		if now.After(settleStartedAt) {
			settleStartedAt = now
		}
		e.waitUntil = settleStartedAt.Add(action.Settle)
		e.log.Info("profile hook action", "hook", hook, "skill_id", action.SkillID, "target", action.Target, "target_unit_id", target.UnitID)
		return Result{Status: StatusAction, Hook: hook, SkillID: action.SkillID}
	}
	return Result{Status: StatusComplete, Hook: hook}
}

// TickResources evaluates prioritized potion policy and sends at most one belt input.
func (e *Executor) TickResources(state world.State, now time.Time) Result {
	if !resourceWorldReady(state) {
		return Result{Status: StatusComplete}
	}
	if state.Identity.Valid && state.Identity.Class != e.definition.CharacterClass {
		return Result{Status: StatusFailed, Reason: "profile_class_mismatch"}
	}
	if now.IsZero() {
		now = state.At
	}
	if e.pending != nil {
		if !beltUnitPresent(state, e.pending.unitID) {
			confirmed := e.pending
			if err := e.emit(Event{Name: EventPotionConfirmed, Resource: confirmed.kind, ThresholdPercent: e.resourceRule(confirmed.kind).UseBelowPercent, BeltSlot: confirmed.slot, PotionUnitID: confirmed.unitID, Confirmed: true}); err != nil {
				e.pending = nil
				return Result{Status: StatusFailed, Resource: confirmed.kind, BeltSlot: confirmed.slot, Reason: "profile_telemetry_failed"}
			}
			e.log.Info("resource consumption confirmed", "resource", confirmed.kind, "belt_slot", confirmed.slot, "unit_id", confirmed.unitID)
			e.pending = nil
			return Result{Status: StatusComplete, Resource: confirmed.kind, BeltSlot: confirmed.slot}
		}
		if now.Sub(e.pending.startedAt) >= e.definition.Resources.VerifyTimeout {
			failed := e.pending
			e.pending = nil
			e.emitFailure(Event{Resource: failed.kind, ThresholdPercent: e.resourceRule(failed.kind).UseBelowPercent, BeltSlot: failed.slot}, "potion_verify_timeout")
			return Result{Status: StatusFailed, Resource: failed.kind, BeltSlot: failed.slot, Reason: "potion_verify_timeout"}
		}
		return Result{Status: StatusPending, Resource: e.pending.kind, BeltSlot: e.pending.slot}
	}
	if !e.lastPotion.IsZero() && now.Sub(e.lastPotion) < e.definition.Resources.Throttle {
		return Result{Status: StatusComplete}
	}
	kind, rule, needed := e.selectResource(state)
	if !needed {
		return Result{Status: StatusComplete}
	}
	if last := e.lastByKind[kind]; !last.IsZero() && now.Sub(last) < rule.Cooldown {
		return Result{Status: StatusComplete, Resource: kind, Reason: string(kind) + "_potion_cooldown"}
	}
	slot, unitID, ok := selectBeltItem(state, kind, rule.BeltSlots)
	if !ok {
		if !e.unavailable[kind] {
			e.unavailable[kind] = true
			e.log.Warn("resource unavailable", "resource", kind, "reason", string(kind)+"_potion_unavailable")
		}
		return Result{Status: StatusComplete, Resource: kind, Reason: string(kind) + "_potion_unavailable"}
	}
	if err := e.actions.CastBelt(slot); err != nil {
		e.emitFailure(Event{Resource: kind, ThresholdPercent: rule.UseBelowPercent, BeltSlot: slot}, "potion_input_failed")
		return Result{Status: StatusFailed, Resource: kind, BeltSlot: slot, Reason: "potion_input_failed"}
	}
	if err := e.emit(Event{Name: EventPotionRequested, Resource: kind, ThresholdPercent: rule.UseBelowPercent, BeltSlot: slot, PotionUnitID: unitID}); err != nil {
		return Result{Status: StatusFailed, Resource: kind, BeltSlot: slot, Reason: "profile_telemetry_failed"}
	}
	e.unavailable[kind] = false
	e.lastPotion = now
	e.lastByKind[kind] = now
	e.pending = &pendingResource{kind: kind, slot: slot, unitID: unitID, startedAt: now}
	e.log.Info("resource potion requested", "resource", kind, "belt_slot", slot, "unit_id", unitID)
	return Result{Status: StatusAction, Resource: kind, BeltSlot: slot}
}

func (e *Executor) resourceRule(kind ResourceKind) ResourceRule {
	switch kind {
	case ResourceHealing:
		return e.definition.Resources.Healing
	case ResourceMana:
		return e.definition.Resources.Mana
	case ResourceRejuvenation:
		return e.definition.Resources.Rejuvenation
	default:
		return ResourceRule{}
	}
}

func (e *Executor) emit(event Event) error {
	if e.telemetry == nil {
		return nil
	}
	event.Profile = e.definition.ID
	return e.telemetry.EmitProfile(event)
}

func (e *Executor) emitFailure(event Event, reason string) {
	event.Name = EventActionFailed
	event.Reason = reason
	if err := e.emit(event); err != nil {
		e.log.Error("profile failure telemetry failed", "reason", reason, "error", err)
	}
}

func (e *Executor) selectResource(state world.State) (ResourceKind, ResourceRule, bool) {
	policy := e.definition.Resources
	if state.Player.MaxHP > 0 && state.Player.HPPercent() <= policy.Rejuvenation.UseBelowPercent {
		return ResourceRejuvenation, policy.Rejuvenation, true
	}
	if state.Player.MaxHP > 0 && state.Player.HPPercent() <= policy.Healing.UseBelowPercent {
		return ResourceHealing, policy.Healing, true
	}
	if state.Player.MaxMana > 0 && state.Player.ManaPercent() <= policy.Mana.UseBelowPercent {
		return ResourceMana, policy.Mana, true
	}
	return "", ResourceRule{}, false
}

func resourceWorldReady(state world.State) bool {
	return state.Valid && state.Phase == world.GamePhaseInGame && !state.UI.InventoryOpen && !state.UI.StashOpen && !state.UI.QuitMenuOpen
}

func selectBeltItem(state world.State, kind ResourceKind, slots []int) (int, uint32, bool) {
	wanted := map[ResourceKind]string{ResourceHealing: "hpot", ResourceMana: "mpot", ResourceRejuvenation: "rpot"}[kind]
	for _, slot := range slots {
		var consumable *world.Item
		for _, item := range state.Items {
			if item.Location != world.ItemLocationBelt || !item.PlayerOwned || item.GridX != slot-1 {
				continue
			}
			candidate := item
			if consumable == nil || candidate.GridY < consumable.GridY {
				consumable = &candidate
			}
		}
		if consumable != nil && consumable.Type == wanted {
			return slot, consumable.UnitID, true
		}
	}
	return 0, 0, false
}

func beltUnitPresent(state world.State, unitID uint32) bool {
	for _, item := range state.Items {
		if item.UnitID == unitID && item.Location == world.ItemLocationBelt {
			return true
		}
	}
	return false
}
