package pathing

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func townPortalState() world.State {
	return world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.TowerCellarLevel5),
		Player: world.Player{
			Position: world.Position{X: 100, Y: 100},
		},
		Objects: []world.Object{{
			Kind:     world.ObjectKindTownPortal,
			ID:       world.TownPortalID,
			UnitID:   77,
			Position: world.Position{X: 102, Y: 102},
			Name:     "Town Portal",
		}},
	}
}

func TestTownPortalActionsWaitsForGeneratedPortalThenHoverClicks(t *testing.T) {
	in := newMockInput()
	actions := NewTownPortalActions(config.NewLogger("error"), in, DefaultConfig())
	now := time.Now()

	res := actions.Tick(context.Background(), world.State{Valid: true}, now)
	if res.Status != TownPortalActionPending {
		t.Fatalf("missing tick = %+v, want pending", res)
	}
	st := townPortalState()
	res = actions.Tick(context.Background(), st, now.Add(time.Millisecond))
	if res.Status != TownPortalActionPending || len(in.moves) != 0 {
		t.Fatalf("discovery tick = %+v moves=%v, want activation wait without input", res, in.moves)
	}
	res = actions.Tick(context.Background(), st, now.Add(time.Millisecond+townPortalActivationSettle))
	if res.Status != TownPortalActionPending || len(in.moves) != 1 {
		t.Fatalf("probe tick = %+v moves=%v, want pending with hover move", res, in.moves)
	}
	st.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 77}
	res = actions.Tick(context.Background(), st, now.Add(2*time.Millisecond+townPortalActivationSettle))
	if res.Status != TownPortalActionClicked || !res.Done {
		t.Fatalf("hover tick = %+v, want clicked", res)
	}
	if len(in.clicks) != 1 || in.clicks[0] != input.MouseLeft {
		t.Fatalf("clicks = %v, want [left]", in.clicks)
	}
}

func TestTownPortalActionsFailsClosed(t *testing.T) {
	t.Run("not found after timeout", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TownPortal.AppearTimeout = time.Second
		actions := NewTownPortalActions(config.NewLogger("error"), newMockInput(), cfg)
		now := time.Now()
		_ = actions.Tick(context.Background(), world.State{Valid: true}, now)
		res := actions.Tick(context.Background(), world.State{Valid: true}, now.Add(time.Second))
		if res.Status != TownPortalActionNotFound || !res.Done {
			t.Fatalf("tick = %+v, want not_found", res)
		}
	})

	t.Run("too far", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.TownPortal.MaxClickDistance = 1
		res := NewTownPortalActions(config.NewLogger("error"), newMockInput(), cfg).
			Tick(context.Background(), townPortalState(), time.Now())
		if res.Status != TownPortalActionTooFar || !res.Done {
			t.Fatalf("tick = %+v, want too_far", res)
		}
	})

	t.Run("hover not found", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Click.MaxHoverAttempts = 1
		actions := NewTownPortalActions(config.NewLogger("error"), newMockInput(), cfg)
		now := time.Now()
		_ = actions.Tick(context.Background(), townPortalState(), now)
		_ = actions.Tick(context.Background(), townPortalState(), now.Add(townPortalActivationSettle))
		res := actions.Tick(context.Background(), townPortalState(), now.Add(townPortalActivationSettle+time.Millisecond))
		if res.Status != TownPortalActionHoverNotFound || !res.Done {
			t.Fatalf("tick = %+v, want hover_not_found", res)
		}
	})
}

func TestTownPortalActionsRestartsActivationSettleWhenPortalMoves(t *testing.T) {
	in := newMockInput()
	actions := NewTownPortalActions(config.NewLogger("error"), in, DefaultConfig())
	now := time.Now()
	st := townPortalState()

	_ = actions.Tick(context.Background(), st, now)
	st.Objects[0].Position.X++
	_ = actions.Tick(context.Background(), st, now.Add(townPortalActivationSettle))
	if len(in.moves) != 0 {
		t.Fatalf("moves=%v, want moved portal to restart activation wait", in.moves)
	}

	_ = actions.Tick(context.Background(), st, now.Add(2*townPortalActivationSettle-time.Millisecond))
	if len(in.moves) != 0 {
		t.Fatalf("moves=%v, want no hover input before restarted wait completes", in.moves)
	}

	_ = actions.Tick(context.Background(), st, now.Add(2*townPortalActivationSettle))
	if len(in.moves) != 1 {
		t.Fatalf("moves=%v, want hover input after stable portal activation", in.moves)
	}
}
