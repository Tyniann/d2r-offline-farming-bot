package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

const (
	// RouteAssignmentSchemaVersion is the first character/run assignment schema.
	RouteAssignmentSchemaVersion = 1
	// RouteCandidateSchemaVersion is the first immutable candidate metadata schema.
	RouteCandidateSchemaVersion = 1
	// RouteRecoveryJournalSchemaVersion is the first management recovery schema.
	RouteRecoveryJournalSchemaVersion = 1
)

// RouteManagementStatus is orthogonal to automatic lifecycle invalidation.
type RouteManagementStatus string

const (
	// RouteManagementActive permits an otherwise valid assigned route to be selected.
	RouteManagementActive RouteManagementStatus = "active"
	// RouteManagementArchived hides a route from normal selection and blocks playback.
	RouteManagementArchived RouteManagementStatus = "archived"
)

// RouteAssignmentManifest is the sole persistent character/run assignment authority.
type RouteAssignmentManifest struct {
	SchemaVersion int                               `yaml:"schema_version"`
	Revision      uint64                            `yaml:"revision"`
	Assignments   map[string]map[tasks.RunID]string `yaml:"assignments"`
}

// Validate rejects ambiguous or incomplete assignment snapshots before use.
func (m RouteAssignmentManifest) Validate() error {
	if m.SchemaVersion != RouteAssignmentSchemaVersion || m.Revision == 0 || m.Assignments == nil {
		return fmt.Errorf("route assignment schema_version, positive revision, and assignments are required")
	}
	for character, runs := range m.Assignments {
		if strings.TrimSpace(character) == "" || character != strings.ToLower(character) || runs == nil {
			return fmt.Errorf("route assignment character key %q must be a non-empty lowercase slug", character)
		}
		for runID, routeID := range runs {
			if _, ok := tasks.DefaultRunRegistry().Definition(runID); !ok || strings.TrimSpace(routeID) == "" {
				return fmt.Errorf("route assignment %s/%s requires a registered run and route ID", character, runID)
			}
		}
	}
	return nil
}

// RouteCandidateState identifies one persisted guided-recording stage.
type RouteCandidateState string

const (
	// RouteCandidateRecorded means the immutable raw route exists.
	RouteCandidateRecorded RouteCandidateState = "recorded"
	// RouteCandidateValidated means structure and terminal semantics passed.
	RouteCandidateValidated RouteCandidateState = "validated"
	// RouteCandidateTestRunning means candidate-only playback owns input.
	RouteCandidateTestRunning RouteCandidateState = "test_running"
	// RouteCandidateTestPassed means terminal playback validation passed.
	RouteCandidateTestPassed RouteCandidateState = "test_passed"
	// RouteCandidateFailed means the candidate remains diagnostic and cannot publish.
	RouteCandidateFailed RouteCandidateState = "failed"
)

// RouteCandidate stores metadata only; the immutable route file remains content authority.
type RouteCandidate struct {
	SchemaVersion            int                 `yaml:"schema_version"`
	CandidateID              string              `yaml:"candidate_id"`
	ImmutableRouteFile       string              `yaml:"immutable_route_file"`
	ImmutableRouteSHA256     string              `yaml:"immutable_route_sha256"`
	RunID                    tasks.RunID         `yaml:"run_id"`
	Character                string              `yaml:"character"`
	Difficulty               string              `yaml:"difficulty"`
	GameVersion              string              `yaml:"game_version"`
	State                    RouteCandidateState `yaml:"state"`
	MeasuredBossDistance     float64             `yaml:"measured_boss_distance"`
	SourceCatalogRevision    uint64              `yaml:"source_catalog_revision"`
	SourceAssignmentRevision uint64              `yaml:"source_assignment_revision"`
	CreatedAt                time.Time           `yaml:"created_at"`
	TestedAt                 *time.Time          `yaml:"tested_at,omitempty"`
	FailureReason            RouteReason         `yaml:"failure_reason,omitempty"`
}

