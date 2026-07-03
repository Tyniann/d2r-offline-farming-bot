package pathing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func townWalkState(pos world.Position) world.State {
	return world.State{
		At:    time.Now(),
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.RogueEncampment),
		Player: world.Player{
			Position: pos,
		},
	}
}

func TestTownWalkerWaypointVisibleAndClickable(t *testing.T) {
	in := newMockInput()
	walker := NewTownWalker(config.NewLogger("error"), in, DefaultConfig())
	st := townWalkState(world.Position{X: 100, Y: 100})
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 1, Position: world.Position{X: 105, Y: 105}}}

	res := walker.TickAct1Waypoint(context.Background(), st)
	if res.Status != TownWalkWaypointVisible || !res.Done {
		t.Fatalf("res = %+v, want waypoint_visible", res)
	}
	if len(in.moves) != 0 || len(in.keys) != 0 {
		t.Fatalf("input moves=%v keys=%v, want none", in.moves, in.keys)
	}
}

func TestTownWalkerWaypointVisibleButTooFarKeepsWalking(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.RouteFile = filepath.Join(t.TempDir(), "missing.yaml")
	cfg.TownWalk.Act1WaypointPoints = []world.Position{{X: 110, Y: 110}, {X: 120, Y: 120}}
	cfg.Waypoint.MaxClickDistance = 5
	walker := NewTownWalker(config.NewLogger("error"), in, cfg)
	st := townWalkState(world.Position{X: 100, Y: 100})
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 1, Position: world.Position{X: 150, Y: 150}}}

	res := walker.TickAct1Waypoint(context.Background(), st)
	if res.Status != TownWalkPending || res.Done {
		t.Fatalf("res = %+v, want pending", res)
	}
	if len(in.keys) != 1 {
		t.Fatalf("keys = %v, want force move despite visible far waypoint", in.keys)
	}
}

func TestTownWalkerUsesForceMoveKey(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.RouteFile = filepath.Join(t.TempDir(), "missing.yaml")
	cfg.TownWalk.Act1WaypointPoints = []world.Position{{X: 110, Y: 110}, {X: 120, Y: 120}}
	cfg.TownWalk.ForceMoveKey = "e"
	walker := NewTownWalker(config.NewLogger("error"), in, cfg)

	res := walker.TickAct1Waypoint(context.Background(), townWalkState(world.Position{X: 100, Y: 100}))
	if res.Status != TownWalkPending || res.Done {
		t.Fatalf("res = %+v, want pending", res)
	}
	if len(in.moves) != 1 {
		t.Fatalf("moves = %v, want one move", in.moves)
	}
	if len(in.keys) != 1 || in.keys[0] != "e" {
		t.Fatalf("keys = %v, want [e]", in.keys)
	}
}

func TestTownWalkerFinalRoutePointWaitsForMovementSettle(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.RouteFile = filepath.Join(t.TempDir(), "missing.yaml")
	cfg.TownWalk.Act1WaypointPoints = []world.Position{{X: 110, Y: 100}}
	cfg.TownWalk.ArrivalDistance = 8
	cfg.TownWalk.SettleTimeout = 200 * time.Millisecond
	cfg.Waypoint.MaxClickDistance = 5
	walker := NewTownWalker(config.NewLogger("error"), in, cfg)
	now := time.Now()

	st := townWalkState(world.Position{X: 100, Y: 100})
	st.At = now
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 1, Position: world.Position{X: 120, Y: 100}}}
	_ = walker.TickAct1Waypoint(context.Background(), st)

	st.Player.Position = world.Position{X: 104, Y: 100}
	st.At = now.Add(100 * time.Millisecond)
	res := walker.TickAct1Waypoint(context.Background(), st)
	if res.Status != TownWalkPending || res.Done {
		t.Fatalf("moving final arrival = %+v, want pending", res)
	}

	st.Player.Position = world.Position{X: 115, Y: 100}
	st.At = now.Add(200 * time.Millisecond)
	res = walker.TickAct1Waypoint(context.Background(), st)
	if res.Status != TownWalkWaypointVisible || !res.Done {
		t.Fatalf("clickable after movement = %+v, want waypoint_visible", res)
	}
}

