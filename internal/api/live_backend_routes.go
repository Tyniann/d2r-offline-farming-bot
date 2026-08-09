package api

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

// RouteLibrary returns a path-free Farming catalog projection.
func (b *LiveBackend) RouteLibrary(character string, includeArchived bool) (RouteLibraryDTO, error) {
	manifest, catalog, err := b.lifecycle.Snapshot()
	if err != nil {
		return RouteLibraryDTO{}, err
	}
	assignments, err := b.routeAssignments.Snapshot()
	if err != nil {
		return RouteLibraryDTO{}, err
	}
	filter := strings.ToLower(strings.TrimSpace(character))
	result := RouteLibraryDTO{SchemaVersion: schemaVersion, Revision: catalog.Revision, Character: character, Routes: []RouteEntryDTO{}}
	for _, entry := range catalog.Entries {
		if filter != "" && !strings.EqualFold(entry.Character, filter) {
			continue
		}
		isArchived := entry.ManagementStatus == app.RouteManagementArchived
		if isArchived && !includeArchived {
			continue
		}
		role := entry.Route.Binding.RouteRole
		assigned := assignments.Assignments[strings.ToLower(entry.Character)][entry.RunID] == entry.ID
		if role != "" {
			assigned = assignments.RouteSets[strings.ToLower(entry.Character)][entry.RunID][role] == entry.ID
		}
		result.Routes = append(result.Routes, RouteEntryDTO{RouteID: entry.ID, DisplayName: entry.Route.Name, RunID: string(entry.RunID), RouteRole: string(role), Character: entry.Character, Difficulty: entry.Difficulty, LifecycleStatus: string(entry.Status), ManagementStatus: string(entry.ManagementStatus), Assigned: assigned, Reason: entry.Reason})
	}
	sort.Slice(result.Routes, func(i, j int) bool { return result.Routes[i].RouteID < result.Routes[j].RouteID })
	_ = manifest
	return result, nil
}

// RecordingOptions returns contracts from the authoritative run registry.
func (b *LiveBackend) RecordingOptions() []RecordingOptionDTO {
	b.mu.RLock()
	selection := b.status.Selection
	supervisorState := b.status.State
	workflowState := b.routeWorkflow.State
	b.mu.RUnlock()
	definitions := tasks.DefaultRunRegistry().Definitions()
	result := make([]RecordingOptionDTO, 0, len(definitions))
	for _, definition := range definitions {
		roles := definition.RouteRoles()
		if definition.RouteSet == nil {
			roles = []pathing.RouteRole{""}
		}
		for _, role := range roles {
			contract, _ := definition.RecordingForRole(role)
			areas := make([]uint32, len(contract.AllowedRouteAreas))
			for i, area := range contract.AllowedRouteAreas {
				areas[i] = uint32(area)
			}
			available, reason := true, ""
			if !b.cfg.Input.Enabled {
				available, reason = false, "input_disabled"
			} else if selection.Character == "" || selection.Difficulty == "" {
				available, reason = false, "selection_unconfirmed"
			} else if routeWorkflowBusy(workflowState) {
				available, reason = false, "route_workflow_active"
			} else if supervisorState != string(app.SupervisorStateIdle) && supervisorState != string(app.SupervisorStateIdleInGame) {
				available, reason = false, "session_active"
			}
			displayName := definition.DisplayName
			switch role {
			case pathing.RouteRoleLegAcquisition:
				displayName = "Wirt-Route"
			case pathing.RouteRoleCowSweep:
				displayName = "Cow-Route"
			}
			result = append(result, RecordingOptionDTO{RunID: string(definition.ID), RouteRole: string(role), DisplayName: displayName, InstructionsDE: contract.InstructionsDE, OperatorHintsDE: recordingOperatorHints(role), StartKind: string(contract.StartKind), StartWaypoint: string(contract.StartWaypoint), AllowedStartAreaID: uint32(contract.AllowedStartArea), AllowedRouteAreaIDs: areas, TerminalAreaID: uint32(contract.TerminalArea), TerminalMaxDistanceTiles: contract.TerminalMaxDistanceTiles, Available: available, Reason: reason, Prerequisites: b.recordingPrerequisites(definition, contract)})
		}
	}
	return result
}

