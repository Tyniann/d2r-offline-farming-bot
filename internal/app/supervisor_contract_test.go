package app

import "testing"

func TestPhase11SupervisorCommandMatrix(t *testing.T) {
	tests := []struct {
		state   SupervisorState
		command SupervisorCommand
		allowed bool
	}{
		{SupervisorStateIdle, SupervisorCommandApplySelection, true},
		{SupervisorStateIdle, SupervisorCommandStartQueue, true},
		{SupervisorStateIdle, SupervisorCommandResume, false},
		{SupervisorStateActivatingSelection, SupervisorCommandEmergencyStop, true},
		{SupervisorStateActivatingSelection, SupervisorCommandStartQueue, false},
		{SupervisorStateIdleInGame, SupervisorCommandApplySelection, true},
		{SupervisorStateIdleInGame, SupervisorCommandStartQueue, true},
		{SupervisorStateStartingGame, SupervisorCommandEmergencyStop, true},
		{SupervisorStateStartingGame, SupervisorCommandPauseAfterRun, false},
		{SupervisorStateStartingRun, SupervisorCommandEmergencyStop, true},
		{SupervisorStateStartingRun, SupervisorCommandPauseAfterRun, false},
		{SupervisorStateRunningRun, SupervisorCommandPauseAfterRun, true},
		{SupervisorStateRunningRun, SupervisorCommandStopAfterRun, true},
		{SupervisorStateRunningRun, SupervisorCommandEmergencyStop, true},
		{SupervisorStateRunningRun, SupervisorCommandApplySelection, false},
		{SupervisorStatePausedBetweenRuns, SupervisorCommandResume, true},
		{SupervisorStatePausedBetweenRuns, SupervisorCommandEmergencyStop, true},
		{SupervisorStatePausedBetweenRuns, SupervisorCommandStartQueue, false},
		{SupervisorStateExitingGame, SupervisorCommandEmergencyStop, true},
		{SupervisorStateExitingGame, SupervisorCommandResume, false},
		{SupervisorStateCancelling, SupervisorCommandEmergencyStop, false},
		{SupervisorStateStoppedError, SupervisorCommandApplySelection, true},
		{SupervisorStateStoppedError, SupervisorCommandStartQueue, true},
	}
	for _, test := range tests {
		if got := SupervisorCommandAllowed(test.state, test.command); got != test.allowed {
			t.Errorf("SupervisorCommandAllowed(%q, %q) = %t, want %t", test.state, test.command, got, test.allowed)
		}
	}
}

func TestPhase11SupervisorContractHasUniqueStableValues(t *testing.T) {
	states := map[SupervisorState]bool{}
	commands := map[SupervisorCommand]bool{}
	for _, contract := range SupervisorTransitionContracts() {
		if contract.State == "" || states[contract.State] {
			t.Fatalf("duplicate or empty state %q", contract.State)
		}
		states[contract.State] = true
		for _, command := range contract.Commands {
			if command == "" {
				t.Fatal("empty command")
			}
			commands[command] = true
		}
	}
	if len(states) != 10 {
		t.Fatalf("state count = %d, want 10", len(states))
	}
	if len(commands) != 6 {
		t.Fatalf("command count = %d, want 6", len(commands))
	}

	copyOfContracts := SupervisorTransitionContracts()
	copyOfContracts[0].Commands[0] = "mutated"
	if !SupervisorCommandAllowed(SupervisorStateIdle, SupervisorCommandApplySelection) {
		t.Fatal("caller mutation changed the authoritative transition contract")
	}
}

func TestPhase11QueueDispositionValuesFreezeAdvanceSemantics(t *testing.T) {
	if QueueRunAdvance != "advance" || QueueRunRetryCurrent != "retry_current" || QueueRunStop != "stop" {
		t.Fatalf("queue dispositions changed: %q %q %q", QueueRunAdvance, QueueRunRetryCurrent, QueueRunStop)
	}
}

func TestExitAuthorizationAllowsOnlySpecifiedQueueDispositions(t *testing.T) {
	tests := []struct {
		name          string
		authorization ExitAuthorization
		disposition   QueueRunDisposition
		allowed       bool
	}{
		{name: "none stops", authorization: ExitAuthorizationNone, disposition: QueueRunStop, allowed: true},
		{name: "none cannot advance", authorization: ExitAuthorizationNone, disposition: QueueRunAdvance},
		{name: "none cannot retry", authorization: ExitAuthorizationNone, disposition: QueueRunRetryCurrent},
		{name: "verified town advances", authorization: ExitAuthorizationVerifiedRogueTown, disposition: QueueRunAdvance, allowed: true},
		{name: "verified town retries", authorization: ExitAuthorizationVerifiedRogueTown, disposition: QueueRunRetryCurrent, allowed: true},
		{name: "verified town stops", authorization: ExitAuthorizationVerifiedRogueTown, disposition: QueueRunStop, allowed: true},
		{name: "current area retries", authorization: ExitAuthorizationMemoryGatedCurrentArea, disposition: QueueRunRetryCurrent, allowed: true},
		{name: "current area stops", authorization: ExitAuthorizationMemoryGatedCurrentArea, disposition: QueueRunStop, allowed: true},
		{name: "current area cannot advance", authorization: ExitAuthorizationMemoryGatedCurrentArea, disposition: QueueRunAdvance},
		{name: "unknown authorization", authorization: ExitAuthorization("unknown"), disposition: QueueRunStop},
		{name: "unknown disposition", authorization: ExitAuthorizationNone, disposition: QueueRunDisposition("unknown")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.authorization.Allows(test.disposition); got != test.allowed {
				t.Fatalf("Allows(%q, %q) = %t, want %t", test.authorization, test.disposition, got, test.allowed)
			}
		})
	}
}

func TestSupervisorRunResultValidateExitContract(t *testing.T) {
	tests := []struct {
		name    string
		result  SupervisorRunResult
		wantErr bool
	}{
		{name: "terminal without exit", result: SupervisorRunResult{Disposition: QueueRunStop, ExitAuthorization: ExitAuthorizationNone}},
		{name: "advance from verified town", result: SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}},
		{name: "retry from verified town", result: SupervisorRunResult{Disposition: QueueRunRetryCurrent, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}},
		{name: "retry from current area", result: SupervisorRunResult{Disposition: QueueRunRetryCurrent, ExitAuthorization: ExitAuthorizationMemoryGatedCurrentArea}},
		{name: "stop from current area", result: SupervisorRunResult{Disposition: QueueRunStop, ExitAuthorization: ExitAuthorizationMemoryGatedCurrentArea}},
		{name: "advance without authorization", result: SupervisorRunResult{Disposition: QueueRunAdvance}, wantErr: true},
		{name: "retry without authorization", result: SupervisorRunResult{Disposition: QueueRunRetryCurrent, ExitAuthorization: ExitAuthorizationNone}, wantErr: true},
		{name: "advance from current area", result: SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationMemoryGatedCurrentArea}, wantErr: true},
		{name: "unknown disposition", result: SupervisorRunResult{Disposition: QueueRunDisposition("unknown"), ExitAuthorization: ExitAuthorizationNone}, wantErr: true},
		{name: "unknown authorization", result: SupervisorRunResult{Disposition: QueueRunStop, ExitAuthorization: ExitAuthorization("unknown")}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.result.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
