package tasks

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// runMachine executes step logic for a configured run name.
type runMachine interface {
	firstStep() string
	nextStep(current string) string
	usesTickTimeout(step string) bool
	onTick(step string, w world.State, ticksInStep int) stepResult
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

func newRunMachine(name string) (runMachine, error) {
	switch name {
	case "countess":
		return &countessRun{}, nil
	default:
		return nil, fmt.Errorf("unknown run %q", name)
	}
}