func (b *LiveBackend) recordingPrerequisites(definition tasks.RunDefinition, contract tasks.RecordingContract) []RecordingPrerequisiteDTO {
	b.mu.RLock()
	character := b.status.Selection.Character
	b.mu.RUnlock()
	teleport, townPortal := b.characterSkillBindingsReady(character)
	pickitReady := false
	if character != "" {
		_, pickitErr := b.pickitAssignments.Resolve(character, definition.ID)
		pickitReady = pickitErr == nil
	}
	result := []RecordingPrerequisiteDTO{
		{ID: "teleport", Ready: teleport, Reason: prerequisiteReason(teleport, "onboarding_teleport_binding_missing")},
		{ID: "town_portal", Ready: townPortal, Reason: prerequisiteReason(townPortal, "onboarding_town_portal_binding_missing")},
		{ID: "pickit", Ready: pickitReady, Reason: prerequisiteReason(pickitReady, "pickit_assignment_missing")},
	}
	if contract.StartKind != tasks.RecordingStartObjectPortalArrival {
		_, waypointReady := pathing.DefaultWaypointTargetRegistry().Action(contract.StartWaypoint)
		result = append([]RecordingPrerequisiteDTO{{ID: "waypoint", Ready: waypointReady, Reason: prerequisiteReason(waypointReady, "onboarding_waypoint_required")}}, result...)
	}
	return result
}

// characterSkillBindingsReady reports whether Teleport and Town Portal F-keys exist
// on the selected character's active combat profile in OperatorSettings Schema 3.
func (b *LiveBackend) characterSkillBindingsReady(character string) (teleport, townPortal bool) {
	character = strings.TrimSpace(character)
	if character == "" || b.operatorSettings == nil {
		return false, false
	}
	settings, err := b.operatorSettings.Snapshot()
	if err != nil {
		return false, false
	}
	entry, ok := settings.Characters[strings.ToLower(character)]
	if !ok {
		return false, false
	}
	profileID := strings.TrimSpace(entry.CombatProfile)
	if profileID == "" {
		return false, false
	}
	bindings, ok := entry.ProfileBindings[profileID]
	if !ok {
		return false, false
	}
	return operatorSkillKeyBound(bindings, "teleport"), operatorSkillKeyBound(bindings, "town_portal")
}

func operatorSkillKeyBound(bindings app.OperatorProfileBindings, name string) bool {
	for candidate, key := range bindings.Skills {
		if strings.EqualFold(strings.TrimSpace(candidate), name) {
			return strings.TrimSpace(key) != ""
		}
	}
	return false
}

func recordingOperatorHints(role pathing.RouteRole) []string {
	switch role {
	case pathing.RouteRoleLegAcquisition:
		return []string{"Das Tristram-Portal muss bereits geöffnet sein.", "Wirt während Aufnahme und Test nicht anklicken; ein vorheriger Clear ist nicht nötig."}
	case pathing.RouteRoleCowSweep:
		return []string{"Das Cow-Portal in diesem Spiel manuell öffnen.", "Das Cow Level vor der Aufnahme vollständig leeren; Kampf- und Rückteleports verfälschen die Route."}
	default:
		return nil
	}
}

func prerequisiteReason(ready bool, reason string) string {
	if ready {
		return ""
	}
	return reason
}

// RouteCandidates returns immutable candidate identity without filesystem locations.
func (b *LiveBackend) RouteCandidates() ([]RouteCandidateDTO, error) {
	entries, err := b.routeCandidates.List()
	if err != nil {
		return nil, err
	}
	result := make([]RouteCandidateDTO, 0, len(entries))
	for _, entry := range entries {
		result = append(result, RouteCandidateDTO{CandidateID: entry.CandidateID, RunID: string(entry.RunID), RouteRole: string(entry.RouteRole), Character: entry.Character, Difficulty: entry.Difficulty, State: string(entry.State), MeasuredBossDistance: entry.MeasuredBossDistance, RouteSHA256: entry.ImmutableRouteSHA256, Reason: string(entry.FailureReason)})
	}
	return result, nil
}

// SystemRouteStatuses reports setup readiness for global Act 2-5 Egresses.
func (b *LiveBackend) SystemRouteStatuses() []SystemRouteStatusDTO {
	acts := []town.OriginAct{town.OriginAct2, town.OriginAct3, town.OriginAct4, town.OriginAct5}
	result := make([]SystemRouteStatusDTO, 0, len(acts))
	for _, act := range acts {
		entry := SystemRouteStatusDTO{Act: string(act)}
		_, reason := b.cfg.Town.EgressFor(act)
		if reason != "" {
			entry.Reason = string(reason)
		} else {
			if err := b.systemEgressReady(act); err != nil {
				entry.Reason = "egress_missing_or_invalid"
			} else {
				entry.Ready = true
			}
		}
		result = append(result, entry)
	}
	return result
}

