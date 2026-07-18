package app

import (
	"context"
	"errors"
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
	expectedArea, supported := town.TownAreaForAct(act)
	if reason != "" || !supported || state.Area.ID != expectedArea {
		return fmt.Errorf("%w: act=%s area=%s", pathing.ErrRouteStartMismatch, act, state.Area.ID)
	}
	path := a.config.ResolvePath(egress.RoutesDirectory + "/" + town.SystemEgressFilename)
	route, err := town.LoadSystemEgressRoute(path)
	if err != nil {
		if errors.Is(err, pathing.ErrRouteNotFound) || errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("%w: town egress %s: %v", pathing.ErrRouteNotFound, act, err)
	}
	if route.Contract.Act != act || route.Contract.TownArea != expectedArea {
		return fmt.Errorf("%w: system egress contract act/area mismatch", pathing.ErrRouteStartMismatch)
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		return fmt.Errorf("town egress %s layout: %w", act, err)
	}
	if route.Contract.GameVersion != a.gameVersion {
		return fmt.Errorf("%w: got %s want %s", pathing.ErrRouteGameVersionMismatch, a.gameVersion, route.Contract.GameVersion)
	}
	bound := route.Contract.LayoutFingerprint
	if fingerprint.Version != bound.Version || fingerprint.AreaID != bound.AreaID || fingerprint.AnchorCount != bound.AnchorCount || fingerprint.Hash != bound.Hash {
		return fmt.Errorf("%w: system egress layout differs", pathing.ErrRouteLayoutMismatch)
	}
	if world.Distance(state.Player.Position, world.Position{X: route.Points[0].X, Y: route.Points[0].Y}) > route.Contract.ArrivalToleranceTiles {
		return fmt.Errorf("%w: portal arrival is outside route start tolerance", pathing.ErrRouteStartMismatch)
	}
	points := make([]world.Position, 0, len(route.Points))
	for _, point := range route.Points {
		points = append(points, world.Position{X: point.X, Y: point.Y})
	}
	a.walker = pathing.NewAreaTownRouteWalker(a.log, a.driver, a.pathCfg, expectedArea, points)
	a.activeAct = act
	a.log.Info("foreign town egress started", "act", act, "route_file", town.SystemEgressFilename, "area_id", state.Area.ID)
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
