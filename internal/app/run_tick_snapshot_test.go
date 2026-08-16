package app

import (
	"context"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestPollQueueSnapshotSkipsBindingsReadinessAndTasks(t *testing.T) {
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.2.92777"},
		status:     process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.2.92777"},
	}
	in := &mockInput{enabled: true, bound: true}
	rt := testRuntimeWithInput(proc, &mockProbe{snap: validSnapshot(100)}, in, Options{})
	rt.Config.Input.Enabled = true
	rt.runReadinessPending = true
	rt.productiveRunActive = true
	rt.Tasks = nil // nil Tasks would panic if snapshot-only accidentally entered the full arm
	state := &runState{attached: true, hasEverAttached: true}

	if err := rt.pollQueueSnapshot(context.Background(), state); err != nil {
		t.Fatalf("pollQueueSnapshot err = %v", err)
	}
	if state.bindingsPrecheckDone {
		t.Fatal("snapshot-only poller must not run BindingsPrecheck")
	}
	if !rt.runReadinessPending {
		t.Fatal("snapshot-only poller must not consume run readiness")
	}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatalf("full runTick err = %v", err)
	}
	if !state.bindingsPrecheckDone {
		t.Fatal("full runTick must still perform BindingsPrecheck")
	}
}

func TestPassiveDesktopSkipsBindingsPrecheckWithoutFrozenLoadout(t *testing.T) {
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.2.92777"},
		status:     process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.2.92777"},
	}
	in := &mockInput{enabled: true, bound: true}
	rt := testRuntimeWithInput(proc, &mockProbe{snap: validSnapshot(100)}, in, Options{Desktop: true})
	rt.Config.Input.Enabled = true
	rt.Bindings = configBindingSource{}
	state := &runState{attached: true, hasEverAttached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatalf("idle desktop runTick err = %v", err)
	}
	if state.bindingsPrecheckDone {
		t.Fatal("idle desktop must not run BindingsPrecheck without a frozen loadout")
	}

	rt.Options.Loadout = &CharacterLoadoutSnapshot{ProfileID: "paladin_hammerdin"}
	err := rt.runTick(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "teleport not configured") {
		t.Fatalf("frozen-loadout desktop err = %v, want teleport not configured", err)
	}

	rt.Bindings = testBindings()
	state.bindingsPrecheckDone = false
	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatalf("frozen-loadout desktop with teleport err = %v", err)
	}
	if !state.bindingsPrecheckDone {
		t.Fatal("frozen-loadout desktop must still perform BindingsPrecheck")
	}
}
