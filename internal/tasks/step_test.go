package tasks

import (
	"testing"
	"time"
)

func TestStepTrackerBeginResetsTicks(t *testing.T) {
	var s stepTracker
	now := time.Now()
	s.begin("precheck", now, 30*time.Second)
	s.incrementTick()
	s.incrementTick()

	s.begin("armed", now.Add(time.Second), 0)
	if s.ticksInStep != 0 {
		t.Fatalf("ticksInStep = %d, want 0 after begin", s.ticksInStep)
	}
	if s.name != "armed" {
		t.Fatalf("name = %q, want armed", s.name)
	}
}

func TestStepTrackerTimedOut(t *testing.T) {
	var s stepTracker
	start := time.Now()
	s.begin("wait", start, 50*time.Millisecond)

	if s.timedOut(start.Add(10 * time.Millisecond)) {
		t.Fatal("should not time out before deadline")
	}
	if !s.timedOut(start.Add(60 * time.Millisecond)) {
		t.Fatal("expected timeout after deadline")
	}
}

func TestStepTrackerZeroTimeoutNeverTimesOut(t *testing.T) {
	var s stepTracker
	start := time.Now()
	s.begin("armed", start, 0)

	if s.timedOut(start.Add(time.Hour)) {
		t.Fatal("zero timeout should never time out")
	}
}

func TestStepTrackerElapsed(t *testing.T) {
	var s stepTracker
	start := time.Now()
	s.begin("precheck", start, time.Second)

	elapsed := s.elapsed(start.Add(25 * time.Millisecond))
	if elapsed < 20*time.Millisecond {
		t.Fatalf("elapsed = %v, want >= 20ms", elapsed)
	}
}
