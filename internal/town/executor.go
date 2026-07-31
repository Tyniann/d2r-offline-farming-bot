package town

import (
	"context"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ExecutorEvent describes one correlated Town plan transition or action.
type ExecutorEvent struct {
	Event              string
	Step               int
	Kind               StepKind
	Service            Service
	Action             string
	Reason             string
	Current            int
	Threshold          int
	BeltSlots          []int
	Mode               BuyMode
	VendorUnitID       uint32
	Vendor             Anchor
	Code               string
	Name               string
	Quality            world.ItemQuality
	IdentityKind       world.ItemIdentityKind
	IdentityKey        string
	IdentityValid      bool
	Cost               int
	VerifiedFinal      int
	ProfileID          string
	RuleID             string
	PickitAction       string
	ProfileRevision    uint64
	AssignmentRevision uint64
	MercUnitID         uint32
	HPBefore           int
	HPAfter            int
}

// ExecutorTelemetry synchronously persists Town events before progression.
type ExecutorTelemetry interface {
	EmitTown(ExecutorEvent) error
}

// StepHandler executes the current already-planned step behind its own gates.
// `InteractionAction` means real input occurred and permanently disables retry
// for that step; completion must still be proven by a later World snapshot.
type StepHandler interface {
	Tick(context.Context, PlanStep, world.State) InteractionResult
	Reset()
}

// stepStateResetter separates service-local state from graph progress shared by
// consecutive plan steps. Falling back to Reset is safe but loses both scopes.
type stepStateResetter interface {
	ResetStep()
}

// ExecutorResult is one finite Town plan outcome.
type ExecutorResult struct {
	Status InteractionStatus
	Reason Reason
	Step   int
	Done   bool
}

// Executor runs a validated plan within global budgets and sticky safety gates.
// It permits retries only before the first real action, records telemetry before
// progressing, and makes telemetry failure terminal so observed input can never
// be followed by an unrecorded action.
type Executor struct {
	plan          Plan
	budgets       Budgets
	handler       StepHandler
	telemetry     ExecutorTelemetry
	step          int
	inputs        int
	verifies      int
	retries       int
	stepAction    bool
	telemetryFail error
	done          bool
}

// NewExecutor creates a resettable Town executor without sending input.
func NewExecutor(plan Plan, budgets Budgets, handler StepHandler, telemetry ExecutorTelemetry) (*Executor, error) {
	if err := plan.Validate(); err != nil {
		return nil, fmt.Errorf("validate town plan: %w", err)
	}
	if err := budgets.Validate(); err != nil {
		return nil, err
	}
	if len(plan.Steps) > budgets.TotalSteps || handler == nil || telemetry == nil {
		return nil, fmt.Errorf("town executor contract unavailable or total step budget exceeded")
	}
	return &Executor{plan: plan, budgets: budgets, handler: handler, telemetry: telemetry}, nil
}

// Tick advances at most one handler action and blocks during pause or after stop.
func (e *Executor) Tick(ctx context.Context, state world.State, paused, stopped bool) ExecutorResult {
	if e == nil {
		return ExecutorResult{Status: InteractionFailed, Reason: ReasonBudgetExhausted, Done: true}
	}
	if e.telemetryFail != nil {
		return ExecutorResult{Status: InteractionFailed, Reason: ReasonTelemetryFailed, Step: e.step, Done: true}
	}
	if ctx == nil || ctx.Err() != nil {
		e.resetHandlers()
		e.done = true
		return ExecutorResult{Status: InteractionFailed, Reason: ReasonStopped, Step: e.step, Done: true}
	}
	if stopped {
		e.resetHandlers()
		e.done = true
		return ExecutorResult{Status: InteractionFailed, Reason: ReasonStopped, Step: e.step, Done: true}
	}
	if paused {
		return ExecutorResult{Status: InteractionPending, Reason: ReasonPaused, Step: e.step}
	}
	if e.done || e.step >= len(e.plan.Steps) {
		e.done = true
		return ExecutorResult{Status: InteractionComplete, Step: e.step, Done: true}
	}
	step := e.plan.Steps[e.step]
	result := e.handler.Tick(ctx, step, state)
	switch result.Status {
	case InteractionAction:
		// From this point the step is non-retryable: D2R may have accepted the
		// input even when the following Memory verification is delayed or lost.
		e.inputs++
		e.stepAction = true
		e.verifies = 0
		if e.inputs > e.budgets.InputAttempts {
			return e.fail(ReasonBudgetExhausted)
		}
		if err := e.telemetry.EmitTown(executorEvent("town_action", e.step, step, result)); err != nil {
			e.telemetryFail = err
			return e.fail(ReasonTelemetryFailed)
		}
		return ExecutorResult{Status: InteractionAction, Step: e.step}
	case InteractionComplete:
		// Emit before advancing. Consumers may therefore treat a persisted
		// completion event as the authoritative plan transition.
		if err := e.telemetry.EmitTown(executorEvent("town_step_completed", e.step, step, result)); err != nil {
			e.telemetryFail = err
			return e.fail(ReasonTelemetryFailed)
		}
		e.step++
		e.verifies, e.retries = 0, 0
		e.stepAction = false
		e.resetStepHandler()
		if e.step >= len(e.plan.Steps) {
			e.done = true
			return ExecutorResult{Status: InteractionComplete, Step: e.step, Done: true}
		}
		return ExecutorResult{Status: InteractionPending, Step: e.step}
	case InteractionFailed:
		// Only failures that provably preceded input may rebuild step-local pins.
		if !e.stepAction && e.retries < e.budgets.RetryAttempts {
			e.retries++
			e.verifies = 0
			e.resetStepHandler()
			return ExecutorResult{Status: InteractionPending, Step: e.step}
		}
		return e.fail(Reason(result.Reason))
	default:
		e.verifies++
		if e.verifies > e.budgets.VerifyAttempts {
			return e.fail(ReasonBudgetExhausted)
		}
		return ExecutorResult{Status: InteractionPending, Step: e.step}
	}
}

func executorEvent(name string, index int, step PlanStep, result InteractionResult) ExecutorEvent {
	return ExecutorEvent{
		Event: name, Step: index, Kind: step.Kind, Service: step.Service, Action: result.Action, Reason: result.Reason,
		Current: result.Current, Threshold: result.Threshold, BeltSlots: append([]int(nil), result.BeltSlots...), Mode: result.Mode,
		VendorUnitID: result.UnitID, Vendor: result.Vendor, Cost: result.Cost, VerifiedFinal: result.VerifiedFinal,
		Code: result.Code, Name: result.Name, Quality: result.Quality, IdentityKind: result.IdentityKind,
		IdentityKey: result.IdentityKey, IdentityValid: result.IdentityValid,
		ProfileID: result.ProfileID, RuleID: result.RuleID, PickitAction: result.PickitAction,
		ProfileRevision: result.ProfileRevision, AssignmentRevision: result.AssignmentRevision,
	}
}

// Reset discards plan progress, sticky errors, pins, and pending handler state.
func (e *Executor) Reset() {
	if e == nil {
		return
	}
	e.resetHandlers()
	e.step, e.inputs, e.verifies, e.retries = 0, 0, 0, 0
	e.stepAction, e.done, e.telemetryFail = false, false, nil
}

func (e *Executor) fail(reason Reason) ExecutorResult {
	e.done = true
	e.resetHandlers()
	return ExecutorResult{Status: InteractionFailed, Reason: reason, Step: e.step, Done: true}
}

func (e *Executor) resetHandlers() {
	if e != nil && e.handler != nil {
		e.handler.Reset()
	}
}

func (e *Executor) resetStepHandler() {
	if e == nil || e.handler == nil {
		return
	}
	if resetter, ok := e.handler.(stepStateResetter); ok {
		// Preserve graph traversal and anchor continuity between service and
		// waypoint steps; only NPC/shop/order state belongs to the finished step.
		resetter.ResetStep()
		return
	}
	e.handler.Reset()
}
