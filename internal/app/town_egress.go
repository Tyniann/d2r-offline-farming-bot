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
	if err := pathing.MatchSystemEgressLayout(fingerprint, bound.Version, bound.AreaID, bound.AnchorCount, bound.Hash, bound.Anchors); err != nil {
		return fmt.Errorf("system egress layout differs: %w", err)
	}
	// D2R does not place the character on the first recorded walk sample. The
	// Memory-visible Town Portal is the authoritative portal_arrival proof; the
	// walker closes the remaining gap to Points[0], matching Act-1 preparation.
	tolerance := route.Contract.ArrivalToleranceTiles
	if tolerance <= 0 {
		tolerance = a.pathCfg.TownPortal.MaxClickDistance
	}
	if !systemEgressRecordingStartReady(state, expectedArea, tolerance) {
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
