package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile/sorceressblizzard"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestSorceressBlizzardLocalClearStartsAfterTeleport(t *testing.T) {
	in := &recordingCombatInput{}
	blizzard := memory.MustSkillID("blizzard")
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{
		blizzard:             {SkillID: blizzard, SelectKey: "f3", CastButton: input.MouseRight},
		memory.SkillTeleport: {SkillID: memory.SkillTeleport, SelectKey: "f1", CastButton: input.MouseRight},
	}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 2*time.Second)
	exec, err := profile.NewExecutor(config.NewLogger("error"), profile.Definition{ID: sorceressblizzard.ID}, &profileActionsAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := sorceressblizzard.NewFactory("lower-kurast")().Configure(exec, blizzard, adapter); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillTeleport}
	if sent, err := adapter.TeleportToward(now, player, world.Position{X: 105, Y: 100}, 0); err != nil || !sent {
		t.Fatalf("teleport=%v err=%v", sent, err)
	}
	// Ein bestätigter Truhenblocker startet den lokalen Clear nach der Landung.
	request := profile.RouteClearRequest{RunID: "lower-kurast", DefinitionID: sorceressblizzard.ID, Player: player, Target: world.Monster{UnitID: 7, Position: world.Position{X: 105, Y: 100}, IsHovered: true}, Mode: profile.RouteClearThreat}
	var attacked bool
	for i := 8; i < 30; i++ {
		at := now.Add(time.Duration(i) * 100 * time.Millisecond)
		request.AssessmentAt = at
		if in.lastSkill == blizzard {
			request.Player.RightSkillID = blizzard
		}
		result := exec.TickRouteClear(context.Background(), request, at)
		if result.Status == profile.StatusFailed {
			t.Fatalf("clear=%+v", result)
		}
		if result.Status == profile.StatusAction {
			attacked = true
			break
		}
	}
	if !attacked {
		t.Fatalf("no Blizzard before the three-second local-clear budget; selections=%d clicks=%v", in.selectCalls, in.clickCalls)
	}
}

