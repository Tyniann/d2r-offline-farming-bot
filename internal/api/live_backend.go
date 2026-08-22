package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// LiveBackend projects read-only runtime changes and publishes bounded events.
type LiveBackend struct {
	mu                     sync.RWMutex
	commandMu              sync.Mutex
	bootstrap              *BootstrapBackend
	cfg                    *config.Config
	lifecycle              *app.RouteLifecycleStore
	publisher              *telemetry.LivePublisher
	status                 StatusDTO
	catalog                CatalogDTO
	selection              func(app.CharacterSelectionRequest) error
	supervisor             *app.SessionSupervisor
	beforeWorker           func() error
	beginQueue             func(bool) error
	loadoutResolver        *app.CharacterLoadoutResolver
	supervisorObserved     uint64
	commands               map[string]apiCommandRecord
	characterCommands      map[string]characterSetupCommandRecord
	previews               map[string]selectionPreviewRecord
	routeCandidates        *app.CandidateStore
	routeAssignments       *app.RouteAssignmentStore
	routeManagement        *app.RouteManagementService
	routeWorkflow          RouteWorkflowDTO
	routeWorkflowRun       func(RouteWorkflowRequest, <-chan struct{}, app.RouteWorkflowReporter) error
	routeWorkflowFinish    chan struct{}
	pickitProfiles         *app.PickitProfileService
	pickitAssignments      *app.PickitAssignmentStore
	operatorSettings       *app.OperatorSettingsStore
	characterCatalog       *app.CharacterCatalogStore
	characterCatalogReload func() (app.CharacterCatalog, error)
	characterSetup         *app.CharacterSetupService
	characterCapture       app.CharacterSetupCaptureFunc
	historyMu              sync.Mutex
	history                *telemetry.HistoryIndex
	historyMaintenance     *telemetry.HistoryMaintenanceService
	diagnostics            *app.DiagnosticBundleCollector
	historyTerminalHash    [sha256.Size]byte
}

// SetSessionSupervisor binds the single Core-owned queue state machine. The
// preparation hook stops the passive monitor before a worker can attach/input.
func (b *LiveBackend) SetSessionSupervisor(supervisor *app.SessionSupervisor, beforeWorker func() error, beginQueue func(bool) error) error {
	if supervisor == nil {
		return fmt.Errorf("session supervisor is required")
	}
	if err := supervisor.SetQueueGuard(func(plan app.FarmQueuePlan, index int) error {
		runIDs := plan.RunIDs
		if index >= 0 && index < len(plan.RunIDs) {
			runIDs = []string{plan.RunIDs[index]}
		}
		if _, _, err := b.validateDesktopCharacterContract(plan.Character, runIDs); err != nil {
			return err
		}
		b.mu.RLock()
		selection := b.status.Selection
		revision := b.catalog.Revision
		b.mu.RUnlock()
		_, err := app.ValidateFarmQueue(b.cfg, app.FarmQueueValidationRequest{
			RunIDs: plan.RunIDs, Character: plan.Character, Difficulty: plan.Difficulty, CatalogRevision: plan.CatalogRevision,
		}, b.farmQueueValidationContext(selection.Character, selection.Difficulty, revision))
		return err
	}); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.supervisor = supervisor
	b.beforeWorker = beforeWorker
	b.beginQueue = beginQueue
	b.supervisorObserved = supervisor.Snapshot().Generation
	return nil
}

// SetLoadoutResolver binds the Schema-3 character loadout authority for queue preflight and freeze.
func (b *LiveBackend) SetLoadoutResolver(resolver *app.CharacterLoadoutResolver) {
	b.mu.Lock()
	b.loadoutResolver = resolver
	b.mu.Unlock()
}

func (b *LiveBackend) evaluateQueueLoadout(character string) error {
	b.mu.RLock()
	store := b.operatorSettings
	profiles := b.cfg.Profiles
	b.mu.RUnlock()
	if store == nil {
		return &commandError{code: string(app.QueueReasonProfileBindingsIncomplete)}
	}
	settings, err := store.Snapshot()
	if err != nil {
		return err
	}
	if reasons := app.EvaluateLoadoutReadiness(settings, character, profiles); len(reasons) > 0 {
		return &commandError{code: reasons[0]}
	}
	return nil
}

func (b *LiveBackend) queueLoadoutWarnings(character string, runIDs []string) []string {
	includesCow := false
	for _, runID := range runIDs {
		if runID == string(tasks.RunIDCows) {
			includesCow = true
			break
		}
	}
	if !includesCow {
		return nil
	}
	b.mu.RLock()
	store := b.operatorSettings
	b.mu.RUnlock()
	if store == nil {
		return nil
	}
	settings, err := store.Snapshot()
	if err != nil {
		return nil
	}
	value, ok := settings.Characters[strings.ToLower(strings.TrimSpace(character))]
	if !ok || value.InventoryLock == nil {
		return nil
	}
	if app.InventoryCowSuitable(value.InventoryLock.Grid) {
		return nil
	}
	return []string{"inventory_layout_unsuitable_for_cows"}
}

type selectionPreviewRecord struct {
	dto       SelectionPreviewDTO
	lifecycle app.RouteLifecyclePreview
}

type apiCommandRecord struct {
	name       string
	generation uint64
	payload    string
	response   CommandResponse
}