// Validate rejects candidate metadata that cannot be correlated fail-closed.
func (c RouteCandidate) Validate() error {
	if c.SchemaVersion != RouteCandidateSchemaVersion || strings.TrimSpace(c.CandidateID) == "" || strings.TrimSpace(c.ImmutableRouteFile) == "" || len(c.ImmutableRouteSHA256) != 64 {
		return fmt.Errorf("route candidate schema, ID, immutable file, and SHA-256 are required")
	}
	if _, ok := tasks.DefaultRunRegistry().Definition(c.RunID); !ok || strings.TrimSpace(c.Character) == "" || strings.TrimSpace(c.Difficulty) == "" || strings.TrimSpace(c.GameVersion) == "" {
		return fmt.Errorf("route candidate requires a registered run and complete binding")
	}
	if !validRouteCandidateState(c.State) || c.MeasuredBossDistance < 0 || c.SourceCatalogRevision == 0 || c.SourceAssignmentRevision == 0 || c.CreatedAt.IsZero() {
		return fmt.Errorf("route candidate state, distance, revisions, and creation time are invalid")
	}
	if c.State == RouteCandidateTestPassed && c.TestedAt == nil {
		return fmt.Errorf("tested_at is required for a passed route candidate")
	}
	if c.State == RouteCandidateFailed && c.FailureReason == "" {
		return fmt.Errorf("failure_reason is required for a failed route candidate")
	}
	return nil
}

func validRouteCandidateState(state RouteCandidateState) bool {
	switch state {
	case RouteCandidateRecorded, RouteCandidateValidated, RouteCandidateTestRunning, RouteCandidateTestPassed, RouteCandidateFailed:
		return true
	default:
		return false
	}
}

// RouteWorkflowState identifies the exclusive recording/test/publish owner state.
type RouteWorkflowState string

// RouteWorkflowProgress reports coarse Core-owned workflow progress without
// exposing input drivers, local paths, or mutable workflow authority.
type RouteWorkflowProgress struct {
	State    RouteWorkflowState
	AreaID   uint32
	Segment  int
	Progress float64
	Reason   string
}

// RouteWorkflowReporter receives observable workflow transitions. Callers
// must treat it as telemetry only; returning from it cannot authorize input.
type RouteWorkflowReporter func(RouteWorkflowProgress)

const (
	// RouteWorkflowIdle accepts a new exclusive workflow.
	RouteWorkflowIdle RouteWorkflowState = "idle"
	// RouteWorkflowPreflight verifies all no-input start conditions.
	RouteWorkflowPreflight RouteWorkflowState = "preflight"
	// RouteWorkflowRecording samples immutable World snapshots.
	RouteWorkflowRecording RouteWorkflowState = "recording"
	// RouteWorkflowFreezing persists the immutable raw candidate.
	RouteWorkflowFreezing RouteWorkflowState = "freezing"
	// RouteWorkflowValidating checks structural and semantic contracts.
	RouteWorkflowValidating RouteWorkflowState = "validating"
	// RouteWorkflowReturningViaPortal performs the post-freeze safety return.
	RouteWorkflowReturningViaPortal RouteWorkflowState = "returning_via_portal"
	// RouteWorkflowCandidateReady exposes a validated unpublished candidate.
	RouteWorkflowCandidateReady RouteWorkflowState = "candidate_ready"
	// RouteWorkflowPreparingPlayback performs Town and Waypoint normalization.
	RouteWorkflowPreparingPlayback RouteWorkflowState = "preparing_playback"
	// RouteWorkflowPlayingCandidate runs navigation-only candidate playback.
	RouteWorkflowPlayingCandidate RouteWorkflowState = "playing_candidate"
	// RouteWorkflowValidatingTerminal checks Memory-confirmed endpoint evidence.
	RouteWorkflowValidatingTerminal RouteWorkflowState = "validating_terminal"
	// RouteWorkflowReturningAfterTest performs the second portal return.
	RouteWorkflowReturningAfterTest RouteWorkflowState = "returning_after_test"
	// RouteWorkflowAwaitingPublishConfirmation waits for explicit publication.
	RouteWorkflowAwaitingPublishConfirmation RouteWorkflowState = "awaiting_publish_confirmation"
	// RouteWorkflowPublishing commits one revision-bound route mutation.
	RouteWorkflowPublishing RouteWorkflowState = "publishing"
	// RouteWorkflowCompleted marks successful workflow completion.
	RouteWorkflowCompleted RouteWorkflowState = "completed"
	// RouteWorkflowFailedSafe marks a fail-closed terminal state.
	RouteWorkflowFailedSafe RouteWorkflowState = "failed_safe"
	// RouteWorkflowEmergencyCancelled marks immediate F11 cancellation.
	RouteWorkflowEmergencyCancelled RouteWorkflowState = "emergency_cancelled"
)

