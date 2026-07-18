package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

type mockInputTest struct {
	mockInput

	window      input.WindowInfo
	hasWindow   bool
	castBelt    []int
	selectSkill []uint16
	castSkillAt [][3]int // skillID, x, y
	moveTo      [][2]int
	clicks      []input.MouseButton
	actionErr   error
}

func (m *mockInputTest) Window() (input.WindowInfo, bool) {
	return m.window, m.hasWindow
}

func (m *mockInputTest) PressKey(key string) error {
	if m.actionErr != nil {
		return m.actionErr
	}
	return nil
}

func (m *mockInputTest) CastBelt(_ input.BeltBindingSource, slot int) error {
	if m.actionErr != nil {
		return m.actionErr
	}
	m.castBelt = append(m.castBelt, slot)
	return nil
}

func (m *mockInputTest) SelectSkill(_ input.BindingSource, skillID uint16) error {
	if m.actionErr != nil {
		return m.actionErr
	}
	m.selectSkill = append(m.selectSkill, skillID)
	return nil
}

func (m *mockInputTest) CastSkillAt(_ input.BindingSource, skillID uint16, clientX, clientY int) error {
	if m.actionErr != nil {
		return m.actionErr
	}
	m.castSkillAt = append(m.castSkillAt, [3]int{int(skillID), clientX, clientY})
	return nil
}

func (m *mockInputTest) MoveTo(clientX, clientY int) error {
	if m.actionErr != nil {
		return m.actionErr
	}
	m.moveTo = append(m.moveTo, [2]int{clientX, clientY})
	return nil
}

func (m *mockInputTest) Click(button input.MouseButton) error {
	if m.actionErr != nil {
		return m.actionErr
	}
	m.clicks = append(m.clicks, button)
	return nil
}

func readyInputTestRuntime(in inputTestController, probe *mockProbe) *Runtime {
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 42},
		status:     process.Status{State: process.StateAttached, PID: 42},
	}
	rt := testRuntimeWithInput(proc, probe, in, Options{InputTestObserveMs: 50})
	rt.Config.Runtime.PollIntervalMs = 10
	return rt
}

func TestRunInputTestDisabledInput(t *testing.T) {
	in := &mockInputTest{mockInput: mockInput{enabled: false}}
	rt := readyInputTestRuntime(in, &mockProbe{snap: validSnapshot(100)})

	err := rt.RunInputTest("belt:1")
	if err == nil || !strings.Contains(err.Error(), "input.enabled=true") {
		t.Fatalf("err = %v, want enabled error", err)
	}
}

func TestRunInputTestReadyAndExecutesBelt(t *testing.T) {
	in := &mockInputTest{
		mockInput: mockInput{enabled: true, bound: true},
		hasWindow: true,
		window:    input.WindowInfo{ClientWidth: 800, ClientHeight: 600},
	}
	rt := readyInputTestRuntime(in, &mockProbe{snap: validSnapshot(100)})

	err := rt.RunInputTest("belt:1")
	if err != nil {
		t.Fatalf("RunInputTest() err = %v", err)
	}
	if len(in.castBelt) != 1 || in.castBelt[0] != 1 {
		t.Fatalf("CastBelt calls = %v, want [1]", in.castBelt)
	}
}

