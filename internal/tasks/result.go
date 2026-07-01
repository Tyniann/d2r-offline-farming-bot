package tasks

// RunOutcome describes the high-level result of a configured run.
type RunOutcome string

// Run outcome values returned in [TickResult].
const (
	RunOutcomeIdle    RunOutcome = "idle"
	RunOutcomeRunning RunOutcome = "running"
	RunOutcomeSuccess RunOutcome = "success"
	RunOutcomeFailed  RunOutcome = "failed"
)

// TickResult summarizes one [Runner.Tick] invocation.
type TickResult struct {
	Active  bool       // False when terminal, reset, or no configured run.
	Outcome RunOutcome // idle | running | success | failed.
	Step    string     // Current step name when active or just finished.
	Reason  string     // Failure or reset reason when set.
}
