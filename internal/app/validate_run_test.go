package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestValidateRunModePassiveOK(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: false}}
	log := config.NewLogger("error")
	if err := validateRunMode(resolveRunSelection(Options{}, cfg), cfg, Options{}, log); err != nil {
		t.Fatalf("passive mode err = %v", err)
	}
}

func TestValidateRunModeAllowsGlobalEgressRecordingWithoutDifficulty(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	opts := Options{Route: "record-egress:act4", RouteName: "Portal bis Wegpunkt"}
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err != nil {
		t.Fatalf("record-egress options error = %v", err)
	}

	opts.RouteDifficulty = "nightmare"
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err == nil {
		t.Fatal("expected global Egress difficulty rejection")
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

func TestWeaponSetProbeSuppressesConfiguredRunAndValidatesReadOnlyMode(t *testing.T) {
	cfg := &config.Config{Runs: config.RunsConfig{Active: "countess"}}
	opts := Options{WeaponSetProbe: weaponSetProbeLabel}
	if got := resolveActiveRun(opts, cfg); got != "" {
		t.Fatalf("resolveActiveRun() = %q, want empty for weapon-set probe", got)
	}
	if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, config.NewLogger("error")); err != nil {
		t.Fatalf("validateRunMode() = %v", err)
	}
	if err := validateRunMode(tasksSelection("", ""), cfg, Options{WeaponSetProbe: weaponSetProbeLabel, WeaponSetProbeTimeoutMs: -1}, config.NewLogger("error")); err == nil {
		t.Fatal("negative weapon-set probe timeout should fail")
	}
	if CharacterLoadoutRequired(opts) {
		t.Fatal("read-only weapon-set probe must not require an operator loadout")
	}
	if farmingRouteRequired(opts, tasksSelection("", "")) {
		t.Fatal("read-only weapon-set probe must not require a farming route")
	}
}

func TestValidateRunModeKnownRunOK(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	opts := Options{Run: "countess", Loadout: loadoutFromConfigBindingsForTest(cfg)}
	if err := validateRunMode(tasksSelection("countess", ""), cfg, opts, log); err != nil {
		t.Fatalf("countess err = %v", err)
	}
}

func TestValidateRunModeRuntimeTraceRequiresExplicitFullRun(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	loadout := loadoutFromConfigBindingsForTest(cfg)
	valid := Options{Run: "countess", RuntimeTraceCapture: "focus-loss", Loadout: loadout}
	if err := validateRunMode(tasksSelection("countess", ""), cfg, valid, log); err != nil {
		t.Fatalf("runtime trace options error = %v", err)
	}
	for _, opts := range []Options{
		{RuntimeTraceCapture: "focus-loss"},
		{Run: "countess", RunPhase: tasks.RunPhaseBoss, RuntimeTraceCapture: "focus-loss", Loadout: loadout},
		{Run: "countess", RuntimeTraceCapture: "../escape", Loadout: loadout},
		{Run: "countess", RuntimeTraceCapture: "focus-loss", Route: "list", Loadout: loadout},
	} {
		if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, log); err == nil {
			t.Fatalf("validateRunMode(%+v) succeeded, want trace-mode rejection", opts)
		}
	}
}

func TestValidateRunModeUsesFrozenProfileForAvailability(t *testing.T) {
	cfg := fullCountessConfig(t)
	alternate := cfg.Profiles["necro_bone_spear"]
	alternate.Setup.Default = false
	cfg.Profiles["necro_bone_spirit"] = alternate
	loadout := loadoutFromConfigBindingsForTest(cfg)
	loadout.ProfileID = "necro_bone_spirit"

	err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess", Loadout: loadout}, config.NewLogger("error"))
	if err == nil || !strings.Contains(err.Error(), string(tasks.RunReasonProfileRunStrategyUnavailable)) {
		t.Fatalf("alternate frozen profile error=%v", err)
	}
}

