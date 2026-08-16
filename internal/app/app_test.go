package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestProcessServiceImplementsProcessAccess(t *testing.T) {
	var _ memory.ProcessAccess = (*process.Service)(nil)
}

func TestInputControllerInterface(t *testing.T) {
	var _ inputController = (*mockInput)(nil)
	var _ inputController = (*input.Controller)(nil)
}

func TestInputTestControllerInterface(t *testing.T) {
	var _ inputTestController = (*mockInputTest)(nil)
	var _ inputTestController = (*input.Controller)(nil)
}

func validSnapshot(hp uint32) memory.Snapshot {
	snap := memory.Snapshot{
		Valid:   true,
		Phase:   memory.GamePhaseInGame,
		HP:      hp,
		MaxHP:   100,
		Mana:    50,
		MaxMana: 50,
		AreaID:  1,
		PosX:    10,
		PosY:    20,
	}
	snap.PlayerSkills = memory.PlayerSkills{
		LeftSkill:  memory.SkillAttack,
		RightSkill: memory.SkillTeleport,
		SkillsKnown: map[uint16]bool{
			memory.SkillTeleport:   true,
			memory.SkillTownPortal: true,
		},
	}
	return snap
}

func testBindings() configBindingSource {
	return configBindingSource{
		skills: map[uint16]input.SkillCast{
			memory.SkillTeleport: {
				SkillID:    memory.SkillTeleport,
				SelectKey:  "f7",
				CastButton: input.MouseRight,
			},
			memory.SkillTownPortal: {
				SkillID:    memory.SkillTownPortal,
				SelectKey:  "f6",
				CastButton: input.MouseRight,
			},
		},
		belt: [4]string{"1", "2", "3", "4"},
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
	mu    sync.RWMutex
	snap  memory.Snapshot
	calls int
}

func (m *mockProbe) Snapshot() memory.Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.snap
}

func (m *mockProbe) setSnapshot(snapshot memory.Snapshot) {
	m.mu.Lock()
	m.snap = snapshot
	m.mu.Unlock()
}

type mockInput struct {
	bindErr     error
	bindCalls   int
	unbindCalls int
	bound       bool
	lastPID     uint32

	enabled          bool
	paused           bool
	stopped          bool
	toggleCalls      int
	stopCalls        int
	listenCalls      int
	lastToggleReason string
	lastStopReason   string
	castBeltCalls    []int
	castSkillCalls   []uint16
	lastClientX      int
	lastClientY      int
	clickCalls       int
	focusCalls       int
	focusErr         error
}

func (m *mockInput) Bind(pid uint32) error {
	m.bindCalls++
	m.lastPID = pid
	if m.bindErr != nil {
		return m.bindErr
	}
	m.bound = true
	return nil
}

func (m *mockInput) Unbind() {
	m.unbindCalls++
	m.bound = false
}

func (m *mockInput) Bound() bool { return m.bound }

func (m *mockInput) Ready() bool { return true }

func (m *mockInput) Status() input.Status {
	return input.Status{Enabled: m.enabled, Paused: m.paused, Stopped: m.stopped}
}

func (m *mockInput) CastBelt(_ input.BeltBindingSource, slot int) error {
	m.castBeltCalls = append(m.castBeltCalls, slot)
	return nil
}

func (m *mockInput) CastBeltWithModifier(_ input.BeltBindingSource, _ string, slot int) error {
	m.castBeltCalls = append(m.castBeltCalls, slot)
	return nil
}

func (m *mockInput) SelectSkill(_ input.BindingSource, skillID uint16) error {
	m.castSkillCalls = append(m.castSkillCalls, skillID)
	return nil
}

func (m *mockInput) CastSkillAt(_ input.BindingSource, skillID uint16, clientX, clientY int) error {
	m.castSkillCalls = append(m.castSkillCalls, skillID)
	m.lastClientX = clientX
	m.lastClientY = clientY
	return nil
}

func (m *mockInput) MoveTo(clientX, clientY int) error {
	m.lastClientX = clientX
	m.lastClientY = clientY
	return nil
}

func (m *mockInput) Click(input.MouseButton) error { m.clickCalls++; return nil }

func (m *mockInput) ClickWithModifier(string, input.MouseButton) error { return nil }

func (m *mockInput) ClickAtWithModifier(clientX, clientY int, _ string, _ input.MouseButton) error {
	m.lastClientX = clientX
	m.lastClientY = clientY
	m.clickCalls++
	return nil
}

