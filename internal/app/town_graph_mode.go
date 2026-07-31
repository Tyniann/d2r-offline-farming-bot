package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const townRecordingAnchorDistance = 15.0

func (rt *Runtime) townGraph() (town.ServiceGraph, string, error) {
	directory := rt.Config.ResolvePath(rt.Config.Town.Hub.RoutesDirectory)
	graph, err := town.LoadServiceGraph(filepath.Join(directory, "graph.yaml"))
	if err != nil {
		return town.ServiceGraph{}, "", err
	}
	return graph, directory, nil
}

func (rt *Runtime) runPathingRecordTownEdge(ctx context.Context, state *runState, hotkeyEvents <-chan input.HotkeyEvent, ticker *time.Ticker, deadline time.Time, cancel context.CancelFunc, edgeID string) error {
	graph, directory, err := rt.townGraph()
	if err != nil {
		return err
	}
	edge, draft, ok := townRecordingEdge(graph, edgeID)
	if !ok {
		return fmt.Errorf("town graph edge %q is neither registered nor an approved recording draft", edgeID)
	}
	pathingCfg := mapPathingConfig(rt.Config.Pathing)
	points := make([]world.Position, 0, 16)
	lastPosition := world.Position{}
	havePosition := false
	lastState := world.State{}
	endpointPosition := world.Position{}
	haveEndpointPosition := false
	endpointNPCID := uint32(0)
	layout := town.TownLayoutFingerprint{}
	routePath := ""
	rt.Log.Info("read-only town edge recording active - walk manually and press Stop at destination", "edge", edge.ID, "from", edge.From, "to", edge.To, "draft", draft)
	for time.Now().Before(deadline) {
		current, stop, tickErr := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if tickErr != nil {
			return tickErr
		}
		observed, sample, observeErr := townRecordingObservation(current, layout)
		if observeErr != nil {
			return observeErr
		}
		if !sample && layout.Hash == "" {
			// Positions observed before any authoritative layout anchor belong to
			// no safe variant and must not leak into the later pinned recording.
			points = points[:0]
			lastPosition = world.Position{}
			havePosition = false
			lastState = world.State{}
			endpointPosition = world.Position{}
			haveEndpointPosition = false
			endpointNPCID = 0
		}
		if sample {
			lastState = current
			if edge.To == town.AnchorCain {
				if npc, ok := current.FindNPC(world.DeckardCain); ok && endpointNPCID != npc.NPCID {
					endpointNPCID = npc.NPCID
					rt.Log.Info("town edge Cain variant observed", "edge", edge.ID, "npc_id", npc.NPCID, "unit_id", npc.UnitID)
				}
			}
			if edge.To == town.AnchorKashya {
				if npc, ok := current.FindNPC(world.Kashya); ok && endpointNPCID != npc.NPCID {
					endpointNPCID = npc.NPCID
					rt.Log.Info("town edge Kashya observed", "edge", edge.ID, "npc_id", npc.NPCID, "unit_id", npc.UnitID)
				}
			}
			if position, ok := townRecordingEndpointPosition(edge.To, current); ok {
				// NPCs and objects can unload near regional boundaries. Retain the
				// last endpoint from this recording, never from an earlier run.
				endpointPosition, haveEndpointPosition = position, true
			}
			if layout.Hash == "" && observed.Hash != "" {
				layout = observed
				routePath = filepath.Join(directory, fmt.Sprintf("%s-%s.yaml", edge.ID, layout.Hash[:12]))
				rt.Log.Info("town edge layout pinned", "edge", edge.ID, "town_layout", layout.Hash, "waypoint_dx", layout.WaypointDeltaX, "waypoint_dy", layout.WaypointDeltaY, "route_file", routePath)
			}
			position := current.Player.Position
			lastPosition, havePosition = position, true
			if len(points) == 0 || world.Distance(points[len(points)-1], position) >= pathingCfg.TownWalk.ArrivalDistance {
				points = append(points, position)
				rt.Log.Info("town edge sample", "edge", edge.ID, "index", len(points)-1, "pos_x", position.X, "pos_y", position.Y)
			}
		}
		if stop {
			if havePosition && (len(points) == 0 || points[len(points)-1] != lastPosition) {
				points = append(points, lastPosition)
				rt.Log.Info("town edge final sample", "edge", edge.ID, "index", len(points)-1, "pos_x", lastPosition.X, "pos_y", lastPosition.Y)
			}
			break
		}
	}
	if len(points) < 2 {
		return fmt.Errorf("town edge %q recording needs at least 2 samples, got %d", edge.ID, len(points))
	}
	if layout.Hash == "" || routePath == "" {
		return fmt.Errorf("%s", town.ReasonTownLayoutUnavailable)
	}
	endpointDistance, endpointOK := townRecordingEndpointDistance(edge.To, lastState)
	endpointSource := "current"
	if !endpointOK && haveEndpointPosition {
		endpointDistance = world.Distance(lastState.Player.Position, endpointPosition)
		endpointOK = true
		endpointSource = "pinned"
	}
	if !endpointOK {
		err := fmt.Errorf("town edge %q endpoint %q is not available in Memory; recording rejected", edge.ID, edge.To)
		rt.Log.Error("town edge recording rejected", "edge", edge.ID, "reason", "endpoint_unavailable", "error", err)
		return err
	}
	if endpointDistance > townRecordingAnchorDistance {
		err := fmt.Errorf("town edge %q endpoint %q is %.1f tiles away; maximum recording distance is %.1f", edge.ID, edge.To, endpointDistance, townRecordingAnchorDistance)
		rt.Log.Error("town edge recording rejected", "edge", edge.ID, "reason", "endpoint_too_far", "endpoint_distance", endpointDistance, "error", err)
		return err
	}
	if err := pathing.SaveLayoutBoundTownRoute(routePath, edge.ID, layout.Hash, world.Position{X: layout.StashX, Y: layout.StashY}, pathingCfg.TownWalk.ArrivalDistance, points); err != nil {
		return fmt.Errorf("save town edge %q: %w", edge.ID, err)
	}
	rt.Log.Info("town edge recording saved", "edge", edge.ID, "from", edge.From, "to", edge.To, "town_layout", layout.Hash, "route_file", routePath, "points", len(points), "endpoint_distance", endpointDistance, "endpoint_source", endpointSource, "endpoint_npc_id", endpointNPCID, "activation", "pending_graph_variant")
	return nil
}

