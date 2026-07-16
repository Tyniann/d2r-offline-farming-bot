package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestValidateRunModePassiveOK(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: false}}
	log := config.NewLogger("error")
	if err := validateRunMode(resolveRunSelection(Options{}, cfg), cfg, Options{}, log); err != nil {
		t.Fatalf("passive mode err = %v", err)
	}
}

func TestValidateRunModeAllowsAct3EgressRecordingOptions(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	opts := Options{Route: "record-egress:act3", RouteName: "Kurast-Docks Portal bis Waypoint", RouteDifficulty: "nightmare"}
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err != nil {
		t.Fatalf("record-egress options error = %v", err)
	}

	opts.RouteDifficulty = ""
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err == nil {
		t.Fatal("expected missing record-egress difficulty error")
	}
}

func TestValidateRunModeOfflineExitTest(t *testing.T) {
	log := config.NewLogger("error")
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	opts := Options{OfflineExitTest: true}
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err != nil {
		t.Fatalf("offline exit test error = %v", err)
	}

	cfg.Input.Enabled = false
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err == nil {
		t.Fatal("expected input.enabled error")
	}

	cfg.Input.Enabled = true
	opts.Run = "countess"
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err == nil {
		t.Fatal("expected mode conflict")
	}
}

func TestValidateRunModeUnknownRun(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("baal", ""), cfg, Options{}, log)
	if err == nil {
		t.Fatal("expected error for unknown run")
	}
	if !errors.Is(err, errUnknownRun) {
		t.Fatalf("err = %v, want errUnknownRun", err)
	}
}

func TestValidateRunModeDisabledInput(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: false}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", ""), cfg, Options{}, log)
	if err == nil {
		t.Fatal("expected error when input disabled")
	}
	if !errors.Is(err, errInputRequiredForRun) {
		t.Fatalf("err = %v, want errInputRequiredForRun", err)
	}
}

func TestValidateRunModeInputTestConflict(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", ""), cfg, Options{InputTest: "belt:1"}, log)
	if err == nil {
		t.Fatal("expected error for --run with --input-test")
	}
	if !errors.Is(err, errRunInputTestConflict) {
		t.Fatalf("err = %v, want errRunInputTestConflict", err)
	}
}

func TestResolveActiveRunCLIPriority(t *testing.T) {
	cfg := &config.Config{Runs: config.RunsConfig{Active: "countess"}}
	if got := resolveActiveRun(Options{Run: "countess"}, cfg); got != "countess" {
		t.Fatalf("CLI run = %q", got)
	}
	if got := resolveActiveRun(Options{}, cfg); got != "countess" {
		t.Fatalf("config run = %q", got)
	}
}

func TestValidateRunModeKnownRunOK(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err != nil {
		t.Fatalf("countess err = %v", err)
	}
}

func TestValidateRunModeFullCountessRequiresBindings(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err == nil {
		t.Fatal("expected missing full-run binding error")
	}

	cfg = fullCountessConfig(t)
	cfg.Input.Bindings.Belt.Slot4 = ""
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err == nil {
		t.Fatal("expected missing belt slot 4 error")
	}

	cfg = fullCountessConfig(t)
	delete(cfg.Input.Bindings.Skills, "bone_armor")
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err == nil {
		t.Fatal("expected missing Bone Armor profile binding error")
	}

	cfg = fullCountessConfig(t)
	cfg.Input.Bindings.Skills["bone_spear"] = config.SkillBindingConfig{Key: "f8", Button: "left"}
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err == nil {
		t.Fatal("expected unsafe left-mouse attack binding error")
	}
}

func TestValidateRunModePhaseRequiresRun(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("", "travel-entry"), cfg, Options{RunPhase: "travel-entry"}, log)
	if !errors.Is(err, errRunPhaseRequiresRun) {
		t.Fatalf("err = %v, want errRunPhaseRequiresRun", err)
	}
}

func TestValidateRunModeTravelMarshOK(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", "travel-entry"), cfg, Options{Run: "countess", RunPhase: "travel-entry"}, log)
	if err != nil {
		t.Fatalf("travel-entry err = %v", err)
	}
}

func TestValidateRunModeTravelCellar5OK(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", tasks.RunPhasePlayRoute), cfg, Options{Run: "countess", RunPhase: tasks.RunPhasePlayRoute}, log)
	if err != nil {
		t.Fatalf("play-route err = %v", err)
	}
}

func TestValidateRunModeKillCountessRequiresPhaseBindings(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", tasks.RunPhaseBoss), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseBoss}, log)
	if err == nil {
		t.Fatal("expected missing binding error")
	}

	cfg = fullCountessConfig(t)
	cfg.Input.Bindings.Skills = map[string]config.SkillBindingConfig{
		"teleport":   {Key: "f7", Button: "right"},
		"bone_spear": {Key: "f8", Button: "right"},
	}
	err = validateRunMode(tasksSelection("countess", tasks.RunPhaseBoss), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseBoss}, log)
	if err != nil {
		t.Fatalf("boss err = %v", err)
	}
}

