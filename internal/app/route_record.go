package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	routeRecordSampleDistance = 4.0
	systemEgressRecordTimeout = 30 * time.Minute
)

// RunRouteRecord starts the candidate-first guided recorder for a registered run.
func (rt *Runtime) RunRouteRecord(runID, _ string, difficultyLabel string) error {
	return rt.runGuidedRouteRecord(tasks.RunID(runID), "", difficultyLabel, rt.Config.Session.Character, nil, nil)
}

// RunRouteRecordWithFinish starts guided recording and accepts the same
// idempotent finish intent from an API workflow as the global F9 hotkey.
func (rt *Runtime) RunRouteRecordWithFinish(runID, difficultyLabel, expectedCharacter string, finishRequests <-chan struct{}, reporter RouteWorkflowReporter) error {
	return rt.runGuidedRouteRecord(tasks.RunID(runID), "", difficultyLabel, expectedCharacter, finishRequests, reporter)
}

// RunRouteRecordWithRoleFinish starts one fixed-role guided recording and
// accepts the same idempotent finish intent as the global F9 hotkey.
func (rt *Runtime) RunRouteRecordWithRoleFinish(runID string, role pathing.RouteRole, difficultyLabel, expectedCharacter string, finishRequests <-chan struct{}, reporter RouteWorkflowReporter) error {
	return rt.runGuidedRouteRecord(tasks.RunID(runID), role, difficultyLabel, expectedCharacter, finishRequests, reporter)
}

// RunSystemEgressRecord reuses the CLI recorder for one configured global Act Egress.
func (rt *Runtime) RunSystemEgressRecord(act town.OriginAct) error {
	return rt.runConfiguredSystemEgressRecord(act, town.AnchorPortalArrival, nil, nil)
}

// RunSystemEgressRecordWithFinish records a global Egress and accepts the
// dashboard finish intent through the same validation path as F9.
func (rt *Runtime) RunSystemEgressRecordWithFinish(act town.OriginAct, finishRequests <-chan struct{}, reporter RouteWorkflowReporter) error {
	return rt.runConfiguredSystemEgressRecord(act, town.AnchorPortalArrival, finishRequests, reporter)
}

func (rt *Runtime) runConfiguredSystemEgressRecord(act town.OriginAct, startAnchor town.Anchor, finishRequests <-chan struct{}, reporter RouteWorkflowReporter) error {
	egress, reason := rt.Config.Town.EgressFor(act)
	if reason != "" {
		return fmt.Errorf("system egress %s unavailable: %s", act, reason)
	}
	area, ok := town.TownAreaForAct(act)
	if !ok {
		return fmt.Errorf("unsupported system egress act %q", act)
	}
	return rt.runSystemEgressRecord(act, startAnchor, "", rt.Config.ResolvePath(egress.RoutesDirectory), area, finishRequests, reporter)
}

