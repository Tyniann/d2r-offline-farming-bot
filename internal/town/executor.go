package town

import (
	"context"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ExecutorEvent describes one correlated Town plan transition or action.
type ExecutorEvent struct {
	Event         string
	Step          int
	Kind          StepKind
	Service       Service
	Action        string
	Reason        string
	Current       int
	Threshold     int
	BeltSlots     []int
	Mode          BuyMode
	VendorUnitID  uint32
	Vendor        Anchor
	Cost          int
	VerifiedFinal int
}

// ExecutorTelemetry synchronously persists Town events before progression.
type ExecutorTelemetry interface {
	EmitTown(ExecutorEvent) error
}

// StepHandler executes the current already-planned step behind its own gates.
type StepHandler interface {
	Tick(context.Context, PlanStep, world.State) InteractionResult
	Reset()
}

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
		resetter.ResetStep()
		return
	}
	e.handler.Reset()
}
