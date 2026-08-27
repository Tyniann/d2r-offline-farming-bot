package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// townEgressAdapter resolves one configured foreign-Town route and delegates
// its binding, start-nearness, drift, and timeout gates to the generic player.
type townEgressAdapter struct {
	log           *slog.Logger
	config        *config.Config
	gameVersion   string
	driver        pathing.InputDriver
	pathCfg       pathing.Config
	walker        *pathing.TownWalker
	activeAct     town.OriginAct
	activeRoute   town.SystemEgressRoute
	startAnchor   town.Anchor
	layoutMatched bool
}

type systemEgressStartProof interface {
	Validate(world.State, world.AreaID, town.SystemEgressRoute) error
}

type portalArrivalStartProof struct{}

func (portalArrivalStartProof) Validate(state world.State, expectedArea world.AreaID, route town.SystemEgressRoute) error {
	if !systemEgressRecordingStartReady(state, expectedArea, route.Contract.ArrivalToleranceTiles, town.AnchorPortalArrival) {
		return fmt.Errorf("%w: portal arrival is outside route start tolerance", pathing.ErrRouteStartMismatch)
	}
	return nil
}

type spawnStartProof struct{}

func (spawnStartProof) Validate(state world.State, expectedArea world.AreaID, route town.SystemEgressRoute) error {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != expectedArea || len(route.Points) == 0 {
		return fmt.Errorf("%w: spawn context is unavailable", pathing.ErrRouteStartMismatch)
	}
	if !state.Identity.Valid {
		// Identity confirmation needs three fresh Memory ticks. Isolated playback
		// may reach this proof one tick earlier and must wait without masking real
		// area or position mismatches as transient.
		return fmt.Errorf("%w: spawn identity is not confirmed", pathing.ErrGameIdentityUnavailable)
	}
	start := world.Position{X: route.Points[0].X, Y: route.Points[0].Y}
	if route.Contract.ArrivalToleranceTiles <= 0 || world.Distance(state.Player.Position, start) > route.Contract.ArrivalToleranceTiles {
		return fmt.Errorf("%w: spawn is outside route start tolerance", pathing.ErrRouteStartMismatch)
	}
	return nil
}

func systemEgressStartProofFor(anchor town.Anchor) (systemEgressStartProof, error) {
	switch anchor {
	case town.AnchorPortalArrival:
		return portalArrivalStartProof{}, nil
	case town.AnchorSpawn:
		return spawnStartProof{}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported start anchor %q", pathing.ErrRouteStartMismatch, anchor)
	}
}

func newTownEgressAdapter(log *slog.Logger, cfg *config.Config, gameVersion string, driver pathing.InputDriver, pathCfg pathing.Config, _ *telemetry.Recorder) *townEgressAdapter {
	return &townEgressAdapter{log: log.With("component", "town_egress"), config: cfg, gameVersion: gameVersion, driver: driver, pathCfg: pathCfg}
}

func (a *townEgressAdapter) setTelemetry(_ *telemetry.Recorder) {}

func (a *townEgressAdapter) Start(act town.OriginAct, state world.State) error {
	return a.StartFrom(act, town.AnchorPortalArrival, state)
}

// StartFrom starts the shared system-Egress walker after the selected start
// proof validates its own portal-arrival or spawn contract.
func (a *townEgressAdapter) StartFrom(act town.OriginAct, startAnchor town.Anchor, state world.State) error {
	if a == nil || a.config == nil || a.driver == nil {
		return fmt.Errorf("%w: egress adapter unavailable", pathing.ErrRouteNotFound)
	}
	egress, reason := a.config.Town.EgressFor(act)
	expectedArea, supported := town.TownAreaForAct(act)
	if reason != "" || !supported || state.Area.ID != expectedArea {
		return fmt.Errorf("%w: act=%s area=%s", pathing.ErrRouteStartMismatch, act, state.Area.ID)
	}
	filename, err := town.SystemEgressFilenameForAnchor(startAnchor)
	if err != nil {
		return fmt.Errorf("%w: %v", pathing.ErrRouteStartMismatch, err)
	}
	proof, err := systemEgressStartProofFor(startAnchor)
	if err != nil {
		return err
	}
	path := a.config.ResolvePath(egress.RoutesDirectory + "/" + filename)
	route, err := town.LoadSystemEgressRoute(path)
	if err != nil {
		if errors.Is(err, pathing.ErrRouteNotFound) || errors.Is(err, context.Canceled) {
			return err
		}
		return fmt.Errorf("%w: town egress %s: %v", pathing.ErrRouteNotFound, act, err)
	}
	if route.Contract.Act != act || route.Contract.TownArea != expectedArea || route.Contract.From != startAnchor {
		return fmt.Errorf("%w: system egress contract act/area mismatch", pathing.ErrRouteStartMismatch)
	}
	if route.Contract.GameVersion != a.gameVersion {
		return fmt.Errorf("%w: got %s want %s", pathing.ErrRouteGameVersionMismatch, a.gameVersion, route.Contract.GameVersion)
	}
	if err := proof.Validate(state, expectedArea, route); err != nil {
		return err
	}
	layoutMatched := false
	fingerprint, layoutErr := pathing.BuildLayoutFingerprint(state)
	if layoutErr == nil {
		if err := matchSystemEgressRouteLayout(fingerprint, route); err != nil {
			return err
		}
		layoutMatched = true
	} else if startAnchor != town.AnchorSpawn || !errors.Is(layoutErr, pathing.ErrLayoutAnchorsUnavailable) {
		return fmt.Errorf("town egress %s layout: %w", act, layoutErr)
	}
	points := make([]world.Position, 0, len(route.Points))
	for _, point := range route.Points {
		points = append(points, world.Position{X: point.X, Y: point.Y})
	}
	a.walker = pathing.NewAreaTownRouteWalker(a.log, a.driver, a.pathCfg, expectedArea, points)
	a.activeAct = act
	a.activeRoute = route
	a.startAnchor = startAnchor
	a.layoutMatched = layoutMatched
	a.log.Info("foreign town egress started", "act", act, "start_anchor", startAnchor, "route_file", filename, "area_id", state.Area.ID)
	return nil
}

