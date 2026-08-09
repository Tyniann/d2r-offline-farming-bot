package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestProfileSelfCastUsesNeutralClientCenter(t *testing.T) {
	in := &mockInput{bound: true}
	adapter := &profileActionsAdapter{input: in, bindings: testBindings()}
	player := world.Player{Position: world.Position{X: 100, Y: 200}}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillBoneArmor, player, player.Position); err != nil {
		t.Fatal(err)
	}
	if in.lastClientX != 640 || in.lastClientY != 360 {
		t.Fatalf("self cast coordinates=%d,%d", in.lastClientX, in.lastClientY)
	}
}

func TestProfileCorpseCastRequiresFocusAndPlayableProjection(t *testing.T) {
	player := world.Player{Position: world.Position{X: 100, Y: 100}}
	target := world.Position{X: 101, Y: 101}
	in := &mockInput{bound: true, focusErr: os.ErrPermission}
	adapter := &profileActionsAdapter{input: in, bindings: testBindings(), projector: pathing.DefaultRelativeProjector()}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, target); err == nil || len(in.castSkillCalls) != 0 {
		t.Fatalf("focus gate err=%v inputs=%v", err, in.castSkillCalls)
	}

	in.focusErr = nil
	farTarget := world.Position{X: 1000, Y: 1000}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, farTarget); !errors.Is(err, profile.ErrCorpseExplosionTargetUnprojectable) || len(in.castSkillCalls) != 0 {
		t.Fatalf("projection gate err=%v inputs=%v", err, in.castSkillCalls)
	}

	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, target); err != nil {
		t.Fatal(err)
	}
	if len(in.castSkillCalls) != 1 || in.castSkillCalls[0] != memory.SkillCorpseExplosion || in.focusCalls != 3 {
		t.Fatalf("inputs=%v focus_calls=%d", in.castSkillCalls, in.focusCalls)
	}
}

func TestProfileCastSkipsSelectionWhenRightSkillIsAlreadyActive(t *testing.T) {
	in := &mockInput{bound: true}
	adapter := &profileActionsAdapter{input: in, bindings: testBindings(), projector: pathing.DefaultRelativeProjector()}
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillCorpseExplosion}
	target := world.Position{X: 101, Y: 101}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, target); err != nil {
		t.Fatal(err)
	}
	if len(in.castSkillCalls) != 0 || in.clickCalls != 1 {
		t.Fatalf("skill selections=%v clicks=%d", in.castSkillCalls, in.clickCalls)
	}
}

func TestProfileCastClearsStaleCombatSelection(t *testing.T) {
	in := &mockInput{bound: true}
	combat := &combatAdapter{pendingSkill: memory.SkillBoneSpear, pendingTargetUnitID: 77, hoverProbeAttempt: 3}
	adapter := &profileActionsAdapter{
		input: in, bindings: testBindings(), projector: pathing.DefaultRelativeProjector(), combat: combat,
	}
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillBoneSpear}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, world.Position{X: 101, Y: 101}); err != nil {
		t.Fatal(err)
	}
	if len(in.castSkillCalls) != 1 || in.castSkillCalls[0] != memory.SkillCorpseExplosion ||
		combat.pendingSkill != 0 || combat.pendingTargetUnitID != 0 || combat.hoverProbeAttempt != 0 {
		t.Fatalf("profile casts=%v stale combat selection=%+v", in.castSkillCalls, combat)
	}
}

func TestProfileTelemetryAdapterMapsCorrelatedJSONLFields(t *testing.T) {
	recorder, err := telemetry.New(t.TempDir(), "countess", "")
	if err != nil {
		t.Fatal(err)
	}
	path := recorder.Path()
	adapter := &profileTelemetryAdapter{recorder: recorder}
	if emitErr := adapter.EmitProfile(profile.Event{Name: profile.EventHookAction, Profile: "necro_bone_spear", Hook: profile.HookBossEngage, SkillID: 88, Target: profile.TargetBoss, TargetUnitID: 273}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if closeErr := recorder.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatalf("missing telemetry event: %v", scanner.Err())
	}
	var event telemetry.Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.Event != telemetry.ProfileHookAction || event.Profile != "necro_bone_spear" || event.Hook != "boss_engage" || event.Skill != "bone_prison" || event.SkillID != 88 || event.Target != "boss" || event.UnitID != 273 {
		t.Fatalf("event=%+v", event)
	}
}

func TestProfileTelemetryAdapterOmitsSkillForPotionEvents(t *testing.T) {
	recorder, err := telemetry.New(t.TempDir(), "countess", "")
	if err != nil {
		t.Fatal(err)
	}
	path := recorder.Path()
	adapter := &profileTelemetryAdapter{recorder: recorder}
	if emitErr := adapter.EmitProfile(profile.Event{Name: profile.EventPotionRequested, Profile: "necro_bone_spear", Resource: profile.ResourceMana, ThresholdPercent: 35, BeltSlot: 2, PotionUnitID: 217}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if closeErr := recorder.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event telemetry.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	if event.Skill != "" || event.SkillID != 0 || event.Resource != "mana" || event.UnitID != 217 {
		t.Fatalf("event=%+v", event)
	}
}