// NewLiveBackend creates an idle live projection from the existing resolver.
func NewLiveBackend(cfg *config.Config, publisher *telemetry.LivePublisher) (*LiveBackend, error) {
	if publisher == nil {
		return nil, fmt.Errorf("live event publisher is required")
	}
	lifecycle, err := app.NewRouteLifecycleStore(cfg)
	if err != nil {
		return nil, err
	}
	manifest, _, err := lifecycle.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("load route lifecycle: %w", err)
	}
	bootstrap, err := NewBootstrapBackend(cfg)
	if err != nil {
		return nil, err
	}
	characterCatalog, err := app.NewCharacterCatalogStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("character catalog: %w", err)
	}
	candidates, err := app.NewCandidateStore(cfg)
	if err != nil {
		return nil, err
	}
	assignments, err := app.NewRouteAssignmentStore(cfg)
	if err != nil {
		return nil, err
	}
	management, err := app.NewRouteManagementService(cfg, app.RouteManagementHooks{})
	if err != nil {
		return nil, err
	}
	pickitProfiles, err := app.NewPickitProfileService(cfg.ResolvePath("pickit/profiles"))
	if err != nil {
		return nil, fmt.Errorf("pickit profiles: %w", err)
	}
	if setupErr := app.ValidateCharacterSetupConfig(cfg, pickitProfiles); setupErr != nil {
		return nil, fmt.Errorf("character setup config: %w", setupErr)
	}
	pickitAssignments, err := app.NewPickitAssignmentStore(cfg.ResolvePath("pickit-assignments.local.yaml"), pickitProfiles)
	if err != nil {
		return nil, fmt.Errorf("pickit assignments: %w", err)
	}
	historyRoot, err := filepath.Abs(cfg.Telemetry.Directory)
	if err != nil {
		return nil, fmt.Errorf("resolve history root: %w", err)
	}
	history, err := telemetry.NewHistoryIndex(historyRoot)
	if err != nil {
		return nil, fmt.Errorf("history index: %w", err)
	}
	if refreshErr := history.Refresh(); refreshErr != nil {
		return nil, fmt.Errorf("initialize history index: %w", refreshErr)
	}
	maintenance, err := telemetry.NewHistoryMaintenanceService(historyRoot, history)
	if err != nil {
		return nil, fmt.Errorf("history maintenance: %w", err)
	}
	var diagnostics *app.DiagnosticBundleCollector
	if cfg.DataRoot != "" {
		diagnostics, err = app.NewDiagnosticBundleCollector(cfg.DataRoot, history)
		if err != nil {
			return nil, fmt.Errorf("diagnostic bundle collector: %w", err)
		}
	}
	status := bootstrap.Status()
	status.D2R = D2RDTO{State: "detached"}
	status.Input = InputDTO{Enabled: false}
	status.World = WorldDTO{Phase: "unknown"}
	status.Queue = QueueStatusDTO{
		Entries: append([]string(nil), cfg.Session.Queue...), DefaultEntries: append([]string(nil), cfg.Session.Queue...),
		Budgets: QueueBudgetsDTO{
			MaxRuns: cfg.Session.MaxRuns, MaxDurationMs: int64(cfg.Session.MaxDurationMs),
			MaxConsecutiveFailures: cfg.Session.MaxConsecutiveFailures, MaxTotalRestarts: cfg.Session.MaxTotalRestarts,
		},
	}
	if character := manifest.Characters[strings.ToLower(cfg.Session.Character)]; character.LastConfirmedDifficulty != "" {
		status.Selection = SelectionStatusDTO{Character: cfg.Session.Character, Difficulty: character.LastConfirmedDifficulty}
	}
	historySnapshot := history.Snapshot("")
	return &LiveBackend{bootstrap: bootstrap, cfg: cfg, lifecycle: lifecycle, publisher: publisher, status: status, catalog: bootstrap.Catalog(), commands: make(map[string]apiCommandRecord), characterCommands: make(map[string]characterSetupCommandRecord), previews: make(map[string]selectionPreviewRecord), routeCandidates: candidates, routeAssignments: assignments, routeManagement: management, routeWorkflow: RouteWorkflowDTO{Generation: 1, State: string(app.RouteWorkflowIdle)}, pickitProfiles: pickitProfiles, pickitAssignments: pickitAssignments, characterCatalog: characterCatalog, characterCatalogReload: characterCatalog.Reload, history: history, historyMaintenance: maintenance, diagnostics: diagnostics, historyTerminalHash: terminalHistoryHash(historySnapshot.Runs)}, nil
}

// SetRouteWorkflowHandler binds UI workflow starts to the existing Runtime recorder/test adapters.
func (b *LiveBackend) SetRouteWorkflowHandler(handler func(RouteWorkflowRequest, <-chan struct{}, app.RouteWorkflowReporter) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.routeWorkflowRun = handler
}

// SetSelectionHandler binds the single Core-owned character activation flow.
func (b *LiveBackend) SetSelectionHandler(handler func(app.CharacterSelectionRequest) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.selection = handler
}

// UpdateRuntime updates passive component state without overwriting a command-owned supervisor state.
func (b *LiveBackend) UpdateRuntime(runtime app.UIStatusSnapshot) {
	b.mu.Lock()
	previous := b.status
	b.status.D2R = D2RDTO{State: runtime.ProcessState, PID: runtime.PID, WindowBound: runtime.WindowBound, ClientWidth: runtime.ClientWidth, ClientHeight: runtime.ClientHeight}
	b.status.Compatibility = compatibilityDTO(runtime.Compatibility)
	b.status.Input = InputDTO{Enabled: runtime.InputEnabled && runtime.Compatibility.State == app.D2RCompatibilityCompatible, Paused: runtime.InputPaused, Stopped: runtime.InputStopped}
	b.status.World = WorldDTO{Valid: runtime.WorldValid, Phase: runtime.WorldPhase, AreaID: runtime.AreaID, AreaName: runtime.AreaName}
	b.status.RunProgress = runProgressDTO(runtime.RunProgress)
	if runtime.RunID != "" {
		b.status.ActiveRunID = runtime.RunID
	}
	if runtime.LastError != "" {
		b.status.LastError = &ProblemDTO{Code: "runtime_read_failed"}
	} else if b.status.LastError != nil && b.status.LastError.Code == "runtime_read_failed" {
		b.status.LastError = nil
	}
	status := b.status
	status.Queue.Entries = append(make([]string, 0, len(b.status.Queue.Entries)), b.status.Queue.Entries...)
	status.Queue.DefaultEntries = append(make([]string, 0, len(b.status.Queue.DefaultEntries)), b.status.Queue.DefaultEntries...)
	b.mu.Unlock()
	b.publishStatusDeltas(previous, status)
}

