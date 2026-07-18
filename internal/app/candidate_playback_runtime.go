package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RunCandidateTest binds the navigation-only candidate orchestrator to the
// existing Runtime pathing, Town, Egress, Waypoint and TP components.
func (rt *Runtime) RunCandidateTest(candidateID string) error {
	return rt.RunCandidateTestWithProgress(candidateID, nil)
}

// RunCandidateTestWithProgress runs candidate-only navigation and reports
// observable workflow stages to the local dashboard.
func (rt *Runtime) RunCandidateTestWithProgress(candidateID string, reporter RouteWorkflowReporter) error {
	store, err := NewCandidateStore(rt.Config)
	if err != nil {
		return err
	}
	orchestrator, err := NewCandidateTestOrchestrator(store, tasks.DefaultRunRegistry())
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
	deadline := time.Now().Add(rt.attachTimeoutOrDefault(2 * time.Minute))
	if readyErr := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, deadline, cancel, true); readyErr != nil {
		return readyErr
	}
	if focusErr := rt.Input.Focus(); focusErr != nil {
		return fmt.Errorf("focus D2R for candidate test: %w", focusErr)
	}
	driver := &runtimeCandidatePlaybackDriver{rt: rt, state: state, hotkeys: hotkeys, ticker: ticker, cancel: cancel}
	candidate, err := orchestrator.TestWithProgress(ctx, candidateID, driver, reporter)
	if err != nil {
		rt.Log.Error("candidate navigation test failed", "candidate_id", candidateID, "error", err)
		return err
	}
	rt.Log.Info("candidate navigation test completed", "candidate_id", candidate.CandidateID, "run_id", candidate.RunID, "state", candidate.State)
	return nil
}

type runtimeCandidatePlaybackDriver struct {
	rt      *Runtime
	state   *runState
	hotkeys <-chan input.HotkeyEvent
	ticker  *time.Ticker
	cancel  context.CancelFunc
}

func (d *runtimeCandidatePlaybackDriver) tick(ctx context.Context) (world.State, error) {
	current, stop, err := d.rt.pathingTestTick(ctx, d.state, d.hotkeys, d.ticker, d.cancel)
	if err != nil {
		return world.State{}, err
	}
	if stop || ctx.Err() != nil {
		return world.State{}, fmt.Errorf("candidate test cancelled: %w", context.Canceled)
	}
	return current, nil
}

func (d *runtimeCandidatePlaybackDriver) EnsureTown(ctx context.Context, act town.OriginAct) error {
	target := recordingOriginTownArea(act)
	deadline := time.Now().Add(2 * time.Minute)
	cast, clicked := false, false
	var townSeenAt time.Time
	for time.Now().Before(deadline) {
		current, err := d.tick(ctx)
		if err != nil {
			return err
		}
		if current.Valid && current.Area.ID == target {
			if candidatePortalArrivalReady(current, d.rt.Config.Pathing.TownPortal.MaxClickDistance) {
				return nil
			}
			if townSeenAt.IsZero() {
				townSeenAt = time.Now()
			}
			if time.Since(townSeenAt) >= 2*time.Second {
				return fmt.Errorf("candidate test requires Memory-confirmed portal_arrival in %s", current.Area.Name)
			}
			continue
		}
		if !current.Valid || current.Phase != world.GamePhaseInGame {
			continue
		}
		if current.Area.ID.IsTown() {
			return fmt.Errorf("candidate test is in wrong town %s", current.Area.Name)
		}
		if !cast {
			if d.rt.taskDeps.Actions == nil || d.rt.taskDeps.Portal == nil {
				return fmt.Errorf("town portal actions not wired")
			}
			d.rt.taskDeps.Portal.Reset()
			if err := d.rt.taskDeps.Actions.CastTownPortal(); err != nil {
				return err
			}
			cast = true
			continue
		}
		if !clicked {
			result := d.rt.taskDeps.Portal.Tick(ctx, current, time.Now())
			if result.Done && result.Status != pathing.TownPortalActionClicked {
				return fmt.Errorf("portal entry failed: status=%s reason=%s", result.Status, result.Reason)
			}
			clicked = result.Status == pathing.TownPortalActionClicked
		}
	}
	return fmt.Errorf("candidate test town return timeout")
}

func candidatePortalArrivalReady(state world.State, tolerance float64) bool {
	return townPortalArrivalReady(state, tolerance)
}

