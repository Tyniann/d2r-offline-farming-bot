package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const defaultPathingTestTimeoutMs = 120000

// pathingTestKind identifies the manual pathing test scenario.
type pathingTestKind string

const (
	pathingTestTeleport    pathingTestKind = "teleport"
	pathingTestHoverWatch  pathingTestKind = "hover_watch"
	pathingTestMoveArea    pathingTestKind = "move_area"
	pathingTestClickEntity pathingTestKind = "click_entity"
)

// pathingTestSpec is a parsed --pathing-test argument.
type pathingTestSpec struct {
	kind    pathingTestKind
	area    world.AreaID // move-area target.
	targetX uint32       // teleport world X.
	targetY uint32       // teleport world Y.
	entity  string       // click-entity target: waypoint | entrance.
}

// requiresInput reports whether the spec performs real input actions.
func (s pathingTestSpec) requiresInput() bool {
	return s.kind != pathingTestHoverWatch
}

// parsePathingTestSpec parses specs like `teleport:5000,5000`, `hover:watch`,
// `move-area:black_marsh`, `click-entity:waypoint`.
func parsePathingTestSpec(spec string) (pathingTestSpec, error) {
	raw := strings.TrimSpace(spec)
	kind, arg, ok := strings.Cut(raw, ":")
	if !ok || strings.TrimSpace(arg) == "" {
		return pathingTestSpec{}, fmt.Errorf("pathing test: invalid spec %q (expected kind:arg)", spec)
	}
	arg = strings.TrimSpace(arg)

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "teleport":
		xs, ys, ok := strings.Cut(arg, ",")
		if !ok {
			return pathingTestSpec{}, fmt.Errorf("pathing test teleport: expected TX,TY, got %q", arg)
		}
		x, err := strconv.ParseUint(strings.TrimSpace(xs), 10, 32)
		if err != nil {
			return pathingTestSpec{}, fmt.Errorf("pathing test teleport: invalid X %q: %w", xs, err)
		}
		y, err := strconv.ParseUint(strings.TrimSpace(ys), 10, 32)
		if err != nil {
			return pathingTestSpec{}, fmt.Errorf("pathing test teleport: invalid Y %q: %w", ys, err)
		}
		return pathingTestSpec{kind: pathingTestTeleport, targetX: uint32(x), targetY: uint32(y)}, nil
	case "hover":
		if strings.ToLower(arg) != "watch" {
			return pathingTestSpec{}, fmt.Errorf("pathing test hover: expected hover:watch, got %q", spec)
		}
		return pathingTestSpec{kind: pathingTestHoverWatch}, nil
	case "move-area":
		id, err := world.ParseAreaSpec(arg)
		if err != nil {
			return pathingTestSpec{}, fmt.Errorf("pathing test move-area: %w", err)
		}
		return pathingTestSpec{kind: pathingTestMoveArea, area: id}, nil
	case "click-entity":
		entity := strings.ToLower(arg)
		if entity != "waypoint" && entity != "entrance" {
			return pathingTestSpec{}, fmt.Errorf("pathing test click-entity: expected waypoint or entrance, got %q", arg)
		}
		return pathingTestSpec{kind: pathingTestClickEntity, entity: entity}, nil
	default:
		return pathingTestSpec{}, fmt.Errorf("pathing test: unknown spec kind %q", kind)
	}
}

// pathingTestIsReadOnly reports whether the configured pathing test performs no
// input actions (hover:watch), so the teleport bindings precheck can be skipped.
func (rt *Runtime) pathingTestIsReadOnly() bool {
	if rt.Options.PathingTest == "" {
		return false
	}
	spec, err := parsePathingTestSpec(rt.Options.PathingTest)
	return err == nil && !spec.requiresInput()
}

// validatePathingTestMode enforces flag exclusivity and input requirements at startup.
func validatePathingTestMode(cfg *config.Config, opts Options) error {
	if opts.PathingTest == "" {
		return nil
	}
	if opts.InputTest != "" || opts.Run != "" {
		return errPathingTestConflict
	}
	spec, err := parsePathingTestSpec(opts.PathingTest)
	if err != nil {
		return err
	}
	if spec.requiresInput() && !cfg.Input.Enabled {
		return fmt.Errorf("%w: %q", errInputRequiredForPathingTest, opts.PathingTest)
	}
	return nil
}

