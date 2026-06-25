package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestProcessServiceImplementsProcessAccess(t *testing.T) {
	var _ memory.ProcessAccess = (*process.Service)(nil)
}

func validSnapshot(hp uint32) memory.Snapshot {
	return memory.Snapshot{
		Valid:   true,
		HP:      hp,
		MaxHP:   100,
		Mana:    50,
		MaxMana: 50,
		AreaID:  1,
		PosX:    10,
		PosY:    20,
	}
}

type mockProcess struct {
	attachErr   error
	pollStatus  process.Status
	status      process.Status
	attachCalls int
	pollCalls   int
}

func (m *mockProcess) Attach(_ context.Context) error {
	m.attachCalls++
	return m.attachErr
}

func (m *mockProcess) Poll() process.Status {
	m.pollCalls++
	return m.pollStatus
}

func (m *mockProcess) Status() process.Status {
	return m.status
}

func (m *mockProcess) Detach() error { return nil }

func (m *mockProcess) Ready() bool { return true }

type mockProbe struct {
	snap  memory.Snapshot
	calls int
}

func (m *mockProbe) Snapshot() memory.Snapshot {
	m.calls++
	return m.snap
}

func testRuntime(proc processController, probe snapshotReader, opts Options) *Runtime {
	return &Runtime{
		Config: &config.Config{
			Process: config.ProcessConfig{ProcessName: "D2R.exe"},
		},
		Options: opts,
		Log:     config.NewLogger("error"),
		Process: proc,
		Probe:   probe,
		World:   world.NewModel(config.NewLogger("error")),
	}
}

func TestRunTickWithoutProbeUpdatesWorld(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached},
		status:     process.Status{State: process.StateAttached},
	}
	rt := testRuntime(proc, probe, Options{Probe: false})
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 {
		t.Fatalf("Snapshot() calls = %d, want 1", probe.calls)
	}
	if proc.pollCalls != 1 {
		t.Fatalf("Poll() calls = %d, want 1", proc.pollCalls)
	}
	cur := rt.World.Current()
	if !cur.Valid {
		t.Fatal("expected world state updated without --probe")
	}
	if cur.Player.HP != 100 {
		t.Fatalf("world HP = %d, want 100", cur.Player.HP)
	}
	if state.world.forceLog {
		t.Fatal("unexpected forceLog when probe disabled")
	}
}

func TestRunTickPositionOnlyKeepsLastLoggedWithoutVerbose(t *testing.T) {
	snap1 := validSnapshot(100)
	snap1.PosX = 10
	snap2 := validSnapshot(100)
	snap2.PosX = 99

	probe := &mockProbe{snap: snap1}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached},
		status:     process.Status{State: process.StateAttached},
	}
	rt := testRuntime(proc, probe, Options{Probe: true})
	state := &runState{
		attached: true,
		world: worldLoopState{
			lastLogged: world.FromSnapshot(snap1),
			lastLog:    time.Now(),
		},
	}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	loggedPos := state.world.lastLogged.Player.Position.X

	probe.snap = snap2
	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if rt.World.Current().Player.Position.X != 99 {
		t.Fatalf("Current pos = %d, want 99 after position-only update", rt.World.Current().Player.Position.X)
	}
	if state.world.lastLogged.Player.Position.X != loggedPos {
		t.Fatalf("lastLogged pos changed from %d to %d; run loop must compare lastLogged, not Current()", loggedPos, state.world.lastLogged.Player.Position.X)
	}
}

func TestRunTickWithProbeCallsSnapshotAfterPoll(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached},
		status:     process.Status{State: process.StateAttached},
	}
	rt := testRuntime(proc, probe, Options{Probe: true})
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if proc.pollCalls != 1 {
		t.Fatalf("Poll() calls = %d, want 1 before Snapshot", proc.pollCalls)
	}
	if probe.calls != 1 {
		t.Fatalf("Snapshot() calls = %d, want 1", probe.calls)
	}
	cur := rt.World.Current()
	if !cur.Valid || cur.Player.HP != 100 {
		t.Fatalf("unexpected world state: %+v", cur)
	}
}

