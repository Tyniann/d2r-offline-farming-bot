package app

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestProfileSelfCastUsesNeutralClientCenter(t *testing.T) {
	in := &mockInput{bound: true}
	adapter := &profileActionsAdapter{input: in, bindings: testBindings()}
	player := world.Position{X: 100, Y: 200}
	if err := adapter.CastSkillAtWorld(time.Now(), memory.SkillBoneArmor, player, player); err != nil {
		t.Fatal(err)
	}
	if in.lastClientX != 640 || in.lastClientY != 360 {
		t.Fatalf("self cast coordinates=%d,%d", in.lastClientX, in.lastClientY)
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