func matchSystemEgressRouteLayout(fingerprint pathing.LayoutFingerprint, route town.SystemEgressRoute) error {
	bound := route.Contract.LayoutFingerprint
	if err := pathing.MatchSystemEgressLayout(fingerprint, bound.Version, bound.AreaID, bound.AnchorCount, bound.Hash, bound.Anchors); err != nil {
		return fmt.Errorf("system egress layout differs: %w", err)
	}
	return nil
}

func (a *townEgressAdapter) Tick(ctx context.Context, state world.State) (bool, error) {
	if a == nil || a.walker == nil || a.activeAct == town.OriginActUnknown {
		return false, fmt.Errorf("town egress not started")
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return false, nil
	}
	expectedArea, _ := town.TownAreaForAct(a.activeAct)
	if state.Area.ID != expectedArea {
		return false, fmt.Errorf("town egress %s: status=%s", a.activeAct, pathing.TownWalkWrongArea)
	}
	if queueGameUIBlocked(state) {
		return false, fmt.Errorf("town egress %s blocked by open UI", a.activeAct)
	}
	if a.startAnchor == town.AnchorSpawn && !a.layoutMatched {
		fingerprint, err := pathing.BuildLayoutFingerprint(state)
		if err == nil {
			if err := matchSystemEgressRouteLayout(fingerprint, a.activeRoute); err != nil {
				return false, err
			}
			a.layoutMatched = true
			a.log.Info("spawn egress layout pinned", "act", a.activeAct, "route_point_index", a.walker.CurrentRoutePointIndex(), "layout_fingerprint", fingerprint.Hash)
		} else if !errors.Is(err, pathing.ErrLayoutAnchorsUnavailable) {
			return false, fmt.Errorf("town egress %s layout: %w", a.activeAct, err)
		}
		if !a.layoutMatched && a.activeRoute.Contract.LayoutProofPointIndex != nil {
			deadlineIndex := spawnLayoutProofDeadlineIndex(a.activeRoute, a.pathCfg)
			if a.walker.CurrentRoutePointIndex() > deadlineIndex {
				return false, fmt.Errorf("spawn egress layout proof deadline after recorded point %d and walker tolerance at point %d: %w", *a.activeRoute.Contract.LayoutProofPointIndex, deadlineIndex, pathing.ErrLayoutAnchorsUnavailable)
			}
		}
	}
	result := a.walker.TickRoute(ctx, state)
	if !result.Done {
		return false, nil
	}
	if result.Status != pathing.TownWalkArrived {
		return false, fmt.Errorf("town egress %s: status=%s reason=%s", a.activeAct, result.Status, result.Reason)
	}
	if a.startAnchor == town.AnchorSpawn && !a.layoutMatched {
		return false, fmt.Errorf("spawn egress reached waypoint without layout proof: %w", pathing.ErrLayoutAnchorsUnavailable)
	}
	if result.Done {
		a.log.Info("foreign town egress completed", "act", a.activeAct, "area_id", state.Area.ID)
	}
	return true, nil
}

func spawnLayoutProofDeadlineIndex(route town.SystemEgressRoute, cfg pathing.Config) int {
	if route.Contract.LayoutProofPointIndex == nil {
		return -1
	}
	// TownWalker may advance its target index before it reaches the exact sample
	// coordinate. Translate its configured arrival radius into route samples so
	// the recorded proof point remains reachable without an unbounded grace path.
	slack := int(math.Ceil(cfg.TownWalk.ArrivalDistance / route.SampleDistanceTiles))
	return *route.Contract.LayoutProofPointIndex + slack
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
	a.activeRoute = town.SystemEgressRoute{}
	a.startAnchor = ""
	a.layoutMatched = false
}