// UpdateSupervisor projects asynchronous worker transitions while keeping the
// externally visible Core generation strictly monotonic across selection and queue commands.
func (b *LiveBackend) UpdateSupervisor(supervisor app.SupervisorSnapshot) {
	b.mu.Lock()
	previous := b.status
	if supervisor.Generation > b.supervisorObserved {
		b.status.Generation += supervisor.Generation - b.supervisorObserved
		b.supervisorObserved = supervisor.Generation
	}
	b.status.State = string(supervisor.State)
	b.status.LifecyclePhase = string(supervisor.State)
	b.status.PendingIntent = string(supervisor.PendingIntent)
	b.status.ActiveRunID = supervisor.ActiveRunID
	b.status.RunInstanceID = supervisor.RunInstanceID
	b.status.GameID = supervisor.GameID
	if supervisor.ActiveRunID == "" {
		b.status.RunProgress = nil
	}
	queue := queueStatusDTO(supervisor)
	queue.DefaultEntries = append(make([]string, 0, len(b.status.Queue.DefaultEntries)), b.status.Queue.DefaultEntries...)
	b.status.Queue = queue
	if supervisor.LastResult.Disposition == "" && supervisor.LastResult.Reason == "" {
		b.status.LastResult = nil
	} else {
		b.status.LastResult = &SessionResultDTO{Disposition: string(supervisor.LastResult.Disposition), Reason: supervisor.LastResult.Reason}
	}
	if supervisor.LastResult.Reason != "" && supervisor.State == app.SupervisorStateStoppedError {
		b.status.LastError = &ProblemDTO{Code: supervisor.LastResult.Reason}
	} else if b.status.LastError != nil && b.status.LastError.Code == "session_stopped" {
		b.status.LastError = nil
	}
	status := b.status
	b.mu.Unlock()
	b.publishStatusDeltas(previous, status)
}

// Update atomically publishes a new Core projection and meaningful deltas.
func (b *LiveBackend) Update(runtime app.UIStatusSnapshot, supervisor app.SupervisorSnapshot) {
	b.mu.Lock()
	previous := b.status
	status := StatusDTO{
		CoreVersion: previous.CoreVersion, AppVersion: previous.AppVersion, State: string(supervisor.State), Generation: supervisor.Generation,
		LifecyclePhase: string(supervisor.State), PendingIntent: string(supervisor.PendingIntent), ActiveRunID: supervisor.ActiveRunID,
		RunInstanceID: supervisor.RunInstanceID, GameID: supervisor.GameID, RunProgress: runProgressDTO(runtime.RunProgress),
		D2R:           D2RDTO{State: runtime.ProcessState, PID: runtime.PID, WindowBound: runtime.WindowBound, ClientWidth: runtime.ClientWidth, ClientHeight: runtime.ClientHeight},
		Compatibility: compatibilityDTO(runtime.Compatibility),
		Input:         InputDTO{Enabled: runtime.InputEnabled && runtime.Compatibility.State == app.D2RCompatibilityCompatible, Paused: runtime.InputPaused, Stopped: runtime.InputStopped},
		World:         WorldDTO{Valid: runtime.WorldValid, Phase: runtime.WorldPhase, AreaID: runtime.AreaID, AreaName: runtime.AreaName},
		Selection:     previous.Selection,
		Queue:         previous.Queue,
	}
	if supervisor.QueueKnown {
		status.Queue = queueStatusDTO(supervisor)
		status.Queue.DefaultEntries = append(make([]string, 0, len(previous.Queue.DefaultEntries)), previous.Queue.DefaultEntries...)
	}
	if runtime.LastError != "" {
		status.LastError = &ProblemDTO{Code: "runtime_read_failed"}
	}
	b.status = status
	b.mu.Unlock()
	b.publishStatusDeltas(previous, status)
}