func TestTownWalkerFinalRoutePointExhaustsAfterSettle(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.RouteFile = filepath.Join(t.TempDir(), "missing.yaml")
	cfg.TownWalk.Act1WaypointPoints = []world.Position{{X: 110, Y: 100}}
	cfg.TownWalk.ArrivalDistance = 8
	cfg.TownWalk.SettleTimeout = 100 * time.Millisecond
	cfg.Waypoint.MaxClickDistance = 5
	walker := NewTownWalker(config.NewLogger("error"), in, cfg)
	now := time.Now()

	st := townWalkState(world.Position{X: 100, Y: 100})
	st.At = now
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 1, Position: world.Position{X: 150, Y: 100}}}
	_ = walker.TickAct1Waypoint(context.Background(), st)

	st.Player.Position = world.Position{X: 104, Y: 100}
	st.At = now.Add(50 * time.Millisecond)
	_ = walker.TickAct1Waypoint(context.Background(), st)

	st.At = now.Add(200 * time.Millisecond)
	res := walker.TickAct1Waypoint(context.Background(), st)
	if res.Status != TownWalkRouteExhausted || !res.Done {
		t.Fatalf("settled final arrival = %+v, want route_exhausted", res)
	}
}

func TestTownWalkerFailures(t *testing.T) {
	t.Run("wrong area", func(t *testing.T) {
		in := newMockInput()
		walker := NewTownWalker(config.NewLogger("error"), in, DefaultConfig())
		st := townWalkState(world.Position{X: 100, Y: 100})
		st.Area = world.LookupArea(world.BlackMarsh)
		res := walker.TickAct1Waypoint(context.Background(), st)
		if res.Status != TownWalkWrongArea {
			t.Fatalf("res = %+v, want wrong_area", res)
		}
	})

	t.Run("projection failed", func(t *testing.T) {
		in := newMockInput()
		in.unbound = true
		walker := NewTownWalker(config.NewLogger("error"), in, DefaultConfig())
		res := walker.TickAct1Waypoint(context.Background(), townWalkState(world.Position{X: 100, Y: 100}))
		if res.Status != TownWalkProjectionFailed {
			t.Fatalf("res = %+v, want projection_failed", res)
		}
	})

	t.Run("input error", func(t *testing.T) {
		in := newMockInput()
		in.keyErr = fmt.Errorf("key failed")
		walker := NewTownWalker(config.NewLogger("error"), in, DefaultConfig())
		res := walker.TickAct1Waypoint(context.Background(), townWalkState(world.Position{X: 100, Y: 100}))
		if res.Status != TownWalkInputError {
			t.Fatalf("res = %+v, want input_error", res)
		}
	})

	t.Run("stuck", func(t *testing.T) {
		in := newMockInput()
		cfg := DefaultConfig()
		cfg.TownWalk.RouteFile = filepath.Join(t.TempDir(), "missing.yaml")
		cfg.TownWalk.Act1WaypointPoints = []world.Position{{X: 200, Y: 200}, {X: 220, Y: 220}}
		cfg.TownWalk.MoveInterval = time.Second
		cfg.TownWalk.SettleTimeout = 50 * time.Millisecond
		cfg.TownWalk.StuckTimeout = 100 * time.Millisecond
		walker := NewTownWalker(config.NewLogger("error"), in, cfg)
		st := townWalkState(world.Position{X: 100, Y: 100})
		now := time.Now()
		st.At = now
		_ = walker.TickAct1Waypoint(context.Background(), st)
		st.At = now.Add(200 * time.Millisecond)
		res := walker.TickAct1Waypoint(context.Background(), st)
		if res.Status != TownWalkStuck {
			t.Fatalf("res = %+v, want stuck", res)
		}
	})
}

func TestTownWalkerInvalidOverrideFallsBackToPreset(t *testing.T) {
	dir := t.TempDir()
	badRoute := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badRoute, []byte("id: nope\narea_id: 1\npoints: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.RouteFile = badRoute
	walker := NewTownWalker(config.NewLogger("error"), in, cfg)

	res := walker.TickAct1Waypoint(context.Background(), townWalkState(world.Position{X: 3900, Y: 5100}))
	if res.Status != TownWalkPending {
		t.Fatalf("res = %+v, want pending via built-in preset", res)
	}
	if len(in.keys) != 1 {
		t.Fatalf("keys = %v, want preset playback input", in.keys)
	}
}
