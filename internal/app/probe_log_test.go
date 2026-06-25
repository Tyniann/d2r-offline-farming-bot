package app

import (
	"testing"
	"time"
)

func TestProbeLogIsHeartbeat(t *testing.T) {
	if probeLogIsHeartbeat(time.Time{}, probeHeartbeat) {
		t.Fatal("zero lastLog must not be heartbeat (first invalid log uses Info)")
	}
	if !probeLogIsHeartbeat(time.Now().Add(-6*time.Second), probeHeartbeat) {
		t.Fatal("expected heartbeat after interval elapsed")
	}
	if probeLogIsHeartbeat(time.Now(), probeHeartbeat) {
		t.Fatal("unexpected heartbeat before interval elapsed")
	}
}