func (b *LiveBackend) publishStatusDeltas(previous, status StatusDTO) {
	if previous.State != status.State || previous.Generation != status.Generation {
		b.publisher.Publish(telemetry.LiveEvent{Event: "supervisor_state_changed", GameID: status.GameID, RunID: status.RunInstanceID, Run: status.ActiveRunID, Details: map[string]any{"state": status.State, "generation": status.Generation}})
	}
	if previous.GameID != status.GameID {
		if previous.GameID != "" {
			b.publisher.Publish(telemetry.LiveEvent{Event: "game_exited", GameID: previous.GameID, RunID: previous.RunInstanceID, Run: previous.ActiveRunID})
		}
		if status.GameID != "" {
			b.publisher.Publish(telemetry.LiveEvent{Event: "game_started", GameID: status.GameID, RunID: status.RunInstanceID, Run: status.ActiveRunID, Details: map[string]any{"cycle": status.Queue.Cycle}})
		}
	}
	if previous.RunInstanceID != status.RunInstanceID {
		if previous.RunInstanceID != "" {
			b.publisher.Publish(telemetry.LiveEvent{Event: "run_finished", GameID: previous.GameID, RunID: previous.RunInstanceID, Run: previous.ActiveRunID})
		}
		if status.RunInstanceID != "" {
			b.publisher.Publish(telemetry.LiveEvent{Event: "run_started", GameID: status.GameID, RunID: status.RunInstanceID, Run: status.ActiveRunID, Details: map[string]any{"queue_index": status.Queue.Index, "cycle": status.Queue.Cycle}})
		}
		if previous.RunInstanceID != "" {
			_, _ = b.refreshHistory(status.RunInstanceID)
		}
	}
	if previous.D2R != status.D2R {
		b.publisher.Publish(telemetry.LiveEvent{Event: "d2r_state_changed", Details: map[string]any{"state": status.D2R.State, "window_bound": status.D2R.WindowBound}})
	}
	if previous.Compatibility != status.Compatibility {
		b.publisher.Publish(telemetry.LiveEvent{Event: "compatibility_changed", Reason: status.Compatibility.Reason, Details: map[string]any{"state": status.Compatibility.State, "supported_version": status.Compatibility.SupportedVersion, "expected_version": status.Compatibility.ExpectedVersion, "offset_version": status.Compatibility.OffsetVersion, "actual_version": status.Compatibility.ActualVersion, "privilege_mismatch": status.Compatibility.PrivilegeMismatch}})
	}
	if previous.Input != status.Input {
		b.publisher.Publish(telemetry.LiveEvent{Event: "input_state_changed", Details: map[string]any{"enabled": status.Input.Enabled, "paused": status.Input.Paused, "stopped": status.Input.Stopped}})
	}
	if previous.World.AreaID != status.World.AreaID || previous.World.AreaName != status.World.AreaName {
		b.publisher.Publish(telemetry.LiveEvent{Event: "area_changed", AreaID: status.World.AreaID, Area: status.World.AreaName})
	}
	if previous.World.Valid != status.World.Valid || previous.World.Phase != status.World.Phase {
		b.publisher.Publish(telemetry.LiveEvent{Event: "world_state_changed", Details: map[string]any{"valid": status.World.Valid, "phase": status.World.Phase}})
	}
	if !equalRunProgress(previous.RunProgress, status.RunProgress) && status.RunProgress != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "run_progress_changed", GameID: status.GameID, RunID: status.RunInstanceID, Run: status.ActiveRunID, Details: map[string]any{"stage_code": status.RunProgress.StageCode, "params": status.RunProgress.Params, "current": status.RunProgress.Current, "total": status.RunProgress.Total}})
	}
	if status.LastError != nil && (previous.LastError == nil || previous.LastError.Code != status.LastError.Code || !reflect.DeepEqual(previous.LastError.Params, status.LastError.Params)) {
		b.publisher.Publish(telemetry.LiveEvent{Event: "runtime_error", Reason: status.LastError.Code, Details: status.LastError.Params})
	}
	if previous.LastError != nil && status.LastError == nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "runtime_error_cleared"})
	}
	if status.LastResult != nil && (previous.LastResult == nil || *previous.LastResult != *status.LastResult) {
		b.publisher.Publish(telemetry.LiveEvent{Event: "session_result", Reason: status.LastResult.Reason, Details: map[string]any{"disposition": status.LastResult.Disposition}})
	}
}

func runProgressDTO(progress *tasks.RunProgress) *RunProgressDTO {
	if progress == nil || progress.StageCode == "" || progress.Total < 1 || progress.Current < 1 || progress.Current > progress.Total {
		return nil
	}
	return &RunProgressDTO{StageCode: progress.StageCode, Params: progress.Params, Current: progress.Current, Total: progress.Total}
}

func equalRunProgress(left, right *RunProgressDTO) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return reflect.DeepEqual(left, right)
}

// History refreshes JSONL and returns one defensive analyzer generation.
func (b *LiveBackend) History(filter telemetry.HistoryFilter) (historyData, error) {
	if b == nil || b.history == nil {
		return historyData{}, fmt.Errorf("%s: history is unavailable", telemetry.HistoryReasonUnavailable)
	}
	b.mu.RLock()
	activeRunID := b.status.RunInstanceID
	b.mu.RUnlock()
	snapshot, err := b.refreshHistory(activeRunID)
	if err != nil {
		return historyData{}, err
	}
	analysis, err := telemetry.AnalyzeHistory(snapshot, filter)
	if err != nil {
		return historyData{}, err
	}
	return historyData{analysis: analysis, snapshot: snapshot}, nil
}

func (b *LiveBackend) refreshHistory(activeRunID string) (telemetry.HistorySnapshot, error) {
	b.historyMu.Lock()
	defer b.historyMu.Unlock()
	if b.history == nil {
		return telemetry.HistorySnapshot{}, nil
	}
	if err := b.history.Refresh(); err != nil {
		return telemetry.HistorySnapshot{}, fmt.Errorf("refresh history index: %w", err)
	}
	snapshot := b.history.Snapshot(activeRunID)
	terminalHash := terminalHistoryHash(snapshot.Runs)
	if terminalHash != b.historyTerminalHash {
		b.historyTerminalHash = terminalHash
		b.publisher.Publish(telemetry.LiveEvent{Event: "history_changed", Details: map[string]any{"generation": snapshot.Generation}})
	}
	return snapshot, nil
}

// terminalHistoryHash changes only when the correlated terminal population
// changes. In-progress writer events remain queryable through manual refresh,
// but do not create an SSE reload storm for every flushed telemetry line.
func terminalHistoryHash(runs []telemetry.HistoryRun) [sha256.Size]byte {
	hash := sha256.New()
	for _, run := range runs {
		if run.EndedAt == nil {
			continue
		}
		// HistoryRun contains only JSON-compatible, reader-owned values. Hash the
		// complete terminal projection so even an isolated external file rewrite
		// causes one refresh signal without exposing its payload through SSE.
		encoded, _ := json.Marshal(run)
		_, _ = hash.Write(encoded)
		_, _ = hash.Write([]byte{0})
	}
	var result [sha256.Size]byte
	copy(result[:], hash.Sum(nil))
	return result
}

