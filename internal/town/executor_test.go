package town

import (
	"context"
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type executorHandlerMock struct {
	results []InteractionResult
	calls   int
	resets  int
	steps   int
}

func (m *executorHandlerMock) Tick(context.Context, PlanStep, world.State) InteractionResult {
	m.calls++
	if len(m.results) == 0 {
		return InteractionResult{Status: InteractionPending}
	}
	r := m.results[0]
	m.results = m.results[1:]
	return r
}
func (m *executorHandlerMock) Reset()     { m.resets++ }
func (m *executorHandlerMock) ResetStep() { m.steps++ }

type executorTelemetryMock struct {
	events []ExecutorEvent
	err    error
}

func (m *executorTelemetryMock) EmitTown(e ExecutorEvent) error {
	m.events = append(m.events, e)
	return m.err
}

func executorPlan() Plan {
	return Plan{Origin: Origin{Act: OriginAct1}, Steps: []PlanStep{{Phase: PlanPhaseServices, Kind: StepService, Service: ServicePotions, Act: OriginAct1}}}
}

func TestExecutorCompletesFinitePlanAndReset(t *testing.T) {
	h := &executorHandlerMock{results: []InteractionResult{{Status: InteractionAction, Action: "vendor_buy_bulk", UnitID: 9}, {Status: InteractionComplete, Done: true}}}
	tm := &executorTelemetryMock{}
	e, err := NewExecutor(executorPlan(), Budgets{InputAttempts: 1, VerifyAttempts: 2, RetryAttempts: 1, TotalSteps: 1}, h, tm)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Tick(context.Background(), world.State{}, false, false); got.Status != InteractionAction {
		t.Fatalf("action=%+v", got)
	}
	if got := e.Tick(context.Background(), world.State{}, false, false); !got.Done || got.Status != InteractionComplete {
		t.Fatalf("complete=%+v", got)
	}
	if len(tm.events) != 2 || h.calls != 2 || h.steps != 1 || h.resets != 0 {
		t.Fatalf("events=%v calls=%d", tm.events, h.calls)
	}
	e.Reset()
	if e.step != 0 || e.inputs != 0 || e.done || h.resets != 1 {
		t.Fatalf("reset state=%+v", e)
	}
}

func TestExecutorCarriesHandlerMetadataIntoTelemetry(t *testing.T) {
	h := &executorHandlerMock{results: []InteractionResult{{Status: InteractionAction, Action: "vendor_buy_bulk", UnitID: 9, Current: 1, Threshold: 2, BeltSlots: []int{1}, Mode: BuyModeBulk, Vendor: AnchorAkara, Cost: 750}}}
	tm := &executorTelemetryMock{}
	e, _ := NewExecutor(executorPlan(), Budgets{InputAttempts: 1, VerifyAttempts: 2, TotalSteps: 1}, h, tm)
	_ = e.Tick(context.Background(), world.State{}, false, false)
	if len(tm.events) != 1 || tm.events[0].Current != 1 || tm.events[0].Threshold != 2 || tm.events[0].Cost != 750 || tm.events[0].Vendor != AnchorAkara || tm.events[0].Mode != BuyModeBulk {
		t.Fatalf("event=%+v", tm.events)
	}
}

func TestExecutorPauseStopAndTelemetryFailureBlockFollowingInput(t *testing.T) {
	h := &executorHandlerMock{results: []InteractionResult{{Status: InteractionAction, Action: "buy"}, {Status: InteractionAction, Action: "repeat"}}}
	tm := &executorTelemetryMock{err: errors.New("disk full")}
	e, _ := NewExecutor(executorPlan(), Budgets{InputAttempts: 2, VerifyAttempts: 2, TotalSteps: 1}, h, tm)
	if got := e.Tick(context.Background(), world.State{}, true, false); got.Reason != ReasonPaused || h.calls != 0 {
		t.Fatalf("pause=%+v calls=%d", got, h.calls)
	}
	if got := e.Tick(context.Background(), world.State{}, false, false); got.Reason != ReasonTelemetryFailed || h.calls != 1 {
		t.Fatalf("telemetry=%+v", got)
	}
	if got := e.Tick(context.Background(), world.State{}, false, false); got.Reason != ReasonTelemetryFailed || h.calls != 1 {
		t.Fatalf("sticky=%+v calls=%d", got, h.calls)
	}
	e.Reset()
	if got := e.Tick(context.Background(), world.State{}, false, true); got.Reason != ReasonStopped || h.calls != 1 {
		t.Fatalf("stop=%+v calls=%d", got, h.calls)
	}
}

func TestExecutorContextCancellationBlocksHandler(t *testing.T) {
	h := &executorHandlerMock{results: []InteractionResult{{Status: InteractionAction, Action: "must_not_run"}}}
	e, _ := NewExecutor(executorPlan(), Budgets{InputAttempts: 1, VerifyAttempts: 1, TotalSteps: 1}, h, &executorTelemetryMock{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := e.Tick(ctx, world.State{}, false, false); got.Reason != ReasonStopped || h.calls != 0 || h.resets != 1 {
		t.Fatalf("cancel=%+v calls=%d resets=%d", got, h.calls, h.resets)
	}
}

func TestExecutorNeverRetriesAfterActionAndBoundsPreActionRetry(t *testing.T) {
	h := &executorHandlerMock{results: []InteractionResult{{Status: InteractionAction, Action: "vendor_buy_bulk"}, {Status: InteractionFailed, Reason: "shop_closed", Done: true}, {Status: InteractionAction, Action: "must_not_run"}}}
	e, _ := NewExecutor(executorPlan(), Budgets{InputAttempts: 2, VerifyAttempts: 2, RetryAttempts: 2, TotalSteps: 1}, h, &executorTelemetryMock{})
	_ = e.Tick(context.Background(), world.State{}, false, false)
	if got := e.Tick(context.Background(), world.State{}, false, false); !got.Done || h.calls != 2 {
		t.Fatalf("post-action failure=%+v calls=%d", got, h.calls)
	}
	h = &executorHandlerMock{results: []InteractionResult{{Status: InteractionFailed, Reason: "npc_lost"}, {Status: InteractionComplete}}}
	e, _ = NewExecutor(executorPlan(), Budgets{InputAttempts: 1, VerifyAttempts: 2, RetryAttempts: 1, TotalSteps: 1}, h, &executorTelemetryMock{})
	if got := e.Tick(context.Background(), world.State{}, false, false); got.Done {
		t.Fatalf("retry=%+v", got)
	}
	if got := e.Tick(context.Background(), world.State{}, false, false); !got.Done || got.Status != InteractionComplete {
		t.Fatalf("retry complete=%+v", got)
	}
}

func TestExecutorVerifyAndTotalBudgets(t *testing.T) {
	h := &executorHandlerMock{}
	e, _ := NewExecutor(executorPlan(), Budgets{InputAttempts: 1, VerifyAttempts: 1, TotalSteps: 1}, h, &executorTelemetryMock{})
	if got := e.Tick(context.Background(), world.State{}, false, false); got.Done {
		t.Fatalf("first pending=%+v", got)
	}
	if got := e.Tick(context.Background(), world.State{}, false, false); got.Reason != ReasonBudgetExhausted {
		t.Fatalf("budget=%+v", got)
	}
	if _, err := NewExecutor(executorPlan(), Budgets{InputAttempts: 1, VerifyAttempts: 1, TotalSteps: 0}, h, &executorTelemetryMock{}); err == nil {
		t.Fatal("invalid total budget accepted")
	}
}