// runSystemEgressRecord is deliberately separate from Farming recording: its
// global contract has no run, boss, candidate, assignment, or publish lifecycle.
func (rt *Runtime) runSystemEgressRecord(act town.OriginAct, startAnchor town.Anchor, name, directory string, expectedArea world.AreaID, finishRequests <-chan struct{}, reporter RouteWorkflowReporter) error {
	if _, err := town.SystemEgressFilenameForAnchor(startAnchor); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name = "System-Egress " + string(act)
	}
	recorder, err := pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: routeRecordSampleDistance, Movement: pathing.RouteMovementWalk})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() { _ = rt.Process.Detach() }()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	layoutCapture := systemEgressLayoutCapture{startAnchor: startAnchor}
	started := false
	var startedAt time.Time
	portalTolerance := rt.Config.Pathing.TownPortal.MaxClickDistance
	waypointTolerance := rt.Config.Pathing.Waypoint.MaxClickDistance
	lastWaitingReport := time.Time{}
	finish := func() error {
		if !started {
			err := fmt.Errorf("system egress not published: recording has not started at %s", startAnchor)
			rt.Log.Warn("system egress recording finish rejected", "act", act, "reason", "town_egress_start_unconfirmed", "error", err)
			return err
		}
		current := rt.World.Current()
		reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowValidating, AreaID: uint32(current.Area.ID), Progress: 0.9})
		if err := rt.finishSystemEgressRecording(recorder, act, startAnchor, directory, expectedArea, layoutCapture.fingerprint, layoutCapture.proofPointIndex, current, portalTolerance, waypointTolerance); err != nil {
			rt.Log.Warn("system egress recording finish rejected", "act", act, "reason", "town_egress_waypoint_unconfirmed", "error", err)
			return err
		}
		return nil
	}

	rt.Log.Info("system egress recording waiting for start anchor", "act", act, "start_anchor", startAnchor, "route_name", name, "finish_hotkey", rt.Config.Input.RecordingFinishHotkey, "emergency_stop_hotkey", rt.Config.Input.StopHotkey)
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreflight})
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("system egress recording cancelled: %w", ctx.Err())
		case event := <-hotkeys:
			if event.Action == input.HotkeyActionRecordingFinish {
				return finish()
			}
			rt.handleHotkeyEvent(event, cancel)
		case <-finishRequests:
			return finish()
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			current := rt.World.Current()
			if started && time.Since(startedAt) > systemEgressRecordTimeout {
				return fmt.Errorf("system egress recording timeout")
			}
			if !started {
				if !systemEgressRecordingStartReady(current, expectedArea, portalTolerance, startAnchor) {
					if lastWaitingReport.IsZero() || time.Since(lastWaitingReport) >= 2*time.Second {
						lastWaitingReport = time.Now()
						portal, portalVisible := current.NearestObject(world.ObjectKindTownPortal)
						distance := -1.0
						if portalVisible {
							distance = world.Distance(current.Player.Position, portal.Position)
						}
						rt.Log.Info("system egress recording preflight waiting", "act", act, "world_valid", current.Valid, "area_id", current.Area.ID, "expected_area_id", expectedArea, "portal_visible", portalVisible, "portal_distance", distance, "maximum_distance", portalTolerance)
						reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreflight, AreaID: uint32(current.Area.ID), Progress: 0, Reason: "town_egress_start_unconfirmed"})
					}
					continue
				}
				if startAnchor == town.AnchorPortalArrival {
					built, buildErr := pathing.BuildLayoutFingerprint(current)
					if buildErr != nil {
						continue
					}
					layoutCapture.fingerprint = built
				}
				started = true
				startedAt = time.Now()
				rt.Log.Info("system egress recording started", "act", act, "start_anchor", startAnchor, "area_id", current.Area.ID, "player_x", current.Player.Position.X, "player_y", current.Player.Position.Y, "layout_proof_pending", layoutCapture.fingerprint.Version == 0)
				reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowRecording, AreaID: uint32(current.Area.ID), Progress: 0.1})
			}
			if current.Valid && current.Area.ID != expectedArea {
				return fmt.Errorf("system egress recording left required area: got %d want %d", current.Area.ID, expectedArea)
			}
			event, err := recorder.Observe(current)
			if err != nil {
				return fmt.Errorf("system egress observe: %w", err)
			}
			if err := layoutCapture.Observe(current, event); err != nil {
				return fmt.Errorf("system egress layout proof: %w", err)
			}
		}
	}
}

type systemEgressLayoutCapture struct {
	startAnchor     town.Anchor
	fingerprint     pathing.LayoutFingerprint
	proofPointIndex *int
	acceptedPoints  int
}

