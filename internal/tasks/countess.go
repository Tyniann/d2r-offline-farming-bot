package tasks

import "github.com/Tyniann/d2r-offline-farming-bot/internal/world"

const (
	countessStepPrecheck = "precheck"
	countessStepArmed    = "armed"
	countessStepComplete = "complete"
)

// countessRun is a Phase-4.1 stub: precheck → armed (tick counter) → complete.
type countessRun struct{}

func (c *countessRun) firstStep() string {
	return countessStepPrecheck
}

func (c *countessRun) nextStep(current string) string {
	switch current {
	case countessStepPrecheck:
		return countessStepArmed
	case countessStepArmed:
		return countessStepComplete
	default:
		return ""
	}
}

func (c *countessRun) usesTickTimeout(step string) bool {
	return step == countessStepArmed
}

func (c *countessRun) onTick(step string, w world.State, ticksInStep int) stepResult {
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Area.IsTown() {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: "not_in_town"}
	case countessStepArmed:
		if ticksInStep >= 2 {
			return stepResult{complete: true}
		}
		return stepResult{}
	case countessStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

type stepResult struct {
	complete bool
	failed   bool
	reason   string
}
