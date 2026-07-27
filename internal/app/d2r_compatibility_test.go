package app

import (
	"context"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestD2RCompatibilityMatrix(t *testing.T) {
	contract := d2rCompatibilityContract{supportedVersion: "3.2.92777", expectedVersion: "3.2.92777", offsetVersion: "3.2.92777"}
	tests := []struct {
		name     string
		contract d2rCompatibilityContract
		status   process.Status
		state    D2RCompatibilityState
		reason   Phase15ReasonCode
	}{
		{name: "not detected", contract: contract, status: process.Status{State: process.StateDetached}, state: D2RCompatibilityNotDetected, reason: Phase15ReasonD2RVersionNotDetected},
		{name: "match", contract: contract, status: process.Status{State: process.StateAttached, FileVersion: "3.2.92777"}, state: D2RCompatibilityCompatible},
		{name: "actual mismatch", contract: contract, status: process.Status{State: process.StateAttached, FileVersion: "3.3.1"}, state: D2RCompatibilityIncompatible, reason: Phase15ReasonD2RVersionUnsupported},
		{name: "missing resource", contract: contract, status: process.Status{State: process.StateAttached, VersionError: "missing"}, state: D2RCompatibilityUnreadable, reason: Phase15ReasonD2RVersionUnreadable},
		{name: "empty resource", contract: contract, status: process.Status{State: process.StateAttached}, state: D2RCompatibilityUnreadable, reason: Phase15ReasonD2RVersionUnreadable},
		{name: "PID or path drift", contract: contract, status: process.Status{State: process.StateDetached, PID: 42, VersionError: "bound identity drift"}, state: D2RCompatibilityUnreadable, reason: Phase15ReasonD2RVersionUnreadable},
		{name: "privilege mismatch", contract: contract, status: process.Status{State: process.StateDetached, PID: 42, PrivilegeMismatch: true}, state: D2RCompatibilityUnreadable, reason: Phase15ReasonPrivilegeMismatch},
		{name: "offset drift", contract: d2rCompatibilityContract{supportedVersion: "3.2.92777", expectedVersion: "3.2.92777", offsetVersion: "3.3.1"}, status: process.Status{State: process.StateAttached, FileVersion: "3.2.92777"}, state: D2RCompatibilityIncompatible, reason: Phase15ReasonOffsetVersionMismatch},
		{name: "expected drift", contract: d2rCompatibilityContract{supportedVersion: "3.2.92777", expectedVersion: "3.3.1", offsetVersion: "3.2.92777"}, status: process.Status{State: process.StateAttached, FileVersion: "3.2.92777"}, state: D2RCompatibilityIncompatible, reason: Phase15ReasonOffsetVersionMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.contract.evaluate(test.status)
			if got.State != test.state || got.Reason != test.reason {
				t.Fatalf("compatibility = %+v, want state=%s reason=%s", got, test.state, test.reason)
			}
			if got.State != D2RCompatibilityCompatible && got.Reason == "" {
				t.Fatal("blocked state requires a stable reason")
			}
		})
	}
}

func TestD2RCompatibilityDetachAndReattachReevaluatesActualVersion(t *testing.T) {
	contract := d2rCompatibilityContract{supportedVersion: "3.2.92777", expectedVersion: "3.2.92777", offsetVersion: "3.2.92777"}
	if got := contract.evaluate(process.Status{State: process.StateAttached, FileVersion: "3.2.92777"}); got.State != D2RCompatibilityCompatible {
		t.Fatalf("initial state = %+v", got)
	}
	if got := contract.evaluate(process.Status{State: process.StateLost, FileVersion: "3.2.92777"}); got.State != D2RCompatibilityNotDetected {
		t.Fatalf("detached state = %+v", got)
	}
	if got := contract.evaluate(process.Status{State: process.StateAttached, FileVersion: "3.3.1"}); got.State != D2RCompatibilityIncompatible {
		t.Fatalf("reattached state = %+v", got)
	}
}

func TestIncompatibleRuntimeCreatesNoHotkeyWindowOrInputPath(t *testing.T) {
	proc := &mockProcess{
		status:     process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.3.1"},
		pollStatus: process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.3.1"},
	}
	inputMock := &mockInput{enabled: true}
	rt := testRuntimeWithInput(proc, &mockProbe{}, inputMock, Options{})
	// Restore the deliberately incompatible version overwritten by the common test helper.
	proc.status.FileVersion = "3.3.1"
	proc.pollStatus.FileVersion = "3.3.1"
	rt.Config.Input.Enabled = true

	if _, err := rt.startHotkeys(context.Background()); err == nil {
		t.Fatal("expected hotkey start to fail before compatibility")
	}
	if err := rt.runTick(context.Background(), &runState{attached: true}); err != nil {
		t.Fatal(err)
	}
	if inputMock.listenCalls != 0 || inputMock.bindCalls != 0 || inputMock.focusCalls != 0 || len(inputMock.castSkillCalls) != 0 || len(inputMock.castBeltCalls) != 0 {
		t.Fatalf("input path was reached: %+v", inputMock)
	}
}

func TestCompatibleRuntimeRegistersHotkeysOnlyAfterGate(t *testing.T) {
	proc := &mockProcess{status: process.Status{State: process.StateAttached, PID: 42, FileVersion: "3.2.92777"}}
	inputMock := &mockInput{enabled: true}
	rt := testRuntimeWithInput(proc, &mockProbe{}, inputMock, Options{})
	if _, err := rt.startHotkeys(context.Background()); err != nil {
		t.Fatal(err)
	}
	if inputMock.listenCalls != 1 {
		t.Fatalf("listen calls = %d, want 1", inputMock.listenCalls)
	}
}