// RouteWorkflowTransition declares one allowed non-error state change.
type RouteWorkflowTransition struct {
	From RouteWorkflowState
	To   RouteWorkflowState
}

var routeWorkflowTransitions = []RouteWorkflowTransition{
	{RouteWorkflowIdle, RouteWorkflowPreflight}, {RouteWorkflowPreflight, RouteWorkflowRecording},
	{RouteWorkflowRecording, RouteWorkflowFreezing}, {RouteWorkflowFreezing, RouteWorkflowValidating},
	{RouteWorkflowValidating, RouteWorkflowReturningViaPortal}, {RouteWorkflowReturningViaPortal, RouteWorkflowCandidateReady},
	{RouteWorkflowCandidateReady, RouteWorkflowPreparingPlayback}, {RouteWorkflowPreparingPlayback, RouteWorkflowPlayingCandidate},
	{RouteWorkflowPlayingCandidate, RouteWorkflowValidatingTerminal}, {RouteWorkflowValidatingTerminal, RouteWorkflowReturningAfterTest},
	{RouteWorkflowReturningAfterTest, RouteWorkflowAwaitingPublishConfirmation}, {RouteWorkflowAwaitingPublishConfirmation, RouteWorkflowPublishing},
	{RouteWorkflowPublishing, RouteWorkflowCompleted}, {RouteWorkflowRecording, RouteWorkflowEmergencyCancelled},
}

// RouteWorkflowTransitions returns a defensive copy of the normative state table.
func RouteWorkflowTransitions() []RouteWorkflowTransition {
	return append([]RouteWorkflowTransition(nil), routeWorkflowTransitions...)
}

// RouteLock identifies one persistent Phase-12 authority lock.
type RouteLock string

const (
	// RouteLockWorkflow serializes the outer workflow owner.
	RouteLockWorkflow RouteLock = "workflow"
	// RouteLockCatalog protects catalog snapshots and revisions.
	RouteLockCatalog RouteLock = "catalog"
	// RouteLockLifecycle protects automatic and management lifecycle state.
	RouteLockLifecycle RouteLock = "lifecycle"
	// RouteLockAssignment protects character/run route selection.
	RouteLockAssignment RouteLock = "assignment"
	// RouteLockCandidate protects immutable candidate metadata transitions.
	RouteLockCandidate RouteLock = "candidate"
	// RouteLockJournal protects durable mutation recovery state.
	RouteLockJournal RouteLock = "journal"
)

// RouteLockOrder returns the only legal outer-to-inner acquisition order.
func RouteLockOrder() []RouteLock {
	return []RouteLock{RouteLockWorkflow, RouteLockCatalog, RouteLockLifecycle, RouteLockAssignment, RouteLockCandidate, RouteLockJournal}
}

// RouteContractOwner names the sole package or component responsible for a contract.
type RouteContractOwner struct {
	Contract string
	Owner    string
}

