package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RunSelection identifies the configured run and optional phase.
type RunSelection struct {
	// Run is the configured farming run name.
	Run string
	// Phase is an optional run phase; empty preserves the run's default behavior.
	Phase string
}

// runMachine executes step logic for a configured run name.
type runMachine interface {
	firstStep() string
	nextStep(current string) string
	usesTickTimeout(step string) bool
	allowsNonInputTick(step string) bool
	onStepEnter(step string)
	onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult
}

// KnownRuns returns registered run names in stable order.
func KnownRuns() []string {
	return []string{"countess"}
}

// IsKnownRun reports whether name is a registered run.
func IsKnownRun(name string) bool {
	for _, known := range KnownRuns() {
		if known == name {
			return true
		}
	}
	return false
}

func newRunMachine(sel RunSelection, cfg RunConfig) (runMachine, error) {
	switch sel.Run {
	case "countess":
		switch sel.Phase {
		case "":
			return &countessRun{combat: cfg.CountessCombat, routeID: cfg.CountessRouteID}, nil
		case CountessPhaseTravelMarsh, CountessPhaseTravelCellar5, CountessPhaseKillCountess, CountessPhaseLootCountess, CountessPhaseStashPersonal:
			return &countessRun{phase: sel.Phase, combat: cfg.CountessCombat, routeID: cfg.CountessRouteID}, nil
		default:
			return nil, fmt.Errorf("unknown countess phase %q", sel.Phase)
		}
	default:
		return nil, fmt.Errorf("unknown run %q", sel.Run)
	}
}
