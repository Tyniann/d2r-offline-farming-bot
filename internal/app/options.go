package app

// Options holds CLI/runtime flags that are separate from YAML config.
type Options struct {
	// Probe enables world-state logging after each successful process poll.
	Probe bool
	// Verbose forces debug-level logging (e.g. position-only world lines with --probe).
	Verbose bool
}
