package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// townEgressAdapter resolves one configured foreign-Town route and delegates
// its binding, start-nearness, drift, and timeout gates to the generic player.
type townEgressAdapter struct {
	log         *slog.Logger
	config      *config.Config
	gameVersion string
	driver      pathing.InputDriver
	pathCfg     pathing.Config
	walker      *pathing.TownWalker
	activeAct   town.OriginAct
}

func newTownEgressAdapter(log *slog.Logger, cfg *config.Config, gameVersion string, driver pathing.InputDriver, pathCfg pathing.Config, _ *telemetry.Recorder) *townEgressAdapter {
	return &townEgressAdapter{log: log.With("component", "town_egress"), config: cfg, gameVersion: gameVersion, driver: driver, pathCfg: pathCfg}
}

func (a *townEgressAdapter) setTelemetry(_ *telemetry.Recorder) {}

func (a *townEgressAdapter) Start(act town.OriginAct, state world.State) error {
	if a == nil || a.config == nil || a.driver == nil {
		return fmt.Errorf("%w: egress adapter unavailable", pathing.ErrRouteNotFound)
	}
	egress, reason := a.config.Town.EgressFor(act)
	if reason != "" || act != town.OriginAct3 || state.Area.ID != world.KurastDocks || egress.Area != "kurast_docks" {
		return fmt.Errorf("%w: act=%s area=%s", pathing.ErrRouteStartMismatch, act, state.Area.ID)
	}
	registry, err := pathing.LoadRouteRegistry(a.config.ResolvePath(egress.RoutesDirectory))
	if err != nil {
		return fmt.Errorf("town egress %s registry: %w", act, err)
	}
	route, err := registry.Get(egress.RouteID)
	if err != nil {
		return fmt.Errorf("town egress %s: %w", act, err)
	}
	if validationErr := validateAct3EgressRoute(route); validationErr != nil {
		return fmt.Errorf("town egress %s: %w", act, validationErr)
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		return fmt.Errorf("town egress %s layout: %w", act, err)
	}
	if err := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{Identity: state.Identity, GameVersion: a.gameVersion, Layout: fingerprint, World: state}); err != nil {
		return fmt.Errorf("town egress %s: %w", act, err)
	}
	points := make([]world.Position, 0, len(route.Segments[0].Points))
	for _, point := range route.Segments[0].Points {
		points = append(points, world.Position{X: point.X, Y: point.Y})
	}
	a.walker = pathing.NewAreaTownRouteWalker(a.log, a.driver, a.pathCfg, world.KurastDocks, points)
	a.activeAct = act
	a.log.Info("foreign town egress started", "act", act, "route_id", egress.RouteID, "area_id", state.Area.ID)
	return nil
}

func (a *townEgressAdapter) Tick(ctx context.Context, state world.State) (bool, error) {
	if a == nil || a.walker == nil || a.activeAct == town.OriginActUnknown {
		return false, fmt.Errorf("town egress not started")
	}
	result := a.walker.TickRoute(ctx, state)
	if !result.Done {
		return false, nil
	}
	if result.Status != pathing.TownWalkArrived {
		return false, fmt.Errorf("town egress %s: status=%s reason=%s", a.activeAct, result.Status, result.Reason)
	}
	if result.Done {
		a.log.Info("foreign town egress completed", "act", a.activeAct, "area_id", state.Area.ID)
	}
	return true, nil
}

func (a *townEgressAdapter) Reset() {
	if a == nil {
		return
	}
	if a.walker != nil {
		a.walker.Reset()
	}
	a.walker = nil
	a.activeAct = town.OriginActUnknown
}

func validateAct3EgressRoute(route pathing.Route) error {
	if len(route.Segments) != 1 {
		return fmt.Errorf("%w: Act-3 egress requires exactly one segment", pathing.ErrRouteStartMismatch)
	}
	segment := route.Segments[0]
	if segment.FromAreaID != world.KurastDocks || segment.ToAreaID != world.KurastDocks || segment.Movement != pathing.RouteMovementWalk || segment.Transition.Type != "terminal" {
		return fmt.Errorf("%w: Act-3 egress requires one terminal Kurast-Docks walk segment", pathing.ErrRouteStartMismatch)
	}
	return nil
}
