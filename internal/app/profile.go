package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
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
	combat    *combatAdapter
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
		Target: string(event.Target), Resource: string(event.Resource), Recipient: event.Recipient,
		ThresholdPercent: event.ThresholdPercent, HPPercent: event.HPPercent, BeltSlot: event.BeltSlot,
		Confirmed: event.Confirmed, MercUnitID: event.MercUnitID})
}

func (a *profileActionsAdapter) CastSkillAtWorld(_ time.Time, skillID uint16, player world.Player, targetPos world.Position) error {
	if err := a.input.Focus(); err != nil {
		return fmt.Errorf("profile skill focus: %w", err)
	}
	win, ok := a.input.Window()
	if !ok {
		return fmt.Errorf("profile skill: window not bound")
	}
	x, y := win.ClientWidth/2, win.ClientHeight/2
	if targetPos != player.Position {
		var projected bool
		x, y, projected = a.projector.Project(player.Position, targetPos, win)
		if !projected || x < 0 || x >= win.ClientWidth || y < 0 || y >= win.ClientHeight {
			if skillID == memory.SkillCorpseExplosion {
				return profile.ErrCorpseExplosionTargetUnprojectable
			}
			return fmt.Errorf("profile skill projection failed")
		}
	}
	if player.RightSkillID == skillID {
		if err := a.input.MoveTo(x, y); err != nil {
			return fmt.Errorf("profile aim selected skill %s(%d): %w", memory.SkillName(skillID), skillID, err)
		}
		if err := a.input.Click(input.MouseRight); err != nil {
			return fmt.Errorf("profile cast selected skill %s(%d): %w", memory.SkillName(skillID), skillID, err)
		}
		return a.clearCombatSelection()
	}
	if err := a.input.CastSkillAt(a.bindings, skillID, x, y); err != nil {
		return fmt.Errorf("profile cast %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	return a.clearCombatSelection()
}

// clearCombatSelection invalidates pending monster aim/skill confirmation
// after an independently authorized profile cast changed the right skill.
func (a *profileActionsAdapter) clearCombatSelection() error {
	if a.combat == nil {
		return nil
	}
	if err := a.combat.StopAttack(); err != nil {
		return fmt.Errorf("profile clear combat selection: %w", err)
	}
	return nil
}

func (a *profileActionsAdapter) CastBelt(slot int) error {
	if err := a.input.CastBelt(a.bindings, slot); err != nil {
		return fmt.Errorf("profile belt slot %d: %w", slot, err)
	}
	return nil
}

func (a *profileActionsAdapter) CastBeltForMercenary(slot int) error {
	if err := a.input.CastBeltWithModifier(a.bindings, "shift", slot); err != nil {
		return fmt.Errorf("profile mercenary belt slot %d: %w", slot, err)
	}
	return nil
}

func newProfileExecutor(log *slog.Logger, profiles config.ProfilesConfig, run config.RunConfig, in inputController, bindings configBindingSource, pathCfg pathing.Config, combat *combatAdapter, trace *profileTelemetryAdapter) (*profile.Executor, error) {
	id := run.Combat.Profile
	definition, err := mapProfileDefinition(id, profiles[id])
	if err != nil {
		return nil, err
	}
	actions := &profileActionsAdapter{input: in, bindings: bindings, projector: pathCfg.Projector(), combat: combat}
	executor, err := profile.NewExecutor(log, definition, actions)
	if err != nil {
		return nil, err
	}
	executor.SetTelemetry(trace)
	if id == "necro_bone_spear" {
		if configureErr := executor.ConfigureCorpseExplosion(memory.SkillCorpseExplosion); configureErr != nil {
			return nil, configureErr
		}
	}
	if run.RouteCombat.EnabledValue() || id == "necro_bone_spear" {
		attackSkillID, parseErr := memory.ParseSkillTestName(run.Combat.AttackSkill)
		if parseErr != nil {
			return nil, fmt.Errorf("profile route clear attack skill: %w", parseErr)
		}
		if configureErr := executor.ConfigureRouteClear(
			profile.RouteClearSingleTarget,
			memory.SkillAmplifyDamage,
			attackSkillID,
			combat,
		); configureErr != nil {
			return nil, configureErr
		}
	}
	return executor, nil
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
	mercEnabled, mercRule := resources.Mercenary.Resolve()
	maintenanceCfg := cfg.RouteMaintenance.BoneArmor
	maintenanceSkillID := uint16(0)
	maintenanceEnabled := maintenanceCfg.Enabled != nil && *maintenanceCfg.Enabled
	if maintenanceEnabled {
		var err error
		maintenanceSkillID, err = memory.ParseSkillTestName(maintenanceCfg.Skill)
		if err != nil {
			return profile.Definition{}, fmt.Errorf("profile %q route maintenance: %w", id, err)
		}
	}
	return profile.Definition{ID: id, CharacterClass: class, Hooks: hooks, Resources: profile.ResourcePolicy{
		Healing: mapResourceRule(resources.Healing), Mana: mapResourceRule(resources.Mana), Rejuvenation: mapResourceRule(resources.Rejuvenation),
		Mercenary: profile.MercenaryResourcePolicy{Enabled: mercEnabled, ResourceRule: mapResourceRule(mercRule)},
		Throttle:  time.Duration(resources.ThrottleMs) * time.Millisecond, VerifyTimeout: time.Duration(resources.VerifyMs) * time.Millisecond,
	}, RouteMaintenance: profile.RouteMaintenancePolicy{BoneArmor: profile.BoneArmorMaintenancePolicy{
		Enabled: maintenanceEnabled, SkillID: maintenanceSkillID,
		RefreshInterval:            time.Duration(maintenanceCfg.RefreshIntervalMs) * time.Millisecond,
		RefreshAfterDamageBelowPct: uint8(maintenanceCfg.RefreshAfterDamageBelowPct),
		MinimumRecastInterval:      time.Duration(maintenanceCfg.MinimumRecastIntervalMs) * time.Millisecond,
		Settle:                     time.Duration(maintenanceCfg.SettleMs) * time.Millisecond,
	}}}, nil
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
