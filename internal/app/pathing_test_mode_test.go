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
		{"inspect:layout", pathingTestSpec{kind: pathingTestInspect, entity: "layout"}},
		{"record-town-edge:portal-cain", pathingTestSpec{kind: pathingTestRecordEdge, route: "portal-cain"}},
		{"play-town-graph:portal_arrival,cain,akara,waypoint", pathingTestSpec{kind: pathingTestPlayGraph, route: "portal_arrival,cain,akara,waypoint"}},
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
		"record-town-route:act1-waypoint", "record-town-route:act2-waypoint", "pickup:gold", "unknown:arg",
		"record-town-edge:../escape", "play-town-graph:portal_arrival",
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
	edgeRecord, err := parsePathingTestSpec("record-town-edge:portal-cain")
	if err != nil || edgeRecord.requiresInput() {
		t.Fatalf("record-town-edge must be read-only: %v", err)
	}
	graphPlay, err := parsePathingTestSpec("play-town-graph:portal_arrival,cain,waypoint")
	if err != nil || !graphPlay.requiresInput() {
		t.Fatalf("play-town-graph must require input: %v", err)
	}
	inspect, err := parsePathingTestSpec("inspect:entrances")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if inspect.requiresInput() {
		t.Fatal("inspect:entrances must not require input")
	}
	layout, err := parsePathingTestSpec("inspect:layout")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if layout.requiresInput() {
		t.Fatal("inspect:layout must not require input")
	}
	pickup, err := parsePathingTestSpec("pickup:item")
	if err != nil {
		t.Fatalf("parse error = %v", err)
	}
	if !pickup.requiresInput() {
		t.Fatal("pickup:item must require input")
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

func TestSessionExecutionRequestedExcludesSpecializedModes(t *testing.T) {
	if !SessionExecutionRequested(Options{}) {
		t.Fatal("empty options should allow configured session ownership")
	}
	specialized := []Options{
		{PathingTest: "inspect:layout"},
		{Route: "inspect-egress:act2"},
		{Run: "summoner"},
		{Probe: true},
		{TownTest: "prepare:act2"},
		{InputTest: "belt:1"},
		{RuntimeTraceCapture: "focus-loss"},
		{WeaponSetProbe: "primary-secondary"},
		{ObjectInspect: "closed"},
	}
	for _, opts := range specialized {
		if SessionExecutionRequested(opts) {
			t.Fatalf("SessionExecutionRequested(%+v) = true, want false", opts)
		}
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
	if err := validatePathingTestMode(disabled, Options{PathingTest: "inspect:layout"}); err != nil {
		t.Fatalf("inspect:layout err = %v, want nil", err)
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
