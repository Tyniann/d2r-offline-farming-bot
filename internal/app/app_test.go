package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
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

func TestProbeShouldLogOnValueChange(t *testing.T) {
	prev := validSnapshot(100)
	cur := validSnapshot(90)
	if !probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, false) {
		t.Fatal("expected log on HP change")
	}
}

func TestProbeShouldNotLogOnlyPositionChange(t *testing.T) {
	prev := validSnapshot(100)
	cur := validSnapshot(100)
	cur.PosX++
	cur.PosY++

	if probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, false) {
		t.Fatal("unexpected log for position-only change within heartbeat")
	}
}

func TestProbeShouldLogPositionChangeInVerbose(t *testing.T) {
	prev := validSnapshot(100)
	cur := validSnapshot(100)
	cur.PosX++
	cur.PosY++

	if !probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, true) {
		t.Fatal("expected log for position-only change in verbose mode")
	}
}

func TestProbeShouldLogAreaChange(t *testing.T) {
	prev := validSnapshot(100)
	cur := validSnapshot(100)
	cur.AreaID++

	if !probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, false) {
		t.Fatal("expected log on area change")
	}
}

func TestProbeShouldLogHeartbeat(t *testing.T) {
	snap := validSnapshot(100)
	last := time.Now().Add(-6 * time.Second)
	if !probeShouldLog(snap, snap, last, probeHeartbeat, false, false) {
		t.Fatal("expected log after heartbeat interval")
	}
}

func TestProbeShouldNotLogUnchanged(t *testing.T) {
	snap := validSnapshot(100)
	last := time.Now()
	if probeShouldLog(snap, snap, last, probeHeartbeat, false, false) {
		t.Fatal("unexpected log for unchanged snapshot within heartbeat")
	}
}

func TestProbeShouldLogInvalidReasonChange(t *testing.T) {
	prev := memory.Snapshot{Valid: false, Reason: memory.ReasonNotInGame}
	cur := memory.Snapshot{Valid: false, Reason: memory.ReasonStatsUnavailable}
	if !probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, false) {
		t.Fatal("expected log when invalid reason changes")
	}
}

func TestProbeShouldNotLogSameInvalidReason(t *testing.T) {
	prev := memory.Snapshot{Valid: false, Reason: memory.ReasonNotInGame}
	cur := memory.Snapshot{Valid: false, Reason: memory.ReasonNotInGame}
	if probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, false) {
		t.Fatal("unexpected log for same invalid reason within heartbeat")
	}
}

func TestProbeShouldLogInvalidOnHeartbeat(t *testing.T) {
	prev := memory.Snapshot{Valid: false, Reason: memory.ReasonNotInGame}
	cur := memory.Snapshot{Valid: false, Reason: memory.ReasonNotInGame}
	last := time.Now().Add(-6 * time.Second)
	if !probeShouldLog(prev, cur, last, probeHeartbeat, false, false) {
		t.Fatal("expected heartbeat log for unchanged invalid snapshot")
	}
}

func TestProbeShouldNotLogPositionOnlyOnHeartbeat(t *testing.T) {
	prev := validSnapshot(100)
	cur := validSnapshot(100)
	cur.PosX++
	cur.PosY++
	last := time.Now().Add(-6 * time.Second)

	if probeShouldLog(prev, cur, last, probeHeartbeat, false, false) {
		t.Fatal("unexpected heartbeat Info log for position-only change without verbose")
	}
	if !probeShouldLog(prev, cur, last, probeHeartbeat, false, true) {
		t.Fatal("expected heartbeat log for position-only change in verbose mode")
	}
}

func TestProbeShouldLogOnForce(t *testing.T) {
	snap := validSnapshot(100)
	if !probeShouldLog(snap, snap, time.Now(), probeHeartbeat, true, false) {
		t.Fatal("expected log when force=true after re-attach")
	}
}

func TestProbeShouldLogValidToInvalid(t *testing.T) {
	prev := validSnapshot(100)
	cur := memory.Snapshot{Valid: false, Reason: memory.ReasonNotInGame}
	if !probeShouldLog(prev, cur, time.Now(), probeHeartbeat, false, false) {
		t.Fatal("expected log when snapshot becomes invalid")
	}
}

func TestProbeLogStateAfterProcessLostAndReattach(t *testing.T) {
	var lastLogged memory.Snapshot
	var lastLog time.Time

	lastLogged = memory.Snapshot{}
	lastLog = time.Time{}

	snap := validSnapshot(100)
	if !probeShouldLog(lastLogged, snap, lastLog, probeHeartbeat, true, false) {
		t.Fatal("expected forced log after re-attach")
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
	}
}

func TestRunTickWithoutProbeDoesNotCallSnapshot(t *testing.T) {
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
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0", probe.calls)
	}
	if proc.pollCalls != 1 {
		t.Fatalf("Poll() calls = %d, want 1", proc.pollCalls)
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
}

func TestRunTickLostResetsProbeState(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntime(proc, probe, Options{Probe: true})
	state := &runState{
		attached: true,
		probe: probeLoopState{
			forceLog:   false,
			lastLogged: validSnapshot(100),
			lastLog:    time.Now(),
		},
	}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if state.attached {
		t.Fatal("expected detached after lost")
	}
	if !state.probe.lastLog.IsZero() || state.probe.lastLogged.Valid {
		t.Fatal("expected probe log state reset on lost")
	}
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0 on lost", probe.calls)
	}
}

func TestRunTickReattachSetsProbeForceLog(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		status: process.Status{State: process.StateAttached, PID: 1, ModuleBase: 0x1000},
	}
	rt := testRuntime(proc, probe, Options{Probe: true})
	state := &runState{attached: false}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if !state.probe.forceLog {
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
	if state.probe.forceLog {
		t.Fatal("unexpected forceLog when probe disabled")
	}
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0", probe.calls)
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
