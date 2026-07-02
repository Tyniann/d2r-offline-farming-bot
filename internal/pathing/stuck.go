package pathing

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// StuckDetector aborts navigation when neither the player position advances by
// at least the configured progress distance nor the area changes within the
// stuck timeout. Area changes always count as progress and reset the timer.
type StuckDetector struct {
	timeout       time.Duration
	progressTiles float64

	started      bool
	lastProgress time.Time
	lastPos      world.Position
	lastArea     world.AreaID
}

// NewStuckDetector builds a detector with the given timeout and progress distance.
func NewStuckDetector(timeout time.Duration, progressTiles float64) *StuckDetector {
	return &StuckDetector{
		timeout:       timeout,
		progressTiles: progressTiles,
	}
}

// Reset clears all progress tracking, e.g. when a new goal starts.
func (d *StuckDetector) Reset() {
	d.started = false
}

// Update records the current position/area and reports whether the navigator
// is stuck (no progress for the full timeout). The first call after Reset
// establishes the baseline and never reports stuck.
func (d *StuckDetector) Update(now time.Time, state world.State) bool {
	if !d.started {
		d.started = true
		d.lastProgress = now
		d.lastPos = state.Player.Position
		d.lastArea = state.Area.ID
		return false
	}

	areaChanged := state.Area.ID != d.lastArea
	moved := world.Distance(d.lastPos, state.Player.Position) >= d.progressTiles
	if areaChanged || moved {
		d.lastProgress = now
		d.lastPos = state.Player.Position
		d.lastArea = state.Area.ID
		return false
	}

	return now.Sub(d.lastProgress) >= d.timeout
}
