package app

import "testing"

func TestRestartableSessionFailureRequiresConfiguredReason(t *testing.T) {
	allowed := []string{"hard_stuck", "route_segment_timeout"}
	if !isRestartableSessionFailure("hard_stuck", allowed) {
		t.Fatal("configured restartable reason was rejected")
	}
	if isRestartableSessionFailure("route_drift_exceeded", allowed) {
		t.Fatal("known but unconfigured reason was accepted")
	}
	if isRestartableSessionFailure("hard_stuck_typo", allowed) {
		t.Fatal("unknown reason was accepted")
	}
}