// RouteContractOwners returns the complete Phase-12 ownership table.
func RouteContractOwners() []RouteContractOwner {
	return []RouteContractOwner{
		{Contract: "automatic_invalidation", Owner: "internal/app.RouteLifecycleStore"},
		{Contract: "assignment", Owner: "internal/app.RouteAssignmentStore"},
		{Contract: "candidate_metadata", Owner: "internal/app.CandidateStore"},
		{Contract: "recording_and_test_input", Owner: "internal/app.RecordingCoordinator"},
		{Contract: "route_content", Owner: "internal/pathing.Route"},
		{Contract: "recording_semantics", Owner: "internal/tasks.RunRegistry"},
		{Contract: "system_egress", Owner: "internal/town.SystemEgressContract"},
		{Contract: "http_and_sse_dto", Owner: "internal/api"},
		{Contract: "management_recovery", Owner: "internal/app.RouteRecoveryJournal"},
	}
}

// RouteReason is a stable machine-readable Phase-12 reason code.
type RouteReason string

const (
	// RouteReasonRecordingConflict rejects a second workflow owner.
	RouteReasonRecordingConflict RouteReason = "recording_conflict"
	// RouteReasonRecordingPreflightFailed rejects incomplete no-input preconditions.
	RouteReasonRecordingPreflightFailed RouteReason = "recording_preflight_failed"
	// RouteReasonRecordingTimeout ends an overlong recording fail-closed.
	RouteReasonRecordingTimeout RouteReason = "recording_timeout"
	// RouteReasonRecordingStartAreaMismatch rejects wrong identity or start area.
	RouteReasonRecordingStartAreaMismatch RouteReason = "recording_start_area_mismatch"
	// RouteReasonRecordingFinishRequested records an authoritative finish intent.
	RouteReasonRecordingFinishRequested RouteReason = "recording_finish_requested"
	// RouteReasonRecordingEmergencyCancelled records immediate F11 cancellation.
	RouteReasonRecordingEmergencyCancelled RouteReason = "recording_emergency_cancelled"
	// RouteReasonRecordingTerminalAreaMismatch rejects the wrong endpoint area.
	RouteReasonRecordingTerminalAreaMismatch RouteReason = "recording_terminal_area_mismatch"
	// RouteReasonRecordingBossMissing rejects missing or mismatched boss evidence.
	RouteReasonRecordingBossMissing RouteReason = "recording_boss_missing"
	// RouteReasonRecordingBossDead rejects explicitly dead boss evidence.
	RouteReasonRecordingBossDead RouteReason = "recording_boss_dead"
	// RouteReasonRecordingEndpointTooFar rejects endpoints outside boss tolerance.
	RouteReasonRecordingEndpointTooFar RouteReason = "recording_endpoint_too_far"
	// RouteReasonCandidateInvalid rejects malformed candidate content or state.
	RouteReasonCandidateInvalid RouteReason = "route_candidate_invalid"
	// RouteReasonCandidateChanged rejects a changed immutable candidate hash.
	RouteReasonCandidateChanged RouteReason = "route_candidate_changed"
	// RouteReasonTestStartFailed marks Town, Egress, or Waypoint preparation failure.
	RouteReasonTestStartFailed RouteReason = "route_test_start_failed"
	// RouteReasonTestPlaybackFailed marks candidate navigation failure.
	RouteReasonTestPlaybackFailed RouteReason = "route_test_playback_failed"
	// RouteReasonTestTerminalMismatch marks invalid Memory terminal evidence.
	RouteReasonTestTerminalMismatch RouteReason = "route_test_terminal_mismatch"
	// RouteReasonTestPassed records successful isolated candidate playback.
	RouteReasonTestPassed RouteReason = "route_test_passed"
	// RouteReasonSafetyReturnFailed marks a failed controlled portal return.
	RouteReasonSafetyReturnFailed RouteReason = "route_safety_return_failed"
	// RouteReasonAssignmentMissing marks an absent character/run assignment.
	RouteReasonAssignmentMissing RouteReason = "route_assignment_missing"
	// RouteReasonAssignmentConflict marks a stale assignment revision.
	RouteReasonAssignmentConflict RouteReason = "route_assignment_conflict"
	// RouteReasonReplaceConfirmationRequired requires explicit predecessor archival.
	RouteReasonReplaceConfirmationRequired RouteReason = "route_replace_confirmation_required"
	// RouteReasonArchived records successful route archival.
	RouteReasonArchived RouteReason = "route_archived"
	// RouteReasonRestoreIncompatible rejects an incompatible archived route.
	RouteReasonRestoreIncompatible RouteReason = "route_restore_incompatible"
	// RouteReasonDeleteAssigned rejects deletion of an assigned route.
	RouteReasonDeleteAssigned RouteReason = "route_delete_assigned"
	// RouteReasonDeleteConfirmationMismatch rejects an incorrect delete route ID.
	RouteReasonDeleteConfirmationMismatch RouteReason = "route_delete_confirmation_mismatch"
	// RouteReasonTransactionRecoveryRequired blocks unknown recovery state.
	RouteReasonTransactionRecoveryRequired RouteReason = "route_transaction_recovery_required"
)