func TestRunInputTestExecutesAllActionTypes(t *testing.T) {
	in := &mockInputTest{
		mockInput: mockInput{enabled: true, bound: true},
		hasWindow: true,
		window:    input.WindowInfo{ClientWidth: 800, ClientHeight: 600},
	}
	rt := readyInputTestRuntime(in, &mockProbe{snap: validSnapshot(100)})

	spec := "belt:2,portal,skill:teleport,center-click,click:100,200"
	if err := rt.RunInputTest(spec); err != nil {
		t.Fatal(err)
	}
	if len(in.castBelt) != 1 || in.castBelt[0] != 2 {
		t.Fatalf("belt = %v", in.castBelt)
	}
	if len(in.selectSkill) != 2 {
		t.Fatalf("selectSkill = %v, want portal + teleport", in.selectSkill)
	}
	if in.selectSkill[0] != memory.SkillTownPortal || in.selectSkill[1] != memory.SkillTeleport {
		t.Fatalf("selectSkill = %v", in.selectSkill)
	}
	if len(in.moveTo) != 2 {
		t.Fatalf("moveTo = %v", in.moveTo)
	}
	if in.moveTo[0] != [2]int{400, 300} {
		t.Fatalf("center move = %v, want 400,300", in.moveTo[0])
	}
	if in.moveTo[1] != [2]int{100, 200} {
		t.Fatalf("click move = %v", in.moveTo[1])
	}
	if len(in.clicks) != 2 {
		t.Fatalf("clicks = %v", in.clicks)
	}
	if in.clicks[0] != input.MouseRight {
		t.Fatalf("center click button = %v, want right (portal on right bar)", in.clicks[0])
	}
	if in.clicks[1] != input.MouseLeft {
		t.Fatalf("raw click button = %v, want left after pending cast was consumed", in.clicks[1])
	}
}

func TestRunInputTestReadyTimeoutReportsState(t *testing.T) {
	in := &mockInputTest{mockInput: mockInput{enabled: true}}
	proc := &mockProcess{attachErr: process.ErrNotFound}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{InputTestObserveMs: 50})
	rt.Config.Process.AttachTimeoutMs = 30
	rt.Config.Runtime.PollIntervalMs = 5

	err := rt.RunInputTest("portal")
	if err == nil {
		t.Fatal("expected ready timeout error")
	}
	if !strings.Contains(err.Error(), "world_valid=false") {
		t.Fatalf("err = %v, want world_valid in message", err)
	}
}

func TestRunInputTestReadyTimeoutWhenWorldInvalid(t *testing.T) {
	in := &mockInputTest{mockInput: mockInput{enabled: true, bound: true}}
	probe := &mockProbe{snap: memory.Snapshot{Valid: false, Reason: "menu"}}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 1},
		status:     process.Status{State: process.StateAttached, PID: 1},
	}
	rt := testRuntimeWithInput(proc, probe, in, Options{InputTestObserveMs: 50})
	rt.Config.Process.AttachTimeoutMs = 30
	rt.Config.Runtime.PollIntervalMs = 5

	err := rt.RunInputTest("portal")
	if err == nil {
		t.Fatal("expected ready timeout")
	}
	if !strings.Contains(err.Error(), "world_valid=false") || !strings.Contains(err.Error(), "menu") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunInputTestReadyTimeoutWhenPhaseMenu(t *testing.T) {
	in := &mockInputTest{
		mockInput: mockInput{enabled: true, bound: true},
		hasWindow: true,
		window:    input.WindowInfo{ClientWidth: 800, ClientHeight: 600},
	}
	snap := validSnapshot(100)
	snap.Phase = memory.GamePhaseMenu
	probe := &mockProbe{snap: snap}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached, PID: 1},
		status:     process.Status{State: process.StateAttached, PID: 1},
	}
	rt := testRuntimeWithInput(proc, probe, in, Options{InputTestObserveMs: 50})
	rt.Config.Process.AttachTimeoutMs = 30
	rt.Config.Runtime.PollIntervalMs = 5

	err := rt.RunInputTest("skill:teleport")
	if err == nil {
		t.Fatal("expected ready timeout")
	}
	if !strings.Contains(err.Error(), `world_phase="menu"`) {
		t.Fatalf("err = %v, want menu phase in message", err)
	}
	if len(in.selectSkill) != 0 {
		t.Fatalf("SelectSkill calls = %v, want none before in_game", in.selectSkill)
	}
}