func (m *mockInput) PressKey(string) error { return nil }

func (m *mockInput) Focus() error {
	m.focusCalls++
	return m.focusErr
}

func (m *mockInput) Window() (input.WindowInfo, bool) {
	return input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}, true
}

func (m *mockInput) TogglePause(reason string) bool {
	m.toggleCalls++
	m.lastToggleReason = reason
	m.paused = !m.paused
	return m.paused
}

func (m *mockInput) Stop(reason string) {
	m.stopCalls++
	m.lastStopReason = reason
	m.stopped = true
	m.paused = false
}

func (m *mockInput) ListenHotkeys(_ context.Context, _ chan<- input.HotkeyEvent, ready chan<- error) {
	m.listenCalls++
	ready <- nil
}

type trackingInput struct {
	mockInput
	onUnbind func()
}

func (m *trackingInput) Unbind() {
	if m.onUnbind != nil {
		m.onUnbind()
	}
	m.mockInput.Unbind()
}

type orderProcess struct {
	order *[]string
	label string
}

func (p *orderProcess) Attach(_ context.Context) error { return nil }
func (p *orderProcess) Poll() process.Status           { return process.Status{} }
func (p *orderProcess) Status() process.Status         { return process.Status{} }
func (p *orderProcess) Detach() error {
	*p.order = append(*p.order, p.label)
	return nil
}
func (p *orderProcess) Ready() bool { return true }

type orderInput struct {
	order *[]string
	label string
}

func (i *orderInput) Bind(_ uint32) error { return nil }
func (i *orderInput) Unbind()             { *i.order = append(*i.order, i.label) }
func (i *orderInput) Bound() bool         { return false }
func (i *orderInput) Ready() bool         { return true }
func (i *orderInput) Status() input.Status {
	return input.Status{}
}
func (i *orderInput) CastBelt(input.BeltBindingSource, int) error { return nil }
func (i *orderInput) CastBeltWithModifier(input.BeltBindingSource, string, int) error {
	return nil
}
func (i *orderInput) CastSkillAt(input.BindingSource, uint16, int, int) error {
	return nil
}
func (i *orderInput) Window() (input.WindowInfo, bool) { return input.WindowInfo{}, false }
func (i *orderInput) TogglePause(string) bool          { return false }
func (i *orderInput) Stop(string)                      {}
func (i *orderInput) ListenHotkeys(context.Context, chan<- input.HotkeyEvent, chan<- error) {
}

func testRuntime(proc processController, probe snapshotReader, opts Options) *Runtime {
	in := &mockInput{}
	return testRuntimeWithInput(proc, probe, in, opts)
}

func testRuntimeWithInput(proc processController, probe snapshotReader, in inputController, opts Options) *Runtime {
	if mocked, ok := proc.(*mockProcess); ok {
		if mocked.status.State == process.StateAttached && mocked.status.FileVersion == "" {
			mocked.status.FileVersion = "3.2.92777"
		}
		if mocked.pollStatus.State == process.StateAttached && mocked.pollStatus.FileVersion == "" {
			mocked.pollStatus.FileVersion = "3.2.92777"
		}
	}
	nav := pathing.NewNavigator(config.NewLogger("error"), pathing.Deps{Config: pathing.DefaultConfig()})
	cfg := &config.Config{
		Process: config.ProcessConfig{ProcessName: "D2R.exe"},
		Input:   config.InputConfig{Enabled: false},
	}
	return &Runtime{
		Config:   cfg,
		Options:  opts,
		Log:      config.NewLogger("error"),
		Process:  proc,
		Probe:    probe,
		World:    world.NewModel(config.NewLogger("error")),
		Input:    in,
		Bindings: testBindings(),
		Pathing:  nav,
		Tasks: tasks.NewRunner(config.NewLogger("error"), tasks.RunSelection{}, tasks.RunConfig{}, tasks.Deps{
			Input:   in,
			Pathing: nav,
		}),
		compatibility: d2rCompatibilityContract{supportedVersion: "3.2.92777", expectedVersion: "3.2.92777", offsetVersion: "3.2.92777"},
	}
}

func testRuntimeWithTasks(proc processController, probe snapshotReader, in inputController, opts Options, runName string) *Runtime {
	rt := testRuntimeWithInput(proc, probe, in, opts)
	rt.Config.Input.Enabled = true
	rt.Tasks = tasks.NewRunner(config.NewLogger("error"), tasks.RunSelection{Run: runName, Phase: opts.RunPhase}, tasks.RunConfig{
		StepTimeout: 30 * time.Second,
	}, tasks.Deps{
		Input:   in,
		Pathing: rt.Pathing,
	})
	return rt
}

