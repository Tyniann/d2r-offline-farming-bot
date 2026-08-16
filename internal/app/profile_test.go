package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestProfileSelfCastUsesNeutralClientCenter(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := testBindings()
	combat := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	adapter := &profileActionsAdapter{input: in, bindings: bindings, combat: combat}
	player := world.Player{Position: world.Position{X: 100, Y: 200}, RightSkillID: memory.SkillBoneArmor}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillBoneArmor, player, player.Position); err != nil {
		t.Fatal(err)
	}
	if in.lastClientX != 640 || in.lastClientY != 360 || len(in.clickCalls) != 1 {
		t.Fatalf("self cast coords=%d,%d clicks=%v", in.lastClientX, in.lastClientY, in.clickCalls)
	}
}

func TestProfileCorpseCastRequiresFocusAndPlayableProjection(t *testing.T) {
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillCorpseExplosion}
	target := world.Position{X: 101, Y: 101}
	in := &recordingCombatInput{}
	in.focusErr = os.ErrPermission
	bindings := testBindings()
	combat := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	adapter := &profileActionsAdapter{input: in, bindings: bindings, projector: pathing.DefaultRelativeProjector(), combat: combat}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, target); err == nil || len(in.clickCalls) != 0 {
		t.Fatalf("focus gate err=%v clicks=%v", err, in.clickCalls)
	}

	in.focusErr = nil
	farTarget := world.Position{X: 1000, Y: 1000}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, farTarget); !errors.Is(err, profile.ErrCorpseExplosionTargetUnprojectable) || len(in.clickCalls) != 0 {
		t.Fatalf("projection gate err=%v clicks=%v", err, in.clickCalls)
	}

	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, target); err != nil {
		t.Fatal(err)
	}
	if len(in.clickCalls) != 1 || in.focusCalls != 3 {
		t.Fatalf("clicks=%v focus_calls=%d", in.clickCalls, in.focusCalls)
	}
}

func TestProfileCastSkipsSelectionWhenRightSkillIsAlreadyActive(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := testBindings()
	combat := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	adapter := &profileActionsAdapter{input: in, bindings: bindings, projector: pathing.DefaultRelativeProjector(), combat: combat}
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillCorpseExplosion}
	target := world.Position{X: 101, Y: 101}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, target); err != nil {
		t.Fatal(err)
	}
	if in.selectCalls != 0 || len(in.clickCalls) != 1 {
		t.Fatalf("selects=%d clicks=%v", in.selectCalls, in.clickCalls)
	}
}

func TestProfileCastClearsStaleCombatSelection(t *testing.T) {
	in := &recordingCombatInput{}
	bindings := testBindings()
	combat := newCombatAdapter(config.NewLogger("error"), in, bindings, pathing.DefaultConfig(), time.Millisecond)
	_, _ = combat.selector.EnsureAndCast(memory.SkillBoneSpear, memory.SkillTeleport, time.Now(), func() error { return nil })
	combat.pendingTargetUnitID = 77
	combat.hoverProbeAttempt = 3
	adapter := &profileActionsAdapter{
		input: in, bindings: bindings, projector: pathing.DefaultRelativeProjector(), combat: combat,
	}
	player := world.Player{Position: world.Position{X: 100, Y: 100}, RightSkillID: memory.SkillCorpseExplosion}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillCorpseExplosion, player, world.Position{X: 101, Y: 101}); err != nil {
		t.Fatal(err)
	}
	if len(in.clickCalls) != 1 || combat.selector.pending != 0 || combat.pendingTargetUnitID != 0 || combat.hoverProbeAttempt != 0 {
		t.Fatalf("clicks=%v pending=%d target=%d hover=%d", in.clickCalls, combat.selector.pending, combat.pendingTargetUnitID, combat.hoverProbeAttempt)
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

func TestNewProfileExecutorAllowsUnregisteredDummyCarrier(t *testing.T) {
	profiles := config.ProfilesConfig{}
	profiles.ApplyDefaults()
	log := config.NewLogger("error")
	in := &mockInput{}
	bindings := testBindings()
	pathCfg := pathing.DefaultConfig()
	combat := newCombatAdapter(log, in, bindings, pathCfg, time.Millisecond)
	registry := NewCombatStrategyRegistry()

	got, err := newProfileExecutor(log, profiles, "paladin_hammerdin", "countess", registry, in, bindings, pathCfg, combat, &profileTelemetryAdapter{}, false)
	if err != nil || got == nil {
		t.Fatalf("dummy carrier err=%v executor=%v", err, got)
	}

	_, err = newProfileExecutor(log, profiles, "paladin_hammerdin", "countess", registry, in, bindings, pathCfg, combat, &profileTelemetryAdapter{}, true)
	if err == nil || !strings.Contains(err.Error(), ReasonProfileRunStrategyUnavailable) {
		t.Fatalf("productive dummy carrier err=%v", err)
	}

	got, err = newProfileExecutor(log, profiles, "paladin_hammerdin", "mephisto", registry, in, bindings, pathCfg, combat, &profileTelemetryAdapter{}, true)
	if err != nil || got == nil {
		t.Fatalf("registered mephisto err=%v executor=%v", err, got)
	}
}
