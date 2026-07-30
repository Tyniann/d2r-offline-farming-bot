package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestConfigBindingsMapAmplifyDamageF1(t *testing.T) {
	bindings, err := newConfigBindingSource(config.InputBindingsConfig{Skills: map[string]config.SkillBindingConfig{
		"amplify_damage": {Key: "f1", Button: "right"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	cast, err := bindings.Resolve(memory.SkillAmplifyDamage)
	if err != nil {
		t.Fatal(err)
	}
	if cast.SelectKey != "f1" || cast.CastButton != input.MouseRight {
		t.Fatalf("amplify damage cast = %+v", cast)
	}
}

func TestBindingsPrecheckAllowsConfiguredTeleport(t *testing.T) {
	snap := validSnapshot(100)
	if err := BindingsPrecheck(config.NewLogger("error"), testBindings(), snap, true); err != nil {
		t.Fatalf("BindingsPrecheck() err = %v, want nil for configured teleport", err)
	}
}

func TestBindingsPrecheckTeleportUnconfigured(t *testing.T) {
	snap := validSnapshot(100)
	bindings := testBindings()
	delete(bindings.skills, memory.SkillTeleport)
	err := BindingsPrecheck(config.NewLogger("error"), bindings, snap, true)
	if err == nil || !strings.Contains(err.Error(), "teleport not configured") {
		t.Fatalf("err = %v, want teleport not configured", err)
	}
}

func TestBindingsPrecheckRejectsLeftButtonTeleport(t *testing.T) {
	snap := validSnapshot(100)
	bindings := testBindings()
	cast := bindings.skills[memory.SkillTeleport]
	cast.CastButton = "left"
	bindings.skills[memory.SkillTeleport] = cast
	err := BindingsPrecheck(config.NewLogger("error"), bindings, snap, true)
	if err == nil || !strings.Contains(err.Error(), "teleport binding unsafe") {
		t.Fatalf("err = %v, want unsafe left-button teleport", err)
	}
}

func TestBindingsPrecheckTownPortalWarnOnly(t *testing.T) {
	snap := validSnapshot(100)
	bindings := testBindings()
	delete(bindings.skills, memory.SkillTownPortal)
	if err := BindingsPrecheck(config.NewLogger("error"), bindings, snap, true); err != nil {
		t.Fatalf("BindingsPrecheck() err = %v, want nil when only TP missing", err)
	}
}

func TestBindingsPrecheckSkippedWhenInputDisabled(t *testing.T) {
	snap := memory.Snapshot{Valid: true, Phase: memory.GamePhaseInGame}
	if err := BindingsPrecheck(config.NewLogger("error"), configBindingSource{}, snap, false); err != nil {
		t.Fatalf("BindingsPrecheck() err = %v, want nil when input inactive", err)
	}
}

func TestBindingsPrecheckResetsAfterProcessLost(t *testing.T) {
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 42},
		status:     process.Status{State: process.StateAttached, PID: 42},
	}
	rt := testRuntimeWithInput(proc, &mockProbe{snap: validSnapshot(100)}, &mockInput{enabled: true}, Options{})
	rt.Config.Input.Enabled = true
	state := &runState{attached: true, hasEverAttached: true, bindingsPrecheckDone: true}

	proc.pollStatus = process.Status{State: process.StateLost, PID: 42}
	proc.status = process.Status{State: process.StateLost, PID: 42}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatalf("runTick() err = %v", err)
	}
	if state.bindingsPrecheckDone {
		t.Fatal("bindingsPrecheckDone should reset after process lost")
	}
}