func (c *systemEgressLayoutCapture) Observe(state world.State, event pathing.RouteRecorderEvent) error {
	if event.SampleAccepted {
		c.acceptedPoints++
	}
	if c.fingerprint.Version != 0 {
		return nil
	}
	built, err := pathing.BuildLayoutFingerprint(state)
	if errors.Is(err, pathing.ErrLayoutAnchorsUnavailable) {
		return nil
	}
	if err != nil {
		return err
	}
	c.fingerprint = built
	if c.startAnchor == town.AnchorSpawn {
		// P1 is persisted as the first buffered point that actually carried the
		// proof. If anchors appeared between samples, the next buffered point is
		// the fail-closed deadline so playback never rejects before reaching it.
		pointIndex := c.acceptedPoints - 1
		if !event.SampleAccepted {
			pointIndex = c.acceptedPoints
		}
		if pointIndex < 0 {
			pointIndex = 0
		}
		c.proofPointIndex = &pointIndex
	}
	return nil
}

func (rt *Runtime) finishSystemEgressRecording(recorder *pathing.RouteRecorder, act town.OriginAct, startAnchor town.Anchor, directory string, expectedArea world.AreaID, fingerprint pathing.LayoutFingerprint, layoutProofPointIndex *int, state world.State, portalTolerance, waypointTolerance float64) error {
	waypoint, ok := state.NearestObject(world.ObjectKindWaypoint)
	if !state.Valid || state.Area.ID != expectedArea || !ok || waypointTolerance <= 0 || world.Distance(state.Player.Position, waypoint.Position) > waypointTolerance {
		return fmt.Errorf("system egress not published: finish requires Memory-confirmed waypoint proximity")
	}
	segments, err := recorder.Finish()
	if err != nil {
		return fmt.Errorf("system egress not published: %w", err)
	}
	if len(segments) != 1 || segments[0].FromAreaID != expectedArea || segments[0].ToAreaID != expectedArea || segments[0].Movement != pathing.RouteMovementWalk {
		return fmt.Errorf("system egress not published: requires one same-town walk segment")
	}
	if portalTolerance <= 0 {
		return fmt.Errorf("system egress not published: portal arrival tolerance is invalid")
	}
	if fingerprint.Version == 0 {
		return fmt.Errorf("system egress not published: layout proof unavailable before waypoint")
	}
	if startAnchor == town.AnchorSpawn && layoutProofPointIndex == nil {
		return fmt.Errorf("system egress not published: spawn layout proof point unavailable before waypoint")
	}
	route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: act, TownArea: expectedArea, GameVersion: rt.Config.Memory.GameVersion, LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: append([]string(nil), fingerprint.Anchors...)}, LayoutProofPointIndex: layoutProofPointIndex, From: startAnchor, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: portalTolerance}, SampleDistanceTiles: routeRecordSampleDistance, Points: make([]town.SystemEgressPoint, 0, len(segments[0].Points))}
	for _, point := range segments[0].Points {
		route.Points = append(route.Points, town.SystemEgressPoint{X: point.X, Y: point.Y})
	}
	if route.Contract.LayoutProofPointIndex != nil && *route.Contract.LayoutProofPointIndex >= len(route.Points) {
		lastPoint := len(route.Points) - 1
		route.Contract.LayoutProofPointIndex = &lastPoint
	}
	filename, err := town.SystemEgressFilenameForAnchor(startAnchor)
	if err != nil {
		return fmt.Errorf("system egress publish: %w", err)
	}
	path := filepath.Join(directory, filename)
	if err := town.SaveSystemEgressRoute(path, route); err != nil {
		return fmt.Errorf("system egress publish: %w", err)
	}
	rt.Log.Info("system egress recording published", "act", act, "path", path, "points", len(route.Points), "layout_fingerprint", fingerprint.Hash)
	return nil
}

func systemEgressRecordingStartReady(state world.State, expectedArea world.AreaID, tolerance float64, startAnchor town.Anchor) bool {
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != expectedArea || tolerance <= 0 {
		return false
	}
	if startAnchor == town.AnchorSpawn {
		return state.Identity.Valid
	}
	if startAnchor != town.AnchorPortalArrival {
		return false
	}
	portal, ok := state.NearestObject(world.ObjectKindTownPortal)
	return ok && world.Distance(state.Player.Position, portal.Position) <= tolerance
}
