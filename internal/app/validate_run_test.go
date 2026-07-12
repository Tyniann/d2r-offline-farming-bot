package app

import (
	"errors"
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
	err := validateRunMode(tasksSelection("mephisto", ""), cfg, Options{}, log)
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
	cfg := fullCountessConfig()
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

	cfg = fullCountessConfig()
	cfg.Input.Bindings.Belt.Slot4 = ""
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err == nil {
		t.Fatal("expected missing belt slot 4 error")
	}
}

func TestValidateRunModePhaseRequiresRun(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("", "travel-marsh"), cfg, Options{RunPhase: "travel-marsh"}, log)
	if !errors.Is(err, errRunPhaseRequiresRun) {
		t.Fatalf("err = %v, want errRunPhaseRequiresRun", err)
	}
}

func TestValidateRunModeTravelMarshOK(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", "travel-marsh"), cfg, Options{Run: "countess", RunPhase: "travel-marsh"}, log)
	if err != nil {
		t.Fatalf("travel-marsh err = %v", err)
	}
}

func TestValidateRunModeTravelCellar5OK(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}, Runs: config.RunsConfig{Countess: config.CountessRunConfig{RouteID: "test-route"}}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", tasks.CountessPhaseTravelCellar5), cfg, Options{Run: "countess", RunPhase: tasks.CountessPhaseTravelCellar5}, log)
	if err != nil {
		t.Fatalf("travel-cellar5 err = %v", err)
	}
}

func TestValidateRunModeKillCountessRequiresPhaseBindings(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", tasks.CountessPhaseKillCountess), cfg, Options{Run: "countess", RunPhase: tasks.CountessPhaseKillCountess}, log)
	if err == nil {
		t.Fatal("expected missing binding error")
	}

	cfg.Input.Bindings.Skills = map[string]config.SkillBindingConfig{
		"teleport":   {Key: "f7", Button: "right"},
		"bone_spear": {Key: "f8", Button: "left"},
	}
	err = validateRunMode(tasksSelection("countess", tasks.CountessPhaseKillCountess), cfg, Options{Run: "countess", RunPhase: tasks.CountessPhaseKillCountess}, log)
	if err != nil {
		t.Fatalf("kill-countess err = %v", err)
	}
}

func TestValidateRunModeLootCountessRequiresTeleportPortalAndBelt(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", tasks.CountessPhaseLootCountess), cfg, Options{Run: "countess", RunPhase: tasks.CountessPhaseLootCountess}, log)
	if err == nil {
		t.Fatal("expected missing loot-countess binding error")
	}

	cfg = fullCountessConfig()
	delete(cfg.Input.Bindings.Skills, "bone_spear")
	err = validateRunMode(tasksSelection("countess", tasks.CountessPhaseLootCountess), cfg, Options{Run: "countess", RunPhase: tasks.CountessPhaseLootCountess}, log)
	if err != nil {
		t.Fatalf("loot-countess err = %v, want no bone spear requirement", err)
	}

	cfg.Input.Bindings.Skills["teleport"] = config.SkillBindingConfig{}
	err = validateRunMode(tasksSelection("countess", tasks.CountessPhaseLootCountess), cfg, Options{Run: "countess", RunPhase: tasks.CountessPhaseLootCountess}, log)
	if err == nil {
		t.Fatal("expected missing teleport error")
	}
}

func TestMapRunConfigResolvesCountessCombatSkill(t *testing.T) {
	cfg := config.RunsConfig{
		StepTimeoutMs: 30000,
		Countess: config.CountessRunConfig{RouteID: "test-route", Combat: config.CountessCombatConfig{
			Profile:                 "necro_bone_spear",
			AttackSkill:             "bone_spear",
			AttackIntervalMs:        350,
			EngageDistanceTiles:     22,
			RepositionDistanceTiles: 32,
			KillConfirmTicks:        3,
		}},
	}
	got := mapRunConfig(cfg)
	if got.CountessCombat.AttackSkillID != 84 || got.CountessCombat.AttackInterval.String() != "350ms" {
		t.Fatalf("CountessCombat = %+v", got.CountessCombat)
	}
	if got.CountessRouteID != "test-route" {
		t.Fatalf("CountessRouteID = %q", got.CountessRouteID)
	}
}

func TestValidateRunModeUnsupportedPhase(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(tasksSelection("countess", "tower"), cfg, Options{Run: "countess", RunPhase: "tower"}, log)
	if !errors.Is(err, errUnsupportedRunPhase) {
		t.Fatalf("err = %v, want errUnsupportedRunPhase", err)
	}
}

func tasksSelection(run, phase string) tasks.RunSelection {
	return tasks.RunSelection{Run: run, Phase: phase}
}

func fullCountessConfig() *config.Config {
	return &config.Config{Runs: config.RunsConfig{Countess: config.CountessRunConfig{RouteID: "test-route"}}, Input: config.InputConfig{
		Enabled: true,
		Bindings: config.InputBindingsConfig{
			Skills: map[string]config.SkillBindingConfig{
				"teleport":    {Key: "f7", Button: "right"},
				"bone_spear":  {Key: "f8", Button: "left"},
				"town_portal": {Key: "f6", Button: "right"},
			},
			Belt: config.BeltBindingsConfig{
				Slot1: "1",
				Slot4: "4",
			},
		},
	}}
}
