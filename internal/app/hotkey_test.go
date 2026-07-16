package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func TestHandleHotkeyEventPause(t *testing.T) {
	in := &mockInput{}
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, in, Options{})

	rt.handleHotkeyEvent(input.HotkeyEvent{Action: input.HotkeyActionPause}, func() {})

	if in.toggleCalls != 1 || in.lastToggleReason != "hotkey" {
		t.Fatalf("toggle calls = %d reason = %q", in.toggleCalls, in.lastToggleReason)
	}
	if !in.paused {
		t.Fatal("expected paused after pause hotkey")
	}
}

func TestHandleHotkeyEventRoutesPauseToQueueIntentWithoutSuspendingInput(t *testing.T) {
	in := &mockInput{}
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, in, Options{})
	calls := 0
	rt.setPauseHotkeyHandler(func() error {
		calls++
		return nil
	})

	rt.handleHotkeyEvent(input.HotkeyEvent{Action: input.HotkeyActionPause}, func() {})

	if calls != 1 {
		t.Fatalf("pause-after-run calls = %d, want 1", calls)
	}
	if in.toggleCalls != 0 || in.paused {
		t.Fatalf("mid-run input pause was changed: calls=%d paused=%t", in.toggleCalls, in.paused)
	}
}

func TestHandleHotkeyEventStop(t *testing.T) {
	in := &mockInput{}
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, in, Options{})

	cancelled := false
	rt.handleHotkeyEvent(input.HotkeyEvent{Action: input.HotkeyActionStop}, func() { cancelled = true })

	if in.stopCalls != 1 || in.lastStopReason != "hotkey" {
		t.Fatalf("stop calls = %d reason = %q", in.stopCalls, in.lastStopReason)
	}
	if !cancelled {
		t.Fatal("expected cancel after stop hotkey")
	}
}

func TestRunHotkeyReadyError(t *testing.T) {
	in := &mockInput{}
	in.listenCalls = 0
	listen := func(_ context.Context, _ chan<- input.HotkeyEvent, ready chan<- error) {
		ready <- input.ErrHotkeyUnavailable
	}
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, &hotkeyInjectInput{mockInput: *in, listenFn: listen}, Options{})
	rt.Config.Runtime.PollIntervalMs = 100000

	err := rt.Run()
	if err == nil || !errors.Is(err, input.ErrHotkeyUnavailable) {
		t.Fatalf("Run() err = %v, want hotkey unavailable", err)
	}
}

func TestRunProcessesStopHotkeyEvent(t *testing.T) {
	in := &hotkeyInjectInput{
		mockInput: mockInput{},
		listenFn: func(ctx context.Context, out chan<- input.HotkeyEvent, rdy chan<- error) {
			rdy <- nil
			go func() {
				<-time.After(10 * time.Millisecond)
				out <- input.HotkeyEvent{Action: input.HotkeyActionStop}
			}()
			<-ctx.Done()
		},
	}
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, in, Options{})
	rt.Config.Runtime.PollIntervalMs = 50

	done := make(chan error, 1)
	go func() { done <- rt.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to stop via hotkey")
	}
	if in.stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", in.stopCalls)
	}
}

func TestRunProcessesPauseHotkeyEvent(t *testing.T) {
	in := &hotkeyInjectInput{
		mockInput: mockInput{},
		listenFn: func(ctx context.Context, out chan<- input.HotkeyEvent, rdy chan<- error) {
			rdy <- nil
			go func() {
				<-time.After(10 * time.Millisecond)
				out <- input.HotkeyEvent{Action: input.HotkeyActionPause}
				<-time.After(50 * time.Millisecond)
				out <- input.HotkeyEvent{Action: input.HotkeyActionStop}
			}()
			<-ctx.Done()
		},
	}
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, in, Options{})
	rt.Config.Runtime.PollIntervalMs = 50

	done := make(chan error, 1)
	go func() { done <- rt.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to stop after pause then stop hotkey")
	}
	if in.toggleCalls != 1 {
		t.Fatalf("toggle calls = %d, want 1", in.toggleCalls)
	}
	if in.stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", in.stopCalls)
	}
}

func TestRunTickContinuesWhenInputDisabled(t *testing.T) {
	probe := &mockProbe{snap: validSnapshot(100)}
	proc := &mockProcess{
		pollStatus: process.Status{State: process.StateAttached},
		status:     process.Status{State: process.StateAttached, PID: 1},
	}
	in := &mockInput{} // Status().Enabled defaults to false
	rt := testRuntimeWithInput(proc, probe, in, Options{Probe: false})
	state := &runState{attached: true}

	if err := rt.runTick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 {
		t.Fatalf("Snapshot() calls = %d, want 1 with input disabled", probe.calls)
	}
	if !rt.World.Current().Valid {
		t.Fatal("expected world update to continue when input is disabled")
	}
}

type hotkeyInjectInput struct {
	mockInput
	listenFn func(context.Context, chan<- input.HotkeyEvent, chan<- error)
}

func (h *hotkeyInjectInput) ListenHotkeys(ctx context.Context, events chan<- input.HotkeyEvent, ready chan<- error) {
	if h.listenFn != nil {
		go h.listenFn(ctx, events, ready)
		return
	}
	h.mockInput.ListenHotkeys(ctx, events, ready)
}