func TestRunTickLostResetsWorldState(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntime(proc, probe, Options{Probe: true})
	rt.World.Update(validSnapshot(100))
	state := &runState{
		attached: true,
		world: worldLoopState{
			forceLog:   false,
			lastLogged: validWorldState(100),
			lastLog:    time.Now(),
		},
	}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if state.attached {
		t.Fatal("expected detached after lost")
	}
	if !state.world.lastLog.IsZero() || state.world.lastLogged.Valid {
		t.Fatal("expected world log state reset on lost")
	}
	cur := rt.World.Current()
	if cur.Valid {
		t.Fatal("expected invalid world state after process lost")
	}
	if cur.Reason != worldResetReasonProcessLost {
		t.Fatalf("Reason = %q, want %q", cur.Reason, worldResetReasonProcessLost)
	}
	if cur.Area != (world.Area{}) || cur.Player != (world.Player{}) {
		t.Fatalf("expected zero Area/Player after reset, got Area=%+v Player=%+v", cur.Area, cur.Player)
	}
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0 on lost", probe.calls)
	}
}

func TestRunTickLostResetsWorldWithoutProbe(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntime(proc, probe, Options{Probe: false})
	rt.World.Update(validSnapshot(100))
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	cur := rt.World.Current()
	if cur.Valid || cur.Reason != worldResetReasonProcessLost {
		t.Fatalf("expected process_lost reset without probe, got %+v", cur)
	}
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0 on lost", probe.calls)
	}
}

func TestRunTickReattachSetsWorldForceLog(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		status: process.Status{State: process.StateAttached, PID: 1, ModuleBase: 0x1000},
	}
	rt := testRuntime(proc, probe, Options{Probe: true})
	state := &runState{attached: false}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if !state.world.forceLog {
		t.Fatal("expected forceLog after attach with probe enabled")
	}
}

type orderProbe struct {
	mockProbe
	pollCalls *int
}

func (m *orderProbe) Snapshot() memory.Snapshot {
	if *m.pollCalls == 0 {
		panic("Snapshot called before Poll")
	}
	return m.mockProbe.Snapshot()
}

func TestRunTickPollBeforeSnapshot(t *testing.T) {
	probe := &orderProbe{
		mockProbe: mockProbe{snap: validSnapshot(100)},
	}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached},
		status:     process.Status{State: process.StateAttached},
	}
	probe.pollCalls = &proc.pollCalls

	rt := testRuntime(proc, probe, Options{Probe: true})
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
}

func TestRunTickReattachWithoutProbeSkipsForceLog(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		status: process.Status{State: process.StateAttached, PID: 1, ModuleBase: 0x1000},
	}
	rt := testRuntime(proc, probe, Options{Probe: false})
	state := &runState{attached: false}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if state.world.forceLog {
		t.Fatal("unexpected forceLog when probe disabled")
	}
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0 on attach tick", probe.calls)
	}
}

func TestRunTickAttachTimeoutOnInitialWait(t *testing.T) {
	proc := &mockProcess{attachErr: process.ErrNotFound}
	rt := testRuntime(proc, &mockProbe{}, Options{})
	rt.Config.Process.AttachTimeoutMs = 50
	state := &runState{attachWaitStarted: time.Now().Add(-100 * time.Millisecond)}

	err := rt.runTick(context.Background(), state)
	if err == nil {
		t.Fatal("expected attach timeout error")
	}
}

func TestRunTickAttachTimeoutNotAppliedAfterLost(t *testing.T) {
	proc := &mockProcess{attachErr: process.ErrNotFound}
	rt := testRuntime(proc, &mockProbe{}, Options{})
	rt.Config.Process.AttachTimeoutMs = 50
	state := &runState{
		hasEverAttached:   true,
		attachWaitStarted: time.Now().Add(-100 * time.Millisecond),
	}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatalf("unexpected error after lost re-attach wait: %v", err)
	}
}
