package pathing

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func personalStashState(distance uint32) world.State {
	return world.State{
		At: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), Valid: true,
		Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Player:  world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{{Kind: world.ObjectKindPersonalStash, UnitID: 22, Name: "Stash", Position: world.Position{X: 100 + distance, Y: 100}}},
	}
}

func TestPersonalStashActionsRejectsUnsupportedResolution(t *testing.T) {
	in := newMockInput()
	in.window.ClientWidth = 1920
	a := NewPersonalStashActions(config.NewLogger("error"), in, DefaultConfig())
	res := a.Tick(context.Background(), personalStashState(2))
	if res.Status != PersonalStashUnsupportedResolution || !res.Done || len(in.moves) != 0 {
		t.Fatalf("res=%+v moves=%v, want unsupported_resolution without input", res, in.moves)
	}
}

func TestPersonalStashActionsAlreadyOpen(t *testing.T) {
	in := newMockInput()
	a := NewPersonalStashActions(config.NewLogger("error"), in, DefaultConfig())
	st := personalStashState(2)
	st.UI = world.UIState{InventoryOpen: true, StashOpen: true}
	res := a.Tick(context.Background(), st)
	if res.Status != PersonalStashOpened || !res.Done || len(in.moves) != 0 {
		t.Fatalf("res=%+v moves=%v, want opened without input", res, in.moves)
	}
}

func TestPersonalStashActionsInventoryOnlyFailsClosed(t *testing.T) {
	in := newMockInput()
	a := NewPersonalStashActions(config.NewLogger("error"), in, DefaultConfig())
	st := personalStashState(2)
	st.UI.InventoryOpen = true
	res := a.Tick(context.Background(), st)
	if res.Status != PersonalStashOpenFailed || !res.Done || len(in.moves) != 0 {
		t.Fatalf("res=%+v moves=%v, want stash_open_failed without input", res, in.moves)
	}
}

func TestPersonalStashActionsWalksWhenTooFar(t *testing.T) {
	in := newMockInput()
	a := NewPersonalStashActions(config.NewLogger("error"), in, DefaultConfig())
	res := a.Tick(context.Background(), personalStashState(30))
	if res.Status != PersonalStashPending || len(in.moves) != 1 || len(in.keys) != 1 || in.keys[0] != "e" {
		t.Fatalf("res=%+v moves=%v keys=%v, want force-move approach", res, in.moves, in.keys)
	}
	if len(in.clicks) != 0 {
		t.Fatalf("clicks=%v, want no stash click while too far", in.clicks)
	}
}

func TestPersonalStashActionsUsesRelativeDetourAnchors(t *testing.T) {
	in := newMockInput()
	a := NewPersonalStashActions(config.NewLogger("error"), in, DefaultConfig())
	st := personalStashState(30)
	_ = a.Tick(context.Background(), st)
	if a.routeIndex != 0 {
		t.Fatalf("routeIndex=%d, want first detour anchor", a.routeIndex)
	}

	stash := st.Objects[0].Position
	st.Player.Position = world.Position{X: stash.X + 10, Y: stash.Y + 18}
	st.At = st.At.Add(time.Second)
	_ = a.Tick(context.Background(), st)
	if a.routeIndex != 1 {
		t.Fatalf("routeIndex=%d, want second detour anchor", a.routeIndex)
	}
}

func TestPersonalStashActionsClicksOnlyAfterHoverAndConfirmsUI(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := personalStashState(2)
	res := a.Tick(context.Background(), st)
	if res.Status != PersonalStashPending || len(in.moves) != 0 || len(in.clicks) != 0 {
		t.Fatalf("settle tick res=%+v moves=%v clicks=%v, want no input", res, in.moves, in.clicks)
	}
	st.At = st.At.Add(cfg.TownWalk.SettleTimeout)
	res = a.Tick(context.Background(), st)
	if res.Status != PersonalStashPending || len(in.moves) != 1 || len(in.clicks) != 0 {
		t.Fatalf("first hover tick res=%+v moves=%v clicks=%v", res, in.moves, in.clicks)
	}
	st.At = st.At.Add(100 * time.Millisecond)
	st.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 22}
	res = a.Tick(context.Background(), st)
	if res.Status != PersonalStashPending || len(in.clicks) != 1 || in.clicks[0] != input.MouseLeft {
		t.Fatalf("hover tick res=%+v clicks=%v, want one left click", res, in.clicks)
	}
	st.UI = world.UIState{InventoryOpen: true, StashOpen: true}
	res = a.Tick(context.Background(), st)
	if res.Status != PersonalStashOpened || !res.Done {
		t.Fatalf("UI confirmation res=%+v, want opened", res)
	}
}

func TestPersonalStashActionsOpenTimeout(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := personalStashState(2)
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(cfg.TownWalk.SettleTimeout)
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(100 * time.Millisecond)
	st.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 22}
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(personalStashOpenTimeout)
	res := a.Tick(context.Background(), st)
	if res.Status != PersonalStashOpenFailed || !res.Done {
		t.Fatalf("res=%+v, want stash_open_failed", res)
	}
}

