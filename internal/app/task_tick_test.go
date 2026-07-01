package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func attachedProc() *mockProcess {
	return &mockProcess{
		pollStatus: process.Status{State: process.StateAttached},
		status:     process.Status{State: process.StateAttached, PID: 1},
	}
}

func TestShouldTickTasksBlocksWhenPhaseLoading(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{}, "countess")
	st := validWorldState(100)
	st.Phase = world.GamePhaseLoading
	if rt.shouldTickTasks(st) {
		t.Fatal("expected false when phase=loading")
	}
}

func TestShouldTickTasksRequiresConfiguredRun(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{}, "")
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false without configured run")
	}
}

func TestShouldTickTasksRequiresValidWorldAndBoundInput(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: false}, Options{}, "countess")
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when input not bound")
	}

	rt = testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{}, "countess")
	if rt.shouldTickTasks(world.State{Valid: false}) {
		t.Fatal("expected false when world invalid")
	}
}

func TestShouldTickTasksBlocksWhenPausedOrStopped(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true, paused: true}, Options{}, "countess")
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when paused")
	}

	rt = testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true, stopped: true}, Options{}, "countess")
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when stopped")
	}
}

func TestShouldTickTasksRequiresConfigInputEnabled(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{}, "countess")
	rt.Config.Input.Enabled = false
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when config input.enabled is false")
	}
}

func TestShouldTickTasksRequiresStatusEnabled(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: false, bound: true}, Options{}, "countess")
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when input status enabled is false")
	}
}

func TestShouldTickTasksBlocksWhenTerminalOrReset(t *testing.T) {
	rt := testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{}, "countess")
	rt.Tasks.Tick(context.Background(), validWorldState(100), time.Now())
	for i := 0; i < 10; i++ {
		rt.Tasks.Tick(context.Background(), validWorldState(100), time.Now())
	}
	if !rt.Tasks.Terminal() {
		t.Fatal("expected terminal run for guard test")
	}
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when terminal")
	}

	rt = testRuntimeWithTasks(attachedProc(), &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{}, "countess")
	rt.Tasks.Reset("process_lost")
	if rt.shouldTickTasks(validWorldState(100)) {
		t.Fatal("expected false when reset")
	}
}

func TestRunTickTicksTasksAfterWorldUpdate(t *testing.T) {
	in := &mockInput{enabled: true, bound: true}
	probe := &mockProbe{snap: validSnapshot(100)}
	rt := testRuntimeWithTasks(attachedProc(), probe, in, Options{}, "countess")
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if rt.Tasks.ConfiguredRun() != "countess" {
		t.Fatalf("ConfiguredRun = %q", rt.Tasks.ConfiguredRun())
	}
	if rt.Tasks.Terminal() {
		t.Fatal("run should not finish on first tick")
	}

	for i := 0; i < 3; i++ {
		if err := rt.runTick(context.Background(), state); err != nil {
			t.Fatal(err)
		}
	}
	if !rt.Tasks.Terminal() {
		t.Fatal("expected stub run to finish after guarded ticks")
	}
}

func TestRunTickSkipsTasksWhenGuardsFail(t *testing.T) {
	in := &mockInput{
		enabled: true,
		bound:   false,
		bindErr: input.ErrWindowNotFound,
	}
	probe := &mockProbe{snap: validSnapshot(100)}
	rt := testRuntimeWithTasks(attachedProc(), probe, in, Options{}, "countess")
	state := &runState{
		attached: true,
		input:    inputLoopState{lastBindAttempt: time.Now().Add(-2 * time.Second)},
	}

	for i := 0; i < 5; i++ {
		if err := rt.runTick(context.Background(), state); err != nil {
			t.Fatal(err)
		}
	}
	if in.bound {
		t.Fatal("input should remain unbound in this test")
	}
	if rt.Tasks.Terminal() || rt.Tasks.WasReset() {
		t.Fatal("task run must not start when input is unbound")
	}
}

type orderWorldProbe struct {
	mockProbe
	worldUpdated *bool
}

func (m *orderWorldProbe) Snapshot() memory.Snapshot {
	*m.worldUpdated = true
	return m.mockProbe.Snapshot()
}

func TestRunTickWorldUpdateBeforeTaskTick(t *testing.T) {
	worldUpdated := false
	probe := &orderWorldProbe{
		mockProbe:    mockProbe{snap: validSnapshot(100)},
		worldUpdated: &worldUpdated,
	}
	in := &mockInput{enabled: true, bound: true}
	rt := testRuntimeWithTasks(attachedProc(), probe, in, Options{}, "countess")
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if !worldUpdated {
		t.Fatal("World.Update must run before task tick")
	}
}

func TestRunTickLostResetsTasksAfterUnbind(t *testing.T) {
	var tasksResetAtUnbind bool
	in := &trackingInput{mockInput: mockInput{bound: true}}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntimeWithTasks(proc, &mockProbe{}, in, Options{}, "countess")
	in.onUnbind = func() {
		tasksResetAtUnbind = rt.Tasks.WasReset()
	}
	rt.World.Update(validSnapshot(100))
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if tasksResetAtUnbind {
		t.Fatal("Tasks.Reset should run after Unbind")
	}
	if !rt.Tasks.WasReset() {
		t.Fatal("expected task reset on process lost")
	}
	if in.unbindCalls != 1 {
		t.Fatalf("Unbind calls = %d, want 1", in.unbindCalls)
	}
}

func TestRunTickLostTasksResetNoOpPassive(t *testing.T) {
	in := &mockInput{bound: true}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{})
	rt.World.Update(validSnapshot(100))
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if rt.Tasks.WasReset() {
		t.Fatal("passive mode should not set WasReset")
	}
}

func TestTestRuntimeWithInputHasTasksAndPathing(t *testing.T) {
	rt := testRuntimeWithInput(attachedProc(), &mockProbe{}, &mockInput{}, Options{})
	if rt.Tasks == nil || rt.Pathing == nil {
		t.Fatal("expected Tasks and Pathing on test runtime")
	}
	if !rt.Tasks.Ready() {
		t.Fatal("expected ready task runner")
	}
}

// Ensure compile-time interface satisfaction for task deps.
func TestTasksDepsCompileCheck(t *testing.T) {
	var _ tasks.Deps
}
