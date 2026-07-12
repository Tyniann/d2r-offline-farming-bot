package tasks

import (
	"fmt"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

func TestRoutePlaybackFailureReasonUsesSentinels(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("wrapped: %w", pathing.ErrRouteHardStuck), "hard_stuck"},
		{fmt.Errorf("wrapped: %w", pathing.ErrRouteDriftExceeded), "route_drift_exceeded"},
		{fmt.Errorf("wrapped: %w", pathing.ErrRouteSegmentTimeout), "route_segment_timeout"},
		{fmt.Errorf("wrapped: %w", pathing.ErrRouteTransitionFailed), "route_transition_failed"},
		{fmt.Errorf("hard_stuck text only"), "route_playback_failed"},
	}
	for _, tc := range tests {
		if got := routePlaybackFailureReason(tc.err); got != tc.want {
			t.Errorf("reason(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}
