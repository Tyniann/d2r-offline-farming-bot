package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	guidedRecordingSampleDistance = 4.0
	guidedRecordingTimeout        = 30 * time.Minute
)

var recordingWorkflowOwner sync.Mutex

// RecordingPreflight binds one guided recording to immutable operator and catalog context.
type RecordingPreflight struct {
	RunID                    tasks.RunID
	RouteRole                pathing.RouteRole
	Character                string
	ExpectedClass            string
	ProfileID                string
	Difficulty               pathing.RouteDifficulty
	GameVersion              string
	SourceCatalogRevision    uint64
	SourceAssignmentRevision uint64
	SourceAssignedRouteID    string
	WaypointContextConfirmed bool
	BlockingUIClosed         bool
	D2RFocused               bool
	InputOwnerAvailable      bool
}

// RecordingTerminalEvidence supplies the Memory-confirmed terminal snapshot and boss state.
type RecordingTerminalEvidence struct {
	World    world.State
	Boss     *world.Monster
	Object   *world.Object
	BossDead bool
}

// RecordingSnapshot is an immutable workflow projection.
type RecordingSnapshot struct {
	State       RouteWorkflowState
	Reason      RouteReason
	CandidateID string
	RunID       tasks.RunID
}

// RecordingCoordinator exclusively owns guided recording, immutable freeze,
// semantic validation, and the post-freeze Town Portal safety handoff.
type RecordingCoordinator struct {
	mu            sync.Mutex
	store         *CandidateStore
	registry      *tasks.RunRegistry
	state         RouteWorkflowState
	reason        RouteReason
	request       RecordingPreflight
	definition    tasks.RunDefinition
	contract      tasks.RecordingContract
	recorder      *pathing.RouteRecorder
	fingerprint   pathing.LayoutFingerprint
	recordedAt    time.Time
	deadline      time.Time
	candidate     RouteCandidate
	workflowOwned bool
}

// NewRecordingCoordinator creates an idle coordinator using the authoritative run registry.
func NewRecordingCoordinator(store *CandidateStore, registry *tasks.RunRegistry) (*RecordingCoordinator, error) {
	if store == nil || registry == nil {
		return nil, fmt.Errorf("recording coordinator requires candidate store")
	}
	return &RecordingCoordinator{store: store, registry: registry, state: RouteWorkflowIdle}, nil
}

// Start performs every no-input preflight and acquires the exclusive workflow owner.
func (c *RecordingCoordinator) Start(request RecordingPreflight, state world.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != RouteWorkflowIdle || !recordingWorkflowOwner.TryLock() {
		return fmt.Errorf("%s", RouteReasonRecordingConflict)
	}
	c.workflowOwned = true
	c.state = RouteWorkflowPreflight
	definition, ok := c.registry.Definition(request.RunID)
	contract, contractOK := definition.RecordingForRole(request.RouteRole)
	if !ok || !contractOK || request.Character == "" || request.ExpectedClass == "" || request.Difficulty == "" || request.GameVersion == "" || request.SourceCatalogRevision == 0 || request.SourceAssignmentRevision == 0 || !request.WaypointContextConfirmed || !request.BlockingUIClosed || !request.D2RFocused || !request.InputOwnerAvailable {
		return c.failLocked(RouteReasonRecordingPreflightFailed)
	}
	if definition.RouteSet != nil && strings.TrimSpace(request.ProfileID) == "" {
		return c.failLocked(RouteReasonRecordingPreflightFailed)
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Identity.Valid || state.Identity.CharacterName != request.Character || !strings.EqualFold(state.Identity.Class.String(), request.ExpectedClass) || state.Area.ID != contract.AllowedStartArea {
		return c.failLocked(RouteReasonRecordingStartAreaMismatch)
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		return c.failLocked(RouteReasonRecordingPreflightFailed)
	}
	recorder, err := pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: guidedRecordingSampleDistance, Movement: contract.Movement})
	if err != nil {
		return c.failLocked(RouteReasonRecordingPreflightFailed)
	}
	c.request = request
	c.definition = definition
	c.contract = contract
	c.fingerprint = fingerprint
	c.recorder = recorder
	c.recordedAt = time.Now().UTC()
	startTime := state.At
	if startTime.IsZero() {
		startTime = c.recordedAt
	}
	c.deadline = startTime.Add(guidedRecordingTimeout)
	c.state = RouteWorkflowRecording
	_, err = c.recorder.Observe(state)
	return err
}

// Tick observes one immutable World snapshot and treats context cancellation as F11 semantics.
func (c *RecordingCoordinator) Tick(ctx context.Context, state world.State) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil {
		c.emergencyCancelLocked()
		return ctx.Err()
	}
	if c.state != RouteWorkflowRecording {
		return fmt.Errorf("recording tick invalid in state %s", c.state)
	}
	if !state.At.IsZero() && state.At.After(c.deadline) {
		return c.failLocked(RouteReasonRecordingTimeout)
	}
	if state.Valid && state.Phase == world.GamePhaseInGame && !areaAllowed(state.Area.ID, c.contract.AllowedRouteAreas) {
		return c.failLocked(RouteReasonRecordingTerminalAreaMismatch)
	}
	_, err := c.recorder.Observe(state)
	if err != nil {
		return c.failLocked(RouteReasonRecordingPreflightFailed)
	}
	return nil
}

