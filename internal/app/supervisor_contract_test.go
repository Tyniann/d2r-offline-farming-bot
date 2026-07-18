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
