package app

// Options holds CLI/runtime flags that are separate from YAML config.
type Options struct {
	// Desktop starts the private local API for the Electron parent without automatic session startup.
	Desktop bool
	// DesktopHandshakePipe selects the private one-shot Electron parent pipe.
	DesktopHandshakePipe string
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
	// RunPhase selects an optional phase for the active run (e.g. travel-entry).
	RunPhase string
	// PathingTest starts the manual pathing-test mode with the given spec (empty = disabled).
	// Specs: teleport:TX,TY | hover:watch | inspect:entrances|layout | move-area:<id|name> | click-entity:waypoint|entrance | pickup:item
	PathingTest string
	// PathingTestTimeoutMs bounds the pathing-test duration (default 120000 when <=0).
	PathingTestTimeoutMs int
	// OfflineDifficulty starts an isolated offline-character-screen selection test.
	OfflineDifficulty string
	// OfflineCharacter names the character that the isolated offline start must verify.
	OfflineCharacter string
	// OfflineExitTest starts the isolated Phase-7.2 Save & Exit test.
	OfflineExitTest bool
	// UIStateProbe labels one read-only Phase-7.1 UI-buffer capture.
	UIStateProbe string
	// UIStateProbeTimeoutMs bounds the read-only UI-state capture.
	UIStateProbeTimeoutMs int
	// ScreenAnchorCapture labels one Phase-7.3 frontend screenshot capture.
	ScreenAnchorCapture string
	// SessionInspect resolves and prints the Phase-7.5 session plan without runtime initialization.
	SessionInspect bool
	// RunsInspect prints deterministic read-only availability for every run definition.
	RunsInspect bool
	// WaypointTargetsInspect prints the registered resolution-bound waypoint actions.
	WaypointTargetsInspect bool
	// SessionMaxRuns overrides the configured finite session count when positive.
	SessionMaxRuns int
	// Route selects a read-only route registry command in Phase 6.2.
	Route string
	// RouteName labels farming-route and Town-egress recordings.
	RouteName string
	// RouteDifficulty is the explicit non-authorizing label for a new recording.
	RouteDifficulty string
	// TownInspect writes one read-only Phase-9.1 Town research report and never sends input.
	TownInspect bool
	// TownTest starts one isolated, fail-closed Town interaction acceptance flow.
	TownTest string
}
