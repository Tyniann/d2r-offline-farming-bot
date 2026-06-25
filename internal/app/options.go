package app

// Options holds CLI/runtime flags that are separate from YAML config.
type Options struct {
	// Probe enables Phase-1 memory snapshot reads after each successful process poll.
	Probe bool
	// Verbose forces debug-level logging (e.g. position-only probe lines with --probe).
	Verbose bool
}
