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

func (rt *Runtime) runGuidedRouteRecord(runID tasks.RunID, difficultyLabel, expectedCharacter string, finishRequests <-chan struct{}, reporter RouteWorkflowReporter) error {
	definition, ok := tasks.DefaultRunRegistry().Definition(runID)
	if !ok {
		return fmt.Errorf("guided recording requires a registered run: %s", runID)
	}
	difficulty, err := parseOfflineDifficulty(difficultyLabel)
	if err != nil {
		return err
	}
	candidateStore, err := NewCandidateStore(rt.Config)
	if err != nil {
		return err
	}
	coordinator, err := NewRecordingCoordinator(candidateStore, tasks.DefaultRunRegistry())
	if err != nil {
		return err
	}
	lifecycle, err := NewRouteLifecycleStore(rt.Config)
	if err != nil {
		return err
	}
	_, catalog, err := lifecycle.Snapshot()
	if err != nil {
		return fmt.Errorf("recording catalog snapshot: %w", err)
	}
	assignments, err := NewRouteAssignmentStore(rt.Config)
	if err != nil {
		return err
	}
	assignment, err := assignments.Snapshot()
	if err != nil {
		return fmt.Errorf("recording assignment snapshot: %w", err)
	}
	// Der bestehende Recordingpfad arbeitet bis zur charakterbezogenen
	// Runtime-Umstellung in 16.5 weiterhin ausdrücklich mit der Run-Config.
	// Diese Ableitung darf nicht in den Charakterkatalog zurückfließen.
	runConfig, runConfigured := rt.Config.Runs.Run(string(runID))
	profile, profileConfigured := rt.Config.Profiles[runConfig.Combat.Profile]
	expectedClass := profile.CharacterClass
	if !runConfigured || !profileConfigured || expectedClass == "" {
		return fmt.Errorf("guided recording requires configured character class")
	}
	if expectedCharacter == "" {
		return fmt.Errorf("guided recording requires confirmed character")
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
	runtimeState := &runState{}
	started := false
	returnStage := 0
	portalClicked := false
	var frozen RouteCandidate
	lastWaitingReport := time.Time{}
	requestFinish := func() error {
		if !started {
			err := fmt.Errorf("%s", RouteReasonRecordingPreflightFailed)
			rt.Log.Warn("guided route recording finish rejected", "run_id", runID, "reason", RouteReasonRecordingPreflightFailed, "error", err)
			return err
		}
		current := rt.World.Current()
		reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowFreezing, AreaID: uint32(current.Area.ID), Progress: 0.75})
		boss := recordingBossEvidence(current, definition.Recording.Boss)
		reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowValidating, AreaID: uint32(current.Area.ID), Progress: 0.85})
		candidate, finishErr := coordinator.Finish(RecordingTerminalEvidence{World: current, Boss: boss})
		if finishErr != nil {
			return finishErr
		}
		frozen = candidate
		reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowReturningViaPortal, AreaID: uint32(current.Area.ID), Progress: 0.9})
		if returnStage == 0 {
			returnStage = 1
		}
		return nil
	}

	rt.Log.Info("guided route recording waiting for confirmed waypoint start", "run_id", runID, "instructions", definition.Recording.InstructionsDE, "finish_hotkey", rt.Config.Input.RecordingFinishHotkey, "emergency_stop_hotkey", rt.Config.Input.StopHotkey)
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreflight})
	lastReportedArea := world.AreaID(0)
	for {
		select {
		case <-ctx.Done():
			coordinator.EmergencyCancel()
			return fmt.Errorf("guided recording cancelled: %w", ctx.Err())
		case event := <-hotkeys:
			if event.Action == input.HotkeyActionRecordingFinish {
				if err := requestFinish(); err != nil {
					return err
				}
				continue
			}
			rt.handleHotkeyEvent(event, cancel)
		case <-finishRequests:
			if err := requestFinish(); err != nil {
				return err
			}
		case <-ticker.C:
			if err := rt.runTick(ctx, runtimeState); err != nil && !errors.Is(err, context.Canceled) {
				coordinator.EmergencyCancel()
				return err
			}
			current := rt.World.Current()
			if !started {
				waypointTolerance := rt.Config.Pathing.Waypoint.MaxClickDistance
				if !guidedRecordingStartReady(current, definition, waypointTolerance) {
					if lastWaitingReport.IsZero() || time.Since(lastWaitingReport) >= 2*time.Second {
						lastWaitingReport = time.Now()
						waypoint, waypointVisible := current.NearestObject(world.ObjectKindWaypoint)
						distance := -1.0
						if waypointVisible {
							distance = world.Distance(current.Player.Position, waypoint.Position)
						}
						rt.Log.Info("guided route recording preflight waiting", "run_id", runID, "world_valid", current.Valid, "phase", current.Phase, "identity_valid", current.Identity.Valid, "area_id", current.Area.ID, "expected_area_id", definition.Recording.AllowedStartArea, "waypoint_visible", waypointVisible, "waypoint_distance", distance, "maximum_distance", waypointTolerance, "inventory_open", current.UI.InventoryOpen, "npc_interact_open", current.UI.NPCInteractOpen, "npc_shop_open", current.UI.NPCShopOpen, "waypoint_flag", current.UI.WaypointOpen, "stash_open", current.UI.StashOpen, "quit_menu_open", current.UI.QuitMenuOpen)
						reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreflight, AreaID: uint32(current.Area.ID), Progress: 0, Reason: string(RouteReasonRecordingPreflightFailed)})
					}
					continue
				}
				if err := rt.Input.Focus(); err != nil {
					return fmt.Errorf("guided recording focus: %w", err)
				}
				request := RecordingPreflight{RunID: runID, Character: expectedCharacter, ExpectedClass: expectedClass, Difficulty: pathing.RouteDifficulty(difficulty), GameVersion: rt.Config.Memory.GameVersion, SourceCatalogRevision: catalog.Revision, SourceAssignmentRevision: assignment.Revision, WaypointContextConfirmed: true, BlockingUIClosed: !blockingRecordingUI(current.UI), D2RFocused: true, InputOwnerAvailable: rt.Input.Status().Enabled && rt.Input.Bound()}
				if err := coordinator.Start(request, current); err != nil {
					return err
				}
				started = true
				lastReportedArea = current.Area.ID
				reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowRecording, AreaID: uint32(current.Area.ID), Segment: recordingAreaIndex(definition, current.Area.ID), Progress: 0.05})
				continue
			}
			if returnStage == 0 {
				if err := coordinator.Tick(ctx, current); err != nil {
					return err
				}
				if current.Area.ID != lastReportedArea {
					lastReportedArea = current.Area.ID
					segment := recordingAreaIndex(definition, current.Area.ID)
					progress := 0.05 + 0.65*float64(segment+1)/float64(len(definition.Recording.AllowedRouteAreas))
					reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowRecording, AreaID: uint32(current.Area.ID), Segment: segment, Progress: progress})
				}
				continue
			}
			if returnStage == 1 {
				if rt.taskDeps.Actions == nil || rt.taskDeps.Portal == nil {
					return rt.completeRecordingReturnFailure(coordinator, fmt.Errorf("town portal actions not wired"))
				}
				rt.taskDeps.Portal.Reset()
				if err := rt.taskDeps.Actions.CastTownPortal(); err != nil {
					return rt.completeRecordingReturnFailure(coordinator, err)
				}
				returnStage = 2
				continue
			}
			if returnStage == 2 && !portalClicked {
				result := rt.taskDeps.Portal.Tick(ctx, current, time.Now())
				if result.Done && result.Status != pathing.TownPortalActionClicked {
					return rt.completeRecordingReturnFailure(coordinator, fmt.Errorf("portal entry failed: status=%s reason=%s", result.Status, result.Reason))
				}
				if result.Status == pathing.TownPortalActionClicked {
					portalClicked = true
				}
				continue
			}
			if portalClicked && current.Valid && current.Area.ID == recordingOriginTownArea(definition.Recording.EgressOriginAct) {
				if err := coordinator.CompleteSafetyReturn(nil); err != nil {
					return err
				}
				reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowCandidateReady, AreaID: uint32(current.Area.ID), Progress: 1})
				rt.Log.Info("guided route candidate ready", "candidate_id", frozen.CandidateID, "run_id", runID, "state", frozen.State, "measured_boss_distance", frozen.MeasuredBossDistance)
				return nil
			}
		}
	}
}

