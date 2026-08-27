package app

// SupervisorState identifies one externally observable state of the Phase-11
// long-lived session supervisor. Stop-after-run and pause-after-run remain
// intents while a run is active; they do not create hidden parallel states.
type SupervisorState string

const (
	// SupervisorStateIdle accepts selection changes and queue starts outside a game.
	SupervisorStateIdle SupervisorState = "idle"
	// SupervisorStateActivatingSelection applies a verified character and difficulty selection.
	SupervisorStateActivatingSelection SupervisorState = "activating_selection"
	// SupervisorStateIdleInGame accepts commands while a verified game is active and no run executes.
	SupervisorStateIdleInGame SupervisorState = "idle_in_game"
	// SupervisorStateStartingGame starts and verifies one game before the first run of a queue cycle.
	SupervisorStateStartingGame SupervisorState = "starting_game"
	// SupervisorStateStartingRun performs the preflight and reset barrier for one queue entry.
	SupervisorStateStartingRun SupervisorState = "starting_run"
	// SupervisorStateRunningRun executes one run through loot and the safe Town handoff.
	SupervisorStateRunningRun SupervisorState = "running_run"
	// SupervisorStatePausedBetweenRuns waits in the verified open game before the next queue entry.
	SupervisorStatePausedBetweenRuns SupervisorState = "paused_between_runs"
	// SupervisorStateExitingGame performs the supervisor-owned verified Save & Exit boundary.
	SupervisorStateExitingGame SupervisorState = "exiting_game"
	// SupervisorStateCancelling propagates immediate cancellation and permits no new gameplay input.
	SupervisorStateCancelling SupervisorState = "cancelling"
	// SupervisorStateStoppedError exposes a terminal queue error until a new valid command resets it.
	SupervisorStateStoppedError SupervisorState = "stopped_error"
)

// SupervisorCommand identifies a mutating domain command independently of its
// later HTTP representation.
type SupervisorCommand string

const (
	// SupervisorCommandApplySelection applies a previously confirmed selection preview.
	SupervisorCommandApplySelection SupervisorCommand = "apply_selection"
	// SupervisorCommandStartQueue starts a fully preflighted runtime queue.
	SupervisorCommandStartQueue SupervisorCommand = "start_queue"
	// SupervisorCommandPauseAfterRun requests a pause after the safe Town handoff without leaving the game.
	SupervisorCommandPauseAfterRun SupervisorCommand = "pause_after_run"
	// SupervisorCommandResume revalidates the open game and starts the next queue entry.
	SupervisorCommandResume SupervisorCommand = "resume"
	// SupervisorCommandStopAfterRun requests one orderly Save & Exit after the safe Town handoff.
	SupervisorCommandStopAfterRun SupervisorCommand = "stop_after_run"
	// SupervisorCommandEmergencyStop requests the same immediate cancellation path as F11.
	SupervisorCommandEmergencyStop SupervisorCommand = "emergency_stop"
)

// SupervisorIntent is an idempotent terminal intent retained while the current
// complete run unit finishes. Emergency stop is a command, not a retained intent.
type SupervisorIntent string

const (
	// SupervisorIntentNone allows cyclic queue advancement after success.
	SupervisorIntentNone SupervisorIntent = "none"
	// SupervisorIntentPauseAfterRun pauses in the open game before the next queue entry.
	SupervisorIntentPauseAfterRun SupervisorIntent = "pause_after_run"
	// SupervisorIntentStopAfterRun discards the active queue after the current run.
	SupervisorIntentStopAfterRun SupervisorIntent = "stop_after_run"
)

// QueueRunDisposition defines how one terminal Phase-10 session result affects
// the runtime queue. Only success advances; retry preserves the exact index.
type QueueRunDisposition string

const (
	// QueueRunAdvance advances to the next queue index and wraps cyclically.
	QueueRunAdvance QueueRunDisposition = "advance"
	// QueueRunRetryCurrent repeats the current queue index within the existing budgets.
	QueueRunRetryCurrent QueueRunDisposition = "retry_current"
	// QueueRunStop terminates the complete queue without starting another entry.
	QueueRunStop QueueRunDisposition = "stop"
)