// ValidateQueue performs a side-effect-free full preflight against the confirmed selection.
func (b *LiveBackend) ValidateQueue(request QueueValidationRequest) (QueueValidationDTO, error) {
	_, freshRevision, contractErr := b.validateDesktopCharacterContract(request.Character, request.Entries)
	if contractErr != nil {
		return QueueValidationDTO{}, contractErr
	}
	b.mu.RLock()
	selection := b.status.Selection
	workflowState := b.routeWorkflow.State
	b.mu.RUnlock()
	if routeWorkflowBusy(workflowState) {
		return QueueValidationDTO{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "queue_validation"}}
	}
	plan, err := app.ValidateFarmQueue(b.cfg, app.FarmQueueValidationRequest{
		RunIDs: append([]string(nil), request.Entries...), Character: request.Character,
		Difficulty: request.Difficulty, CatalogRevision: request.CatalogRevision,
	}, b.farmQueueValidationContext(selection.Character, selection.Difficulty, freshRevision))
	if err != nil {
		var queueErr *app.QueueValidationError
		if errors.As(err, &queueErr) {
			return QueueValidationDTO{}, queueCommandError(queueErr)
		}
		var supervisorErr *app.SupervisorCommandError
		if errors.As(err, &supervisorErr) {
			return QueueValidationDTO{}, &commandError{code: string(supervisorErr.Code), cause: fmt.Errorf("validate queue: %w", err)}
		}
		return QueueValidationDTO{}, err
	}
	if loadoutErr := b.evaluateQueueLoadout(plan.Character); loadoutErr != nil {
		return QueueValidationDTO{}, loadoutErr
	}
	return QueueValidationDTO{
		Entries: append([]string(nil), plan.RunIDs...), Character: plan.Character, Difficulty: plan.Difficulty,
		CatalogRevision: plan.CatalogRevision, Budgets: queueBudgetsDTO(plan.Budgets),
		Warnings: append([]string(nil), b.queueLoadoutWarnings(plan.Character, plan.RunIDs)...),
	}, nil
}

// Status returns an immutable current projection.
func (b *LiveBackend) Status() StatusDTO {
	b.mu.RLock()
	defer b.mu.RUnlock()
	status := b.status
	status.Queue.Entries = append(make([]string, 0, len(b.status.Queue.Entries)), b.status.Queue.Entries...)
	status.Queue.DefaultEntries = append(make([]string, 0, len(b.status.Queue.DefaultEntries)), b.status.Queue.DefaultEntries...)
	if b.status.LastError != nil {
		copyOfError := *b.status.LastError
		status.LastError = &copyOfError
	}
	if b.status.LastResult != nil {
		copyOfResult := *b.status.LastResult
		status.LastResult = &copyOfResult
	}
	return status
}

// Catalog delegates to the bootstrap catalog and enriches farm-ready loadout readiness.
func (b *LiveBackend) Catalog() CatalogDTO {
	b.mu.RLock()
	catalog := cloneCatalogDTO(b.catalog)
	store := b.operatorSettings
	profiles := b.cfg.Profiles
	b.mu.RUnlock()
	return enrichCatalogFarmReady(catalog, store, profiles)
}

func enrichCatalogFarmReady(catalog CatalogDTO, store *app.OperatorSettingsStore, profiles config.ProfilesConfig) CatalogDTO {
	var settings app.OperatorSettings
	if store != nil {
		if snapshot, err := store.Snapshot(); err == nil {
			settings = snapshot
		}
	}
	for i := range catalog.Characters {
		catalog.Characters[i] = withFarmReady(catalog.Characters[i], settings, profiles)
	}
	return catalog
}

func withFarmReady(entry CharacterCatalogEntry, settings app.OperatorSettings, profiles config.ProfilesConfig) CharacterCatalogEntry {
	entry.FarmReadyReasons = nil
	entry.FarmReady = false
	if !entry.Selectable {
		return entry
	}
	reasons := app.EvaluateLoadoutReadiness(settings, entry.Name, profiles)
	if len(reasons) > 0 {
		entry.FarmReadyReasons = append([]string(nil), reasons...)
		return entry
	}
	entry.FarmReady = true
	return entry
}

// PreviewSelection computes and stores only an in-memory revision-bound confirmation capability.
func (b *LiveBackend) PreviewSelection(request SelectionPreviewRequest) (SelectionPreviewDTO, error) {
	if _, err := b.reloadCharacterCatalog(); err != nil {
		return SelectionPreviewDTO{}, err
	}
	b.mu.RLock()
	catalogRevision := b.catalog.Revision
	state := b.status.State
	routeState := b.routeWorkflow.State
	difficulties := append([]DifficultyCatalogEntry(nil), b.catalog.Difficulties...)
	b.mu.RUnlock()
	if routeWorkflowBusy(routeState) {
		return SelectionPreviewDTO{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "selection"}}
	}
	if request.CatalogRevision != catalogRevision {
		return SelectionPreviewDTO{}, &commandError{code: "state_changed"}
	}
	if state != string(app.SupervisorStateIdle) && state != string(app.SupervisorStateIdleInGame) {
		return SelectionPreviewDTO{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "selection"}}
	}
	entry, ok := b.bootstrap.character(request.Character)
	if !ok || !entry.Selectable || entry.CombatProfile == "" {
		return SelectionPreviewDTO{}, &commandError{code: app.CharacterReasonProfileMissing}
	}
	if !knownDifficulty(difficulties, request.Difficulty) {
		return SelectionPreviewDTO{}, &commandError{code: "request_invalid", params: map[string]any{"field": "difficulty"}}
	}
	preview, err := b.lifecycle.Preview(entry.Name, request.Difficulty)
	if err != nil {
		return SelectionPreviewDTO{}, &commandError{code: lifecycleErrorCode(err), cause: fmt.Errorf("read route lifecycle for selection preview: %w", err)}
	}
	token, err := randomToken(32)
	if err != nil {
		return SelectionPreviewDTO{}, err
	}
	dto := SelectionPreviewDTO{
		Character: entry.Name, OldDifficulty: preview.OldDifficulty, NewDifficulty: preview.NewDifficulty,
		AffectedRoutes: append([]string(nil), preview.AffectedRoutes...), InvalidationReason: preview.Reason,
		RequiresConfirmation: preview.Reason != "", ConfirmationToken: token,
		CatalogRevision: request.CatalogRevision, LifecycleRevision: preview.Revision,
	}
	b.mu.Lock()
	// Preview capabilities are short-lived process memory. Bounding them avoids
	// turning repeated read-only previews into an unbounded allocation surface.
	if len(b.previews) >= 64 {
		b.previews = make(map[string]selectionPreviewRecord)
	}
	b.previews[token] = selectionPreviewRecord{dto: dto, lifecycle: preview}
	b.mu.Unlock()
	return dto, nil
}