func (d *runtimeCandidatePlaybackDriver) NormalizeToAct1(ctx context.Context, act town.OriginAct) error {
	if act == town.OriginAct1 {
		if current := d.rt.World.Current(); current.Valid && current.Area.ID == world.RogueEncampment {
			return nil
		}
		return fmt.Errorf("candidate test expected Rogue Encampment")
	}
	d.rt.townEgress.Reset()
	d.rt.taskDeps.Waypoint.Reset()
	stage := 0
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		current, err := d.tick(ctx)
		if err != nil {
			return err
		}
		switch stage {
		case 0:
			if err := d.rt.townEgress.Start(act, current); err != nil {
				return err
			}
			stage = 1
		case 1:
			done, err := d.rt.townEgress.Tick(ctx, current)
			if err != nil {
				return err
			}
			if done {
				stage = 2
			}
		case 2:
			result := d.rt.taskDeps.Waypoint.TickTownWaypoint(ctx, current)
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("waypoint open failed: %s", result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = 3
			}
		case 3:
			if !current.UI.WaypointOpen {
				continue
			}
			result := d.rt.taskDeps.Waypoint.SelectWaypointTarget(ctx, current, pathing.WaypointTargetRogueEncampment, time.Now())
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("select Rogue Encampment failed: %s", result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = 4
			}
		case 4:
			if current.Area.ID == world.RogueEncampment {
				return nil
			}
		}
	}
	return fmt.Errorf("candidate test Egress timeout")
}

func (d *runtimeCandidatePlaybackDriver) TravelToStart(ctx context.Context, target pathing.WaypointTargetID) error {
	action, ok := pathing.DefaultWaypointTargetRegistry().Action(target)
	if !ok {
		return fmt.Errorf("candidate waypoint target %q unsupported", target)
	}
	walker, ok := d.rt.taskDeps.TownWalk.(*layoutTownWaypointWalker)
	if !ok || walker.adapter == nil {
		return fmt.Errorf("candidate Town walker not wired")
	}
	walker.Reset()
	walker.adapter.startAnchor = town.AnchorPortalArrival
	defer func() { walker.adapter.startAnchor = town.AnchorStash; walker.Reset() }()
	d.rt.taskDeps.Waypoint.Reset()
	stage := 0
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		current, err := d.tick(ctx)
		if err != nil {
			return err
		}
		switch stage {
		case 0:
			if current.Area.ID != world.RogueEncampment {
				return fmt.Errorf("candidate start travel requires Rogue Encampment")
			}
			result := walker.TickAct1Waypoint(ctx, current)
			if !result.Done {
				continue
			}
			if result.Status != pathing.TownWalkWaypointVisible {
				return fmt.Errorf("town waypoint walk failed: %s", result.Reason)
			}
			stage = 1
		case 1:
			result := d.rt.taskDeps.Waypoint.TickTownWaypoint(ctx, current)
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("waypoint open failed: %s", result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = 2
			}
		case 2:
			if !current.UI.WaypointOpen {
				continue
			}
			result := d.rt.taskDeps.Waypoint.SelectWaypointTarget(ctx, current, target, time.Now())
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("candidate waypoint selection failed: %s", result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = 3
			}
		case 3:
			if current.Area.ID == action.ExpectedAreaID {
				return nil
			}
		}
	}
	return fmt.Errorf("candidate start waypoint timeout")
}

func (d *runtimeCandidatePlaybackDriver) PlayCandidate(ctx context.Context, route pathing.Route) error {
	current := d.rt.World.Current()
	fingerprint, err := pathing.BuildLayoutFingerprint(current)
	if err != nil {
		return err
	}
	if precheckErr := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{Identity: current.Identity, GameVersion: d.rt.Config.Memory.GameVersion, Layout: fingerprint, World: current}); precheckErr != nil {
		return precheckErr
	}
	player, err := pathing.NewRoutePlayer(d.rt.Pathing, route)
	if err != nil {
		return err
	}
	defer player.Reset()
	deadline := time.Now().Add(time.Duration(len(route.Segments)+1) * time.Minute)
	for time.Now().Before(deadline) {
		current, tickErr := d.tick(ctx)
		if tickErr != nil {
			return tickErr
		}
		done, playErr := player.Tick(ctx, current)
		if playErr != nil {
			return playErr
		}
		if done {
			return nil
		}
	}
	return fmt.Errorf("candidate route playback timeout")
}

func (d *runtimeCandidatePlaybackDriver) TerminalEvidence(context.Context) (RecordingTerminalEvidence, error) {
	current := d.rt.World.Current()
	if !current.Valid {
		return RecordingTerminalEvidence{}, errors.New("candidate terminal world is invalid")
	}
	for _, definition := range tasks.DefaultRunRegistry().Definitions() {
		if definition.Recording.TerminalArea == current.Area.ID {
			boss := recordingBossEvidence(current, definition.Recording.Boss)
			return RecordingTerminalEvidence{World: current, Boss: boss}, nil
		}
	}
	return RecordingTerminalEvidence{World: current}, nil
}

func (d *runtimeCandidatePlaybackDriver) ReturnAfterTest(ctx context.Context, act town.OriginAct) error {
	return d.EnsureTown(ctx, act)
}
