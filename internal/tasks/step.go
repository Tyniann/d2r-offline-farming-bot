package tasks

import "time"

// stepTracker holds per-step timing and tick-count state for [Runner].
type stepTracker struct {
	name        string
	startedAt   time.Time
	timeout     time.Duration
	ticksInStep int
}

func (s *stepTracker) begin(name string, now time.Time, timeout time.Duration) {
	s.name = name
	s.startedAt = now
	s.timeout = timeout
	s.ticksInStep = 0
}

func (s *stepTracker) incrementTick() {
	s.ticksInStep++
}

func (s *stepTracker) elapsed(now time.Time) time.Duration {
	return now.Sub(s.startedAt)
}

func (s *stepTracker) timedOut(now time.Time) bool {
	if s.timeout <= 0 {
		return false
	}
	return now.Sub(s.startedAt) >= s.timeout
}