// Command serializes all mutations through the Core-owned selection or session boundary.
func (b *LiveBackend) Command(name string, request CommandRequest) (CommandResponse, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	if name == "apply_selection" {
		return b.applySelectionCommand(request)
	}
	return b.sessionCommand(name, request)
}

func (b *LiveBackend) applySelectionCommand(request CommandRequest) (CommandResponse, error) {
	if _, err := b.reloadCharacterCatalog(); err != nil {
		return CommandResponse{}, err
	}
	b.mu.Lock()
	if err := b.requireCompatibleLocked(); err != nil {
		b.mu.Unlock()
		return CommandResponse{}, err
	}
	if response, ok, err := b.replayCommandLocked("apply_selection", request); ok || err != nil {
		b.mu.Unlock()
		return response, err
	}
	if request.ExpectedGeneration != b.status.Generation || (b.status.State != string(app.SupervisorStateIdle) && b.status.State != string(app.SupervisorStateIdleInGame)) {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "state_changed"}
	}
	if routeWorkflowBusy(b.routeWorkflow.State) {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "selection"}}
	}
	var payload struct {
		Character         string `json:"character"`
		Difficulty        string `json:"difficulty"`
		CatalogRevision   uint64 `json:"catalog_revision"`
		ConfirmationToken string `json:"confirmation_token"`
	}
	decoder := json.NewDecoder(bytes.NewReader(request.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "selection"}}
	}
	entry, ok := b.bootstrap.character(payload.Character)
	previewRecord, previewOK := b.previews[payload.ConfirmationToken]
	if !ok || !entry.Selectable || entry.CombatProfile == "" || payload.Difficulty == "" || !previewOK ||
		previewRecord.dto.Character != entry.Name || previewRecord.dto.NewDifficulty != payload.Difficulty ||
		previewRecord.dto.CatalogRevision != payload.CatalogRevision || payload.CatalogRevision != b.catalog.Revision {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "selection_confirmation_invalid"}
	}
	if !b.status.Input.Enabled || b.status.Input.Paused || b.status.Input.Stopped {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "input_not_ready"}
	}
	manifest, _, lifecycleErr := b.lifecycle.Snapshot()
	if lifecycleErr != nil || manifest.Revision != previewRecord.dto.LifecycleRevision {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "selection_confirmation_invalid"}
	}
	handler := b.selection
	if handler == nil {
		b.mu.Unlock()
		return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "selection_handler"}}
	}
	characterCount := len(b.catalog.Characters)
	b.status.State = string(app.SupervisorStateActivatingSelection)
	b.status.Generation++
	generation := b.status.Generation
	b.mu.Unlock()
	b.publisher.Publish(telemetry.LiveEvent{Event: "supervisor_state_changed", Details: map[string]any{"state": app.SupervisorStateActivatingSelection, "generation": generation}})

	err := handler(app.CharacterSelectionRequest{Character: entry.Name, Difficulty: payload.Difficulty, CatalogRevision: payload.CatalogRevision, CharacterCount: characterCount, AnchorPath: entry.AnchorPath, ExpectedClass: entry.ExpectedClass})
	var commitErr error
	var settingsErr error
	var settingsChange app.OperatorSettingsChange
	var refreshedRuns []RunCatalogEntry
	var refreshErr error
	if err == nil {
		_, commitErr = b.lifecycle.Confirm(previewRecord.lifecycle, time.Now().UTC())
		if commitErr == nil {
			if b.operatorSettings != nil {
				settingsChange, settingsErr = b.operatorSettings.ConfirmSelection(entry.Name, payload.Difficulty)
			}
			refreshedRuns, refreshErr = b.resolveRunsForEntry(entry, payload.Difficulty)
		}
	}
	b.mu.Lock()
	if err != nil {
		b.status.State = string(app.SupervisorStateIdle)
		b.status.LastError = &ProblemDTO{Code: "character_selection_unconfirmed"}
	} else if commitErr != nil {
		b.status.State = string(app.SupervisorStateIdleInGame)
		b.status.LastError = &ProblemDTO{Code: lifecycleErrorCode(commitErr)}
	} else {
		b.status.State = string(app.SupervisorStateIdleInGame)
		if settingsErr != nil {
			b.status.LastError = &ProblemDTO{Code: "selection_persistence_failed"}
		} else if refreshErr == nil {
			b.status.LastError = nil
		} else {
			b.status.LastError = &ProblemDTO{Code: "run_catalog_refresh_failed"}
		}
		b.status.Selection = SelectionStatusDTO{Character: entry.Name, Difficulty: payload.Difficulty}
		b.cfg.Session.Character = entry.Name
		b.cfg.Session.Difficulty = payload.Difficulty
		if value, ok := settingsChange.Settings.Characters[strings.ToLower(entry.Name)]; ok {
			b.cfg.Session.Queue = append([]string(nil), value.Queue...)
			b.status.Queue.Entries = append([]string(nil), value.Queue...)
			b.status.Queue.DefaultEntries = append([]string(nil), value.Queue...)
		}
		b.previews = make(map[string]selectionPreviewRecord)
		if refreshErr == nil {
			b.catalog.Runs = refreshedRuns
		} else {
			for i := range b.catalog.Runs {
				b.catalog.Runs[i].Status = "unavailable"
				b.catalog.Runs[i].Reasons = []string{"route_lifecycle_unavailable"}
			}
		}
	}
	b.status.Generation++
	response := CommandResponse{CommandID: request.CommandID, Generation: b.status.Generation, State: b.status.State}
	if err == nil && commitErr == nil {
		b.rememberCommandLocked("apply_selection", request, response)
	}
	b.mu.Unlock()
	b.publisher.Publish(telemetry.LiveEvent{Event: "supervisor_state_changed", Details: map[string]any{"state": response.State, "generation": response.Generation}})
	if err != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "selection_failed", Reason: "character_selection_unconfirmed", Details: map[string]any{"character": entry.Name, "difficulty": payload.Difficulty}})
		return response, &commandError{code: "character_selection_unconfirmed", cause: fmt.Errorf("confirm character selection: %w", err)}
	}
	if commitErr != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "selection_lifecycle_failed", Reason: lifecycleErrorCode(commitErr), Details: map[string]any{"character": entry.Name, "difficulty": payload.Difficulty}})
		return response, &commandError{code: lifecycleErrorCode(commitErr), cause: fmt.Errorf("commit route lifecycle after character selection: %w", commitErr)}
	}
	if refreshErr != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "runtime_error", Reason: "run_catalog_refresh_failed"})
	}
	if settingsErr != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "runtime_error", Reason: "selection_persistence_failed"})
	}
	b.publisher.Publish(telemetry.LiveEvent{Event: "selection_completed", Reason: response.State, Details: map[string]any{"character": entry.Name, "difficulty": payload.Difficulty}})
	return response, nil
}