func TestValidateRunModeFullCountessRequiresBindings(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess"}, log); err == nil {
		t.Fatal("expected missing full-run binding error")
	}

	cfg = fullCountessConfig(t)
	bindings := fullCountessBindings()
	bindings.Belt.Slot4 = ""
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess", Loadout: loadoutFromBindingsForTest(cfg.Session.Character, bindings)}, log); err == nil {
		t.Fatal("expected missing belt slot 4 error")
	}

	cfg = fullCountessConfig(t)
	bindings = fullCountessBindings()
	delete(bindings.Skills, "bone_armor")
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess", Loadout: loadoutFromBindingsForTest(cfg.Session.Character, bindings)}, log); err == nil {
		t.Fatal("expected missing Bone Armor profile binding error")
	}

	cfg = fullCountessConfig(t)
	bindings = fullCountessBindings()
	bindings.Skills["bone_spear"] = config.SkillBindingConfig{Key: "f8", Button: "left"}
	if err := validateRunMode(tasksSelection("countess", ""), cfg, Options{Run: "countess", Loadout: loadoutFromBindingsForTest(cfg.Session.Character, bindings)}, log); err == nil {
		t.Fatal("expected unsafe left-mouse attack binding error")
	}
}

func TestValidateHammerdinAcceptsLeftMouseBlessedHammer(t *testing.T) {
	enabled := true
	cfg := &config.Config{
		Runs: config.RunsConfig{Definitions: map[string]config.RunConfig{
			"countess": {}, "mephisto": {}, "nihlathak": {},
			"summoner": {RouteCombat: config.RouteCombatConfig{Enabled: &enabled}},
			"cows":     {RouteCombat: config.RouteCombatConfig{Enabled: &enabled}},
		}},
		Profiles: config.ProfilesConfig{},
	}
	cfg.Profiles.ApplyDefaults()
	source, err := newConfigBindingSource(config.InputBindingsConfig{
		Skills: map[string]config.SkillBindingConfig{
			"teleport":       {Key: "f7", Button: "right"},
			"blessed_hammer": {Key: "f1", Button: "left"},
			"town_portal":    {Key: "f6", Button: "right"},
			"concentration":  {Key: "f2", Button: "right"},
			"holy_shield":    {Key: "f4", Button: "right"},
		},
		Belt: config.BeltBindingsConfig{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{"countess", "cows", "mephisto", "nihlathak", "summoner"} {
		if err = validateBossBindingsWithProfile(cfg, runID, source, "paladin_hammerdin"); err != nil {
			t.Fatalf("%s boss LMB Blessed Hammer rejected: %v", runID, err)
		}
		if err = validateFullRunBindingsWithProfile(cfg, runID, source, "paladin_hammerdin"); err != nil {
			t.Fatalf("%s full-run LMB Blessed Hammer rejected: %v", runID, err)
		}
	}
	hammerdinFactory, ok := DefaultCombatStrategyRegistry().Resolve("paladin_hammerdin", "cows")
	if !ok || !cowStrategyBindingsReady(source, hammerdinFactory()) {
		t.Fatal("strategy-required Hammerdin bindings were treated as Necromancer Cow bindings")
	}
	missingConcentration := cloneConfigBindingSource(source)
	delete(missingConcentration.skills, memory.MustSkillID("concentration"))
	if cowStrategyBindingsReady(missingConcentration, hammerdinFactory()) {
		t.Fatal("strategy-required Hammerdin binding gap was accepted")
	}

	necroWithoutOpener, err := newConfigBindingSource(config.InputBindingsConfig{
		Skills: map[string]config.SkillBindingConfig{
			"teleport":    {Key: "f7", Button: "right"},
			"bone_spear":  {Key: "f8", Button: "right"},
			"town_portal": {Key: "f6", Button: "right"},
		},
		Belt: config.BeltBindingsConfig{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = validateFullRunBindingsWithProfile(cfg, "summoner", necroWithoutOpener, "necro_bone_spear"); err == nil ||
		!strings.Contains(err.Error(), "amplify_damage") {
		t.Fatalf("Necromancer Summoner without Amplify Damage error = %v", err)
	}

	rightHammer, err := newConfigBindingSource(config.InputBindingsConfig{
		Skills: map[string]config.SkillBindingConfig{
			"teleport":       {Key: "f7", Button: "right"},
			"blessed_hammer": {Key: "f1", Button: "right"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBossBindingsWithProfile(cfg, "mephisto", rightHammer, "paladin_hammerdin"); err == nil {
		t.Fatal("RMB Blessed Hammer accepted against the profile LMB slot")
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
	opts := Options{Run: "countess", RunPhase: "travel-entry", Loadout: loadoutFromConfigBindingsForTest(cfg)}
	err := validateRunMode(tasksSelection("countess", "travel-entry"), cfg, opts, log)
	if err != nil {
		t.Fatalf("travel-entry err = %v", err)
	}
}

func TestValidateRunModeTravelCellar5OK(t *testing.T) {
	cfg := fullCountessConfig(t)
	log := config.NewLogger("error")
	opts := Options{Run: "countess", RunPhase: tasks.RunPhasePlayRoute, Loadout: loadoutFromConfigBindingsForTest(cfg)}
	err := validateRunMode(tasksSelection("countess", tasks.RunPhasePlayRoute), cfg, opts, log)
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
	bindings := fullCountessBindings()
	bindings.Skills = map[string]config.SkillBindingConfig{
		"teleport":   {Key: "f7", Button: "right"},
		"bone_spear": {Key: "f8", Button: "right"},
	}
	err = validateRunMode(tasksSelection("countess", tasks.RunPhaseBoss), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseBoss, Loadout: loadoutFromBindingsForTest(cfg.Session.Character, bindings)}, log)
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
	bindings := fullCountessBindings()
	delete(bindings.Skills, "bone_spear")
	err = validateRunMode(tasksSelection("countess", tasks.RunPhaseLootAndReturn), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseLootAndReturn, Loadout: loadoutFromBindingsForTest(cfg.Session.Character, bindings)}, log)
	if err != nil {
		t.Fatalf("loot-and-return err = %v, want no bone spear requirement", err)
	}

	bindings.Skills["teleport"] = config.SkillBindingConfig{}
	err = validateRunMode(tasksSelection("countess", tasks.RunPhaseLootAndReturn), cfg, Options{Run: "countess", RunPhase: tasks.RunPhaseLootAndReturn, Loadout: loadoutFromBindingsForTest(cfg.Session.Character, bindings)}, log)
	if err == nil {
		t.Fatal("expected missing teleport error")
	}
}

func TestValidateRunModeLootAndReturnDoesNotRequireFarmingRouteLifecycle(t *testing.T) {
	cfg := fullCountessConfig(t)
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "missing-or-archived-route"})
	log := config.NewLogger("error")
	loadout := loadoutFromConfigBindingsForTest(cfg)

	err := validateRunMode(
		tasksSelection("countess", tasks.RunPhaseLootAndReturn),
		cfg,
		Options{Run: "countess", RunPhase: tasks.RunPhaseLootAndReturn, Loadout: loadout},
		log,
	)
	if err != nil {
		t.Fatalf("loot-and-return must not consume a Farming route: %v", err)
	}
	for _, phase := range []string{"", tasks.RunPhaseTravelEntry, tasks.RunPhasePlayRoute, tasks.RunPhaseBoss, tasks.RunPhaseStashPersonal, tasks.RunPhaseTownReady} {
		t.Run("unchanged_"+phase, func(t *testing.T) {
			err := validateRunMode(
				tasksSelection("countess", phase),
				cfg,
				Options{Run: "countess", RunPhase: phase, Loadout: loadout},
				log,
			)
			if err == nil || !strings.Contains(err.Error(), string(tasks.RunReasonRouteMissing)) {
				t.Fatalf("phase %q error = %v, want route_missing", phase, err)
			}
		})
	}
}

func TestMapRunConfigSkipsAssignmentOnlyForLootAndReturn(t *testing.T) {
	cfg := fullCountessConfig(t)
	cfg.Routes.AssignmentsFile = filepath.Join(t.TempDir(), "missing-assignments.yaml")

	got, err := mapRunConfig(cfg, "countess", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.RouteID != "" {
		t.Fatalf("RouteID = %q, want empty for route-independent phase", got.RouteID)
	}
}

func TestPassiveDesktopUIAllowsMissingInitialFarmingAssignment(t *testing.T) {
	opts := Options{Desktop: true}
	selection := resolveRunSelection(opts, &config.Config{})
	if farmingRouteRequired(opts, selection) {
		t.Fatal("passive desktop UI must start before the first route assignment exists")
	}
}

func TestPassiveDesktopDoesNotRequireDummyCountessPickitOrStrategy(t *testing.T) {
	if CharacterLoadoutRequired(Options{Desktop: true}) {
		t.Fatal("passive desktop UI must construct without a frozen loadout")
	}
	if CharacterLoadoutRequired(Options{Desktop: true, Loadout: &CharacterLoadoutSnapshot{ProfileID: "paladin_hammerdin"}}) {
		t.Fatal("route recording must not require Countess pickit or strategy for the dummy carrier")
	}
	if !CharacterLoadoutRequired(Options{TownTest: "hammerdin-prebuff"}) {
		t.Fatal("isolated town-test must still require character pickit and strategy")
	}
	if !CharacterLoadoutRequired(Options{Run: "mephisto"}) {
		t.Fatal("productive runs must still require pickit and strategy")
	}
	pickit, err := loadRuntimePickitPolicy(Options{Desktop: true}, nil, "MrHammer", tasks.RunIDCountess)
	if err != nil || pickit == nil {
		t.Fatalf("idle desktop pickit err=%v pickit=%v", err, pickit)
	}
}

func TestTownTestAllowsMissingFarmingAssignment(t *testing.T) {
	opts := Options{TownTest: "mercenary-heal"}
	selection := resolveRunSelection(opts, &config.Config{})
	if selection.Run != "" {
		t.Fatalf("TownTest must not select a farming run, got %q", selection.Run)
	}
	if farmingRouteRequired(opts, selection) {
		t.Fatal("isolated town-test must not require a published farming route")
	}
}

func TestMapRunConfigResolvesCountessCombatSkill(t *testing.T) {
	cfg := &config.Config{Session: config.SessionConfig{Character: "MrBones"}, Routes: config.RoutesConfig{AssignmentsFile: filepath.Join(t.TempDir(), "assignments.yaml")}, Loot: config.LootConfig{Pickup: config.LootPickupConfig{MaxDistanceTiles: 6}}, Runs: config.RunsConfig{
		StepTimeoutMs: 30000,
		Definitions:   map[string]config.RunConfig{"countess": {}},
	}}
	cfg.Profiles.ApplyDefaults()
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "test-route"})
	got, err := mapRunConfig(cfg, "countess", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Combat.AttackSkillID != 84 || got.Combat.AttackInterval.String() != "350ms" {
		t.Fatalf("Combat = %+v", got.Combat)
	}
	if got.RouteID != "test-route" {
		t.Fatalf("RouteID = %q", got.RouteID)
	}
	if got.LootPickupDistanceTiles != 6 {
		t.Fatalf("LootPickupDistanceTiles = %v, want 6", got.LootPickupDistanceTiles)
	}
}

func TestMapCowCombatUsesSelectedProfileStrategyCapabilities(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	necroBindings, err := newConfigBindingSource(config.InputBindingsConfig{Skills: map[string]config.SkillBindingConfig{
		"teleport":         {Key: "f7", Button: "right"},
		"town_portal":      {Key: "f6", Button: "right"},
		"bone_spear":       {Key: "f8", Button: "right"},
		"amplify_damage":   {Key: "f1", Button: "right"},
		"corpse_explosion": {Key: "f2", Button: "right"},
		"bone_armor":       {Key: "f5", Button: "right"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	cow := mapCowConfig(cfg, necroBindings, nil, "necro_bone_spear")
	if !cow.ExpectedClassKnown || cow.ExpectedClass.String() != "necromancer" || !cow.RequiredSkillsReady {
		t.Fatalf("Necromancer Cow preflight config = %+v", cow)
	}
	necro, err := mapRunConfigWithProfile(cfg, "cows", "necro_bone_spear", false)
	if err != nil {
		t.Fatal(err)
	}
	if necro.Combat.Profile != "necro_bone_spear" || !necro.Combat.UseCorpseExplosion {
		t.Fatalf("Necromancer Cow combat = %+v", necro.Combat)
	}

	hammerdin, err := mapRunConfigWithProfile(cfg, "cows", "paladin_hammerdin", false)
	if err != nil {
		t.Fatal(err)
	}
	if hammerdin.Combat.Profile != "paladin_hammerdin" || hammerdin.Combat.UseCorpseExplosion {
		t.Fatalf("Hammerdin Cow combat inherited Necromancer capability: %+v", hammerdin.Combat)
	}

	hammerdinBindings, err := newConfigBindingSource(config.InputBindingsConfig{Skills: map[string]config.SkillBindingConfig{
		"teleport":       {Key: "f7", Button: "right"},
		"town_portal":    {Key: "f6", Button: "right"},
		"blessed_hammer": {Key: "f1", Button: "left"},
		"concentration":  {Key: "f2", Button: "right"},
		"holy_shield":    {Key: "f4", Button: "right"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	hammerdinCow := mapCowConfig(cfg, hammerdinBindings, nil, "paladin_hammerdin")
	if !hammerdinCow.ExpectedClassKnown || hammerdinCow.ExpectedClass.String() != "paladin" || !hammerdinCow.RequiredSkillsReady {
		t.Fatalf("Hammerdin Cow preflight config = %+v", hammerdinCow)
	}
	if mapCowConfig(cfg, necroBindings, nil, "paladin_hammerdin").RequiredSkillsReady {
		t.Fatal("Hammerdin Cow preflight accepted Necromancer skill bindings")
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
	cfg := &config.Config{Memory: config.MemoryConfig{GameVersion: "3.2.92777"}, Routes: config.RoutesConfig{FarmingRoot: "../../configs/routes/farming", LifecycleFile: filepath.Join(t.TempDir(), "route-lifecycle.local.yaml"), AssignmentsFile: filepath.Join(t.TempDir(), "route-assignments.local.yaml")}, Session: config.SessionConfig{Character: "MrBones", Difficulty: "nightmare"}, Runs: config.RunsConfig{Definitions: map[string]config.RunConfig{"countess": {Combat: config.CombatConfig{}}}}, Profiles: config.ProfilesConfig{
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
	}}
	cfg.Profiles.ApplyDefaults()
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "black-marsh-cellar5-nightmare-mrbones"})
	return cfg
}

func fullCountessBindings() config.InputBindingsConfig {
	return config.InputBindingsConfig{
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
	}
}

// loadoutFromBindingsForTest builds a CharacterLoadoutSnapshot from in-memory bindings for tests only.
func loadoutFromBindingsForTest(character string, bindings config.InputBindingsConfig) *CharacterLoadoutSnapshot {
	source, err := newConfigBindingSource(bindings)
	if err != nil {
		panic(err)
	}
	return &CharacterLoadoutSnapshot{
		Character: character,
		ProfileID: "necro_bone_spear",
		Bindings:  source,
	}
}

// loadoutFromConfigBindingsForTest builds a CharacterLoadoutSnapshot with the full Countess fixture bindings.
func loadoutFromConfigBindingsForTest(cfg *config.Config) *CharacterLoadoutSnapshot {
	return loadoutFromBindingsForTest(cfg.Session.Character, fullCountessBindings())
}
