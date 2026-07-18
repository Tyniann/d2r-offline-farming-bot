package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func systemEgressRecordingState(area world.AreaID, player, portal, waypoint world.Position) world.State {
	return world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(area),
		Player:   world.Player{Position: player},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
		Objects: []world.Object{
			{ID: 1, Kind: world.ObjectKindTownPortal, Position: portal},
			{ID: 2, Kind: world.ObjectKindWaypoint, Position: waypoint},
		},
	}
}

func TestSystemEgressRecordingRequiresPortalArrival(t *testing.T) {
	area := world.LutGholein
	ready := systemEgressRecordingState(area, world.Position{X: 100, Y: 100}, world.Position{X: 103, Y: 100}, world.Position{X: 120, Y: 120})
	if !systemEgressRecordingStartReady(ready, area, 3) {
		t.Fatal("exact portal-arrival tolerance was rejected")
	}
	tooFar := ready
	tooFar.Objects[0].Position.X = 104
	if systemEgressRecordingStartReady(tooFar, area, 3) {
		t.Fatal("recording started away from portal_arrival")
	}
	wrongArea := ready
	wrongArea.Area = world.LookupArea(world.RogueEncampment)
	if systemEgressRecordingStartReady(wrongArea, area, 3) {
		t.Fatal("recording started in the wrong town")
	}
}

func TestSystemEgressRecordingUsesConfiguredPortalDistance(t *testing.T) {
	area := world.LutGholein
	state := systemEgressRecordingState(area, world.Position{X: 100, Y: 100}, world.Position{X: 110, Y: 100}, world.Position{X: 120, Y: 120})
	if !systemEgressRecordingStartReady(state, area, 15) {
		t.Fatal("portal inside configured interaction distance was rejected")
	}
	if systemEgressRecordingStartReady(state, area, 3) {
		t.Fatal("portal outside the supplied interaction distance was accepted")
	}
}

func TestSystemEgressRecordingPublishesOnlyAtWaypoint(t *testing.T) {
	area := world.LutGholein
	start := systemEgressRecordingState(area, world.Position{X: 100, Y: 100}, world.Position{X: 100, Y: 100}, world.Position{X: 120, Y: 120})
	fingerprint, err := pathing.BuildLayoutFingerprint(start)
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: routeRecordSampleDistance, Movement: pathing.RouteMovementWalk})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = recorder.Observe(start); err != nil {
		t.Fatal(err)
	}
	finish := start
	finish.Player.Position = world.Position{X: 116, Y: 120}
	if _, err = recorder.Observe(finish); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{Config: &config.Config{Memory: config.MemoryConfig{GameVersion: "3.2.92777"}, Pathing: config.PathingConfig{TownPortal: config.PathingTownPortalConfig{MaxClickDistance: 3}, Waypoint: config.PathingWaypointConfig{MaxClickDistance: 3}}}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	directory := t.TempDir()
	if err = rt.finishSystemEgressRecording(recorder, town.OriginAct2, directory, area, fingerprint, finish, 3, 3); err == nil || !strings.Contains(err.Error(), "waypoint proximity") {
		t.Fatalf("far waypoint finish error=%v", err)
	}

	recorder, _ = pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: routeRecordSampleDistance, Movement: pathing.RouteMovementWalk})
	_, _ = recorder.Observe(start)
	finish.Player.Position = world.Position{X: 120, Y: 120}
	_, _ = recorder.Observe(finish)
	if err = rt.finishSystemEgressRecording(recorder, town.OriginAct2, directory, area, fingerprint, finish, 3, 3); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(directory, town.SystemEgressFilename)); err != nil {
		t.Fatalf("published system route missing: %v", err)
	}
}