func TestPersonalStashActionsWaitsForApproachToStopBeforeClicking(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := personalStashState(30)

	_ = a.Tick(context.Background(), st)
	if len(in.keys) != 1 {
		t.Fatalf("keys=%v, want initial force-move input", in.keys)
	}

	st.At = st.At.Add(200 * time.Millisecond)
	st.Player.Position = world.Position{X: 115, Y: 100}
	_ = a.Tick(context.Background(), st)
	if len(in.moves) != 1 || len(in.clicks) != 0 {
		t.Fatalf("moves=%v clicks=%v, want settle without hover input", in.moves, in.clicks)
	}

	st.At = st.At.Add(cfg.TownWalk.SettleTimeout)
	st.Player.Position = world.Position{X: 116, Y: 100}
	_ = a.Tick(context.Background(), st)
	if len(in.moves) != 1 || len(in.clicks) != 0 {
		t.Fatalf("moves=%v clicks=%v, want movement to restart settle period", in.moves, in.clicks)
	}

	st.At = st.At.Add(cfg.TownWalk.SettleTimeout - time.Millisecond)
	_ = a.Tick(context.Background(), st)
	if len(in.moves) != 1 {
		t.Fatalf("moves=%v, want no hover input before full settle period", in.moves)
	}

	st.At = st.At.Add(time.Millisecond)
	_ = a.Tick(context.Background(), st)
	if len(in.moves) != 2 || len(in.clicks) != 0 {
		t.Fatalf("moves=%v clicks=%v, want hover input after confirmed stop", in.moves, in.clicks)
	}
}

func stashApproachConfig() Config {
	cfg := DefaultConfig()
	cfg.TownWalk.StuckTimeout = 100 * time.Millisecond
	cfg.TownWalk.MoveInterval = 10 * time.Millisecond
	return cfg
}

func farStashFromPortal() world.State {
	st := personalStashState(50)
	st.Objects = append(st.Objects, world.Object{
		Kind: world.ObjectKindTownPortal, UnitID: 9, Name: "Town Portal", Position: world.Position{X: 100, Y: 100},
	})
	return st
}

func TestPersonalStashActionsReturnsToOriginAfterFirstStuck(t *testing.T) {
	in := newMockInput()
	cfg := stashApproachConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := farStashFromPortal()

	if res := a.Tick(context.Background(), st); res.Done || res.Status != PersonalStashPending {
		t.Fatalf("start res=%+v, want pending approach", res)
	}

	st.At = st.At.Add(20 * time.Millisecond)
	st.Player.Position = world.Position{X: 120, Y: 100}
	if res := a.Tick(context.Background(), st); res.Done {
		t.Fatalf("progress res=%+v, want still approaching", res)
	}

	st.At = st.At.Add(cfg.TownWalk.StuckTimeout)
	movesBeforeRetreat := len(in.moves)
	res := a.Tick(context.Background(), st)
	if res.Done || res.Status != PersonalStashPending {
		t.Fatalf("first stuck res=%+v, want local return to origin", res)
	}
	if len(in.moves) <= movesBeforeRetreat {
		t.Fatalf("moves=%d, want retreat force-move after first stuck", len(in.moves))
	}
}

func TestPersonalStashActionsRetriesApproachAfterReturningToOrigin(t *testing.T) {
	in := newMockInput()
	cfg := stashApproachConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := farStashFromPortal()

	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(20 * time.Millisecond)
	st.Player.Position = world.Position{X: 120, Y: 100}
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(cfg.TownWalk.StuckTimeout)
	if res := a.Tick(context.Background(), st); res.Done {
		t.Fatalf("first stuck res=%+v, want local origin return", res)
	}

	st.At = st.At.Add(cfg.TownWalk.MoveInterval)
	st.Player.Position = world.Position{X: 100, Y: 100}
	res := a.Tick(context.Background(), st)
	if res.Done || res.Status != PersonalStashPending {
		t.Fatalf("origin arrival res=%+v, want retry from portal", res)
	}
	if a.routeIndex != 0 {
		t.Fatalf("routeIndex=%d, want detour restart after origin return", a.routeIndex)
	}
}

func TestPersonalStashActionsFailsAfterSecondApproachStuck(t *testing.T) {
	in := newMockInput()
	cfg := stashApproachConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := farStashFromPortal()

	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(20 * time.Millisecond)
	st.Player.Position = world.Position{X: 120, Y: 100}
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(cfg.TownWalk.StuckTimeout)
	if res := a.Tick(context.Background(), st); res.Done {
		t.Fatalf("first stuck res=%+v, want local origin return", res)
	}

	st.At = st.At.Add(cfg.TownWalk.MoveInterval)
	st.Player.Position = world.Position{X: 100, Y: 100}
	_ = a.Tick(context.Background(), st)

	st.At = st.At.Add(cfg.TownWalk.MoveInterval)
	st.Player.Position = world.Position{X: 120, Y: 100}
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(cfg.TownWalk.StuckTimeout)
	res := a.Tick(context.Background(), st)
	if res.Status != PersonalStashApproachFailed || !res.Done || res.Reason != "stuck" {
		t.Fatalf("second stuck res=%+v, want stash_approach_failed", res)
	}
}

func TestPersonalStashActionsFailsWhenOriginReturnIsAlsoStuck(t *testing.T) {
	in := newMockInput()
	cfg := stashApproachConfig()
	a := NewPersonalStashActions(config.NewLogger("error"), in, cfg)
	st := farStashFromPortal()

	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(20 * time.Millisecond)
	st.Player.Position = world.Position{X: 120, Y: 100}
	_ = a.Tick(context.Background(), st)
	st.At = st.At.Add(cfg.TownWalk.StuckTimeout)
	if res := a.Tick(context.Background(), st); res.Done {
		t.Fatalf("first stuck res=%+v, want retreat", res)
	}

	st.At = st.At.Add(cfg.TownWalk.StuckTimeout)
	res := a.Tick(context.Background(), st)
	if res.Status != PersonalStashApproachFailed || !res.Done || res.Reason != "stuck" {
		t.Fatalf("retreat stuck res=%+v, want stash_approach_failed", res)
	}
}