// RunPathingTest waits for process attach, window binding, and a valid in-game
// world state, then executes the parsed pathing test scenario. Pause and stop
// hotkeys stay active; the mode ends on completion, timeout, or stop.
func (rt *Runtime) RunPathingTest(specStr string) error {
	spec, err := parsePathingTestSpec(specStr)
	if err != nil {
		return err
	}
	if spec.requiresInput() && !rt.Input.Status().Enabled {
		return fmt.Errorf("pathing test %q requires input.enabled=true", specStr)
	}

	timeoutMs := rt.Options.PathingTestTimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultPathingTestTimeoutMs
	}

	rt.Log.Info("pathing test started",
		"spec", specStr,
		"timeout_ms", timeoutMs,
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
	if waitErr := rt.waitInputTestReady(ctx, state, hotkeyEvents, ticker, readyDeadline, cancel); waitErr != nil {
		return waitErr
	}
	if ctx.Err() != nil || rt.Input.Status().Stopped {
		rt.Log.Info("pathing test stopped", "reason", inputTestStopReason(ctx))
		return nil
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	switch spec.kind {
	case pathingTestHoverWatch:
		err = rt.runPathingHoverWatch(ctx, state, hotkeyEvents, ticker, deadline, cancel)
	case pathingTestTeleport:
		err = rt.runPathingTeleport(ctx, state, hotkeyEvents, ticker, deadline, cancel, spec)
	case pathingTestMoveArea:
		err = rt.runPathingMoveArea(ctx, state, hotkeyEvents, ticker, deadline, cancel, spec)
	case pathingTestClickEntity:
		err = rt.runPathingClickEntity(ctx, state, hotkeyEvents, ticker, deadline, cancel, spec)
	default:
		err = fmt.Errorf("pathing test: unsupported kind %q", spec.kind)
	}
	if err != nil {
		return err
	}

	rt.Log.Info("pathing test completed", "spec", specStr)
	return nil
}

// pathingTestTick advances one poll cycle and reports whether the loop must end.
func (rt *Runtime) pathingTestTick(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	cancel context.CancelFunc,
) (cur world.State, stop bool, err error) {
	select {
	case <-ctx.Done():
		return world.State{}, true, nil
	case ev := <-hotkeyEvents:
		rt.handleHotkeyEvent(ev, cancel)
		return rt.World.Current(), rt.Input.Status().Stopped, nil
	case <-ticker.C:
		if err := rt.runTick(ctx, state); err != nil {
			if errors.Is(err, context.Canceled) {
				return world.State{}, true, nil
			}
			return world.State{}, true, err
		}
		if state.hasEverAttached && !state.attached {
			return world.State{}, true, fmt.Errorf("process lost during pathing test")
		}
		return rt.World.Current(), rt.Input.Status().Stopped, nil
	}
}

// runPathingHoverWatch logs hover transitions while the operator moves the
// mouse manually (Slice-2 validation, read-only).
func (rt *Runtime) runPathingHoverWatch(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
) error {
	var last world.HoverInfo
	rt.Log.Info("hover watch active — Maus im Spiel über Entities bewegen (Stop-Hotkey beendet)")

	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		if cur.Hover != last {
			rt.logHoverChange(cur)
			last = cur.Hover
		}
	}
	return nil
}

func (rt *Runtime) logHoverChange(cur world.State) {
	if !cur.Hover.IsHovered {
		rt.Log.Info("hover cleared")
		return
	}
	args := []any{
		"unit_type", cur.Hover.UnitType.String(),
		"unit_id", cur.Hover.UnitID,
	}
	if name := hoverEntityName(cur); name != "" {
		args = append(args, "entity_name", name)
	}
	rt.Log.Info("hover changed", args...)
}

// hoverEntityName resolves the hovered unit against enumerated entities.
func hoverEntityName(cur world.State) string {
	switch cur.Hover.UnitType {
	case world.HoverUnitTypeObject:
		for _, o := range cur.Objects {
			if o.UnitID == cur.Hover.UnitID {
				return o.Name
			}
		}
	case world.HoverUnitTypeEntrance:
		for _, e := range cur.Entrances {
			if e.UnitID == cur.Hover.UnitID {
				return e.Name
			}
		}
	case world.HoverUnitTypeMonster:
		for _, m := range cur.Monsters {
			if m.UnitID == cur.Hover.UnitID {
				return m.Name
			}
		}
	}
	return ""
}

