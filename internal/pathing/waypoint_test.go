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

func waypointMenuState() world.State {
	return world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), UI: world.UIState{WaypointOpen: true}}
}

func TestDefaultWaypointTargetRegistryGeometryCandidates(t *testing.T) {
	registry := DefaultWaypointTargetRegistry()
	want := map[WaypointTargetID]WaypointTargetAction{
		WaypointTargetBlackMarsh:          {Act: 1, TabX: 159, TabY: 148, RowX: 200, RowY: 342, ExpectedAreaID: world.BlackMarsh},
		WaypointTargetDuranceOfHateLevel2: {Act: 3, TabX: 273, TabY: 148, RowX: 200, RowY: 506, ExpectedAreaID: world.DuranceOfHateLevel2},
		WaypointTargetArcaneSanctuary:     {Act: 2, TabX: 216, TabY: 148, RowX: 200, RowY: 465, ExpectedAreaID: world.ArcaneSanctuary},
		WaypointTargetHallsOfPain:         {Act: 5, TabX: 387, TabY: 148, RowX: 200, RowY: 383, ExpectedAreaID: world.HallsOfPain},
		WaypointTargetRogueEncampment:     {Act: 1, TabX: 159, TabY: 148, RowX: 200, RowY: 178, ExpectedAreaID: world.RogueEncampment},
	}
	if len(registry.Actions()) != len(want) {
		t.Fatalf("Actions = %d, want %d", len(registry.Actions()), len(want))
	}
	for id, expected := range want {
		action, ok := registry.Action(id)
		if !ok || action.Act != expected.Act || action.TabX != expected.TabX || action.TabY != expected.TabY || action.RowX != expected.RowX || action.RowY != expected.RowY || action.ExpectedAreaID != expected.ExpectedAreaID || action.ClientWidth != 1280 || action.ClientHeight != 720 || action.SettleMs != 200 {
			t.Fatalf("Action(%s) = %+v, want geometry candidate %+v", id, action, expected)
		}
	}
}

func TestWaypointActionsSelectTargetTabSettleAndRow(t *testing.T) {
	in := newMockInput()
	actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
	now := time.Now()
	state := waypointMenuState()

	res := actions.SelectWaypointTarget(context.Background(), state, WaypointTargetDuranceOfHateLevel2, now)
	if res.Status != WaypointActionPending || len(in.moves) != 1 || in.moves[0] != [2]int{273, 148} || len(in.clicks) != 1 {
		t.Fatalf("tab tick = %+v moves=%v clicks=%v", res, in.moves, in.clicks)
	}
	res = actions.SelectWaypointTarget(context.Background(), state, WaypointTargetDuranceOfHateLevel2, now.Add(199*time.Millisecond))
	if res.Status != WaypointActionPending || len(in.clicks) != 1 {
		t.Fatalf("settle tick = %+v clicks=%v, want no input", res, in.clicks)
	}
	res = actions.SelectWaypointTarget(context.Background(), state, WaypointTargetDuranceOfHateLevel2, now.Add(200*time.Millisecond))
	if res.Status != WaypointActionClicked || !res.Done || len(in.moves) != 2 || in.moves[1] != [2]int{200, 506} || len(in.clicks) != 2 {
		t.Fatalf("row tick = %+v moves=%v clicks=%v", res, in.moves, in.clicks)
	}
	res = actions.SelectWaypointTarget(context.Background(), state, WaypointTargetDuranceOfHateLevel2, now.Add(time.Second))
	if res.Status != WaypointActionClicked || len(in.clicks) != 2 {
		t.Fatalf("completed tick = %+v clicks=%v, want no repeated click", res, in.clicks)
	}
}

func TestWaypointActionsSelectTargetFailsClosed(t *testing.T) {
	t.Run("menu unconfirmed", func(t *testing.T) {
		in := newMockInput()
		actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
		res := actions.SelectWaypointTarget(context.Background(), world.State{Valid: true, Phase: world.GamePhaseInGame}, WaypointTargetBlackMarsh, time.Now())
		if res.Status != WaypointActionUIUnconfirmed || len(in.clicks) != 0 {
			t.Fatalf("res=%+v clicks=%v", res, in.clicks)
		}
	})
	t.Run("unsupported resolution", func(t *testing.T) {
		in := newMockInput()
		in.window.ClientWidth = 1920
		actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
		res := actions.SelectWaypointTarget(context.Background(), waypointMenuState(), WaypointTargetBlackMarsh, time.Now())
		if res.Status != WaypointActionUnsupportedResolution || len(in.clicks) != 0 {
			t.Fatalf("res=%+v clicks=%v", res, in.clicks)
		}
	})
	t.Run("unknown target", func(t *testing.T) {
		in := newMockInput()
		actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
		res := actions.SelectWaypointTarget(context.Background(), waypointMenuState(), WaypointTargetID("unknown"), time.Now())
		if res.Status != WaypointActionTargetUnsupported || len(in.clicks) != 0 {
			t.Fatalf("res=%+v clicks=%v", res, in.clicks)
		}
	})
}

func TestWaypointActionsSelectionResetRestartsAtActTab(t *testing.T) {
	in := newMockInput()
	actions := NewWaypointActions(config.NewLogger("error"), in, DefaultConfig())
	now := time.Now()
	_ = actions.SelectWaypointTarget(context.Background(), waypointMenuState(), WaypointTargetBlackMarsh, now)
	actions.Reset()
	res := actions.SelectWaypointTarget(context.Background(), waypointMenuState(), WaypointTargetBlackMarsh, now.Add(time.Second))
	if res.Status != WaypointActionPending || len(in.moves) != 2 || in.moves[1] != [2]int{159, 148} || len(in.clicks) != 2 {
		t.Fatalf("res=%+v moves=%v clicks=%v", res, in.moves, in.clicks)
	}
}
