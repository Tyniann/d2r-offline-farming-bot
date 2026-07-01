package app

// Options holds CLI/runtime flags that are separate from YAML config.
type Options struct {
	// Probe enables world-state logging after each successful process poll.
	Probe bool
	// Verbose forces debug-level logging (e.g. position-only world lines with --probe).
	Verbose bool
	// InputTest starts the manual input-test mode with the given action spec (empty = disabled).
	InputTest string
	// InputTestObserveMs is how long to poll world state after actions (default 3000 when <=0).
	InputTestObserveMs int
	// Run selects an active farming run; overrides config runs.active when set.
	Run string
}
