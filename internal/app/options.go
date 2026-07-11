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
	// RunPhase selects an optional phase for the active run (e.g. travel-marsh).
	RunPhase string
	// PathingTest starts the manual pathing-test mode with the given spec (empty = disabled).
	// Specs: teleport:TX,TY | hover:watch | inspect:entrances|layout | move-area:<id|name> | click-entity:waypoint|entrance | pickup:item
	PathingTest string
	// PathingTestTimeoutMs bounds the pathing-test duration (default 120000 when <=0).
	PathingTestTimeoutMs int
	// OfflineDifficulty starts an isolated offline-character-screen selection test.
	OfflineDifficulty string
	// Route selects a read-only route registry command in Phase 6.2.
	Route string
	// RouteName is reserved for the Phase 6.3 record command.
	RouteName string
	// RouteDifficulty is the explicit non-authorizing label for a new recording.
	RouteDifficulty string
}