func TestSorceressBlizzardFillsCooldownWithFreshTargetsAndPrioritizesBlizzard(t *testing.T) {
	for _, run := range []string{"countess", "mephisto", "summoner", "nihlathak", "lower-kurast", "cows"} {
		t.Run(run, func(t *testing.T) {
			blizzard, ice := memory.MustSkillID("blizzard"), memory.MustSkillID("ice_blast")
			in := &recordingCombatInput{}
			bindings := configBindingSource{skills: map[uint16]input.SkillCast{
				blizzard: {SkillID: blizzard, SelectKey: "f3", CastButton: input.MouseRight},
				ice:      {SkillID: ice, SelectKey: "f6", CastButton: input.MouseRight},
			}}
			adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 1800*time.Millisecond)
			exec, err := profile.NewExecutor(config.NewLogger("error"), profile.Definition{ID: sorceressblizzard.ID}, &profileActionsAdapter{})
			if err != nil {
				t.Fatal(err)
			}
			if err := sorceressblizzard.NewFactory(run)().Configure(exec, blizzard, adapter); err != nil {
				t.Fatal(err)
			}
			base := time.Now()
			req := profile.RouteClearRequest{RunID: run, DefinitionID: sorceressblizzard.ID, Mode: profile.RouteClearThreat,
				Player: world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: blizzard},
				Target: world.Monster{UnitID: 7, Position: world.Position{X: 105, Y: 100}, IsHovered: true}}
			tick := func(ms int, wantSkill uint16, wantSent bool) {
				t.Helper()
				req.AssessmentAt = base.Add(time.Duration(ms) * time.Millisecond)
				got := exec.TickRouteClear(context.Background(), req, req.AssessmentAt)
				if got.Status == profile.StatusFailed || (got.Status == profile.StatusAction) != wantSent || (wantSent && (got.SkillID != wantSkill || got.TargetUnitID != req.Target.UnitID)) {
					t.Fatalf("%dms: result=%+v target=%+v", ms, got, req.Target)
				}
			}
			tick(0, blizzard, true)
			// Blizzard hat das alte Ziel getötet. Im nächsten Snapshot ist nur
			// dessen Nachfolger vorhanden; Auswahl und Aim passieren zusammen.
			req.Target = world.Monster{UnitID: 8, Position: world.Position{X: 106, Y: 100}}
			tick(100, ice, false)
			if in.lastSkill != ice || adapter.pendingTargetUnitID != 8 || adapter.hoverProbeAttempt != 1 {
				t.Fatalf("selection did not overlap aim: input=%+v adapter=%+v", in, adapter)
			}
			req.Player.RightSkillID = ice
			req.Target = world.Monster{UnitID: 9, Position: world.Position{X: 107, Y: 100}, IsHovered: true}
			tick(100, ice, false) // Kein Cast aus dem Auswahl-Snapshot.
			tick(200, ice, true)
			tick(200, ice, false) // Kein Burst aus mehrfach verarbeitetem Tick.
			req.Target.UnitID = 10
			tick(300, ice, true)
			exec.ResetRouteClear()
			tick(1700, ice, true)
			tick(1800, blizzard, false)
			if in.lastSkill != blizzard {
				t.Fatal("Ice Blast postponed the Blizzard deadline")
			}
			req.Player.RightSkillID = blizzard
			tick(1900, blizzard, true)
			if len(in.clickCalls) != 5 {
				t.Fatalf("casts=%v", in.clickCalls)
			}
			if got, err := adapter.CastAttackAtMonster(base.Add(2*time.Second), blizzard, req.Player, world.Monster{}); err != nil || got.Sent {
				t.Fatalf("cast without living target: %+v, %v", got, err)
			}
		})
	}
}

