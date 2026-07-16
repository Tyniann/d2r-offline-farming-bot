package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestCurrentUIStatusProjectsReadOnlyComponentState(t *testing.T) {
	proc := &mockProcess{status: process.Status{State: process.StateAttached, PID: 42}}
	in := &mockInput{enabled: true, bound: true}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{UI: true})

	snapshot := rt.CurrentUIStatus("test error")
	if snapshot.ProcessState != "attached" || snapshot.PID != 42 || !snapshot.InputEnabled || !snapshot.WindowBound {
		t.Fatalf("UI snapshot = %+v", snapshot)
	}
	if snapshot.LastError != "test error" || snapshot.InputPaused || snapshot.InputStopped {
		t.Fatalf("UI safety snapshot = %+v", snapshot)
	}
}