func TestConfiguredTaskResultEndsSuccessfulExplicitRun(t *testing.T) {
	rt := testRuntime(&mockProcess{}, &mockProbe{}, Options{})
	rt.Tasks = tasks.NewRunner(
		config.NewLogger("error"),
		tasks.RunSelection{Run: "countess", Phase: tasks.RunPhasePlayRoute},
		tasks.RunConfig{StepTimeout: time.Second},
		tasks.Deps{},
	)
	w := world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.TowerCellarLevel5),
	}
	result := rt.Tasks.Tick(context.Background(), w, time.Now())
	if result.Outcome != tasks.RunOutcomeSuccess || !rt.Tasks.Terminal() {
		t.Fatalf("terminal result = %+v, want success", result)
	}

	done, err := rt.configuredTaskResult()
	if !done || err != nil {
		t.Fatalf("configuredTaskResult() = (%t, %v), want (true, nil)", done, err)
	}
}

func TestConfiguredTaskResultKeepsPassiveRuntimeActive(t *testing.T) {
	rt := testRuntime(&mockProcess{}, &mockProbe{}, Options{})
	done, err := rt.configuredTaskResult()
	if done || err != nil {
		t.Fatalf("configuredTaskResult() = (%t, %v), want (false, nil)", done, err)
	}
}

func TestConfiguredTaskResultReturnsExplicitRunFailure(t *testing.T) {
	rt := testRuntime(&mockProcess{}, &mockProbe{}, Options{})
	rt.Tasks = tasks.NewRunner(
		config.NewLogger("error"),
		tasks.RunSelection{Run: "countess", Phase: tasks.RunPhasePlayRoute},
		tasks.RunConfig{StepTimeout: time.Second},
		tasks.Deps{},
	)
	w := world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.TowerCellarLevel4),
	}
	result := rt.Tasks.Tick(context.Background(), w, time.Now())
	if result.Outcome != tasks.RunOutcomeFailed || !rt.Tasks.Terminal() {
		t.Fatalf("terminal result = %+v, want failure", result)
	}

	done, err := rt.configuredTaskResult()
	if !done || err == nil || !strings.Contains(err.Error(), "not_act1_town") {
		t.Fatalf("configuredTaskResult() = (%t, %v), want terminal error", done, err)
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
	if cur.Area != (world.Area{}) || cur.Player.HP != 0 || cur.Player.LeftSkillID != 0 || len(cur.Player.SkillsKnown) != 0 {
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

func TestRunTickAttachCallsInputBind(t *testing.T) {
	in := &mockInput{}
	proc := &mockProcess{
		status: process.Status{State: process.StateAttached, PID: 4242, ModuleBase: 0x1000},
	}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{})
	state := &runState{attached: false}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if in.bindCalls != 1 {
		t.Fatalf("Bind calls = %d, want 1 on attach tick", in.bindCalls)
	}
	if in.lastPID != 4242 {
		t.Fatalf("Bind pid = %d, want 4242", in.lastPID)
	}
}

func TestRunTickAttachFailureSkipsInputBind(t *testing.T) {
	in := &mockInput{}
	proc := &mockProcess{attachErr: process.ErrNotFound}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{})

	if err := rt.runTick(context.Background(), &runState{}); err != nil {
		t.Fatal(err)
	}
	if in.bindCalls != 0 {
		t.Fatalf("Bind calls = %d, want 0 when attach fails", in.bindCalls)
	}
}

func TestRunTickRetryableBindErrorContinues(t *testing.T) {
	in := &mockInput{bindErr: input.ErrWindowNotFound}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 99},
		status:     process.Status{State: process.StateAttached, PID: 99},
	}
	probe := &mockProbe{snap: validSnapshot(100)}
	rt := testRuntimeWithInput(proc, probe, in, Options{Probe: false})
	state := &runState{attached: true, input: inputLoopState{lastBindAttempt: time.Now().Add(-2 * time.Second)}}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatalf("retryable bind error should not stop runTick: %v", err)
	}
	if probe.calls != 1 {
		t.Fatalf("Snapshot() calls = %d, want 1 after retryable bind failure", probe.calls)
	}
}