// RouteMutationOperation identifies a preview/confirm management transaction.
type RouteMutationOperation string

const (
	// RouteMutationPublish publishes a candidate into an empty assignment slot.
	RouteMutationPublish RouteMutationOperation = "publish"
	// RouteMutationReplace publishes a candidate and archives its predecessor.
	RouteMutationReplace RouteMutationOperation = "replace"
	// RouteMutationArchive removes assignment and hides an active route.
	RouteMutationArchive RouteMutationOperation = "archive"
	// RouteMutationRestore activates one compatible archived route.
	RouteMutationRestore RouteMutationOperation = "restore"
	// RouteMutationDelete permanently removes an unassigned archived route.
	RouteMutationDelete RouteMutationOperation = "delete"
)

// RouteCrashContract fixes one durable checkpoint and its deterministic recovery.
type RouteCrashContract struct {
	Operation  RouteMutationOperation
	Checkpoint string
	Recovery   string
}

// RouteCrashMatrix returns the checkpoints that future persistent operations must inject in tests.
func RouteCrashMatrix() []RouteCrashContract {
	return []RouteCrashContract{
		{RouteMutationPublish, "before_route_publish", "retain_candidate"}, {RouteMutationPublish, "after_route_publish", "remove_unassigned_route"},
		{RouteMutationReplace, "after_new_route_publish", "keep_old_active_and_candidate"}, {RouteMutationReplace, "after_old_archive_prepare", "restore_old_active"},
		{RouteMutationArchive, "before_assignment_remove", "leave_active"}, {RouteMutationArchive, "after_assignment_remove", "restore_assignment"},
		{RouteMutationRestore, "after_current_archive_prepare", "restore_current_active"},
		{RouteMutationDelete, "after_quarantine_rename", "restore_from_quarantine"}, {RouteMutationDelete, "after_manifest_commit", "complete_quarantine_delete"},
	}
}

// RouteRecoveryJournal records one in-flight mutation; unknown stages block management.
type RouteRecoveryJournal struct {
	SchemaVersion   int                    `yaml:"schema_version"`
	Operation       RouteMutationOperation `yaml:"operation"`
	RouteID         string                 `yaml:"route_id"`
	CandidateID     string                 `yaml:"candidate_id,omitempty"`
	Checkpoint      string                 `yaml:"checkpoint"`
	PreviousRouteID string                 `yaml:"previous_route_id,omitempty"`
	Character       string                 `yaml:"character,omitempty"`
	RunID           tasks.RunID            `yaml:"run_id,omitempty"`
	RoutePath       string                 `yaml:"route_path,omitempty"`
	StartedAt       time.Time              `yaml:"started_at"`
}
