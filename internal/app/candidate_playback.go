package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

// CandidatePlaybackDriver exposes only navigation and verification primitives.
// Combat, loot, Town services, and Save & Exit are deliberately absent.
type CandidatePlaybackDriver interface {
	EnsureTown(context.Context, town.OriginAct) error
	TravelToStart(context.Context, tasks.RecordingContract) error
	PlayCandidate(context.Context, pathing.Route) error
	TerminalEvidence(context.Context, tasks.RecordingContract) (RecordingTerminalEvidence, error)
	ReturnAfterTest(context.Context, town.OriginAct) error
}

// CandidateTestOrchestrator owns isolated candidate-only playback and terminal validation.
type CandidateTestOrchestrator struct {
	store    *CandidateStore
	registry *tasks.RunRegistry
}

// NewCandidateTestOrchestrator creates a candidate test authority.
func NewCandidateTestOrchestrator(store *CandidateStore, registry *tasks.RunRegistry) (*CandidateTestOrchestrator, error) {
	if store == nil || registry == nil {
		return nil, fmt.Errorf("candidate test requires store and run registry")
	}
	return &CandidateTestOrchestrator{store: store, registry: registry}, nil
}

// Test runs only the immutable candidate, validates terminal semantics, and returns to Town.
func (o *CandidateTestOrchestrator) Test(ctx context.Context, candidateID string, driver CandidatePlaybackDriver) (RouteCandidate, error) {
	return o.TestWithProgress(ctx, candidateID, driver, nil)
}

// TestWithProgress runs the same navigation-only test while reporting coarse,
// non-authoritative workflow transitions for the local dashboard.
func (o *CandidateTestOrchestrator) TestWithProgress(ctx context.Context, candidateID string, driver CandidatePlaybackDriver, reporter RouteWorkflowReporter) (RouteCandidate, error) {
	if driver == nil {
		return RouteCandidate{}, fmt.Errorf("candidate test driver missing")
	}
	candidate, route, err := o.store.Load(candidateID)
	if err != nil {
		return RouteCandidate{}, err
	}
	if candidate.State != RouteCandidateValidated && candidate.State != RouteCandidateTestPassed {
		return RouteCandidate{}, fmt.Errorf("%s", RouteReasonCandidateInvalid)
	}
	definition, ok := o.registry.Definition(candidate.RunID)
	if !ok {
		return RouteCandidate{}, fmt.Errorf("%s", RouteReasonCandidateInvalid)
	}
	contract, ok := definition.RecordingForRole(candidate.RouteRole)
	if !ok || route.Binding.RouteRole != candidate.RouteRole {
		return RouteCandidate{}, fmt.Errorf("%s", RouteReasonCandidateInvalid)
	}
	if candidate.State == RouteCandidateTestPassed {
		return candidate, nil
	}
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreparingPlayback, Progress: 0.05})
	if candidate, err = o.store.UpdateState(candidateID, RouteCandidateTestRunning, "", nil); err != nil {
		return RouteCandidate{}, err
	}
	startFail := func(cause error) (RouteCandidate, error) {
		// A missing or wrong Town handoff is an operator/environment preflight
		// failure, not evidence against the immutable route. Restore the prior
		// validated state so the same candidate can be retried from portal_arrival.
		retained, updateErr := o.store.UpdateState(candidateID, RouteCandidateValidated, "", nil)
		if updateErr != nil {
			return RouteCandidate{}, updateErr
		}
		return retained, fmt.Errorf("%s: %w", RouteReasonTestStartFailed, cause)
	}
	fail := func(reason RouteReason, cause error) (RouteCandidate, error) {
		_, _ = o.store.UpdateState(candidateID, RouteCandidateFailed, reason, nil)
		_ = driver.ReturnAfterTest(ctx, contract.EgressOriginAct)
		return RouteCandidate{}, fmt.Errorf("%s: %w", reason, cause)
	}
	if actionErr := driver.EnsureTown(ctx, contract.EgressOriginAct); actionErr != nil {
		return startFail(actionErr)
	}
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreparingPlayback, Progress: 0.2})
	if actionErr := driver.TravelToStart(ctx, contract); actionErr != nil {
		return startFail(actionErr)
	}
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPreparingPlayback, Progress: 0.35})
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowPlayingCandidate, AreaID: uint32(contract.AllowedStartArea), Progress: 0.5})
	if actionErr := driver.PlayCandidate(ctx, route); actionErr != nil {
		return fail(RouteReasonTestPlaybackFailed, actionErr)
	}
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowValidatingTerminal, AreaID: uint32(contract.TerminalArea), Segment: len(route.Segments) - 1, Progress: 0.8})
	evidence, err := driver.TerminalEvidence(ctx, contract)
	if err != nil {
		return fail(RouteReasonTestTerminalMismatch, err)
	}
	if reason := validateRecordingTerminal(contract, route, evidence); reason != "" {
		return fail(RouteReasonTestTerminalMismatch, fmt.Errorf("%s", reason))
	}
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowReturningAfterTest, AreaID: uint32(evidence.World.Area.ID), Progress: 0.9})
	if actionErr := driver.ReturnAfterTest(ctx, contract.EgressOriginAct); actionErr != nil {
		_, _ = o.store.UpdateState(candidateID, RouteCandidateFailed, RouteReasonSafetyReturnFailed, nil)
		return RouteCandidate{}, fmt.Errorf("%s: %w", RouteReasonSafetyReturnFailed, actionErr)
	}
	now := time.Now().UTC()
	candidate, err = o.store.UpdateState(candidateID, RouteCandidateTestPassed, "", &now)
	if err != nil {
		return RouteCandidate{}, err
	}
	reportRouteWorkflow(reporter, RouteWorkflowProgress{State: RouteWorkflowAwaitingPublishConfirmation, Progress: 1})
	return candidate, nil
}