func (b *LiveBackend) sessionCommand(name string, request CommandRequest) (CommandResponse, error) {
	b.mu.RLock()
	if response, ok, err := b.replayCommandLocked(name, request); ok || err != nil {
		b.mu.RUnlock()
		return response, err
	}
	statusGeneration := b.status.Generation
	statusState := b.status.State
	compatibility := b.status.Compatibility
	startRuntime := app.UIStatusSnapshot{
		ProcessState: b.status.D2R.State,
		WindowBound:  b.status.D2R.WindowBound,
		WorldValid:   b.status.World.Valid,
		WorldPhase:   b.status.World.Phase,
		AreaID:       b.status.World.AreaID,
	}
	selection := b.status.Selection
	supervisor := b.supervisor
	beforeWorker := b.beforeWorker
	beginQueue := b.beginQueue
	routeState := b.routeWorkflow.State
	b.mu.RUnlock()
	if routeWorkflowBusy(routeState) {
		return CommandResponse{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "session"}}
	}
	if (name == "start_queue" || name == "resume") && compatibility.State != string(app.D2RCompatibilityCompatible) {
		return CommandResponse{}, compatibilityCommandError(compatibility)
	}
	if request.ExpectedGeneration != statusGeneration {
		return CommandResponse{}, &commandError{code: string(app.SupervisorReasonStateChanged)}
	}
	if supervisor == nil {
		return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "session_supervisor"}}
	}
	meta := app.SupervisorCommandMeta{CommandID: request.CommandID, ExpectedGeneration: supervisor.Snapshot().Generation}
	var (
		snapshot app.SupervisorSnapshot
		err      error
	)
	switch name {
	case "start_queue":
		var payload SessionStartPayload
		if decodeErr := decodeCommandPayload(request.Payload, &payload); decodeErr != nil {
			return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "payload"}, cause: fmt.Errorf("decode queue start payload: %w", decodeErr)}
		}
		_, freshRevision, contractErr := b.validateDesktopCharacterContract(payload.Character, payload.Entries)
		if contractErr != nil {
			return CommandResponse{}, contractErr
		}
		plan, validateErr := app.ValidateFarmQueue(b.cfg, app.FarmQueueValidationRequest{
			RunIDs: payload.Entries, Character: payload.Character, Difficulty: payload.Difficulty, CatalogRevision: payload.CatalogRevision,
		}, b.farmQueueValidationContext(selection.Character, selection.Difficulty, freshRevision))
		if validateErr != nil {
			return CommandResponse{}, mapQueueCommandError(validateErr)
		}
		if loadoutErr := b.evaluateQueueLoadout(plan.Character); loadoutErr != nil {
			return CommandResponse{}, loadoutErr
		}
		if beforeWorker != nil {
			if monitorErr := beforeWorker(); monitorErr != nil {
				return CommandResponse{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "start_queue"}, cause: fmt.Errorf("stop passive monitor before queue start: %w", monitorErr)}
			}
		}
		if beginQueue != nil {
			if beginErr := beginQueue(app.CanAdoptQueueGame(app.SupervisorState(statusState), startRuntime)); beginErr != nil {
				// Loadout readiness already passed; freeze/resolve failures must not be
				// papered over as profile_bindings_incomplete.
				return CommandResponse{}, &commandError{
					code:   "command_conflict",
					params: map[string]any{"operation": "freeze_loadout"},
					cause:  fmt.Errorf("freeze character loadout for queue: %w", beginErr),
				}
			}
		}
		snapshot, err = supervisor.StartQueue(meta, plan)
	case "pause_after_run":
		if !emptyCommandPayload(request.Payload) {
			return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "payload"}}
		}
		snapshot, err = supervisor.PauseAfterRun(meta)
	case "stop_after_run":
		if !emptyCommandPayload(request.Payload) {
			return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "payload"}}
		}
		snapshot, err = supervisor.StopAfterRun(meta)
	case "resume":
		if !emptyCommandPayload(request.Payload) {
			return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "payload"}}
		}
		if beforeWorker != nil {
			if monitorErr := beforeWorker(); monitorErr != nil {
				return CommandResponse{}, &commandError{code: "command_conflict", params: map[string]any{"operation": "resume"}, cause: fmt.Errorf("stop passive monitor before resume: %w", monitorErr)}
			}
		}
		snapshot, err = supervisor.Resume(meta)
	case "emergency_stop":
		if !emptyCommandPayload(request.Payload) {
			return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "payload"}}
		}
		snapshot, err = supervisor.EmergencyStop(meta)
	default:
		return CommandResponse{}, &commandError{code: "request_invalid", params: map[string]any{"field": "command"}}
	}
	if err != nil {
		return CommandResponse{}, mapSupervisorCommandError(err)
	}
	b.UpdateSupervisor(snapshot)
	current := b.Status()
	response := CommandResponse{CommandID: request.CommandID, Generation: current.Generation, State: current.State}
	b.mu.Lock()
	b.rememberCommandLocked(name, request, response)
	b.mu.Unlock()
	return response, nil
}