// Finish applies authoritative F9 semantics. Repeated calls return the same
// frozen candidate and never initiate a second safety return.
func (c *RecordingCoordinator) Finish(evidence RecordingTerminalEvidence) (RouteCandidate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.candidate.CandidateID != "" {
		return c.candidate, nil
	}
	if c.state != RouteWorkflowRecording {
		return RouteCandidate{}, fmt.Errorf("finish invalid in state %s", c.state)
	}
	c.reason = RouteReasonRecordingFinishRequested
	c.state = RouteWorkflowFreezing
	segments, err := c.recorder.Finish()
	if err != nil {
		return RouteCandidate{}, c.failLocked(RouteReasonCandidateInvalid)
	}
	candidateID, err := newCandidateID()
	if err != nil {
		return RouteCandidate{}, c.failLocked(RouteReasonCandidateInvalid)
	}
	seed := evidence.World.Identity.MapSeed
	name := c.definition.DisplayName + " Kandidat"
	if c.request.RouteRole != "" {
		name += " " + string(c.request.RouteRole)
	}
	route := pathing.Route{Version: pathing.RouteVersion, ID: candidateID, Name: name, Kind: pathing.RouteKindNavigation,
		Binding:   pathing.RouteBinding{RouteRole: c.request.RouteRole, CharacterName: c.request.Character, CharacterClass: evidence.World.Identity.Class.String(), ProfileID: c.request.ProfileID, Difficulty: c.request.Difficulty, MapSeed: &seed, GameVersion: c.request.GameVersion, LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: c.fingerprint.Version, AreaID: c.fingerprint.AreaID, AnchorCount: c.fingerprint.AnchorCount, Hash: c.fingerprint.Hash}},
		Recording: pathing.RouteRecording{RecordedAt: c.recordedAt, SampleDistanceTiles: guidedRecordingSampleDistance}, Playback: pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000}, Segments: segments}
	distance := terminalBossDistance(evidence)
	metadata := RouteCandidate{CandidateID: candidateID, RunID: c.request.RunID, RouteRole: c.request.RouteRole, Character: c.request.Character, Difficulty: string(c.request.Difficulty), GameVersion: c.request.GameVersion, State: RouteCandidateRecorded, MeasuredBossDistance: distance, SourceCatalogRevision: c.request.SourceCatalogRevision, SourceAssignmentRevision: c.request.SourceAssignmentRevision, SourceAssignedRouteID: c.request.SourceAssignedRouteID, CreatedAt: time.Now().UTC()}
	candidate, err := c.store.Freeze(route, metadata)
	if err != nil {
		return RouteCandidate{}, c.failLocked(RouteReasonCandidateInvalid)
	}
	c.candidate = candidate
	c.state = RouteWorkflowValidating
	reason := validateRecordingTerminal(c.contract, route, evidence)
	if reason == "" {
		candidate, err = c.store.UpdateState(candidate.CandidateID, RouteCandidateValidated, "", nil)
	} else {
		candidate, err = c.store.UpdateState(candidate.CandidateID, RouteCandidateFailed, reason, nil)
	}
	if err != nil {
		return RouteCandidate{}, c.failLocked(RouteReasonCandidateInvalid)
	}
	c.candidate = candidate
	c.reason = reason
	c.state = RouteWorkflowReturningViaPortal
	return candidate, nil
}

// CompleteSafetyReturn confirms the single post-freeze TP flow.
func (c *RecordingCoordinator) CompleteSafetyReturn(returnErr error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != RouteWorkflowReturningViaPortal {
		return fmt.Errorf("safety return invalid in state %s", c.state)
	}
	if returnErr != nil {
		candidate, err := c.store.UpdateState(c.candidate.CandidateID, RouteCandidateFailed, RouteReasonSafetyReturnFailed, nil)
		if err != nil {
			return c.failLocked(RouteReasonCandidateInvalid)
		}
		c.candidate = candidate
		return c.failLocked(RouteReasonSafetyReturnFailed)
	}
	if c.candidate.State == RouteCandidateValidated {
		c.state = RouteWorkflowCandidateReady
		c.reason = ""
		c.releaseWorkflowLocked()
		return nil
	}
	reason := c.candidate.FailureReason
	if reason == "" {
		reason = RouteReasonCandidateInvalid
	}
	return c.failLocked(reason)
}

// EmergencyCancel applies F11 semantics. Before freeze it creates no candidate
// and makes no Town Portal promise; after freeze immutable content is retained.
func (c *RecordingCoordinator) EmergencyCancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emergencyCancelLocked()
}

