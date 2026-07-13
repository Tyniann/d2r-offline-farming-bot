package pathing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func townWalkState(pos world.Position) world.State {
	return world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), Player: world.Player{Position: pos}}
}

func TestTownRouteWalkerUsesForceMoveKey(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	walker := NewTownRouteWalker(config.NewLogger("error"), in, cfg, []world.Position{{X: 110, Y: 110}, {X: 120, Y: 120}})
	res := walker.TickRoute(context.Background(), townWalkState(world.Position{X: 100, Y: 100}))
	if res.Status != TownWalkPending || res.Done || len(in.moves) != 1 || len(in.keys) != 1 || in.keys[0] != "e" {
		t.Fatalf("res=%+v moves=%v keys=%v", res, in.moves, in.keys)
	}
}

func TestTownRouteWalkerArrivesWithoutWaypoint(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.SettleTimeout = 100 * time.Millisecond
	walker := NewTownRouteWalker(config.NewLogger("error"), in, cfg, []world.Position{{X: 100, Y: 100}, {X: 110, Y: 100}})
	now := time.Now()
	state := townWalkState(world.Position{X: 100, Y: 100})
	state.At = now
	if result := walker.TickRoute(context.Background(), state); result.Done {
		t.Fatalf("first tick = %+v", result)
	}
	state.Player.Position = world.Position{X: 110, Y: 100}
	state.At = now.Add(10 * time.Millisecond)
	if result := walker.TickRoute(context.Background(), state); result.Done {
		t.Fatalf("settle start = %+v", result)
	}
	state.At = now.Add(200 * time.Millisecond)
	if result := walker.TickRoute(context.Background(), state); !result.Done || result.Status != TownWalkArrived {
		t.Fatalf("result = %+v", result)
	}
}

func TestTownRouteWalkerFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*mockInput, *Config) (world.State, []world.Position)
		want  TownWalkStatus
	}{
		{"missing route", func(_ *mockInput, _ *Config) (world.State, []world.Position) {
			return townWalkState(world.Position{X: 100, Y: 100}), nil
		}, TownWalkRouteMissing},
		{"wrong area", func(_ *mockInput, _ *Config) (world.State, []world.Position) {
			s := townWalkState(world.Position{X: 100, Y: 100})
			s.Area = world.LookupArea(world.BlackMarsh)
			return s, []world.Position{{X: 110, Y: 110}, {X: 120, Y: 120}}
		}, TownWalkWrongArea},
		{"projection", func(in *mockInput, _ *Config) (world.State, []world.Position) {
			in.unbound = true
			return townWalkState(world.Position{X: 100, Y: 100}), []world.Position{{X: 110, Y: 110}, {X: 120, Y: 120}}
		}, TownWalkProjectionFailed},
		{"input", func(in *mockInput, _ *Config) (world.State, []world.Position) {
			in.keyErr = fmt.Errorf("key failed")
			return townWalkState(world.Position{X: 100, Y: 100}), []world.Position{{X: 110, Y: 110}, {X: 120, Y: 120}}
		}, TownWalkInputError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := newMockInput()
			cfg := DefaultConfig()
			state, points := tc.setup(in, &cfg)
			walker := NewTownRouteWalker(config.NewLogger("error"), in, cfg, points)
			res := walker.TickRoute(context.Background(), state)
			if res.Status != tc.want || (tc.want != TownWalkPending && !res.Done) {
				t.Fatalf("res=%+v want=%s", res, tc.want)
			}
		})
	}
}

func TestTownRouteWalkerStuck(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	cfg.TownWalk.MoveInterval = time.Second
	cfg.TownWalk.SettleTimeout = 50 * time.Millisecond
	cfg.TownWalk.StuckTimeout = 100 * time.Millisecond
	walker := NewTownRouteWalker(config.NewLogger("error"), in, cfg, []world.Position{{X: 200, Y: 200}, {X: 220, Y: 220}})
	st := townWalkState(world.Position{X: 100, Y: 100})
	now := time.Now()
	st.At = now
	_ = walker.TickRoute(context.Background(), st)
	st.At = now.Add(200 * time.Millisecond)
	if res := walker.TickRoute(context.Background(), st); res.Status != TownWalkStuck {
		t.Fatalf("res=%+v", res)
	}
}