func (b *LiveBackend) systemEgressReady(act town.OriginAct) error {
	cfg, reason := b.cfg.Town.EgressFor(act)
	if reason != "" {
		return fmt.Errorf("system Egress unavailable: %s", reason)
	}
	route, err := town.LoadSystemEgressRoute(filepath.Join(b.cfg.ResolvePath(cfg.RoutesDirectory), town.SystemEgressFilename))
	if err != nil {
		return err
	}
	expectedArea, ok := town.TownAreaForAct(act)
	if !ok || route.Contract.Act != act || route.Contract.TownArea != expectedArea || route.Contract.GameVersion != b.cfg.Memory.GameVersion {
		return fmt.Errorf("system Egress contract does not match act or game version")
	}
	return nil
}

// HotkeyHelp returns the effective values read by the Core.
func (b *LiveBackend) HotkeyHelp() HotkeyHelpDTO {
	return HotkeyHelpDTO{RecordingFinish: b.cfg.Input.RecordingFinishHotkey, StopAfterRun: b.cfg.Input.StopAfterRunHotkey, EmergencyStop: b.cfg.Input.StopHotkey, Pause: b.cfg.Input.PauseHotkey}
}

// RouteWorkflow returns the current exclusive workflow projection.
func (b *LiveBackend) RouteWorkflow() RouteWorkflowDTO {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.routeWorkflow
}

// PreviewRouteMutation delegates revision binding to RouteManagementService.
func (b *LiveBackend) PreviewRouteMutation(request RouteMutationPreviewRequest) (RouteMutationPreviewDTO, error) {
	b.mu.RLock()
	workflowState := b.routeWorkflow.State
	selection := b.status.Selection
	compatibility := b.status.Compatibility
	b.mu.RUnlock()
	if compatibility.State != string(app.D2RCompatibilityCompatible) {
		return RouteMutationPreviewDTO{}, compatibilityCommandError(compatibility)
	}
	if routeWorkflowBusy(workflowState) {
		return RouteMutationPreviewDTO{}, fmt.Errorf("route mutation conflicts with active workflow")
	}
	var preview app.RouteMutationPreview
	var err error
	if request.CandidateID != "" {
		candidate, _, loadErr := b.routeCandidates.Load(request.CandidateID)
		if loadErr != nil {
			return RouteMutationPreviewDTO{}, loadErr
		}
		if !strings.EqualFold(selection.Character, candidate.Character) || !strings.EqualFold(selection.Difficulty, candidate.Difficulty) {
			return RouteMutationPreviewDTO{}, fmt.Errorf("live candidate context changed")
		}
		preview, err = b.routeManagement.PreviewCandidate(request.CandidateID)
	} else {
		preview, err = b.routeManagement.PreviewRoute(app.RouteMutationOperation(request.Operation), request.RouteID)
	}
	if err != nil {
		return RouteMutationPreviewDTO{}, err
	}
	return RouteMutationPreviewDTO{Operation: string(preview.Operation), RouteID: preview.RouteID, CandidateID: preview.CandidateID, ReplacedRouteID: preview.PreviousRouteID, CatalogRevision: preview.CatalogRevision, LifecycleRevision: preview.LifecycleRevision, AssignmentRevision: preview.AssignmentRevision, ConfirmationToken: preview.Token}, nil
}

// ConfirmRouteMutation consumes the one-use preview and refreshes the catalog generation.
func (b *LiveBackend) ConfirmRouteMutation(request RouteMutationConfirmRequest) error {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	b.mu.RLock()
	workflowState := b.routeWorkflow.State
	selection := b.status.Selection
	compatibility := b.status.Compatibility
	b.mu.RUnlock()
	if compatibility.State != string(app.D2RCompatibilityCompatible) {
		return compatibilityCommandError(compatibility)
	}
	if routeWorkflowBusy(workflowState) {
		return fmt.Errorf("route mutation conflicts with active workflow")
	}
	// Candidate publication is revalidated immediately before the one-use
	// management capability is consumed; route-only operations have no candidate.
	if preview, ok := b.routeManagement.PreviewForToken(request.ConfirmationToken); ok && preview.CandidateID != "" {
		candidate, _, err := b.routeCandidates.Load(preview.CandidateID)
		if err != nil {
			return err
		}
		if !strings.EqualFold(selection.Character, candidate.Character) || !strings.EqualFold(selection.Difficulty, candidate.Difficulty) {
			return fmt.Errorf("live candidate context changed")
		}
	}
	if err := b.routeManagement.Confirm(app.RouteMutationConfirm{Token: request.ConfirmationToken, ConfirmRouteID: request.ConfirmRouteID}); err != nil {
		return err
	}
	report, refreshErr := app.ResolveRunAvailabilities(b.cfg, app.RunAvailabilityContext{
		Character: selection.Character, Difficulty: selection.Difficulty, GameVersion: b.cfg.Memory.GameVersion,
	})
	b.mu.Lock()
	b.catalog.Revision++
	if refreshErr == nil {
		b.catalog.Runs = runCatalogEntries(report, b.cfg)
	} else {
		// Die Mutation ist zu diesem Zeitpunkt bereits atomar veröffentlicht.
		// Ein fehlgeschlagener Re-Read darf keinen alten, scheinbar verfügbaren
		// Katalog behalten; bis zur nächsten erfolgreichen Projektion gilt fail-closed.
		for i := range b.catalog.Runs {
			b.catalog.Runs[i].Status = "unavailable"
			b.catalog.Runs[i].Reasons = []string{"route_lifecycle_unavailable"}
		}
	}
	generation := b.catalog.Revision
	b.mu.Unlock()
	b.publisher.Publish(routeEvent("route_library_changed", generation, ""))
	if refreshErr != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "runtime_error", Reason: "run_catalog_refresh_failed", Details: map[string]any{"message": refreshErr.Error()}})
	}
	return nil
}

