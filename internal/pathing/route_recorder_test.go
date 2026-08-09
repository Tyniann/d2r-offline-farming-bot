package pathing

import (
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func recorderState(area world.AreaID, x, y uint32) world.State {
	return world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(area),
		Player:   world.Player{Position: world.Position{X: x, Y: y}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
	}
}

func TestRouteRecorderSamplesAndCompletesTransitions(t *testing.T) {
	recorder, err := NewRouteRecorder(RouteRecorderConfig{SampleDistanceTiles: 4, Movement: RouteMovementTeleport})
	if err != nil {
		t.Fatal(err)
	}
	start := recorderState(world.BlackMarsh, 100, 100)
	if event, observeErr := recorder.Observe(start); observeErr != nil || !event.SampleAccepted {
		t.Fatalf("start = %+v, %v", event, observeErr)
	}
	near := recorderState(world.BlackMarsh, 102, 100)
	if event, _ := recorder.Observe(near); event.SampleAccepted {
		t.Fatal("near point was sampled")
	}
	far := recorderState(world.BlackMarsh, 108, 100)
	far.Entrances = []world.Entrance{{Kind: world.EntranceKindWildernessToTower, Position: world.Position{X: 109, Y: 100}}}
	if event, _ := recorder.Observe(far); !event.SampleAccepted {
		t.Fatal("far point was not sampled")
	}
	next := recorderState(world.ForgottenTower, 200, 200)
	event, err := recorder.Observe(next)
	if err != nil || !event.SegmentComplete {
		t.Fatalf("transition = %+v, %v", event, err)
	}
	if event.Segment.FromAreaID != world.BlackMarsh || event.Segment.ToAreaID != world.ForgottenTower || event.Segment.Transition.EntranceKind != "wilderness_to_tower" {
		t.Fatalf("segment = %+v", event.Segment)
	}
	segments, err := recorder.Finish()
	if err != nil || len(segments) != 1 {
		t.Fatalf("Finish() = %+v, %v", segments, err)
	}
}

func TestRouteRecorderFinishesSummonerAsSameAreaTerminalSegment(t *testing.T) {
	recorder, err := NewRouteRecorder(RouteRecorderConfig{SampleDistanceTiles: 4, Movement: RouteMovementTeleport})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = recorder.Observe(recorderState(world.ArcaneSanctuary, 100, 100))
	_, _ = recorder.Observe(recorderState(world.ArcaneSanctuary, 110, 100))

	segments, err := recorder.Finish()
	if err != nil || len(segments) != 1 {
		t.Fatalf("Finish() = %+v, %v", segments, err)
	}
	segment := segments[0]
	if segment.FromAreaID != world.ArcaneSanctuary || segment.ToAreaID != world.ArcaneSanctuary || segment.Transition.Type != "terminal" {
		t.Fatalf("Summoner terminal segment = %+v", segment)
	}
}

func TestRouteRecorderPinsPainToVaughtTransitionToHallsDown(t *testing.T) {
	recorder, err := NewRouteRecorder(RouteRecorderConfig{SampleDistanceTiles: 4, Movement: RouteMovementTeleport})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = recorder.Observe(recorderState(world.HallsOfPain, 100, 100))
	atEntrance := recorderState(world.HallsOfPain, 110, 100)
	atEntrance.Entrances = []world.Entrance{
		{Kind: world.EntranceKindHallsUp, Position: world.Position{X: 110, Y: 100}},
		{Kind: world.EntranceKindHallsDown, Position: world.Position{X: 114, Y: 100}},
	}
	_, _ = recorder.Observe(atEntrance)
	event, err := recorder.Observe(recorderState(world.HallsOfVaught, 200, 200))
	if err != nil || !event.SegmentComplete || event.Segment.Transition.EntranceKind != "halls_down" {
		t.Fatalf("Pain→Vaught transition = %+v, %v", event, err)
	}
	_, _ = recorder.Observe(recorderState(world.HallsOfVaught, 210, 200))
	segments, err := recorder.Finish()
	if err != nil || len(segments) != 2 || segments[1].Transition.Type != "terminal" || segments[1].ToAreaID != world.HallsOfVaught {
		t.Fatalf("Finish() = %+v, %v", segments, err)
	}
}

func TestRouteRecorderRecordsPermanentPortalTransition(t *testing.T) {
	recorder, _ := NewRouteRecorder(RouteRecorderConfig{SampleDistanceTiles: 4, Movement: RouteMovementTeleport})
	_, _ = recorder.Observe(recorderState(world.StonyField, 100, 100))
	portal := recorderState(world.StonyField, 110, 100)
	portal.Objects = []world.Object{{Kind: world.ObjectKindPermanentPortal, UnitID: 77, Position: world.Position{X: 112, Y: 100}}}
	_, _ = recorder.Observe(portal)
	event, err := recorder.Observe(recorderState(world.Tristram, 200, 200))
	if err != nil || !event.SegmentComplete {
		t.Fatalf("portal transition = %+v, %v", event, err)
	}
	transition := event.Segment.Transition
	if transition.Type != "object_portal" || transition.ObjectKind != world.ObjectKindPermanentPortal || transition.ExpectedToArea != world.Tristram {
		t.Fatalf("portal transition = %+v", transition)
	}
}

func TestRecordingTransitionKindFailsClosedWithoutExpectedHallsEntrance(t *testing.T) {
	state := recorderState(world.HallsOfPain, 100, 100)
	state.Entrances = []world.Entrance{{Kind: world.EntranceKindHallsUp, Position: world.Position{X: 101, Y: 100}}}
	if got := recordingTransitionKind(world.HallsOfPain, world.HallsOfVaught, state); got != "unknown" {
		t.Fatalf("transition kind = %q, want unknown", got)
	}
}

func TestRouteRecorderIgnoresInvalidAndLoading(t *testing.T) {
	recorder, _ := NewRouteRecorder(RouteRecorderConfig{SampleDistanceTiles: 4, Movement: RouteMovementWalk})
	if _, err := recorder.Observe(world.State{Phase: world.GamePhaseLoading}); err != nil {
		t.Fatal(err)
	}
	if _, err := recorder.Finish(); !errors.Is(err, ErrRouteRecordingNotStarted) {
		t.Fatalf("Finish error = %v", err)
	}
}

func TestRouteRecorderRejectsIdentityChange(t *testing.T) {
	recorder, _ := NewRouteRecorder(RouteRecorderConfig{SampleDistanceTiles: 4, Movement: RouteMovementTeleport})
	_, _ = recorder.Observe(recorderState(world.BlackMarsh, 100, 100))
	changed := recorderState(world.BlackMarsh, 110, 100)
	changed.Identity.CharacterName = "MrHammer"
	if _, err := recorder.Observe(changed); !errors.Is(err, ErrRouteRecordingIdentityChanged) {
		t.Fatalf("error = %v", err)
	}
}
