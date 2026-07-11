package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
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
	pathingTestInspect     pathingTestKind = "inspect"
	pathingTestPlayTown    pathingTestKind = "play_town_route"
	pathingTestRecordTown  pathingTestKind = "record_town_route"
	pathingTestPickupItem  pathingTestKind = "pickup_item"
)

// pathingTestSpec is a parsed --pathing-test argument.
type pathingTestSpec struct {
	kind    pathingTestKind
	area    world.AreaID // move-area target.
	targetX uint32       // teleport world X.
	targetY uint32       // teleport world Y.
	entity  string       // click-entity target: waypoint | entrance.
	route   string       // town route id.
}

// requiresInput reports whether the spec performs real input actions.
func (s pathingTestSpec) requiresInput() bool {
	return s.kind != pathingTestHoverWatch && s.kind != pathingTestInspect && s.kind != pathingTestRecordTown
}

// parsePathingTestSpec parses specs like `teleport:5000,5000`, `hover:watch`,
// `move-area:black_marsh`, `click-entity:waypoint`,
// `inspect:entrances`, `inspect:layout`, `play-town-route:act1-waypoint`,
// `record-town-route:act1-waypoint`, or `pickup:item`.
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
	case "inspect":
		entity := strings.ToLower(arg)
		if entity != "entrances" && entity != "layout" {
			return pathingTestSpec{}, fmt.Errorf("pathing test inspect: expected entrances or layout, got %q", arg)
		}
		return pathingTestSpec{kind: pathingTestInspect, entity: entity}, nil
	case "play-town-route":
		route := strings.ToLower(arg)
		if route != "act1-waypoint" {
			return pathingTestSpec{}, fmt.Errorf("pathing test play-town-route: expected act1-waypoint, got %q", arg)
		}
		return pathingTestSpec{kind: pathingTestPlayTown, route: route}, nil
	case "record-town-route":
		route := strings.ToLower(arg)
		if route != "act1-waypoint" {
			return pathingTestSpec{}, fmt.Errorf("pathing test record-town-route: expected act1-waypoint, got %q", arg)
		}
		return pathingTestSpec{kind: pathingTestRecordTown, route: route}, nil
	case "pickup":
		target := strings.ToLower(arg)
		if target != "item" {
			return pathingTestSpec{}, fmt.Errorf("pathing test pickup: expected item, got %q", arg)
		}
		return pathingTestSpec{kind: pathingTestPickupItem, entity: target}, nil
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
	if waitErr := rt.waitPathingTestReady(ctx, state, hotkeyEvents, ticker, readyDeadline, cancel, spec.requiresInput()); waitErr != nil {
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
	case pathingTestInspect:
		err = rt.runPathingInspect(ctx, state, hotkeyEvents, ticker, deadline, cancel, spec)
	case pathingTestPlayTown:
		err = rt.runPathingPlayTownRoute(ctx, state, hotkeyEvents, ticker, deadline, cancel)
	case pathingTestRecordTown:
		err = rt.runPathingRecordTownRoute(ctx, state, hotkeyEvents, ticker, deadline, cancel)
	case pathingTestPickupItem:
		err = rt.runPathingPickupItem(ctx, state, hotkeyEvents, ticker, deadline, cancel)
	default:
		err = fmt.Errorf("pathing test: unsupported kind %q", spec.kind)
	}
	if err != nil {
		return err
	}

	rt.Log.Info("pathing test completed", "spec", specStr)
	return nil
}