func TestSorceressBlizzardSetupRuntimeAndCowPreflight(t *testing.T) {
	service, _, _, _ := newCharacterSetupTestService(t, nil)
	preview, err := service.buildPreview(CharacterCatalog{}, OperatorSettings{}, PickitAssignmentManifest{}, CharacterCatalogEntry{Name: "Frost", Slug: "frost", ExpectedClass: "sorceress"})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Supported || preview.DefaultProfileID != sorceressblizzard.ID || len(preview.Profiles) != 1 || len(preview.Profiles[0].SupportedRuns) != 6 || len(preview.Profiles[0].RequiredSkills) != 6 || !preview.Profiles[0].RequiresMercenary {
		t.Fatalf("setup=%+v", preview)
	}
	bindingsConfig := config.InputBindingsConfig{Skills: map[string]config.SkillBindingConfig{
		"teleport": {Key: "f1", Button: "right"}, "town_portal": {Key: "f2", Button: "right"}, "blizzard": {Key: "f3", Button: "right"}, "static_field": {Key: "f4", Button: "right"}, "frozen_armor": {Key: "f5", Button: "right"}, "ice_blast": {Key: "f6", Button: "right"},
	}}
	bindings, err := newConfigBindingSource(bindingsConfig)
	if err != nil {
		t.Fatal(err)
	}
	registry := NewCombatStrategyRegistry()
	log := config.NewLogger("error")
	combat := newCombatAdapter(log, &recordingCombatInput{}, bindings, pathing.DefaultConfig(), 2*time.Second)
	for _, run := range []string{"countess", "cows", "lower-kurast", "mephisto", "nihlathak", "summoner"} {
		runCfg, mappingErr := mapRunConfigWithProfile(service.cfg, run, sorceressblizzard.ID, false)
		if mappingErr != nil {
			t.Fatal(mappingErr)
		}
		if runCfg.Combat.UseCorpseExplosion || runCfg.Combat.AttackSkillID != memory.MustSkillID("blizzard") || runCfg.Combat.AttackInterval != 1800*time.Millisecond {
			t.Fatalf("%s combat=%+v", run, runCfg.Combat)
		}
		if _, profileErr := newProfileExecutor(log, service.cfg.Profiles, sorceressblizzard.ID, run, registry, &recordingCombatInput{}, bindings, pathing.DefaultConfig(), combat, nil, true); profileErr != nil {
			t.Fatal(profileErr)
		}
	}
	cow := mapCowConfig(service.cfg, bindings, nil, sorceressblizzard.ID)
	if !cow.ExpectedClassKnown || cow.ExpectedClass != world.CharacterClassSorceress || !cow.RequiredSkillsReady {
		t.Fatalf("cow=%+v", cow)
	}
	delete(bindingsConfig.Skills, "ice_blast")
	missing, err := newConfigBindingSource(bindingsConfig)
	if err != nil {
		t.Fatal(err)
	}
	if mapCowConfig(service.cfg, missing, nil, sorceressblizzard.ID).RequiredSkillsReady {
		t.Fatal("Cow preflight accepted missing Ice Blast")
	}
	availability, err := ResolveRunAvailabilities(service.cfg, RunAvailabilityContext{
		Character: "Frost", CharacterClass: "sorceress", CombatProfile: sorceressblizzard.ID,
		Difficulty: "hell", GameVersion: "3.2.92777",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(availability.Runs) != 6 {
		t.Fatalf("availability=%+v", availability)
	}
	for _, run := range availability.Runs {
		for _, reason := range run.Reasons {
			if reason == tasks.RunReasonProfileClassMismatch || reason == tasks.RunReasonProfileRunStrategyUnavailable {
				t.Fatalf("Blizzard blocked by profile: %+v", run)
			}
		}
	}
}

func TestSorceressBlizzardConfirmedCastsKeepCooldownAcrossRouteClearReset(t *testing.T) {
	in := &recordingCombatInput{}
	id := memory.MustSkillID("blizzard")
	bindings := configBindingSource{skills: map[uint16]input.SkillCast{id: {SkillID: id, SelectKey: "f3", CastButton: input.MouseRight}, memory.MustSkillID("ice_blast"): {SkillID: memory.MustSkillID("ice_blast"), SelectKey: "f6", CastButton: input.MouseRight}}}
	adapter := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), 2*time.Second)
	w := world.Player{Position: world.Position{X: 100, Y: 100}}
	target := world.Monster{UnitID: 7, Position: world.Position{X: 105, Y: 100}, IsHovered: true}
	now := time.Now()
	if sent, err := adapter.CastAttackAtWorld(now, id, w, target.Position); err != nil || sent {
		t.Fatalf("selection sent=%v err=%v", sent, err)
	}
	w.RightSkillID = id
	if sent, err := adapter.CastAttackAtWorld(now.Add(2*time.Second), id, w, target.Position); err != nil || !sent {
		t.Fatalf("confirmed cast=%v err=%v", sent, err)
	}
	exec, err := profile.NewExecutor(config.NewLogger("error"), profile.Definition{ID: sorceressblizzard.ID}, &profileActionsAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := sorceressblizzard.NewFactory("cows")().Configure(exec, id, adapter); err != nil {
		t.Fatal(err)
	}
	exec.ResetRouteClear()
	if got, err := adapter.CastAttackAtMonster(now.Add(3*time.Second), id, w, target); err != nil || got.Sent {
		t.Fatalf("cooldown bypass=%+v err=%v", got, err)
	}
	if got, err := adapter.CastAttackAtMonster(now.Add(4*time.Second), id, w, target); err != nil || !got.Sent {
		t.Fatalf("route cast=%+v err=%v", got, err)
	}
	if len(in.clickCalls) != 2 || len(in.holds) != 0 {
		t.Fatalf("clicks=%v holds=%v", in.clickCalls, in.holds)
	}
}