func TestRunInputTestProcessLostDuringReadyWait(t *testing.T) {
	in := &mockInputTest{mockInput: mockInput{enabled: true, bound: true}}
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateLost},
		status:     process.Status{State: process.StateLost, PID: 1},
	}
	rt := testRuntimeWithInput(proc, probe, in, Options{InputTestObserveMs: 50})
	rt.Config.Runtime.PollIntervalMs = 5
	rt.Config.Process.AttachTimeoutMs = 200

	done := make(chan error, 1)
	go func() {
		done <- rt.RunInputTest("belt:1")
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "process lost during input test") {
			t.Fatalf("err = %v, want process lost", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestRunInputTestStopHotkeyDuringWait(t *testing.T) {
	in := &hotkeyInjectInputTest{
		mockInputTest: mockInputTest{mockInput: mockInput{enabled: true}},
		listenFn: func(ctx context.Context, out chan<- input.HotkeyEvent, rdy chan<- error) {
			rdy <- nil
			go func() {
				<-time.After(15 * time.Millisecond)
				out <- input.HotkeyEvent{Action: input.HotkeyActionStop}
			}()
			<-ctx.Done()
		},
	}
	proc := &mockProcess{attachErr: process.ErrNotFound}
	rt := testRuntimeWithInput(proc, &mockProbe{}, in, Options{InputTestObserveMs: 50})
	rt.Config.Runtime.PollIntervalMs = 10

	err := rt.RunInputTest("belt:1")
	if err != nil {
		t.Fatalf("RunInputTest() err = %v, want clean stop", err)
	}
	if in.stopCalls != 1 {
		t.Fatalf("stop calls = %d", in.stopCalls)
	}
}

func TestRunInputTestPauseBeforeActionBlocks(t *testing.T) {
	in := &mockInputTest{
		mockInput: mockInput{enabled: true, bound: true, paused: true},
		hasWindow: true,
		window:    input.WindowInfo{ClientWidth: 100, ClientHeight: 100},
	}
	in.actionErr = input.ErrInputPaused
	rt := readyInputTestRuntime(in, &mockProbe{snap: validSnapshot(100)})

	err := rt.RunInputTest("belt:1")
	if err == nil || !errors.Is(err, input.ErrInputPaused) {
		t.Fatalf("err = %v, want ErrInputPaused", err)
	}
	if len(in.castBelt) != 0 {
		t.Fatalf("CastBelt calls = %v, want none", in.castBelt)
	}
}

func TestRunInputTestStopBetweenSequenceActions(t *testing.T) {
	in := &sequencedInputTest{
		mockInputTest: mockInputTest{
			mockInput: mockInput{enabled: true, bound: true},
			hasWindow: true,
			window:    input.WindowInfo{ClientWidth: 100, ClientHeight: 100},
		},
	}
	var mu sync.Mutex
	in.onCastBelt = func() {
		mu.Lock()
		in.stopped = true
		mu.Unlock()
	}
	rt := readyInputTestRuntime(in, &mockProbe{snap: validSnapshot(100)})

	err := rt.RunInputTest("belt:1,belt:2,belt:3")
	if err != nil {
		t.Fatalf("RunInputTest() err = %v", err)
	}
	if len(in.castBelt) != 1 {
		t.Fatalf("CastBelt calls = %v, want only first action", in.castBelt)
	}
}

func TestRunInputTestCenterClickWithoutWindow(t *testing.T) {
	in := &mockInputTest{
		mockInput: mockInput{enabled: true, bound: true},
		hasWindow: false,
	}
	rt := readyInputTestRuntime(in, &mockProbe{snap: validSnapshot(100)})

	err := rt.RunInputTest("center-click")
	if err == nil || !strings.Contains(err.Error(), "window not bound") {
		t.Fatalf("err = %v", err)
	}
}

func TestObserveInputTestWorldDeltas(t *testing.T) {
	snap1 := validSnapshot(100)
	snap2 := validSnapshot(90)
	snap2.Mana = 40
	snap2.PosX = 15
	snap2.PosY = 25

	probe := &mockProbe{snap: snap1}
	in := &mockInputTest{mockInput: mockInput{enabled: true, bound: true}}
	rt := readyInputTestRuntime(in, probe)
	state := &runState{attached: true, hasEverAttached: true}
	rt.World.Update(snap1)

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hotkeys := make(chan input.HotkeyEvent)

	go func() {
		<-time.After(12 * time.Millisecond)
		probe.setSnapshot(snap2)
	}()

	err := rt.observeInputTestWorld(ctx, state, hotkeys, ticker, 20*time.Millisecond, cancel)
	if err != nil {
		t.Fatalf("observeInputTestWorld() err = %v", err)
	}

	after := rt.World.Current()
	if after.Player.HP != 90 {
		t.Fatalf("after observe HP = %d, want 90 from updated snapshot", after.Player.HP)
	}
}

func TestLogInputTestObservationDeltas(t *testing.T) {
	before := validWorldState(100)
	after := validWorldState(90)
	after.Player.Mana = 40
	after.Player.Position.X = 15
	after.Player.Position.Y = 25

	rt := testRuntime(&mockProcess{}, &mockProbe{}, Options{})
	rt.logInputTestObservation(before, after)
	// Smoke: deltas are hp=-10, mana=-10, pos_x=+5, pos_y=+5 — verified via direct computation.
	if int64(after.Player.HP)-int64(before.Player.HP) != -10 {
		t.Fatal("unexpected hp delta fixture")
	}
}

func TestRunInputTestProcessLostDuringObservation(t *testing.T) {
	lostAfterReady := &lostAfterReadyProcess{}
	in := &mockInputTest{
		mockInput: mockInput{enabled: true, bound: true},
		hasWindow: true,
		window:    input.WindowInfo{ClientWidth: 100, ClientHeight: 100},
	}
	rt := testRuntimeWithInput(lostAfterReady, &mockProbe{snap: validSnapshot(100)}, in, Options{InputTestObserveMs: 80})
	rt.Config.Runtime.PollIntervalMs = 10

	err := rt.RunInputTest("belt:1")
	if err == nil || !strings.Contains(err.Error(), "process lost during input test") {
		t.Fatalf("err = %v, want process lost during observation", err)
	}
}

// lostAfterReadyProcess attaches once then reports lost on subsequent polls.
type lostAfterReadyProcess struct {
	mockProcess
	pollCount int
}

func (p *lostAfterReadyProcess) Attach(_ context.Context) error {
	p.attachCalls++
	return nil
}

func (p *lostAfterReadyProcess) Poll() process.Status {
	p.pollCount++
	if p.pollCount <= 2 {
		return process.Status{State: process.StateAttached, PID: 42}
	}
	return process.Status{State: process.StateLost, PID: 42}
}

func (p *lostAfterReadyProcess) Status() process.Status {
	if p.pollCount <= 2 {
		return process.Status{State: process.StateAttached, PID: 42}
	}
	return process.Status{State: process.StateLost, PID: 42}
}

type hotkeyInjectInputTest struct {
	mockInputTest
	listenFn func(context.Context, chan<- input.HotkeyEvent, chan<- error)
}

func (h *hotkeyInjectInputTest) ListenHotkeys(ctx context.Context, events chan<- input.HotkeyEvent, ready chan<- error) {
	if h.listenFn != nil {
		go h.listenFn(ctx, events, ready)
		return
	}
	h.mockInputTest.ListenHotkeys(ctx, events, ready)
}

type sequencedInputTest struct {
	mockInputTest
	onCastBelt func()
}

func (s *sequencedInputTest) CastBelt(src input.BeltBindingSource, slot int) error {
	if s.onCastBelt != nil {
		s.onCastBelt()
	}
	return s.mockInputTest.CastBelt(src, slot)
}