// runPathingInspect logs read-only world/entity measurements while the operator
// manually positions the character.
func (rt *Runtime) runPathingInspect(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
	spec pathingTestSpec,
) error {
	rt.Log.Info("pathing inspect active - manually position the character; Stop-Hotkey ends the probe",
		"entity", spec.entity,
	)
	var last string
	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		if !cur.Valid || cur.Phase != world.GamePhaseInGame {
			continue
		}
		switch spec.entity {
		case "entrances":
			fp := inspectEntrancesFingerprint(cur)
			if fp != last {
				logInspectEntrances(rt.Log, cur)
				last = fp
			}
		case "layout":
			fp, err := pathing.BuildLayoutFingerprint(cur)
			if errors.Is(err, pathing.ErrLayoutAnchorsUnavailable) {
				continue
			}
			if err != nil {
				return fmt.Errorf("pathing inspect layout: %w", err)
			}
			if fp.Hash != last {
				rt.Log.Info("layout fingerprint observed",
					"layout_fingerprint", fp.Hash,
					"version", fp.Version,
					"area_id", fp.AreaID,
					"player_x", fp.PlayerX,
					"player_y", fp.PlayerY,
					"anchor_count", fp.AnchorCount,
				)
				last = fp.Hash
			}
		default:
			return fmt.Errorf("pathing inspect: unsupported entity %q", spec.entity)
		}
	}
	return nil
}

func (rt *Runtime) waitPathingTestReady(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
	needsInput bool,
) error {
	for {
		if time.Now().After(deadline) {
			st := rt.World.Current()
			rt.Log.Error("pathing test ready timeout",
				"attached", state.attached,
				"bound", rt.Input.Bound(),
				"needs_input", needsInput,
				"world_valid", st.Valid,
				"world_phase", st.Phase.String(),
				"world_reason", st.Reason,
			)
			return fmt.Errorf(
				"pathing test ready timeout: attached=%t bound=%t needs_input=%t world_valid=%t world_phase=%q world_reason=%q",
				state.attached,
				rt.Input.Bound(),
				needsInput,
				st.Valid,
				st.Phase.String(),
				st.Reason,
			)
		}

		select {
		case <-ctx.Done():
			rt.Log.Info("pathing test stopped", "reason", inputTestStopReason(ctx))
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
				return fmt.Errorf("process lost during pathing test")
			}
			cur := rt.World.Current()
			if state.attached && (!needsInput || rt.Input.Bound()) && cur.Valid && cur.Phase == world.GamePhaseInGame {
				if needsInput {
					state.bindingsPrecheckDone = true
				}
				rt.Log.Info("pathing test ready",
					"pid", rt.Process.Status().PID,
					"bound", rt.Input.Bound(),
					"needs_input", needsInput,
				)
				return nil
			}
		}
	}
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

func (rt *Runtime) runPathingPickupItem(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
) error {
	driver, ok := rt.Input.(pathing.InputDriver)
	if !ok {
		return fmt.Errorf("pathing test: input controller does not support pathing actions")
	}
	pathingCfg := mapPathingConfig(rt.Config.Pathing)
	clicker := pathing.NewEntityClicker(rt.Log, driver, pathingCfg.Projector(), pathingCfg.Click)
	adapter := &pickupClickerAdapter{input: driver, clicker: clicker}

	var exec *loot.PickupExecutor
	var target loot.PickupTarget
	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		if !cur.Valid || cur.Phase != world.GamePhaseInGame {
			continue
		}

		if exec == nil {
			report := rt.Loot.Decide(cur)
			var found bool
			target, found = loot.SelectPickupCandidate(cur, report)
			if !found {
				rt.Log.Info("loot pickup test: no candidate",
					"ground_item_count", len(cur.GroundItems()),
					"decision_count", len(report.Decisions),
				)
				return nil
			}
			rt.Log.Info("loot pickup test target selected",
				"unit_id", target.UnitID,
				"name", target.Name,
				"code", target.Code,
				"pos_x", target.Position.X,
				"pos_y", target.Position.Y,
			)
			exec = loot.NewPickupExecutor(rt.Log, mapLootPickupConfig(rt.Config.Loot.Pickup), adapter, target)
		}

		res := exec.Tick(cur, time.Now())
		if res.Done {
			rt.Log.Info("pathing test pickup:item finished",
				"status", string(res.Status),
				"unit_id", target.UnitID,
				"name", target.Name,
				"code", target.Code,
				"retry", res.Retry,
				"hover_attempts", res.HoverAttempt,
			)
			return nil
		}
	}
	rt.Log.Warn("pathing test pickup:item timeout")
	return nil
}

