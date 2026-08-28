package app

import "testing"

func TestRestartableSessionFailureRequiresConfiguredReason(t *testing.T) {
	allowed := []string{"hard_stuck", "route_segment_timeout", "stash_approach_failed"}
	if !isRestartableSessionFailure("hard_stuck", allowed) {
		t.Fatal("configured restartable reason was rejected")
	}
	if !isRestartableSessionFailure("stash_approach_failed", allowed) {
		t.Fatal("configured stash approach reason was rejected")
	}
	if isRestartableSessionFailure("route_drift_exceeded", allowed) {
		t.Fatal("known but unconfigured reason was accepted")
	}
	if isRestartableSessionFailure("hard_stuck_typo", allowed) {
		t.Fatal("unknown reason was accepted")
	}
}

func TestMandatoryControlledExitReasonsIgnoreConfigurableRetryList(t *testing.T) {
	for _, reason := range []string{reasonMercenaryDiedDuringRun, "combat_resource_exhausted", "route_mana_recovery_failed"} {
		if !isMandatoryControlledExit(reason) {
			t.Fatalf("mandatory controlled exit rejected %q", reason)
		}
	}
	if isMandatoryControlledExit("route_segment_timeout") {
		t.Fatal("ordinary configured retry became mandatory")
	}
	if isDirectCurrentAreaRetry("hard_stuck") {
		t.Fatal("configured retry became a direct current-area retry")
	}
	if !isDirectCurrentAreaRetry("town_portal_enter_failed") {
		t.Fatal("town portal enter failure must retry from the current area")
	}
}
