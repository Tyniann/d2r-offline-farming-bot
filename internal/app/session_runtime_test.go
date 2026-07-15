package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPrepareSessionRunBindsSharedPipelineTelemetry(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Telemetry: config.TelemetryConfig{Directory: t.TempDir()},
			Session:   config.SessionConfig{Run: string(tasks.RunIDCountess)},
		},
		Log:              config.NewLogger("error"),
		sessionSelection: tasks.RunSelection{Run: string(tasks.RunIDCountess)},
		routePlayback:    &routePlaybackAdapter{},
		lootActions:      &lootActionsAdapter{},
	}

	if _, err := rt.prepareSessionRun(); err != nil {
		t.Fatal(err)
	}
	telemetryPath := rt.Telemetry.Path()
	if rt.taskDeps.Telemetry == nil {
		t.Fatal("shared pipeline telemetry is nil after session run preparation")
	}

	result := rt.Tasks.Tick(context.Background(), world.State{}, time.Now())
	if result.Reason == "telemetry_failed" {
		t.Fatalf("first session tick failed because pipeline telemetry was not bound: %+v", result)
	}
	if err := rt.closeSessionRunTelemetry(); err != nil {
		t.Fatal(err)
	}
	if rt.taskDeps.Telemetry != nil {
		t.Fatal("shared pipeline telemetry remains bound after session run close")
	}

	data, err := os.ReadFile(telemetryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"event":"run_step_started"`) {
		t.Fatalf("session run telemetry does not contain the first shared pipeline transition: %s", data)
	}
}
