package app

import (
	"context"
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