func TestRunTickNonRetryableBindErrorStops(t *testing.T) {
	in := &mockInput{bindErr: input.ErrInvalidPID}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 99},
		status:     process.Status{State: process.StateAttached, PID: 99},
	}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{})
	state := &runState{attached: true, input: inputLoopState{lastBindAttempt: time.Now().Add(-2 * time.Second)}}

	err := rt.runTick(context.Background(), state)
	if err == nil {
		t.Fatal("expected non-retryable bind error")
	}
	if !errors.Is(err, input.ErrInvalidPID) {
		t.Fatalf("error = %v, want ErrInvalidPID", err)
	}
}

func TestRunTickBindRetryThrottled(t *testing.T) {
	in := &mockInput{bindErr: input.ErrWindowNotFound}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 99},
		status:     process.Status{State: process.StateAttached, PID: 99},
	}
	probe := &mockProbe{snap: validSnapshot(100)}
	rt := testRuntimeWithInput(proc, probe, in, Options{})
	state := &runState{
		attached: true,
		input:    inputLoopState{lastBindAttempt: time.Now()},
	}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if in.bindCalls != 0 {
		t.Fatalf("Bind calls = %d, want 0 when throttled", in.bindCalls)
	}
	if probe.calls != 1 {
		t.Fatalf("Snapshot() calls = %d, want 1 while bind is throttled", probe.calls)
	}
}

func TestRunTickLostUnbindsInput(t *testing.T) {
	in := &mockInput{bound: true}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{Probe: false})
	rt.World.Update(validSnapshot(100))
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if in.unbindCalls != 1 {
		t.Fatalf("Unbind calls = %d, want 1 on process lost", in.unbindCalls)
	}
}

func TestRunTickLostUnbindsBeforeWorldReset(t *testing.T) {
	var worldValidAtUnbind bool
	in := &trackingInput{mockInput: mockInput{bound: true}}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{Probe: false})
	rt.World.Update(validSnapshot(100))
	in.onUnbind = func() {
		worldValidAtUnbind = rt.World.Current().Valid
	}
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if !worldValidAtUnbind {
		t.Fatal("Unbind should run while world state is still valid (before Reset)")
	}
	if rt.World.Current().Valid {
		t.Fatal("world should be invalid after process lost")
	}
}

func TestRunTickAttachRetryableBindErrorSkipsSnapshot(t *testing.T) {
	in := &mockInput{bindErr: input.ErrWindowNotFound}
	proc := &mockProcess{
		status: process.Status{State: process.StateAttached, PID: 4242, ModuleBase: 0x1000},
	}
	probe := &mockProbe{snap: validSnapshot(100)}
	rt := testRuntimeWithInput(proc, probe, in, Options{Probe: true})
	state := &runState{attached: false}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if !state.attached {
		t.Fatal("expected attached despite retryable bind failure")
	}
	if in.bindCalls != 1 {
		t.Fatalf("Bind calls = %d, want 1 on attach tick", in.bindCalls)
	}
	if probe.calls != 0 {
		t.Fatalf("Snapshot() calls = %d, want 0 on attach tick", probe.calls)
	}
}

func TestRunTickReattachAfterLostCallsBind(t *testing.T) {
	in := &mockInput{}
	lostProc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 42},
	}
	rt := testRuntimeWithInput(lostProc, &mockProbe{}, in, Options{})
	state := &runState{attached: true}
	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	attachProc := &mockProcess{
		status: process.Status{State: process.StateAttached, PID: 99, ModuleBase: 0x1000, FileVersion: "3.2.92777"},
	}
	rt.Process = attachProc
	state.attached = false
	in.bindCalls = 0

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if in.bindCalls != 1 {
		t.Fatalf("Bind calls after re-attach = %d, want 1", in.bindCalls)
	}
	if in.lastPID != 99 {
		t.Fatalf("Bind pid = %d, want 99", in.lastPID)
	}
}

func TestShutdownCleanupUnbindsBeforeDetach(t *testing.T) {
	var order []string
	proc := &orderProcess{order: &order, label: "detach"}
	in := &orderInput{order: &order, label: "unbind"}

	func() {
		defer func() {
			if err := proc.Detach(); err != nil {
				t.Fatal(err)
			}
		}()
		defer in.Unbind()
	}()

	if len(order) != 2 || order[0] != "unbind" || order[1] != "detach" {
		t.Fatalf("cleanup order = %v, want [unbind detach]", order)
	}
}