// runPathingTeleport casts a single teleport at world coordinates and logs the
// projected client position — the calibration aid for playable_center/tile_width.
func (rt *Runtime) runPathingTeleport(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
	spec pathingTestSpec,
) error {
	driver, ok := rt.Input.(pathing.InputDriver)
	if !ok {
		return fmt.Errorf("pathing test: input controller does not support pathing actions")
	}
	pathingCfg := mapPathingConfig(rt.Config.Pathing)
	mover := pathing.NewTeleportMover(rt.Log, driver, rt.Bindings, pathingCfg.Projector(), pathingCfg.MoveInterval)

	cur := rt.World.Current()
	before := cur.Player.Position
	target := world.Position{X: spec.targetX, Y: spec.targetY}

	clientX, clientY, err := mover.TeleportTo(time.Now(), before, target)
	if err != nil {
		return fmt.Errorf("pathing test teleport: %w", err)
	}
	rt.Log.Info("pathing test teleport cast",
		"player_x", before.X,
		"player_y", before.Y,
		"target_x", target.X,
		"target_y", target.Y,
		"client_x", clientX,
		"client_y", clientY,
	)

	// Observe the position change briefly so the operator can verify the landing spot.
	observeUntil := time.Now().Add(3 * time.Second)
	if observeUntil.After(deadline) {
		observeUntil = deadline
	}
	var after world.Position
	for time.Now().Before(observeUntil) {
		st, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		if st.Valid {
			after = st.Player.Position
		}
	}
	rt.Log.Info("pathing test teleport result",
		"pos_x_delta", int64(after.X)-int64(before.X),
		"pos_y_delta", int64(after.Y)-int64(before.Y),
		"after_x", after.X,
		"after_y", after.Y,
	)
	return nil
}

// runPathingMoveArea drives the navigator toward the target area (Stufe A).
func (rt *Runtime) runPathingMoveArea(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
	spec pathingTestSpec,
) error {
	goal := pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: spec.area}
	if err := rt.Pathing.Start(goal); err != nil {
		return fmt.Errorf("pathing test move-area: %w", err)
	}

	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		res := rt.Pathing.Tick(ctx, cur)
		if res.Done {
			rt.Log.Info("pathing test move-area finished",
				"status", string(res.Status),
				"reason", res.Reason,
				"area", cur.Area.Name,
				"area_id", uint32(cur.Area.ID),
			)
			return nil
		}
	}
	rt.Log.Warn("pathing test move-area timeout", "target_area", uint32(spec.area))
	return nil
}

// runPathingClickEntity drives the hover-feedback click loop on the nearest
// waypoint object or entrance (Stufe B).
func (rt *Runtime) runPathingClickEntity(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
	spec pathingTestSpec,
) error {
	driver, ok := rt.Input.(pathing.InputDriver)
	if !ok {
		return fmt.Errorf("pathing test: input controller does not support pathing actions")
	}
	pathingCfg := mapPathingConfig(rt.Config.Pathing)
	clicker := pathing.NewEntityClicker(rt.Log, driver, pathingCfg.Projector(), pathingCfg.Click)

	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		if !cur.Valid || cur.Phase != world.GamePhaseInGame {
			continue
		}

		target, found := pathingClickTarget(cur, spec.entity)
		if !found {
			rt.Log.Warn("pathing test click-entity: no matching entity visible", "entity", spec.entity)
			return nil
		}

		res, err := clicker.Tick(cur, target, pathingCfg.Explore.MaxEntranceClickDistance)
		if err != nil {
			rt.Log.Debug("pathing test click blocked", "error", err)
			continue
		}
		if res.Done {
			rt.Log.Info("pathing test click-entity finished",
				"status", string(res.Status),
				"target", target.Name,
				"unit_id", target.UnitID,
				"hover_attempts", res.Attempt,
			)
			return nil
		}
	}
	rt.Log.Warn("pathing test click-entity timeout", "entity", spec.entity)
	return nil
}

// pathingClickTarget selects the nearest waypoint object or entrance as click target.
func pathingClickTarget(cur world.State, entity string) (pathing.ClickTarget, bool) {
	switch entity {
	case "waypoint":
		o, ok := cur.NearestObject(world.ObjectKindWaypoint)
		if !ok {
			return pathing.ClickTarget{}, false
		}
		return pathing.ClickTarget{
			UnitID:   o.UnitID,
			UnitType: world.HoverUnitTypeObject,
			Position: o.Position,
			Name:     o.Name,
		}, true
	case "entrance":
		var best world.Entrance
		bestDist := 0.0
		found := false
		for _, e := range cur.Entrances {
			d := world.Distance(cur.Player.Position, e.Position)
			if !found || d < bestDist {
				best = e
				bestDist = d
				found = true
			}
		}
		if !found {
			return pathing.ClickTarget{}, false
		}
		return pathing.ClickTarget{
			UnitID:   best.UnitID,
			UnitType: world.HoverUnitTypeEntrance,
			Position: best.Position,
			Name:     best.Name,
		}, true
	default:
		return pathing.ClickTarget{}, false
	}
}
