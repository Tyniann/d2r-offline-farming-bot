package profile

// StrategyFactory creates one executable run strategy for a profile/run pair.
type StrategyFactory func() RunStrategy

// RunStrategy is the task-neutral combat strategy surface for one (profile, run) pair.
// Tasks keep geometry, hold, recovery and budgets; the strategy only configures
// profile-owned combat behavior and declares its skill dependencies.
type RunStrategy interface {
	// ProfileID returns the combat profile this strategy belongs to.
	ProfileID() string
	// RunID returns the farming run this strategy supports.
	RunID() string
	// RequiredSkills returns canonical catalog keys this strategy depends on.
	RequiredSkills() []string
	// Configure applies profile-owned combat wiring to the shared executor.
	Configure(exec *Executor, standardAttackID uint16, routeClear RouteCombatActions) error
}

// SupportsRouteClear reports whether the strategy needs the single-target route-clear surface.
type SupportsRouteClear interface {
	RequiresRouteClear() bool
}

// SupportsCorpseExplosion reports whether the strategy needs authorized CE wiring.
type SupportsCorpseExplosion interface {
	RequiresCorpseExplosion() bool
}