// Snapshot returns the current immutable coordinator projection.
func (c *RecordingCoordinator) Snapshot() RecordingSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return RecordingSnapshot{State: c.state, Reason: c.reason, CandidateID: c.candidate.CandidateID, RunID: c.request.RunID}
}

func validateRecordingTerminal(contract tasks.RecordingContract, route pathing.Route, evidence RecordingTerminalEvidence) RouteReason {
	if evidence.World.Area.ID != contract.TerminalArea || len(route.Segments) == 0 || route.Segments[len(route.Segments)-1].ToAreaID != contract.TerminalArea {
		return RouteReasonRecordingTerminalAreaMismatch
	}
	switch contract.TerminalKind {
	case tasks.RecordingTerminalBoss:
		if evidence.BossDead {
			return RouteReasonRecordingBossDead
		}
		if evidence.Boss == nil || evidence.Boss.NPCID != contract.Boss.NPCID || contract.Boss.RequireSuperUnique && evidence.Boss.MonsterTypeFlag != world.SuperUniqueMonsterFlag {
			return RouteReasonRecordingBossMissing
		}
		if world.Distance(evidence.World.Player.Position, evidence.Boss.Position) > contract.TerminalMaxDistanceTiles {
			return RouteReasonRecordingEndpointTooFar
		}
	case tasks.RecordingTerminalObject:
		if evidence.Object == nil || evidence.Object.Kind != contract.TerminalObjectKind {
			return RouteReasonRecordingObjectMissing
		}
		if world.Distance(evidence.World.Player.Position, evidence.Object.Position) > contract.TerminalMaxDistanceTiles {
			return RouteReasonRecordingEndpointTooFar
		}
	case tasks.RecordingTerminalEndpoint:
		// F9 is the explicit endpoint authority after the in-area route checks.
	default:
		return RouteReasonCandidateInvalid
	}
	for _, segment := range route.Segments {
		if segment.Movement != contract.Movement || !areaAllowed(segment.FromAreaID, contract.AllowedRouteAreas) || !areaAllowed(segment.ToAreaID, contract.AllowedRouteAreas) {
			return RouteReasonCandidateInvalid
		}
	}
	if contract.RouteRole == pathing.RouteRoleLegAcquisition && !validLegAcquisitionSegments(route.Segments) {
		return RouteReasonCandidateInvalid
	}
	if contract.RouteRole == pathing.RouteRoleCowSweep && !validCowSweepSegments(route.Segments) {
		return RouteReasonCandidateInvalid
	}
	return ""
}

func validLegAcquisitionSegments(segments []pathing.RouteSegment) bool {
	if len(segments) != 2 {
		return false
	}
	transition := segments[0].Transition
	return segments[0].FromAreaID == world.StonyField && segments[0].ToAreaID == world.Tristram &&
		transition.Type == "object_portal" && transition.ObjectKind == world.ObjectKindPermanentPortal && transition.ExpectedToArea == world.Tristram &&
		segments[1].FromAreaID == world.Tristram && segments[1].ToAreaID == world.Tristram && segments[1].Transition.Type == "terminal"
}

func validCowSweepSegments(segments []pathing.RouteSegment) bool {
	return len(segments) == 1 && segments[0].FromAreaID == world.MooMooFarm && segments[0].ToAreaID == world.MooMooFarm && segments[0].Transition.Type == "terminal"
}

func (c *RecordingCoordinator) emergencyCancelLocked() {
	if c.state == RouteWorkflowRecording {
		c.state = RouteWorkflowEmergencyCancelled
		c.reason = RouteReasonRecordingEmergencyCancelled
		c.releaseWorkflowLocked()
		return
	}
	if c.state != RouteWorkflowIdle && c.state != RouteWorkflowCandidateReady && c.state != RouteWorkflowCompleted && c.state != RouteWorkflowFailedSafe && c.state != RouteWorkflowEmergencyCancelled {
		c.state = RouteWorkflowEmergencyCancelled
		c.reason = RouteReasonRecordingEmergencyCancelled
		c.releaseWorkflowLocked()
	}
}

func (c *RecordingCoordinator) failLocked(reason RouteReason) error {
	c.state = RouteWorkflowFailedSafe
	c.reason = reason
	c.releaseWorkflowLocked()
	return fmt.Errorf("%s", reason)
}

func (c *RecordingCoordinator) releaseWorkflowLocked() {
	if c.workflowOwned {
		c.workflowOwned = false
		recordingWorkflowOwner.Unlock()
	}
}

func areaAllowed(area world.AreaID, allowed []world.AreaID) bool {
	for _, candidate := range allowed {
		if candidate == area {
			return true
		}
	}
	return false
}

func terminalBossDistance(evidence RecordingTerminalEvidence) float64 {
	if evidence.Boss == nil {
		return 0
	}
	return world.Distance(evidence.World.Player.Position, evidence.Boss.Position)
}
