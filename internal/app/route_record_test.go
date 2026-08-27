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
	if !systemEgressRecordingStartReady(ready, area, 3, town.AnchorPortalArrival) {
		t.Fatal("exact portal-arrival tolerance was rejected")
	}
	tooFar := ready
	tooFar.Objects[0].Position.X = 104
	if systemEgressRecordingStartReady(tooFar, area, 3, town.AnchorPortalArrival) {
		t.Fatal("recording started away from portal_arrival")
	}
	wrongArea := ready
	wrongArea.Area = world.LookupArea(world.RogueEncampment)
	if systemEgressRecordingStartReady(wrongArea, area, 3, town.AnchorPortalArrival) {
		t.Fatal("recording started in the wrong town")
	}
}

func TestSystemEgressRecordingUsesConfiguredPortalDistance(t *testing.T) {
	area := world.LutGholein
	state := systemEgressRecordingState(area, world.Position{X: 100, Y: 100}, world.Position{X: 110, Y: 100}, world.Position{X: 120, Y: 120})
	if !systemEgressRecordingStartReady(state, area, 15, town.AnchorPortalArrival) {
		t.Fatal("portal inside configured interaction distance was rejected")
	}
	if systemEgressRecordingStartReady(state, area, 3, town.AnchorPortalArrival) {
		t.Fatal("portal outside the supplied interaction distance was accepted")
	}
}

func TestSystemEgressSpawnRecordingUsesSpawnContractAndFilename(t *testing.T) {
	area := world.LutGholein
	start := systemEgressRecordingState(area, world.Position{X: 100, Y: 100}, world.Position{X: 140, Y: 100}, world.Position{X: 120, Y: 120})
	start.Objects = start.Objects[1:]
	if !systemEgressRecordingStartReady(start, area, 3, town.AnchorSpawn) {
		t.Fatal("valid explicit spawn recording start rejected")
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(start)
	if err != nil {
		t.Fatal(err)
	}
	recorder, _ := pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: routeRecordSampleDistance, Movement: pathing.RouteMovementWalk})
	_, _ = recorder.Observe(start)
	finish := start
	finish.Player.Position = world.Position{X: 120, Y: 120}
	_, _ = recorder.Observe(finish)
	rt := &Runtime{Config: &config.Config{Memory: config.MemoryConfig{GameVersion: "3.2.92777"}}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	directory := t.TempDir()
	proofPoint := 0
	if err = rt.finishSystemEgressRecording(recorder, town.OriginAct2, town.AnchorSpawn, directory, area, fingerprint, &proofPoint, finish, 3, 3); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, town.SystemEgressSpawnFilename)
	route, err := town.LoadSystemEgressRoute(path)
	if err != nil || route.Contract.From != town.AnchorSpawn {
		t.Fatalf("route=%+v err=%v", route, err)
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
	if err = rt.finishSystemEgressRecording(recorder, town.OriginAct2, town.AnchorPortalArrival, directory, area, fingerprint, nil, finish, 3, 3); err == nil || !strings.Contains(err.Error(), "waypoint proximity") {
		t.Fatalf("far waypoint finish error=%v", err)
	}

	recorder, _ = pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: routeRecordSampleDistance, Movement: pathing.RouteMovementWalk})
	_, _ = recorder.Observe(start)
	finish.Player.Position = world.Position{X: 120, Y: 120}
	_, _ = recorder.Observe(finish)
	if err = rt.finishSystemEgressRecording(recorder, town.OriginAct2, town.AnchorPortalArrival, directory, area, fingerprint, nil, finish, 3, 3); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(directory, town.SystemEgressFilename)); err != nil {
		t.Fatalf("published system route missing: %v", err)
	}
}

func TestSpawnEgressLayoutCaptureBuffersMovementUntilFirstAnchor(t *testing.T) {
	capture := systemEgressLayoutCapture{startAnchor: town.AnchorSpawn}
	state := systemEgressRecordingState(world.LutGholein, world.Position{X: 100, Y: 100}, world.Position{}, world.Position{})
	state.Objects = nil
	if err := capture.Observe(state, pathing.RouteRecorderEvent{SampleAccepted: true}); err != nil {
		t.Fatal(err)
	}
	if capture.fingerprint.Version != 0 || capture.proofPointIndex != nil {
		t.Fatalf("anchorless spawn pinned layout: %+v", capture)
	}
	state.Player.Position = world.Position{X: 104, Y: 100}
	if err := capture.Observe(state, pathing.RouteRecorderEvent{SampleAccepted: true}); err != nil {
		t.Fatal(err)
	}
	state.Player.Position = world.Position{X: 108, Y: 100}
	state.Objects = []world.Object{{ID: 156, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 120, Y: 100}}}
	if err := capture.Observe(state, pathing.RouteRecorderEvent{SampleAccepted: true}); err != nil {
		t.Fatal(err)
	}
	if capture.fingerprint.Version == 0 || capture.proofPointIndex == nil || *capture.proofPointIndex != 2 {
		t.Fatalf("delayed proof capture=%+v", capture)
	}
	larger := state
	larger.Objects = append(larger.Objects, world.Object{ID: 267, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 130, Y: 100}})
	if err := capture.Observe(larger, pathing.RouteRecorderEvent{SampleAccepted: true}); err != nil {
		t.Fatal(err)
	}
	if *capture.proofPointIndex != 2 {
		t.Fatalf("later anchors moved pinned proof to %d", *capture.proofPointIndex)
	}
}
