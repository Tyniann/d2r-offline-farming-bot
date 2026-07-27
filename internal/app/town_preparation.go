package app

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// townPreparationController is the shared input surface required by graph
// walking and gated shop actions; Status keeps pause/stop outside Town policy.
type townPreparationController interface {
	pathing.InputDriver
	town.ShopInput
	Status() input.Status
}

// townPreparationAdapter has two deliberately narrow modes. Initial run setup
// uses only the layout-bound Stash-to-Waypoint path; post-run preparation may
// additionally create and execute a demand-driven service plan.
type townPreparationAdapter struct {
	log          *slog.Logger
	driver       pathing.InputDriver
	controller   townPreparationController
	pathCfg      pathing.Config
	graph        town.ServiceGraph
	directory    string
	thresholds   town.Thresholds
	traversals   []town.Traversal
	index        int
	walker       *pathing.TownWalker
	started      bool
	done         bool
	layout       string
	layoutOrigin world.Position
	layoutPin    *townLayoutPin
	townCfg      town.Config
	profile      config.ProfileResourcesConfig
	telemetry    town.ExecutorTelemetry
	services     bool
	executor     *town.Executor
	handler      *townPreparationStepHandler
	lootFilter   *loot.Filter
	stashConfig  config.LootStashConfig
	nextRunID    string
	startAnchor  town.Anchor
}

func (a *townPreparationAdapter) setItemPolicies(filter *loot.Filter, stash config.LootStashConfig) {
	a.lootFilter, a.stashConfig = filter, stash
}

// layoutTownWaypointWalker adapts the initial no-service mode to the legacy
// task dependency without reintroducing difficulty-selected Town routes.
type layoutTownWaypointWalker struct{ adapter *townPreparationAdapter }

func (w *layoutTownWaypointWalker) TickAct1Waypoint(ctx context.Context, state world.State) pathing.TownWalkResult {
	if w == nil || w.adapter == nil {
		return pathing.TownWalkResult{Status: pathing.TownWalkRouteMissing, Reason: string(town.ReasonTownLayoutRouteMissing), Done: true}
	}
	// A previous same-game run may already stand at the live Waypoint. Skipping
	// the fresh run's Stash origin then avoids replaying the wrong graph edge.
	// This cold-start fast path must not fire after walking has begun: entering
	// MaxClickDistance mid Force-Move would otherwise open/select the waypoint
	// while the character is still sliding past it.
	if !w.adapter.started {
		if waypoint, ok := state.NearestObject(world.ObjectKindWaypoint); ok &&
			world.Distance(state.Player.Position, waypoint.Position) <= w.adapter.pathCfg.Waypoint.MaxClickDistance {
			w.adapter.log.Info("town waypoint handoff reused", "distance", world.Distance(state.Player.Position, waypoint.Position))
			return pathing.TownWalkResult{Status: pathing.TownWalkWaypointVisible, Done: true}
		}
	}
	result := w.adapter.Tick(ctx, state)
	if !result.Done {
		return pathing.TownWalkResult{Status: pathing.TownWalkPending}
	}
	if result.Status == "complete" {
		return pathing.TownWalkResult{Status: pathing.TownWalkWaypointVisible, Done: true}
	}
	return pathing.TownWalkResult{Status: pathing.TownWalkRouteMissing, Reason: result.Reason, Done: true}
}

func (w *layoutTownWaypointWalker) Reset() {
	if w != nil && w.adapter != nil {
		w.adapter.Reset()
	}
}

func newTownPreparationAdapter(log *slog.Logger, controller townPreparationController, pathCfg pathing.Config, cfg *config.Config, runID string, run config.RunConfig, layoutPin *townLayoutPin, telemetry town.ExecutorTelemetry, services bool) (*townPreparationAdapter, error) {
	directory := cfg.ResolvePath(cfg.Town.Hub.RoutesDirectory)
	graph, err := town.LoadServiceGraph(filepath.Join(directory, "graph.yaml"))
	if err != nil {
		return nil, fmt.Errorf("load central town graph: %w", err)
	}
	profile := cfg.Profiles[run.Combat.Profile].Resources
	return &townPreparationAdapter{log: log, driver: controller, controller: controller, pathCfg: pathCfg, graph: graph, directory: directory, thresholds: cfg.Town.Thresholds, layoutPin: layoutPin, townCfg: cfg.Town, profile: profile, telemetry: telemetry, services: services, nextRunID: runID, startAnchor: town.AnchorStash}, nil
}

