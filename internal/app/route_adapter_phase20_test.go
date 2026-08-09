package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestRoutePlaybackAdapterEmitsCowRoleWithoutReplacingPrimaryRoute(t *testing.T) {
	trace, err := telemetry.NewRunRecorder(t.TempDir(), telemetry.RunRecorderContext{
		RunID: "cows-adapter", Mode: telemetry.HistoryModeDiagnostic, Run: "cows", DefinitionID: "cows",
		RouteID: "cow-sweep", RouteLayoutFingerprint: "cow-layout",
		SetupRouteID: "leg-acquisition", SetupRouteLayoutFingerprint: "leg-layout", StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := trace.Path()
	adapter := &routePlaybackAdapter{
		telemetry: trace,
		route:     pathing.Route{ID: "leg-acquisition", Binding: pathing.RouteBinding{RouteRole: pathing.RouteRoleLegAcquisition}},
	}
	if emitErr := adapter.emit(telemetry.Event{Event: telemetry.RoutePlaybackStarted, RouteID: "leg-acquisition", SegmentID: "stony-field"}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if closeErr := trace.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	reader, err := telemetry.NewHistoryReader(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	file, err := reader.Read(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Events) != 1 || file.Events[0].RouteID != "cow-sweep" || file.Events[0].SetupRouteID != "leg-acquisition" || file.Events[0].RouteRole != string(pathing.RouteRoleLegAcquisition) {
		t.Fatalf("role-bound playback event = %+v", file.Events)
	}
}