func townRecordingObservation(state world.State, pinned town.TownLayoutFingerprint) (town.TownLayoutFingerprint, bool, error) {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != world.RogueEncampment {
		return pinned, false, nil
	}
	observed, reason := town.InspectTownLayout(state)
	if pinned.Hash == "" {
		if reason != "" {
			// Buffer valid Town movement until Stash and Waypoint become visible;
			// the caller binds those samples only when the same observation pins.
			return pinned, true, nil
		}
		return observed, true, nil
	}
	if reason == "" && observed.Hash != pinned.Hash {
		// Once anchors reappear, a mismatch invalidates the whole recording. A
		// temporarily unloaded anchor is tolerated; a different preset is not.
		return pinned, false, fmt.Errorf("%s", town.ReasonTownLayoutMismatch)
	}
	return pinned, true, nil
}

func townRecordingEndpointDistance(anchor town.Anchor, state world.State) (float64, bool) {
	position, ok := townRecordingEndpointPosition(anchor, state)
	if !ok {
		return 0, false
	}
	return world.Distance(state.Player.Position, position), true
}

func townRecordingEndpointPosition(anchor town.Anchor, state world.State) (world.Position, bool) {
	var position world.Position
	switch anchor {
	case town.AnchorAkara, town.AnchorCain, town.AnchorCharsi, town.AnchorKashya:
		var npc world.Monster
		var ok bool
		npcID := map[town.Anchor]uint32{
			town.AnchorAkara:  world.Akara,
			town.AnchorCain:   world.DeckardCain,
			town.AnchorCharsi: world.Charsi,
			town.AnchorKashya: world.Kashya,
		}[anchor]
		npc, ok = state.FindNPC(npcID)
		if !ok {
			return world.Position{}, false
		}
		position = npc.Position
	case town.AnchorWaypoint:
		object, ok := state.NearestObject(world.ObjectKindWaypoint)
		if !ok {
			return world.Position{}, false
		}
		position = object.Position
	case town.AnchorStash, town.AnchorSpawn:
		object, ok := state.NearestObject(world.ObjectKindPersonalStash)
		if !ok {
			return world.Position{}, false
		}
		position = object.Position
	default:
		return world.Position{}, false
	}
	return position, true
}

func townRecordingEdge(graph town.ServiceGraph, id string) (town.GraphEdge, bool, bool) {
	if edge, ok := graph.Edge(id); ok {
		return edge, false, true
	}
	if id == "stash-waypoint" {
		return town.GraphEdge{ID: id, From: town.AnchorStash, To: town.AnchorWaypoint, Route: "stash-waypoint.yaml", Cost: 1, Reversible: true}, true, true
	}
	if id == "waypoint-kashya" {
		// Phase 18.4 activates layout variants in graph.yaml after MrHammer
		// recordings. The draft lets operators record without a premature edge.
		return town.GraphEdge{ID: id, From: town.AnchorWaypoint, To: town.AnchorKashya, Route: "waypoint-kashya.yaml", Cost: 1, Reversible: true}, true, true
	}
	return town.GraphEdge{}, false, false
}