func (a *townPreparationAdapter) Tick(ctx context.Context, state world.State) tasks.TownPreparationResult {
	if a == nil || a.driver == nil || !state.Valid || state.Area.ID != world.RogueEncampment || state.UI.StashOpen || (!a.started && (state.UI.NPCInteractOpen || state.UI.NPCShopOpen)) {
		return tasks.TownPreparationResult{Status: "failed", Reason: "town_preparation_state_invalid", Done: true}
	}
	if a.done {
		return tasks.TownPreparationResult{Status: "complete", Done: true}
	}
	if a.layoutPin == nil {
		a.layoutPin = &townLayoutPin{}
	}
	fingerprint, layoutReason, _ := a.layoutPin.Resolve(state)
	if layoutReason != "" {
		return tasks.TownPreparationResult{Status: "failed", Reason: string(layoutReason), Done: true}
	}
	if a.layout != "" && a.layout != fingerprint.Hash {
		return tasks.TownPreparationResult{Status: "failed", Reason: string(town.ReasonTownLayoutMismatch), Done: true}
	}
	if !a.started {
		// Freeze the preset and translation origin before planning. Every later
		// tick still revalidates the shared pin before it can send more input.
		a.layout = fingerprint.Hash
		a.layoutOrigin = world.Position{X: fingerprint.StashX, Y: fingerprint.StashY}
		if reason := a.start(state); reason != "" {
			return tasks.TownPreparationResult{Status: "failed", Reason: reason, Done: true}
		}
	}
	if a.executor != nil {
		// Service plans own their navigation inside the handler so graph progress
		// survives the executor's per-step reset boundary.
		status := a.controller.Status()
		result := a.executor.Tick(ctx, state, status.Paused, status.Stopped)
		if result.Done && result.Status == town.InteractionComplete {
			a.done = true
			a.log.Info("central town preparation completed", "anchor", "waypoint", "next_run", a.nextRunID)
			return tasks.TownPreparationResult{Status: "complete", Done: true}
		}
		if result.Done {
			return tasks.TownPreparationResult{Status: "failed", Reason: string(result.Reason), Done: true}
		}
		return tasks.TownPreparationResult{Status: "pending"}
	}
	if a.index >= len(a.traversals) {
		// Finishing route samples is insufficient: the handoff requires the live
		// Waypoint entity to be present and within interaction distance.
		waypoint, ok := state.NearestObject(world.ObjectKindWaypoint)
		if !ok || world.Distance(state.Player.Position, waypoint.Position) > a.pathCfg.Waypoint.MaxClickDistance {
			return tasks.TownPreparationResult{Status: "failed", Reason: "waypoint_handoff_unconfirmed", Done: true}
		}
		a.done = true
		a.log.Info("central town preparation completed", "anchor", "waypoint", "next_run", a.nextRunID)
		return tasks.TownPreparationResult{Status: "complete", Done: true}
	}
	if a.walker == nil {
		traversal := a.traversals[a.index]
		points, err := pathing.LoadLayoutBoundTownRoute(filepath.Join(a.directory, traversal.Edge.Route), traversal.Edge.ID, a.layout, a.layoutOrigin)
		if err != nil {
			return tasks.TownPreparationResult{Status: "failed", Reason: err.Error(), Done: true}
		}
		if traversal.Reverse {
			reversePositions(points)
		}
		// A portal arrival varies within the portal's interaction radius, so its
		// Memory-confirmed entity is the authoritative external-anchor proof. Other
		// origins retain the stricter first-point check. The walker may safely close
		// the small gap from a confirmed portal arrival to the recorded first point.
		if a.index == 0 && !a.externalStartConfirmed(state, points[0]) {
			return tasks.TownPreparationResult{Status: "failed", Reason: "town_edge_start_unconfirmed", Done: true}
		}
		a.walker = pathing.NewTownRouteWalker(a.log, a.driver, a.pathCfg, points)
		a.log.Info("central town edge started", "edge", traversal.Edge.ID, "index", a.index)
	}
	result := a.walker.TickRoute(ctx, state)
	if !result.Done {
		return tasks.TownPreparationResult{Status: "pending"}
	}
	if result.Status != pathing.TownWalkArrived {
		return tasks.TownPreparationResult{Status: "failed", Reason: result.Reason, Done: true}
	}
	a.log.Info("central town edge completed", "edge", a.traversals[a.index].Edge.ID, "index", a.index)
	a.index++
	a.walker = nil
	return tasks.TownPreparationResult{Status: "pending"}
}

func (a *townPreparationAdapter) externalStartConfirmed(state world.State, firstPoint world.Position) bool {
	if a.startAnchor == town.AnchorPortalArrival {
		return townPortalArrivalReady(state, a.pathCfg.TownPortal.MaxClickDistance)
	}
	return world.Distance(state.Player.Position, firstPoint) <= a.pathCfg.TownWalk.ArrivalDistance
}

func townPortalArrivalReady(state world.State, tolerance float64) bool {
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Area.ID.IsTown() || tolerance <= 0 {
		return false
	}
	portal, visible := state.NearestObject(world.ObjectKindTownPortal)
	return visible && world.Distance(state.Player.Position, portal.Position) <= tolerance
}

func (a *townPreparationAdapter) Reset() {
	if a == nil {
		return
	}
	if a.walker != nil {
		a.walker.Reset()
	}
	a.traversals, a.walker = nil, nil
	a.index, a.started, a.done, a.layout, a.layoutOrigin = 0, false, false, "", world.Position{}
	if a.executor != nil {
		a.executor.Reset()
	}
	a.executor, a.handler = nil, nil
}
