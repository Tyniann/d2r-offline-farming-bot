package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const defaultInputTestObserveMs = 3000

// inputTestController extends inputController with primitives used by the manual input-test mode.
type inputTestController interface {
	inputController
	Window() (input.WindowInfo, bool)
	PressKey(key string) error
	CastBelt(src input.BeltBindingSource, slot int) error
	SelectSkill(src input.BindingSource, skillID uint16) error
	CastSkillAt(src input.BindingSource, skillID uint16, clientX, clientY int) error
	MoveTo(clientX, clientY int) error
	Click(button input.MouseButton) error
}

type inputTestBindingSource interface {
	input.BindingSource
	input.BeltBindingSource
	TownPortalSkillID() (uint16, bool)
}

// RunInputTest waits for process attach, window binding, and a valid world state, executes
// the configured test actions, observes world state afterward, and exits cleanly.
// Pause and stop hotkeys remain active for the duration of the test.
func (rt *Runtime) RunInputTest(spec string) error {
	actions, err := parseInputTestSpec(spec)
	if err != nil {
		return err
	}

	if !rt.Input.Status().Enabled {
		return fmt.Errorf("input test requires input.enabled=true")
	}

	ctrl, ok := rt.Input.(inputTestController)
	if !ok {
		return fmt.Errorf("input test: controller does not implement test actions")
	}

	observeMs := rt.Options.InputTestObserveMs
	if observeMs <= 0 {
		observeMs = defaultInputTestObserveMs
	}

	rt.Log.Info("input test started",
		"spec", spec,
		"action_count", len(actions),
		"observe_ms", observeMs,
		"version", version.Version,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt.startShutdownSignals(ctx, cancel)

	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Error("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()

	hotkeyEvents, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	state := &runState{}
	readyDeadline := time.Now().Add(rt.attachTimeoutOrDefault(60 * time.Second))

	if err := rt.waitInputTestReady(ctx, state, hotkeyEvents, ticker, readyDeadline, cancel); err != nil {
		return err
	}
	if ctx.Err() != nil || rt.Input.Status().Stopped {
		rt.Log.Info("input test stopped", "reason", inputTestStopReason(ctx))
		return nil
	}

	rt.logInputTestReady(ctrl)

	var src inputTestBindingSource = rt.bindingSource()

	if err := rt.executeInputTestActions(ctx, ctrl, src, actions, hotkeyEvents, cancel); err != nil {
		return err
	}
	if ctx.Err() != nil || rt.Input.Status().Stopped {
		rt.Log.Info("input test stopped", "reason", inputTestStopReason(ctx))
		return nil
	}

	if err := rt.observeInputTestWorld(ctx, state, hotkeyEvents, ticker, time.Duration(observeMs)*time.Millisecond, cancel); err != nil {
		return err
	}
	if ctx.Err() != nil || rt.Input.Status().Stopped {
		rt.Log.Info("input test stopped", "reason", inputTestStopReason(ctx))
		return nil
	}

	rt.Log.Info("input test completed")
	return nil
}

func (rt *Runtime) attachTimeoutOrDefault(fallback time.Duration) time.Duration {
	if timeout := rt.Config.Process.AttachTimeoutMs; timeout > 0 {
		return time.Duration(timeout) * time.Millisecond
	}
	return fallback
}

func (rt *Runtime) bindingSource() configBindingSource {
	return rt.Bindings
}

func (rt *Runtime) waitInputTestReady(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
) error {
	for {
		if time.Now().After(deadline) {
			st := rt.World.Current()
			rt.Log.Error("input test ready timeout",
				"attached", state.attached,
				"bound", rt.Input.Bound(),
				"world_valid", st.Valid,
				"world_phase", st.Phase.String(),
				"world_reason", st.Reason,
			)
			return fmt.Errorf(
				"input test ready timeout: attached=%t bound=%t world_valid=%t world_phase=%q world_reason=%q",
				state.attached,
				rt.Input.Bound(),
				st.Valid,
				st.Phase.String(),
				st.Reason,
			)
		}

		select {
		case <-ctx.Done():
			rt.Log.Info("input test stopped", "reason", inputTestStopReason(ctx))
			return nil
		case ev := <-hotkeyEvents:
			rt.handleHotkeyEvent(ev, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			if state.hasEverAttached && !state.attached {
				return fmt.Errorf("process lost during input test")
			}
			cur := rt.World.Current()
			if state.attached && rt.Input.Bound() && cur.Valid && cur.Phase == world.GamePhaseInGame {
				state.bindingsPrecheckDone = true
				return nil
			}
		}
	}
}

func (rt *Runtime) logInputTestReady(ctrl inputTestController) {
	args := []any{
		"pid", rt.Process.Status().PID,
		"bound", rt.Input.Bound(),
	}
	if win, ok := ctrl.Window(); ok {
		args = append(args,
			"client_width", win.ClientWidth,
			"client_height", win.ClientHeight,
		)
	}
	args = append(args, attrsToArgs(worldLogAttrs(rt.World.Current()))...)
	rt.Log.Info("input test ready", args...)
	rt.logInputTestWorld("before_action", rt.World.Current())
}

func (rt *Runtime) executeInputTestActions(
	ctx context.Context,
	ctrl inputTestController,
	src inputTestBindingSource,
	actions []inputTestAction,
	hotkeyEvents <-chan input.HotkeyEvent,
	cancel context.CancelFunc,
) error {
	var pendingCastButton *input.MouseButton

	for _, action := range actions {
		rt.drainHotkeyEvents(hotkeyEvents, cancel)
		if ctx.Err() != nil || rt.Input.Status().Stopped {
			return nil
		}

		rt.logInputTestActionStart(action)
		if err := rt.executeInputTestAction(ctrl, src, action, &pendingCastButton); err != nil {
			return err
		}
		rt.Log.Info("input test action completed", "action", action.String())
	}
	return nil
}

func (rt *Runtime) executeInputTestAction(
	ctrl inputTestController,
	src inputTestBindingSource,
	action inputTestAction,
	pendingCastButton **input.MouseButton,
) error {
	switch action.kind {
	case inputTestBelt:
		return ctrl.CastBelt(src, action.slot)
	case inputTestPortal:
		id, ok := src.TownPortalSkillID()
		if !ok {
			return fmt.Errorf("input test portal: town portal not bound on skill bar")
		}
		cast, err := src.Resolve(id)
		if err != nil {
			return err
		}
		*pendingCastButton = &cast.CastButton
		return ctrl.SelectSkill(src, id)
	case inputTestSkill:
		cast, err := src.Resolve(action.skillID)
		if err != nil {
			return err
		}
		btn := cast.CastButton
		*pendingCastButton = &btn
		return ctrl.SelectSkill(src, action.skillID)
	case inputTestCenterClick:
		win, ok := ctrl.Window()
		if !ok {
			return fmt.Errorf("input test: window not bound")
		}
		x := win.ClientWidth / 2
		y := win.ClientHeight / 2
		if err := ctrl.MoveTo(x, y); err != nil {
			return err
		}
		btn := input.MouseLeft
		if pendingCastButton != nil && *pendingCastButton != nil {
			btn = **pendingCastButton
			*pendingCastButton = nil
		}
		return ctrl.Click(btn)
	case inputTestClick:
		if err := ctrl.MoveTo(action.x, action.y); err != nil {
			return err
		}
		btn := input.MouseLeft
		if pendingCastButton != nil && *pendingCastButton != nil {
			btn = **pendingCastButton
			*pendingCastButton = nil
		}
		return ctrl.Click(btn)
	default:
		return fmt.Errorf("input test: unknown action %q", action.kind)
	}
}

func (rt *Runtime) observeInputTestWorld(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	duration time.Duration,
	cancel context.CancelFunc,
) error {
	deadline := time.Now().Add(duration)
	var before world.State
	var beforeSet bool
	var after world.State

	for !time.Now().After(deadline) {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-hotkeyEvents:
			rt.handleHotkeyEvent(ev, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return fmt.Errorf("input test observation tick: %w", err)
			}
			if state.hasEverAttached && !state.attached {
				return fmt.Errorf("process lost during input test")
			}
			cur := rt.World.Current()
			if !beforeSet {
				before = cur
				beforeSet = true
			}
			after = cur
		}
	}

	if !beforeSet {
		return nil
	}

	rt.logInputTestObservation(before, after)
	return nil
}

func (rt *Runtime) logInputTestWorld(label string, st world.State) {
	args := []any{"label", label}
	args = append(args, attrsToArgs(worldLogAttrs(st))...)
	rt.Log.Info("input test world", args...)
}

func (rt *Runtime) logInputTestActionStart(action inputTestAction) {
	args := []any{"action", action.String()}
	switch action.kind {
	case inputTestBelt:
		args = append(args, "slot", action.slot)
	case inputTestSkill:
		args = append(args, "skill_id", action.skillID, "skill", memory.SkillName(action.skillID))
	case inputTestClick:
		args = append(args, "x", action.x, "y", action.y)
	}
	rt.Log.Info("input test action started", args...)
}

func (rt *Runtime) logInputTestObservation(before, after world.State) {
	args := []any{
		"before_valid", before.Valid,
		"after_valid", after.Valid,
		"hp_delta", int64(after.Player.HP) - int64(before.Player.HP),
		"mana_delta", int64(after.Player.Mana) - int64(before.Player.Mana),
		"pos_x_delta", int64(after.Player.Position.X) - int64(before.Player.Position.X),
		"pos_y_delta", int64(after.Player.Position.Y) - int64(before.Player.Position.Y),
	}
	args = append(args, prefixedWorldLogAttrs("before", before)...)
	args = append(args, prefixedWorldLogAttrs("after", after)...)
	rt.Log.Info("input test observation", args...)
}

func prefixedWorldLogAttrs(prefix string, st world.State) []any {
	attrs := worldLogAttrs(st)
	args := make([]any, 0, len(attrs)*2)
	for _, a := range attrs {
		args = append(args, prefix+"_"+a.Key, a.Value.Any())
	}
	return args
}

func (rt *Runtime) drainHotkeyEvents(hotkeyEvents <-chan input.HotkeyEvent, cancel context.CancelFunc) {
	for {
		select {
		case ev := <-hotkeyEvents:
			rt.handleHotkeyEvent(ev, cancel)
		default:
			return
		}
	}
}

func inputTestStopReason(ctx context.Context) string {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return "hotkey/signal/context"
		}
		return ctx.Err().Error()
	}
	return "stopped"
}