// StartRouteWorkflow starts the already-wired Core workflow asynchronously.
func (b *LiveBackend) StartRouteWorkflow(request RouteWorkflowRequest) (RouteWorkflowDTO, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	var testCandidate *app.RouteCandidate
	switch request.Operation {
	case "record":
		definition, ok := tasks.DefaultRunRegistry().Definition(tasks.RunID(request.RunID))
		if !ok {
			return RouteWorkflowDTO{}, fmt.Errorf("unknown recording run")
		}
		if _, ok := definition.RecordingForRole(pathing.RouteRole(request.RouteRole)); !ok {
			return RouteWorkflowDTO{}, fmt.Errorf("unknown recording route role")
		}
	case "test":
		if strings.TrimSpace(request.CandidateID) == "" {
			return RouteWorkflowDTO{}, fmt.Errorf("candidate ID is required")
		}
		candidate, _, err := b.routeCandidates.Load(request.CandidateID)
		if err != nil {
			return RouteWorkflowDTO{}, err
		}
		testCandidate = &candidate
	case "system_record", "system_test":
		act := town.OriginAct(request.Act)
		if act != town.OriginAct2 && act != town.OriginAct3 && act != town.OriginAct4 && act != town.OriginAct5 {
			return RouteWorkflowDTO{}, fmt.Errorf("unsupported system Egress act")
		}
		readyErr := b.systemEgressReady(act)
		if request.Operation == "system_record" && readyErr == nil {
			return RouteWorkflowDTO{}, fmt.Errorf("system Egress is already ready")
		}
		if request.Operation == "system_test" && readyErr != nil {
			return RouteWorkflowDTO{}, fmt.Errorf("system Egress is missing or invalid")
		}
	default:
		return RouteWorkflowDTO{}, fmt.Errorf("unsupported route workflow")
	}
	b.mu.Lock()
	selection := b.status.Selection
	if err := b.requireCompatibleLocked(); err != nil {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, err
	}
	if request.ExpectedGeneration != b.routeWorkflow.Generation {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("route workflow generation changed")
	}
	if b.status.State != string(app.SupervisorStateIdle) && b.status.State != string(app.SupervisorStateIdleInGame) {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("route workflow conflicts with active session")
	}
	if !b.cfg.Input.Enabled {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("route workflow requires enabled input")
	}
	if request.Operation == "record" && (selection.Character == "" || selection.Difficulty == "") {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("route recording requires confirmed character and difficulty")
	}
	if testCandidate != nil && (!strings.EqualFold(selection.Character, testCandidate.Character) || !strings.EqualFold(selection.Difficulty, testCandidate.Difficulty)) {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("live candidate context changed")
	}
	if routeWorkflowBusy(b.routeWorkflow.State) {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("%s", app.RouteReasonRecordingConflict)
	}
	handler := b.routeWorkflowRun
	if handler == nil {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, fmt.Errorf("route workflow is not available")
	}
	token, err := randomToken(12)
	if err != nil {
		b.mu.Unlock()
		return RouteWorkflowDTO{}, err
	}
	state := app.RouteWorkflowPreflight
	if request.Operation == "test" || request.Operation == "system_test" {
		state = app.RouteWorkflowPreparingPlayback
	}
	runID, character := request.RunID, selection.Character
	if testCandidate != nil {
		runID, character, request.RouteRole = string(testCandidate.RunID), testCandidate.Character, string(testCandidate.RouteRole)
	}
	if request.Operation == "system_record" || request.Operation == "system_test" {
		character = ""
	}
	b.routeWorkflow = RouteWorkflowDTO{WorkflowID: token, Generation: b.routeWorkflow.Generation + 1, State: string(state), RunID: runID, RouteRole: request.RouteRole, Character: character, Act: request.Act}
	finishRequests := make(chan struct{}, 1)
	b.routeWorkflowFinish = finishRequests
	snapshot := b.routeWorkflow
	b.mu.Unlock()
	b.publisher.Publish(routeWorkflowEvent("route_workflow_changed", snapshot))
	report := func(progress app.RouteWorkflowProgress) {
		b.mu.Lock()
		if b.routeWorkflow.WorkflowID != token || !routeWorkflowBusy(b.routeWorkflow.State) {
			b.mu.Unlock()
			return
		}
		if b.routeWorkflow.State == string(progress.State) && b.routeWorkflow.AreaID == progress.AreaID && b.routeWorkflow.Segment == progress.Segment && b.routeWorkflow.Progress == progress.Progress && b.routeWorkflow.Reason == progress.Reason {
			b.mu.Unlock()
			return
		}
		b.routeWorkflow.Generation++
		b.routeWorkflow.State = string(progress.State)
		b.routeWorkflow.AreaID = progress.AreaID
		b.routeWorkflow.Segment = progress.Segment
		b.routeWorkflow.Progress = progress.Progress
		b.routeWorkflow.Reason = progress.Reason
		updated := b.routeWorkflow
		b.mu.Unlock()
		b.publisher.Publish(routeWorkflowEvent("route_workflow_changed", updated))
	}
	go func() {
		runErr := handler(request, finishRequests, report)
		b.mu.Lock()
		b.routeWorkflow.Generation++
		if runErr != nil {
			if strings.Contains(strings.ToLower(runErr.Error()), "cancel") {
				b.routeWorkflow.State = string(app.RouteWorkflowEmergencyCancelled)
			} else {
				b.routeWorkflow.State = string(app.RouteWorkflowFailedSafe)
			}
			b.routeWorkflow.Reason = runErr.Error()
		} else {
			b.routeWorkflow.State = string(app.RouteWorkflowCompleted)
			b.routeWorkflow.Reason = ""
		}
		completed := b.routeWorkflow
		b.routeWorkflowFinish = nil
		b.mu.Unlock()
		b.publisher.Publish(routeWorkflowEvent("route_workflow_changed", completed))
	}()
	return snapshot, nil
}