type pickupClickerAdapter struct {
	input   pathing.InputDriver
	clicker *pathing.EntityClicker
}

func (a *pickupClickerAdapter) Reset() {
	a.clicker.Reset()
}

func (a *pickupClickerAdapter) Tick(state world.State, target loot.PickupClickTarget, maxDistance float64) (loot.PickupClickResult, error) {
	if _, ok := a.input.Window(); !ok {
		return loot.PickupClickResult{}, fmt.Errorf("input window not bound")
	}
	res, err := a.clicker.Tick(state, pathing.ClickTarget{
		UnitID:   target.UnitID,
		UnitType: world.HoverUnitTypeItem,
		Position: target.Position,
		Name:     target.Name,
	}, maxDistance)
	if err != nil {
		return loot.PickupClickResult{Attempt: res.Attempt}, err
	}
	return loot.PickupClickResult{
		Status:  mapPickupClickStatus(res.Status),
		Attempt: res.Attempt,
		Done:    res.Done,
	}, nil
}

func mapPickupClickStatus(status pathing.ClickStatus) loot.PickupClickStatus {
	switch status {
	case pathing.ClickHit:
		return loot.PickupClickHit
	case pathing.ClickTooFar:
		return loot.PickupClickTooFar
	case pathing.ClickHoverNotFound:
		return loot.PickupClickHoverNotFound
	case pathing.ClickProjectionFailed:
		return loot.PickupClickProjectionFailed
	default:
		return loot.PickupClickPending
	}
}

func mapLootPickupConfig(cfg config.LootPickupConfig) loot.PickupConfig {
	return loot.PickupConfig{
		MaxRetries:                cfg.MaxRetries,
		MaxDistanceTiles:          cfg.MaxDistanceTiles,
		VerifyTicks:               cfg.VerifyTicks,
		VerifyTimeout:             time.Duration(cfg.VerifyTimeoutMs) * time.Millisecond,
		MonsterAbortDistanceTiles: cfg.MonsterAbortDistanceTiles,
	}
}

func (rt *Runtime) runPathingPlayTownRoute(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
) error {
	if rt.TownWalk == nil {
		return fmt.Errorf("pathing test play-town-route: town walker not wired")
	}
	rt.TownWalk.Reset()
	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil || stop {
			return err
		}
		res := rt.TownWalk.TickAct1Waypoint(ctx, cur)
		if res.Done {
			rt.Log.Info("pathing test play-town-route finished",
				"status", string(res.Status),
				"reason", res.Reason,
				"area", cur.Area.Name,
				"area_id", uint32(cur.Area.ID),
			)
			return nil
		}
	}
	rt.Log.Warn("pathing test play-town-route timeout")
	return nil
}

func (rt *Runtime) runPathingRecordTownRoute(
	ctx context.Context,
	state *runState,
	hotkeyEvents <-chan input.HotkeyEvent,
	ticker *time.Ticker,
	deadline time.Time,
	cancel context.CancelFunc,
) error {
	pathingCfg := mapPathingConfig(rt.Config.Pathing)
	routeFile := pathingCfg.TownWalk.RouteFile
	sampleDistance := pathingCfg.TownWalk.ArrivalDistance
	points := make([]world.Position, 0, 16)
	rt.Log.Info("town route recording active - manually walk from stash/spawn to Act-1 waypoint",
		"route_file", routeFile,
		"sample_distance_tiles", sampleDistance,
	)

	for time.Now().Before(deadline) {
		cur, stop, err := rt.pathingTestTick(ctx, state, hotkeyEvents, ticker, cancel)
		if err != nil {
			return err
		}
		if cur.Valid && cur.Phase == world.GamePhaseInGame && cur.Area.ID == world.RogueEncampment {
			pos := cur.Player.Position
			if len(points) == 0 || world.Distance(points[len(points)-1], pos) >= sampleDistance {
				points = append(points, pos)
				rt.Log.Info("town route sample",
					"index", len(points)-1,
					"pos_x", pos.X,
					"pos_y", pos.Y,
				)
			}
			if waypoint, ok := townRouteWaypointClickable(cur, pathingCfg.Waypoint.MaxClickDistance); ok && len(points) >= 2 {
				if world.Distance(points[len(points)-1], waypoint.Position) > 0 {
					points = append(points, waypoint.Position)
				}
				if err := pathing.SaveTownRoute(routeFile, sampleDistance, points); err != nil {
					return fmt.Errorf("save town route: %w", err)
				}
				rt.Log.Info("town route recording completed",
					"route_file", routeFile,
					"points", len(points),
					"waypoint_x", waypoint.Position.X,
					"waypoint_y", waypoint.Position.Y,
				)
				return nil
			}
		}
		if stop {
			break
		}
	}
	if len(points) < 2 {
		return fmt.Errorf("town route recording: need at least 2 samples, got %d", len(points))
	}
	if err := pathing.SaveTownRoute(routeFile, sampleDistance, points); err != nil {
		return fmt.Errorf("save town route: %w", err)
	}
	rt.Log.Info("town route recording saved after stop/timeout",
		"route_file", routeFile,
		"points", len(points),
	)
	return nil
}