// ExitAuthorization defines which supervisor-owned game exit may follow a
// terminal route execution. The authorization never performs the exit itself.
type ExitAuthorization string

const (
	// ExitAuthorizationNone permits no automatic exit input.
	ExitAuthorizationNone ExitAuthorization = "none"
	// ExitAuthorizationVerifiedRogueTown permits the existing exit after the
	// route owner confirmed Rogue Encampment and the active character identity.
	ExitAuthorizationVerifiedRogueTown ExitAuthorization = "verified_rogue_town"
	// ExitAuthorizationMemoryGatedCurrentArea requires the exit owner to prove a
	// stable current in-game area, character identity, and game generation.
	ExitAuthorizationMemoryGatedCurrentArea ExitAuthorization = "memory_gated_current_area"
)

// Allows reports whether this authorization may accompany the disposition.
func (a ExitAuthorization) Allows(disposition QueueRunDisposition) bool {
	switch a {
	case ExitAuthorizationNone:
		return disposition == QueueRunStop
	case ExitAuthorizationVerifiedRogueTown:
		return disposition == QueueRunAdvance || disposition == QueueRunRetryCurrent || disposition == QueueRunStop
	case ExitAuthorizationMemoryGatedCurrentArea:
		return disposition == QueueRunRetryCurrent || disposition == QueueRunStop
	default:
		return false
	}
}

// SupervisorTransitionContract declares which commands are accepted in one
// state. Internal worker completions are deliberately absent from this command
// contract and will be specified by the supervisor implementation.
type SupervisorTransitionContract struct {
	State    SupervisorState
	Commands []SupervisorCommand
}

var supervisorTransitionContracts = []SupervisorTransitionContract{
	{State: SupervisorStateIdle, Commands: []SupervisorCommand{SupervisorCommandApplySelection, SupervisorCommandStartQueue}},
	{State: SupervisorStateActivatingSelection, Commands: []SupervisorCommand{SupervisorCommandEmergencyStop}},
	{State: SupervisorStateIdleInGame, Commands: []SupervisorCommand{SupervisorCommandApplySelection, SupervisorCommandStartQueue}},
	{State: SupervisorStateStartingGame, Commands: []SupervisorCommand{SupervisorCommandEmergencyStop}},
	{State: SupervisorStateStartingRun, Commands: []SupervisorCommand{SupervisorCommandEmergencyStop}},
	{State: SupervisorStateRunningRun, Commands: []SupervisorCommand{SupervisorCommandPauseAfterRun, SupervisorCommandStopAfterRun, SupervisorCommandEmergencyStop}},
	{State: SupervisorStatePausedBetweenRuns, Commands: []SupervisorCommand{SupervisorCommandResume, SupervisorCommandEmergencyStop}},
	{State: SupervisorStateExitingGame, Commands: []SupervisorCommand{SupervisorCommandEmergencyStop}},
	{State: SupervisorStateCancelling},
	{State: SupervisorStateStoppedError, Commands: []SupervisorCommand{SupervisorCommandApplySelection, SupervisorCommandStartQueue}},
}

// SupervisorTransitionContracts returns a defensive copy of the command matrix.
func SupervisorTransitionContracts() []SupervisorTransitionContract {
	contracts := make([]SupervisorTransitionContract, len(supervisorTransitionContracts))
	for i, contract := range supervisorTransitionContracts {
		contracts[i] = contract
		contracts[i].Commands = append([]SupervisorCommand(nil), contract.Commands...)
	}
	return contracts
}

// SupervisorCommandAllowed reports whether the immutable Phase-11 command
// matrix permits a command in the supplied state.
func SupervisorCommandAllowed(state SupervisorState, command SupervisorCommand) bool {
	for _, contract := range supervisorTransitionContracts {
		if contract.State != state {
			continue
		}
		for _, allowed := range contract.Commands {
			if allowed == command {
				return true
			}
		}
		return false
	}
	return false
}