func (rt *Runtime) runPathingPlayTownGraph(ctx context.Context, state *runState, hotkeyEvents <-chan input.HotkeyEvent, ticker *time.Ticker, deadline time.Time, cancel context.CancelFunc, rawAnchors string) error {
	graph, directory, err := rt.townGraph()
	if err != nil {
		return err
	}
	parts := strings.Split(rawAnchors, ",")
	anchors := make([]town.Anchor, len(parts))
	for i, part := range parts {
		anchors[i] = town.Anchor(strings.TrimSpace(part))
	}
	start, end := anchors[0], anchors[len(anchors)-1]
	current := rt.World.Current()
	for !current.Identity.Valid {
		var stop bool
		current, stop, err = rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("town graph playback timeout waiting for confirmed game identity")
		}
	}
	if rt.townLayout == nil {
		rt.townLayout = &townLayoutPin{}
	}
	layout, reason, directlyObserved := rt.townLayout.Resolve(current)
	cachePath := filepath.Join("diagnostics", "town", "layout-pin.json")
	if reason == town.ReasonTownLayoutUnavailable && start == town.AnchorPortalArrival {
		// Portal arrival may regionally unload both fingerprint anchors. Only the
		// short-lived, same-game diagnostic pin may bridge this CLI test case.
		cached, cacheErr := loadTownLayoutTestPin(cachePath, rt.Process.Status().PID, current, time.Now())
		if cacheErr == nil {
			if seedErr := rt.townLayout.Seed(cached, current.Identity); seedErr != nil {
				return seedErr
			}
			layout, reason, directlyObserved = rt.townLayout.Resolve(current)
			rt.Log.Info("Town layout test pin restored", "town_layout", layout.Hash, "stash_x", layout.StashX, "stash_y", layout.StashY)
		} else if !os.IsNotExist(cacheErr) {
			rt.Log.Warn("Town layout test pin rejected", "reason", cacheErr)
		}
	}
	if reason != "" {
		return fmt.Errorf("%s", reason)
	}
	if directlyObserved {
		if saveErr := saveTownLayoutTestPin(cachePath, rt.Process.Status().PID, current, layout, time.Now()); saveErr != nil {
			return saveErr
		}
		rt.Log.Info("Town layout test pin saved", "town_layout", layout.Hash, "path", cachePath)
	}
	traversals, err := graph.RouteForLayout(layout.Hash, start, anchors[1:len(anchors)-1], end)
	if err != nil {
		return err
	}
	driver, ok := rt.Input.(pathing.InputDriver)
	if !ok {
		return fmt.Errorf("town graph playback: input controller does not support pathing actions")
	}
	pathingCfg := mapPathingConfig(rt.Config.Pathing)
	for index, traversal := range traversals {
		points, loadErr := pathing.LoadLayoutBoundTownRoute(filepath.Join(directory, traversal.Edge.Route), traversal.Edge.ID, layout.Hash, world.Position{X: layout.StashX, Y: layout.StashY})
		if loadErr != nil {
			return fmt.Errorf("load town graph edge %q: %w", traversal.Edge.ID, loadErr)
		}
		if traversal.Reverse {
			reversePositions(points)
		}
		// Confirm the operator-provided start once. Later graph edges compose at
		// semantic anchors and are allowed to approach their own recording boundary.
		if index == 0 && (!current.Valid || world.Distance(current.Player.Position, points[0]) > pathingCfg.TownWalk.ArrivalDistance) {
			return fmt.Errorf("town graph edge %q start not confirmed: distance %.1f exceeds %.1f", traversal.Edge.ID, world.Distance(current.Player.Position, points[0]), pathingCfg.TownWalk.ArrivalDistance)
		}
		walker := pathing.NewTownRouteWalker(rt.Log, driver, pathingCfg, points)
		rt.Log.Info("town graph edge playback started", "index", index, "edge", traversal.Edge.ID, "reverse", traversal.Reverse)
		for time.Now().Before(deadline) {
			var stop bool
			current, stop, err = rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
			resolved, layoutReason, observed := rt.townLayout.Resolve(current)
			if layoutReason != "" {
				return fmt.Errorf("town layout validation during edge %q: %s", traversal.Edge.ID, layoutReason)
			}
			if resolved.Hash != layout.Hash {
				return fmt.Errorf("town layout validation during edge %q: %s", traversal.Edge.ID, town.ReasonTownLayoutMismatch)
			}
			if observed {
				layout = resolved
			}
			result := walker.TickRoute(ctx, current)
			if !result.Done {
				continue
			}
			if result.Status != pathing.TownWalkArrived {
				return fmt.Errorf("town graph edge %q failed: status=%s reason=%s", traversal.Edge.ID, result.Status, result.Reason)
			}
			rt.Log.Info("town graph edge playback completed", "index", index, "edge", traversal.Edge.ID, "reverse", traversal.Reverse)
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("town graph playback timeout at edge %q", traversal.Edge.ID)
		}
	}
	rt.Log.Info("town graph plan completed", "start", start, "required", anchors[1:len(anchors)-1], "end", end, "edge_count", len(traversals), "town_layout", layout.Hash)
	return nil
}

func reversePositions(points []world.Position) {
	for left, right := 0, len(points)-1; left < right; left, right = left+1, right-1 {
		points[left], points[right] = points[right], points[left]
	}
}