func recordingAreaIndex(definition tasks.RunDefinition, area world.AreaID) int {
	for index, allowed := range definition.Recording.AllowedRouteAreas {
		if allowed == area {
			return index
		}
	}
	return 0
}

func reportRouteWorkflow(reporter RouteWorkflowReporter, progress RouteWorkflowProgress) {
	if reporter != nil {
		reporter(progress)
	}
}

func guidedRecordingStartReady(state world.State, definition tasks.RunDefinition, waypointTolerance float64) bool {
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Identity.Valid || state.Area.ID != definition.Recording.AllowedStartArea || blockingRecordingUI(state.UI) || waypointTolerance <= 0 {
		return false
	}
	waypoint, waypointVisible := state.NearestObject(world.ObjectKindWaypoint)
	return waypointVisible && world.Distance(state.Player.Position, waypoint.Position) <= waypointTolerance
}

func blockingRecordingUI(state world.UIState) bool {
	// D2R keeps the waypoint flag set after a completed waypoint transfer even
	// though the panel is visibly closed. Recording starts read-only and already
	// requires the destination area, waypoint entity and proximity, so this stale
	// bit must not block sampling. Every UI that can hide or redirect gameplay
	// input remains fail-closed.
	return state.InventoryOpen || state.NPCInteractOpen || state.NPCShopOpen || state.StashOpen || state.QuitMenuOpen
}

func recordingBossEvidence(state world.State, descriptor tasks.BossDescriptor) *world.Monster {
	var boss world.Monster
	var ok bool
	if descriptor.RequireSuperUnique {
		boss, ok = state.FindSuperUnique(descriptor.NPCID)
	} else {
		boss, ok = state.FindNPC(descriptor.NPCID)
	}
	if !ok {
		return nil
	}
	return &boss
}

func recordingOriginTownArea(act town.OriginAct) world.AreaID {
	if act == town.OriginAct1 {
		return world.RogueEncampment
	}
	area, _ := town.TownAreaForAct(act)
	return area
}

func (rt *Runtime) completeRecordingReturnFailure(coordinator *RecordingCoordinator, cause error) error {
	_ = coordinator.CompleteSafetyReturn(cause)
	return fmt.Errorf("%s: %w", RouteReasonSafetyReturnFailed, cause)
}