func townRouteWaypointClickable(cur world.State, maxDistance float64) (world.Object, bool) {
	wp, ok := cur.NearestObject(world.ObjectKindWaypoint)
	if !ok {
		return world.Object{}, false
	}
	if maxDistance <= 0 {
		return wp, true
	}
	return wp, world.Distance(cur.Player.Position, wp.Position) <= maxDistance
}

type inspectEntrance struct {
	entrance world.Entrance
	distance float64
	dx       int64
	dy       int64
}

func inspectEntrances(cur world.State) []inspectEntrance {
	out := make([]inspectEntrance, 0, len(cur.Entrances))
	player := cur.Player.Position
	for _, e := range cur.Entrances {
		out = append(out, inspectEntrance{
			entrance: e,
			distance: world.Distance(player, e.Position),
			dx:       int64(e.Position.X) - int64(player.X),
			dy:       int64(e.Position.Y) - int64(player.Y),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].distance != out[j].distance {
			return out[i].distance < out[j].distance
		}
		return out[i].entrance.UnitID < out[j].entrance.UnitID
	})
	return out
}

func inspectEntrancesFingerprint(cur world.State) string {
	entries := inspectEntrances(cur)
	var b strings.Builder
	fmt.Fprintf(&b, "area=%d;pos=%d,%d;hover=%t/%s/%d;",
		cur.Area.ID,
		cur.Player.Position.X,
		cur.Player.Position.Y,
		cur.Hover.IsHovered,
		cur.Hover.UnitType.String(),
		cur.Hover.UnitID,
	)
	for _, entry := range entries {
		e := entry.entrance
		fmt.Fprintf(&b, "%d/%d/%s/%d,%d;",
			e.ID,
			e.UnitID,
			e.Kind.String(),
			e.Position.X,
			e.Position.Y,
		)
	}
	return b.String()
}

func logInspectEntrances(log *slog.Logger, cur world.State) {
	entries := inspectEntrances(cur)
	log.Info("pathing inspect entrances",
		"area", cur.Area.Name,
		"area_id", uint32(cur.Area.ID),
		"player_x", cur.Player.Position.X,
		"player_y", cur.Player.Position.Y,
		"entrance_count", len(entries),
		"hovered", cur.Hover.IsHovered,
		"hover_type", cur.Hover.UnitType.String(),
		"hover_unit_id", cur.Hover.UnitID,
	)
	for i, entry := range entries {
		e := entry.entrance
		name := e.Name
		if name == "" {
			name = "unknown"
		}
		log.Info("pathing inspect entrance",
			"rank", i,
			"id", e.ID,
			"unit_id", e.UnitID,
			"kind", e.Kind.String(),
			"name", name,
			"x", e.Position.X,
			"y", e.Position.Y,
			"dx", entry.dx,
			"dy", entry.dy,
			"distance", fmt.Sprintf("%.2f", entry.distance),
			"hovered", e.IsHovered,
		)
	}
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
