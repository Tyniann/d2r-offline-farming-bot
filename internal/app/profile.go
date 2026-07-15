package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type profileActionsAdapter struct {
	input     inputController
	bindings  configBindingSource
	projector pathing.RelativeProjector
}

type profileTelemetryAdapter struct {
	recorder *telemetry.Recorder
}

func (a *profileTelemetryAdapter) setTelemetry(recorder *telemetry.Recorder) { a.recorder = recorder }

func (a *profileTelemetryAdapter) EmitProfile(event profile.Event) error {
	if a == nil || a.recorder == nil {
		return nil
	}
	eventName := map[profile.EventName]telemetry.EventName{
		profile.EventHookAction:      telemetry.ProfileHookAction,
		profile.EventPotionRequested: telemetry.ResourcePotionRequested,
		profile.EventPotionConfirmed: telemetry.ResourceConsumptionConfirmed,
		profile.EventActionFailed:    telemetry.ProfileActionFailed,
	}[event.Name]
	if eventName == "" {
		return fmt.Errorf("unsupported profile telemetry event %q", event.Name)
	}
	unitID := event.TargetUnitID
	if event.PotionUnitID != 0 {
		unitID = event.PotionUnitID
	}
	skill := ""
	if event.SkillID != 0 {
		skill = memory.SkillName(event.SkillID)
	}
	return a.recorder.Emit(telemetry.Event{Event: eventName, UnitID: unitID, Reason: event.Reason,
		Profile: event.Profile, Hook: string(event.Hook), Skill: skill, SkillID: event.SkillID,
		Target: string(event.Target), Resource: string(event.Resource), ThresholdPercent: event.ThresholdPercent,
		BeltSlot: event.BeltSlot, Confirmed: event.Confirmed})
}

func (a *profileActionsAdapter) CastSkillAtWorld(_ time.Time, skillID uint16, playerPos, targetPos world.Position) error {
	win, ok := a.input.Window()
	if !ok {
		return fmt.Errorf("profile skill: window not bound")
	}
	x, y := win.ClientWidth/2, win.ClientHeight/2
	if targetPos != playerPos {
		var projected bool
		x, y, projected = a.projector.Project(playerPos, targetPos, win)
		if !projected {
			return fmt.Errorf("profile skill projection failed")
		}
	}
	if err := a.input.CastSkillAt(a.bindings, skillID, x, y); err != nil {
		return fmt.Errorf("profile cast %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	return nil
}

func (a *profileActionsAdapter) CastBelt(slot int) error {
	if err := a.input.CastBelt(a.bindings, slot); err != nil {
		return fmt.Errorf("profile belt slot %d: %w", slot, err)
	}
	return nil
}

func newProfileExecutor(log *slog.Logger, profiles config.ProfilesConfig, run config.RunConfig, in inputController, bindings configBindingSource, pathCfg pathing.Config, trace *profileTelemetryAdapter) (*profile.Executor, error) {
	id := run.Combat.Profile
	definition, err := mapProfileDefinition(id, profiles[id])
	if err != nil {
		return nil, err
	}
	actions := &profileActionsAdapter{input: in, bindings: bindings, projector: pathCfg.Projector()}
	executor, err := profile.NewExecutor(log, definition, actions)
	if err == nil {
		executor.SetTelemetry(trace)
	}
	return executor, err
}

func mapProfileDefinition(id string, cfg config.ProfileConfig) (profile.Definition, error) {
	class, ok := mapProfileClass(cfg.CharacterClass)
	if !ok {
		return profile.Definition{}, fmt.Errorf("profile %q character class %q unsupported", id, cfg.CharacterClass)
	}
	hooks := map[profile.Hook][]profile.Action{}
	for hook, actions := range map[profile.Hook][]config.ProfileActionConfig{profile.HookTownReady: cfg.Hooks.TownReady, profile.HookBossEngage: cfg.Hooks.BossEngage} {
		for _, actionCfg := range actions {
			skillID, err := memory.ParseSkillTestName(actionCfg.Skill)
			if err != nil {
				return profile.Definition{}, fmt.Errorf("profile %q hook %s: %w", id, hook, err)
			}
			hooks[hook] = append(hooks[hook], profile.Action{SkillID: skillID, Target: profile.TargetKind(actionCfg.Target), OncePerGame: actionCfg.OncePerGame, OncePerEncounter: actionCfg.OncePerEncounter, Delay: time.Duration(actionCfg.DelayMs) * time.Millisecond, Settle: time.Duration(actionCfg.SettleMs) * time.Millisecond})
		}
	}
	resources := cfg.Resources
	return profile.Definition{ID: id, CharacterClass: class, Hooks: hooks, Resources: profile.ResourcePolicy{
		Healing: mapResourceRule(resources.Healing), Mana: mapResourceRule(resources.Mana), Rejuvenation: mapResourceRule(resources.Rejuvenation),
		Throttle: time.Duration(resources.ThrottleMs) * time.Millisecond, VerifyTimeout: time.Duration(resources.VerifyMs) * time.Millisecond,
	}}, nil
}

func mapResourceRule(cfg config.ResourceRuleConfig) profile.ResourceRule {
	return profile.ResourceRule{UseBelowPercent: uint8(cfg.UseBelowPercent), BeltSlots: append([]int(nil), cfg.BeltSlots...), Cooldown: time.Duration(cfg.CooldownMs) * time.Millisecond}
}

func mapProfileClass(value string) (world.CharacterClass, bool) {
	switch value {
	case "amazon":
		return world.CharacterClassAmazon, true
	case "necromancer":
		return world.CharacterClassNecromancer, true
	case "paladin":
		return world.CharacterClassPaladin, true
	case "sorceress":
		return world.CharacterClassSorceress, true
	case "barbarian":
		return world.CharacterClassBarbarian, true
	case "druid":
		return world.CharacterClassDruid, true
	case "assassin":
		return world.CharacterClassAssassin, true
	default:
		return 0, false
	}
}