func TestValidateRunModeLootCountessRequiresTeleportPortalAndBelt(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", tasks.RunPhaseLootAndReturn), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseLootAndReturn}, log)
	if err == nil {
		t.Fatal("expected missing loot-and-return binding error")
	}

	cfg = fullCountessConfig(t)
	delete(cfg.Input.Bindings.Skills, "bone_spear")
	err = validateRunMode(tasksSelection("countess", tasks.RunPhaseLootAndReturn), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseLootAndReturn}, log)
	if err != nil {
		t.Fatalf("loot-and-return err = %v, want no bone spear requirement", err)
	}

	cfg.Input.Bindings.Skills["teleport"] = config.SkillBindingConfig{}
	err = validateRunMode(tasksSelection("countess", tasks.RunPhaseLootAndReturn), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseLootAndReturn}, log)
	if err == nil {
		t.Fatal("expected missing teleport error")
	}
}

func TestMapRunConfigResolvesCountessCombatSkill(t *testing.T) {
	cfg := config.RunsConfig{
		StepTimeoutMs: 30000,
		Definitions: map[string]config.RunConfig{"countess": {RouteID: "test-route", Combat: config.CombatConfig{
			Profile:                 "necro_bone_spear",
			AttackSkill:             "bone_spear",
			AttackIntervalMs:        350,
			EngageDistanceTiles:     22,
			RepositionDistanceTiles: 32,
			KillConfirmTicks:        3,
		}, Loot: config.RunLootConfig{PickupFile: "pickit/countess.nip"}}},
	}
	got, err := mapRunConfig(cfg, "countess")
	if err != nil {
		t.Fatal(err)
	}
	if got.Combat.AttackSkillID != 84 || got.Combat.AttackInterval.String() != "350ms" {
		t.Fatalf("Combat = %+v", got.Combat)
	}
	if got.RouteID != "test-route" {
		t.Fatalf("RouteID = %q", got.RouteID)
	}
}

func TestValidateRunModeUnsupportedPhase(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", "tower"), cfg, Options{Run: "countess", RunPhase: "tower"}, log)
	if !errors.Is(err, errUnsupportedRunPhase) {
		t.Fatalf("err = %v, want errUnsupportedRunPhase", err)
	}
}

func tasksSelection(run, phase string) tasks.RunSelection {
	return tasks.RunSelection{Run: run, Phase: phase}
}

func fullCountessConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{Memory: config.MemoryConfig{GameVersion: "3.2.92777"}, Routes: config.RoutesConfig{FarmingRoot: "../../configs/routes/farming", LifecycleFile: filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")}, Session: config.SessionConfig{Character: "MrBones", Difficulty: "nightmare"}, Runs: config.RunsConfig{Definitions: map[string]config.RunConfig{"countess": {RouteID: "black-marsh-cellar5-nightmare-mrbones", Combat: config.CombatConfig{Profile: "necro_bone_spear", AttackSkill: "bone_spear", AttackIntervalMs: 350, EngageDistanceTiles: 22, RepositionDistanceTiles: 32, KillConfirmTicks: 3}, Loot: config.RunLootConfig{PickupFile: "pickit/countess.nip"}}}}, Profiles: config.ProfilesConfig{
		"necro_bone_spear": {CharacterClass: "necromancer", Hooks: config.ProfileHooksConfig{
			TownReady:  []config.ProfileActionConfig{{Skill: "bone_armor", Target: "self", OncePerGame: true}},
			BossEngage: []config.ProfileActionConfig{{Skill: "bone_prison", Target: "boss", OncePerEncounter: true}},
		}, Resources: config.ProfileResourcesConfig{
			Healing:      config.ResourceRuleConfig{UseBelowPercent: 65, BeltSlots: []int{1}, CooldownMs: 4000},
			Mana:         config.ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{2, 3}, CooldownMs: 4000},
			Rejuvenation: config.ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{4}, CooldownMs: 1500},
			ThrottleMs:   1500, VerifyMs: 1500,
		}},
	}, Input: config.InputConfig{
		Enabled: true,
		Bindings: config.InputBindingsConfig{
			Skills: map[string]config.SkillBindingConfig{
				"teleport":    {Key: "f7", Button: "right"},
				"bone_spear":  {Key: "f8", Button: "right"},
				"town_portal": {Key: "f6", Button: "right"},
				"bone_armor":  {Key: "f5", Button: "right"},
				"bone_prison": {Key: "f3", Button: "right"},
			},
			Belt: config.BeltBindingsConfig{
				Slot1: "1",
				Slot2: "2",
				Slot3: "3",
				Slot4: "4",
			},
		},
	}}
}
