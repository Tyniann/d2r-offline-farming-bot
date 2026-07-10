package app

import (
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestParsePathingTestSpec(t *testing.T) {
	cases := []struct {
		spec string
		want pathingTestSpec
	}{
		{"teleport:5000,5100", pathingTestSpec{kind: pathingTestTeleport, targetX: 5000, targetY: 5100}},
		{"hover:watch", pathingTestSpec{kind: pathingTestHoverWatch}},
		{"move-area:black_marsh", pathingTestSpec{kind: pathingTestMoveArea, area: world.BlackMarsh}},
		{"move-area:6", pathingTestSpec{kind: pathingTestMoveArea, area: world.BlackMarsh}},
		{"click-entity:waypoint", pathingTestSpec{kind: pathingTestClickEntity, entity: "waypoint"}},
		{"click-entity:entrance", pathingTestSpec{kind: pathingTestClickEntity, entity: "entrance"}},
		{"inspect:entrances", pathingTestSpec{kind: pathingTestInspect, entity: "entrances"}},
		{"play-town-route:act1-waypoint", pathingTestSpec{kind: pathingTestPlayTown, route: "act1-waypoint"}},
		{"record-town-route:act1-waypoint", pathingTestSpec{kind: pathingTestRecordTown, route: "act1-waypoint"}},
		{"pickup:item", pathingTestSpec{kind: pathingTestPickupItem, entity: "item"}},
	}
	for _, tc := range cases {
		got, err := parsePathingTestSpec(tc.spec)
		if err != nil {
			t.Fatalf("parsePathingTestSpec(%q) error = %v", tc.spec, err)
		}
		if got != tc.want {
			t.Fatalf("parsePathingTestSpec(%q) = %+v, want %+v", tc.spec, got, tc.want)
		}
	}
}

func TestParsePathingTestSpecInvalid(t *testing.T) {
	for _, spec := range []string{
		"", "teleport", "teleport:abc,5", "teleport:5000", "hover:foo",
		"move-area:unknown_area", "click-entity:monster", "inspect:objects", "play-town-route:act2-waypoint",
		"record-town-route:act2-waypoint", "pickup:gold", "unknown:arg",
	} {
		if _, err := parsePathingTestSpec(spec); err == nil {
			t.Fatalf("parsePathingTestSpec(%q) expected error", spec)
		}
	}
}

func TestPathingTestSpecRequiresInput(t *testing.T) {
	hover, err := parsePathingTestSpec("hover:watch")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if hover.requiresInput() {
		t.Fatal("hover:watch must not require input")
	}
	move, err := parsePathingTestSpec("move-area:6")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !move.requiresInput() {
		t.Fatal("move-area must require input")
	}
	record, err := parsePathingTestSpec("record-town-route:act1-waypoint")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if record.requiresInput() {
		t.Fatal("record-town-route must not require input")
	}
	inspect, err := parsePathingTestSpec("inspect:entrances")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if inspect.requiresInput() {
		t.Fatal("inspect:entrances must not require input")
	}
	play, err := parsePathingTestSpec("play-town-route:act1-waypoint")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !play.requiresInput() {
		t.Fatal("play-town-route must require input")
	}
	pickup, err := parsePathingTestSpec("pickup:item")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !pickup.requiresInput() {
		t.Fatal("pickup:item must require input")
	}
}

func TestTownRouteWaypointClickableReturnsWaypointPosition(t *testing.T) {
	cur := world.State{
		Valid:   true,
		Player:  world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 42, Position: world.Position{X: 105, Y: 104}}},
	}
	waypoint, ok := townRouteWaypointClickable(cur, 8)
	if !ok || waypoint.UnitID != 42 || waypoint.Position != (world.Position{X: 105, Y: 104}) {
		t.Fatalf("townRouteWaypointClickable() = %+v, %t; want waypoint 42", waypoint, ok)
	}
}

func TestValidatePathingTestModeConflicts(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}

	err := validatePathingTestMode(cfg, Options{PathingTest: "hover:watch", InputTest: "belt:1"})
	if !errors.Is(err, errPathingTestConflict) {
		t.Fatalf("input-test conflict err = %v, want errPathingTestConflict", err)
	}

	err = validatePathingTestMode(cfg, Options{PathingTest: "hover:watch", Run: "countess"})
	if !errors.Is(err, errPathingTestConflict) {
		t.Fatalf("run conflict err = %v, want errPathingTestConflict", err)
	}
}

func TestValidatePathingTestModeInputRequired(t *testing.T) {
	disabled := &config.Config{Input: config.InputConfig{Enabled: false}}

	err := validatePathingTestMode(disabled, Options{PathingTest: "move-area:6"})
	if !errors.Is(err, errInputRequiredForPathingTest) {
		t.Fatalf("err = %v, want errInputRequiredForPathingTest", err)
	}
	err = validatePathingTestMode(disabled, Options{PathingTest: "pickup:item"})
	if !errors.Is(err, errInputRequiredForPathingTest) {
		t.Fatalf("pickup:item err = %v, want errInputRequiredForPathingTest", err)
	}

	// hover:watch is read-only and works without input.
	if err := validatePathingTestMode(disabled, Options{PathingTest: "hover:watch"}); err != nil {
		t.Fatalf("hover:watch err = %v, want nil", err)
	}
	if err := validatePathingTestMode(disabled, Options{PathingTest: "inspect:entrances"}); err != nil {
		t.Fatalf("inspect:entrances err = %v, want nil", err)
	}

	enabled := &config.Config{Input: config.InputConfig{Enabled: true}}
	if err := validatePathingTestMode(enabled, Options{PathingTest: "move-area:6"}); err != nil {
		t.Fatalf("move-area with input err = %v, want nil", err)
	}
}

func TestValidateRunModePathingTestConflict(t *testing.T) {
	cfg := &config.Config{Input: config.InputConfig{Enabled: true}}
	log := config.NewLogger("error")
	err := validateRunMode(resolveRunSelection(Options{Run: "countess", PathingTest: "move-area:6"}, cfg), cfg, Options{Run: "countess", PathingTest: "move-area:6"}, log)
	if !errors.Is(err, errPathingTestConflict) {
		t.Fatalf("err = %v, want errPathingTestConflict", err)
	}
}

func TestPathingClickTargetSelection(t *testing.T) {
	st := world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{
			{Kind: world.ObjectKindWaypoint, UnitID: 1, Position: world.Position{X: 105, Y: 105}, Name: "Waypoint"},
		},
		Entrances: []world.Entrance{
			{UnitID: 2, Position: world.Position{X: 200, Y: 200}, Name: "Far"},
			{UnitID: 3, Position: world.Position{X: 110, Y: 110}, Name: "Near"},
		},
	}

	wp, ok := pathingClickTarget(st, "waypoint")
	if !ok || wp.UnitID != 1 || wp.UnitType != world.HoverUnitTypeObject {
		t.Fatalf("waypoint target = %+v ok=%t", wp, ok)
	}

	en, ok := pathingClickTarget(st, "entrance")
	if !ok || en.UnitID != 3 || en.UnitType != world.HoverUnitTypeEntrance {
		t.Fatalf("entrance target = %+v ok=%t, want nearest entrance (unit 3)", en, ok)
	}

	if _, ok := pathingClickTarget(world.State{Valid: true}, "waypoint"); ok {
		t.Fatal("empty state must not yield a target")
	}
}
