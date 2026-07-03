package pathing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func waypointState() world.State {
	return world.State{
		At:    time.Now(),
		Valid: true,
		Area:  world.LookupArea(world.RogueEncampment),
		Player: world.Player{
			Position: world.Position{X: 100, Y: 100},
		},
		Objects: []world.Object{
			{Kind: world.ObjectKindWaypoint, UnitID: 7, Position: world.Position{X: 105, Y: 105}, Name: "Waypoint"},
		},
	}
}

func TestWaypointActionsTickTownWaypointPendingThenClicked(t *testing.T) {
	in := newMockInput()
	actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
	st := waypointState()

	res := actions.TickTownWaypoint(context.Background(), st)
	if res.Status != WaypointActionPending || len(in.moves) != 1 {
		t.Fatalf("first tick = %+v moves=%v, want pending with move", res, in.moves)
	}

	st.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 7}
	res = actions.TickTownWaypoint(context.Background(), st)
	if res.Status != WaypointActionClicked || !res.Done {
		t.Fatalf("second tick = %+v, want clicked done", res)
	}
	if len(in.clicks) != 1 || in.clicks[0] != input.MouseLeft {
		t.Fatalf("clicks = %v, want [left]", in.clicks)
	}
}

func TestWaypointActionsTickTownWaypointFailures(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		in := newMockInput()
		actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
		res := actions.TickTownWaypoint(context.Background(), world.State{Valid: true})
		if res.Status != WaypointActionNotFound || !res.Done {
			t.Fatalf("res = %+v, want not_found", res)
		}
	})

	t.Run("too far", func(t *testing.T) {
		in := newMockInput()
		cfg := DefaultConfig()
		cfg.Waypoint.MaxClickDistance = 1
		actions := NewWaypointActions(config.NewLogger("error"), in, cfg)
		res := actions.TickTownWaypoint(context.Background(), waypointState())
		if res.Status != WaypointActionTooFar || !res.Done {
			t.Fatalf("res = %+v, want too_far", res)
		}
	})

	t.Run("projection failed", func(t *testing.T) {
		in := newMockInput()
		in.unbound = true
		actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
		res := actions.TickTownWaypoint(context.Background(), waypointState())
		if res.Status != WaypointActionProjectionFailed || !res.Done {
			t.Fatalf("res = %+v, want projection_failed", res)
		}
	})

	t.Run("input error", func(t *testing.T) {
		in := newMockInput()
		in.moveErr = fmt.Errorf("move failed")
		actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
		res := actions.TickTownWaypoint(context.Background(), waypointState())
		if res.Status != WaypointActionInputError || !res.Done {
			t.Fatalf("res = %+v, want input_error", res)
		}
	})

	t.Run("hover not found", func(t *testing.T) {
		in := newMockInput()
		cfg := DefaultConfig()
		cfg.Click.MaxHoverAttempts = 1
		actions := NewWaypointActions(config.NewLogger("error"), in, cfg)
		_ = actions.TickTownWaypoint(context.Background(), waypointState())
		res := actions.TickTownWaypoint(context.Background(), waypointState())
		if res.Status != WaypointActionHoverNotFound || !res.Done {
			t.Fatalf("res = %+v, want hover_not_found", res)
		}
	})
}

func TestWaypointActionsResetClearsHoverState(t *testing.T) {
	in := newMockInput()
	actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
	st := waypointState()

	_ = actions.TickTownWaypoint(context.Background(), st)
	actions.Reset()
	st.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 7}
	res := actions.TickTownWaypoint(context.Background(), st)
	if res.Status != WaypointActionPending {
		t.Fatalf("res = %+v, want pending after reset", res)
	}
	if len(in.clicks) != 0 {
		t.Fatalf("clicks = %v, want none after reset", in.clicks)
	}
}

func TestWaypointActionsSelectBlackMarsh(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.WaypointUI.BlackMarshX = 201
	cfg.WaypointUI.BlackMarshY = 343
	actions := NewWaypointActions(config.NewLogger("error"), in, cfg)

	res := actions.SelectBlackMarsh(context.Background())
	if res.Status != WaypointActionClicked || !res.Done {
		t.Fatalf("res = %+v, want clicked", res)
	}
	if len(in.moves) != 1 || in.moves[0] != [2]int{201, 343} {
		t.Fatalf("moves = %v, want [201 343]", in.moves)
	}
	if len(in.clicks) != 1 || in.clicks[0] != input.MouseLeft {
		t.Fatalf("clicks = %v, want [left]", in.clicks)
	}
}