// FinishRouteWorkflow submits the same one-shot finish intent as F9.
func (b *LiveBackend) FinishRouteWorkflow(workflowID string, request RouteWorkflowFinishRequest) (RouteWorkflowDTO, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.requireCompatibleLocked(); err != nil {
		return RouteWorkflowDTO{}, err
	}
	if workflowID == "" || workflowID != b.routeWorkflow.WorkflowID {
		return RouteWorkflowDTO{}, fmt.Errorf("route workflow ID changed")
	}
	if b.routeWorkflow.State != string(app.RouteWorkflowRecording) {
		switch b.routeWorkflow.State {
		case string(app.RouteWorkflowFreezing), string(app.RouteWorkflowValidating), string(app.RouteWorkflowReturningViaPortal), string(app.RouteWorkflowCandidateReady), string(app.RouteWorkflowCompleted):
			return b.routeWorkflow, nil
		default:
			return RouteWorkflowDTO{}, fmt.Errorf("route workflow is not recording")
		}
	}
	if request.ExpectedGeneration != b.routeWorkflow.Generation || b.routeWorkflowFinish == nil {
		return RouteWorkflowDTO{}, fmt.Errorf("route workflow generation changed")
	}
	select {
	case b.routeWorkflowFinish <- struct{}{}:
	default:
	}
	b.routeWorkflow.Generation++
	b.routeWorkflow.State = string(app.RouteWorkflowFreezing)
	snapshot := b.routeWorkflow
	b.publisher.Publish(routeWorkflowEvent("route_workflow_changed", snapshot))
	return snapshot, nil
}

func routeEvent(name string, generation uint64, state string) telemetry.LiveEvent {
	return telemetry.LiveEvent{Event: name, State: state, Details: map[string]any{"generation": generation}}
}

func routeWorkflowEvent(name string, workflow RouteWorkflowDTO) telemetry.LiveEvent {
	return telemetry.LiveEvent{Event: name, WorkflowID: workflow.WorkflowID, State: workflow.State, Run: workflow.RunID, Act: workflow.Act, AreaID: workflow.AreaID, Segment: workflow.Segment, Progress: workflow.Progress, Reason: workflow.Reason, Details: map[string]any{"generation": workflow.Generation}}
}