func compatibilityDTO(snapshot app.D2RCompatibilitySnapshot) CompatibilityDTO {
	return CompatibilityDTO{
		State: string(snapshot.State), Reason: string(snapshot.Reason), SupportedVersion: snapshot.SupportedVersion,
		ExpectedVersion: snapshot.ExpectedVersion, OffsetVersion: snapshot.OffsetVersion, ActualVersion: snapshot.ActualVersion,
		PrivilegeMismatch: snapshot.PrivilegeMismatch,
	}
}

func (b *LiveBackend) requireCompatibleLocked() error {
	if b.status.Compatibility.State == string(app.D2RCompatibilityCompatible) {
		return nil
	}
	return compatibilityCommandError(b.status.Compatibility)
}

func compatibilityCommandError(compatibility CompatibilityDTO) error {
	code := compatibility.Reason
	if code == "" {
		code = string(app.Phase15ReasonD2RVersionNotDetected)
	}
	return &commandError{code: code}
}

func routeWorkflowBusy(state string) bool {
	switch app.RouteWorkflowState(state) {
	case app.RouteWorkflowIdle, app.RouteWorkflowCompleted, app.RouteWorkflowFailedSafe, app.RouteWorkflowEmergencyCancelled:
		return false
	default:
		return true
	}
}

func (b *LiveBackend) replayCommandLocked(name string, request CommandRequest) (CommandResponse, bool, error) {
	record, ok := b.commands[request.CommandID]
	if !ok {
		return CommandResponse{}, false, nil
	}
	if record.name != name || record.generation != request.ExpectedGeneration || record.payload != compactCommandPayload(request.Payload) {
		return CommandResponse{}, false, &commandError{code: "request_invalid", params: map[string]any{"field": "command_id"}}
	}
	return record.response, true, nil
}

func (b *LiveBackend) rememberCommandLocked(name string, request CommandRequest, response CommandResponse) {
	b.commands[request.CommandID] = apiCommandRecord{name: name, generation: request.ExpectedGeneration, payload: compactCommandPayload(request.Payload), response: response}
}

func compactCommandPayload(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return string(raw)
	}
	return compact.String()
}

func decodeCommandPayload(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return fmt.Errorf("payload is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("payload contains multiple values")
	}
	return nil
}

func emptyCommandPayload(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func mapQueueCommandError(err error) error {
	var queueErr *app.QueueValidationError
	if errors.As(err, &queueErr) {
		return queueCommandError(queueErr)
	}
	var supervisorErr *app.SupervisorCommandError
	if errors.As(err, &supervisorErr) {
		return &commandError{code: string(supervisorErr.Code), cause: fmt.Errorf("map queue command: %w", err)}
	}
	return &commandError{code: "queue_entry_unavailable", cause: fmt.Errorf("map queue command: %w", err)}
}

func queueCommandError(queueErr *app.QueueValidationError) *commandError {
	params := map[string]any{"run_id": queueErr.RunID, "duplicate_index": queueErr.EntryIndex}
	if queueErr.Code == app.QueueReasonDuplicateRun {
		params["first_index"] = queueErr.FirstIndex
	}
	return &commandError{code: string(queueErr.Code), params: params, cause: queueErr}
}

func mapSupervisorCommandError(err error) error {
	var supervisorErr *app.SupervisorCommandError
	if errors.As(err, &supervisorErr) {
		return &commandError{code: string(supervisorErr.Code), cause: fmt.Errorf("map supervisor command: %w", err)}
	}
	var queueErr *app.QueueValidationError
	if errors.As(err, &queueErr) {
		return queueCommandError(queueErr)
	}
	return &commandError{code: "state_changed", cause: fmt.Errorf("map supervisor command: %w", err)}
}

func knownDifficulty(entries []DifficultyCatalogEntry, value string) bool {
	for _, entry := range entries {
		if entry.ID == value {
			return true
		}
	}
	return false
}

func lifecycleErrorCode(_ error) string {
	return "state_changed"
}

func cloneCatalogDTO(source CatalogDTO) CatalogDTO {
	catalog := source
	catalog.Characters = append([]CharacterCatalogEntry(nil), source.Characters...)
	for i := range catalog.Characters {
		catalog.Characters[i].Reasons = append([]string(nil), catalog.Characters[i].Reasons...)
		catalog.Characters[i].FarmReadyReasons = append([]string(nil), catalog.Characters[i].FarmReadyReasons...)
	}
	catalog.Difficulties = append([]DifficultyCatalogEntry(nil), source.Difficulties...)
	catalog.Profiles = append([]ProfileCatalogEntry(nil), source.Profiles...)
	catalog.Runs = append([]RunCatalogEntry(nil), source.Runs...)
	for i := range catalog.Runs {
		catalog.Runs[i].Reasons = append([]string(nil), catalog.Runs[i].Reasons...)
	}
	return catalog
}

func queueStatusDTO(snapshot app.SupervisorSnapshot) QueueStatusDTO {
	return QueueStatusDTO{
		Entries: append(make([]string, 0, len(snapshot.Queue)), snapshot.Queue...), Index: snapshot.QueueIndex, Cycle: snapshot.Cycle,
		Retry: snapshot.Retry, StartedRuns: snapshot.StartedRuns, ConsecutiveFailures: snapshot.ConsecutiveFailures,
		TotalRestarts: snapshot.TotalRestarts, Budgets: queueBudgetsDTO(snapshot.Budgets),
	}
}

func queueBudgetsDTO(budgets app.FarmQueueBudgets) QueueBudgetsDTO {
	return QueueBudgetsDTO{
		MaxRuns: budgets.MaxRuns, MaxDurationMs: budgets.MaxDuration.Milliseconds(),
		MaxConsecutiveFailures: budgets.MaxConsecutiveFailures, MaxTotalRestarts: budgets.MaxTotalRestarts,
	}
}
