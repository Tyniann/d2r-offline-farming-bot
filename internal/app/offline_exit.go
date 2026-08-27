package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	offlineExitClientWidth       = 1280
	offlineExitClientHeight      = 720
	offlineExitClickX            = 640
	offlineExitClickY            = 327
	offlineExitStableTicks       = 3
	offlineExitTownSettle        = 500 * time.Millisecond
	offlineExitOverallTimeout    = 30 * time.Second
	offlineExitQuitMenuTimeout   = 5 * time.Second
	offlineExitQuitMenuRetry     = 1500 * time.Millisecond
	offlineExitQuitMenuAttempts  = 2
	offlineExitCompletionTimeout = 15 * time.Second
)

type offlineExitController interface {
	inputController
	Click(button input.MouseButton) error
}

type offlineWindowController interface {
	Window() (input.WindowInfo, bool)
}

type offlineExitStage uint8

type offlineExitMode uint8

const (
	offlineExitVerifiedRogueTown offlineExitMode = iota
	offlineExitCurrentArea
)

const (
	offlineExitAwaitSafeTown offlineExitStage = iota
	offlineExitAwaitQuitMenu
	offlineExitAwaitCompletion
	offlineExitComplete
)

type offlineExitAction uint8

const (
	offlineExitNoAction offlineExitAction = iota
	offlineExitPressEscape
	offlineExitClickSave
)

type offlineExitMachine struct {
	mode                offlineExitMode
	stage               offlineExitStage
	stableTicks         int
	startedAt           time.Time
	stageAt             time.Time
	safeTownSince       time.Time
	quitMenuRequestedAt time.Time
	quitMenuRequests    int
	contextArea         world.AreaID
	contextCharacter    string
	contextClass        world.CharacterClass
	contextMapSeed      uint32
	contextLastAt       time.Time
}

func offlineExitModeForAuthorization(authorization ExitAuthorization) (offlineExitMode, error) {
	switch authorization {
	case ExitAuthorizationVerifiedRogueTown:
		return offlineExitVerifiedRogueTown, nil
	case ExitAuthorizationMemoryGatedCurrentArea:
		return offlineExitCurrentArea, nil
	default:
		return offlineExitVerifiedRogueTown, fmt.Errorf("offline exit authorization %q is not executable", authorization)
	}
}

func (m *offlineExitMachine) tick(now time.Time, state world.State) (offlineExitAction, bool, error) {
	if m.startedAt.IsZero() {
		m.startedAt = now
		m.stageAt = now
	}
	if now.Sub(m.startedAt) >= offlineExitOverallTimeout {
		return offlineExitNoAction, false, fmt.Errorf("offline exit timeout")
	}

	switch m.stage {
	case offlineExitAwaitSafeTown:
		if m.mode == offlineExitCurrentArea {
			return m.tickCurrentAreaPrecondition(now, state)
		}
		if state.Phase == world.GamePhaseLoading || state.Phase == world.GamePhaseUnknown {
			return offlineExitNoAction, false, nil
		}
		if state.Phase != world.GamePhaseInGame {
			return offlineExitNoAction, false, fmt.Errorf("offline exit requires in_game, got %s", state.Phase)
		}
		if !state.Valid {
			return offlineExitNoAction, false, nil
		}
		if state.Area.ID != world.RogueEncampment {
			return offlineExitNoAction, false, fmt.Errorf("offline exit requires Rogue Encampment, got %s", state.Area.Name)
		}
		if !state.Identity.Valid {
			return offlineExitNoAction, false, nil
		}
		if state.UI.QuitMenuOpen {
			m.advance(offlineExitAwaitQuitMenu, now)
			return offlineExitNoAction, false, nil
		}
		if m.safeTownSince.IsZero() {
			// Rogue Encampment plus stable identity is the authoritative handoff
			// from egress. D2R may keep moving the player or expose stale town-UI
			// flags after a waypoint transition, so neither may restart this
			// one-time input-settle window indefinitely.
			m.safeTownSince = now
			return offlineExitNoAction, false, nil
		}
		if now.Sub(m.safeTownSince) < offlineExitTownSettle {
			return offlineExitNoAction, false, nil
		}
		m.advance(offlineExitAwaitQuitMenu, now)
		m.quitMenuRequestedAt = now
		m.quitMenuRequests = 1
		return offlineExitPressEscape, false, nil

	case offlineExitAwaitQuitMenu:
		if now.Sub(m.stageAt) >= offlineExitQuitMenuTimeout {
			return offlineExitNoAction, false, fmt.Errorf("offline exit timeout waiting for quit menu")
		}
		if !state.Valid || state.Phase == world.GamePhaseLoading {
			if m.mode == offlineExitCurrentArea {
				return offlineExitNoAction, false, fmt.Errorf("offline exit current-area context changed before quit menu confirmation")
			}
			m.stableTicks = 0
			return offlineExitNoAction, false, nil
		}
		if state.Phase != world.GamePhaseInGame || !m.matchesAuthorizedContext(state) {
			return offlineExitNoAction, false, fmt.Errorf("offline exit context changed before quit menu confirmation")
		}
		if !state.UI.QuitMenuOpen {
			m.stableTicks = 0
			// The first Escape can close a stale town surface or be dropped
			// while D2R restores focus. Retry once only while Memory still
			// proves that the quit menu is closed.
			if m.quitMenuRequests < offlineExitQuitMenuAttempts &&
				now.Sub(m.quitMenuRequestedAt) >= offlineExitQuitMenuRetry {
				m.quitMenuRequests++
				m.quitMenuRequestedAt = now
				return offlineExitPressEscape, false, nil
			}
			return offlineExitNoAction, false, nil
		}
		m.stableTicks++
		if m.stableTicks < offlineExitStableTicks {
			return offlineExitNoAction, false, nil
		}
		m.advance(offlineExitAwaitCompletion, now)
		return offlineExitClickSave, false, nil

	case offlineExitAwaitCompletion:
		if now.Sub(m.stageAt) >= offlineExitCompletionTimeout {
			return offlineExitNoAction, false, fmt.Errorf("offline exit timeout waiting for menu arrival")
		}
		if state.Phase == world.GamePhaseLoading {
			m.stableTicks = 0
			return offlineExitNoAction, false, nil
		}
		if state.Phase == world.GamePhaseMenu && !state.Valid && !state.UI.QuitMenuOpen {
			m.stableTicks++
			if m.stableTicks >= offlineExitStableTicks {
				m.stage = offlineExitComplete
				return offlineExitNoAction, true, nil
			}
			return offlineExitNoAction, false, nil
		}
		m.stableTicks = 0
		return offlineExitNoAction, false, nil

	case offlineExitComplete:
		return offlineExitNoAction, true, nil
	default:
		return offlineExitNoAction, false, fmt.Errorf("offline exit unknown stage %d", m.stage)
	}
}

func (m *offlineExitMachine) tickCurrentAreaPrecondition(now time.Time, state world.State) (offlineExitAction, bool, error) {
	if state.Phase == world.GamePhaseLoading || state.Phase == world.GamePhaseUnknown || !state.Valid {
		if m.contextArea != world.None {
			return offlineExitNoAction, false, fmt.Errorf("offline exit current-area context changed during authorization")
		}
		return offlineExitNoAction, false, nil
	}
	if state.Phase != world.GamePhaseInGame {
		return offlineExitNoAction, false, fmt.Errorf("offline exit requires in_game, got %s", state.Phase)
	}
	if state.Area.ID == world.None || !state.Identity.Valid || state.Identity.MapSeed == 0 {
		return offlineExitNoAction, false, fmt.Errorf("offline exit current-area context is incomplete")
	}
	if state.At.IsZero() || (!m.contextLastAt.IsZero() && !state.At.After(m.contextLastAt)) {
		return offlineExitNoAction, false, fmt.Errorf("offline exit current-area authorization requires fresh snapshots")
	}
	if m.contextArea == world.None {
		m.contextArea = state.Area.ID
		m.contextCharacter = state.Identity.CharacterName
		m.contextClass = state.Identity.Class
		m.contextMapSeed = state.Identity.MapSeed
	} else if !m.matchesAuthorizedContext(state) {
		return offlineExitNoAction, false, fmt.Errorf("offline exit current-area context changed during authorization")
	}
	m.contextLastAt = state.At
	m.stableTicks++
	if m.stableTicks < offlineExitStableTicks {
		return offlineExitNoAction, false, nil
	}
	m.advance(offlineExitAwaitQuitMenu, now)
	if state.UI.QuitMenuOpen {
		return offlineExitNoAction, false, nil
	}
	m.quitMenuRequestedAt = now
	m.quitMenuRequests = 1
	return offlineExitPressEscape, false, nil
}

func (m *offlineExitMachine) matchesAuthorizedContext(state world.State) bool {
	if m.mode == offlineExitVerifiedRogueTown {
		return state.Area.ID == world.RogueEncampment
	}
	return state.Area.ID == m.contextArea && state.Identity.Valid &&
		state.Identity.CharacterName == m.contextCharacter && state.Identity.Class == m.contextClass &&
		state.Identity.MapSeed == m.contextMapSeed
}

func (m *offlineExitMachine) advance(stage offlineExitStage, now time.Time) {
	m.stage = stage
	m.stageAt = now
	m.stableTicks = 0
}

func (s offlineExitStage) String() string {
	switch s {
	case offlineExitAwaitSafeTown:
		return "await_safe_town"
	case offlineExitAwaitQuitMenu:
		return "await_quit_menu"
	case offlineExitAwaitCompletion:
		return "await_completion"
	case offlineExitComplete:
		return "complete"
	default:
		return fmt.Sprintf("unknown_%d", s)
	}
}

func validateOfflineExitWindow(ctrl offlineWindowController) error {
	win, ok := ctrl.Window()
	if !ok {
		return fmt.Errorf("offline exit: input window not bound")
	}
	if win.ClientWidth != offlineExitClientWidth || win.ClientHeight != offlineExitClientHeight {
		return fmt.Errorf("offline exit requires %dx%d, got %dx%d", offlineExitClientWidth, offlineExitClientHeight, win.ClientWidth, win.ClientHeight)
	}
	return nil
}

// RunOfflineExitTest performs one isolated Memory-gated Save & Exit from a
// stable Rogue Encampment state and confirms arrival in the offline menu.
func (rt *Runtime) RunOfflineExitTest() error {
	err := rt.runOfflineExitTest(context.Background())
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (rt *Runtime) runOfflineExitTest(parent context.Context) error {
	return rt.runOfflineExit(parent, offlineExitVerifiedRogueTown)
}

func (rt *Runtime) runOfflineExit(parent context.Context, mode offlineExitMode) error {
	ctrl, ok := rt.Input.(offlineExitController)
	if !ok {
		return fmt.Errorf("offline exit: controller lacks click support")
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	machine := &offlineExitMachine{mode: mode}
	escapePresses := 0
	saveClicked := false
	lastDiagnosticAt := time.Time{}

	requiredArea := world.LookupArea(world.RogueEncampment).Name
	if mode == offlineExitCurrentArea {
		requiredArea = "stable_current_area"
	}
	rt.Log.Info("offline exit waiting for authorized state",
		"mode", mode.String(),
		"required_area", requiredArea,
		"required_client_width", offlineExitClientWidth,
		"required_client_height", offlineExitClientHeight,
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("offline exit poll: %w", err)
			}
			now := time.Now()
			current := rt.World.Current()
			action, done, err := machine.tick(now, current)
			if lastDiagnosticAt.IsZero() || now.Sub(lastDiagnosticAt) >= time.Second {
				rt.Log.Info("offline exit state", offlineExitLogArgs(machine, current, now, escapePresses, saveClicked)...)
				lastDiagnosticAt = now
			}
			if err != nil {
				rt.Log.Error("offline exit failed", append(offlineExitLogArgs(machine, current, now, escapePresses, saveClicked), "error", err)...)
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			switch action {
			case offlineExitPressEscape:
				if escapePresses >= offlineExitQuitMenuAttempts {
					return fmt.Errorf("offline exit invariant: escape retry budget exhausted")
				}
				if err := validateOfflineExitWindow(ctrl); err != nil {
					return err
				}
				if err := ctrl.Focus(); err != nil {
					return fmt.Errorf("offline exit focus before quit menu: %w", err)
				}
				if err := ctrl.PressKey("esc"); err != nil {
					return fmt.Errorf("offline exit open quit menu: %w", err)
				}
				escapePresses++
				rt.Log.Info("offline exit quit menu requested", "attempt", escapePresses)
			case offlineExitClickSave:
				if saveClicked {
					return fmt.Errorf("offline exit invariant: invalid Save & Exit click order")
				}
				if err := validateOfflineExitWindow(ctrl); err != nil {
					return err
				}
				if err := ctrl.Focus(); err != nil {
					return fmt.Errorf("offline exit focus before Save & Exit: %w", err)
				}
				if err := ctrl.MoveTo(offlineExitClickX, offlineExitClickY); err != nil {
					return fmt.Errorf("offline exit move to Save & Exit: %w", err)
				}
				if err := ctrl.Click(input.MouseLeft); err != nil {
					return fmt.Errorf("offline exit click Save & Exit: %w", err)
				}
				saveClicked = true
				rt.Log.Info("offline exit Save & Exit clicked",
					"client_x", offlineExitClickX,
					"client_y", offlineExitClickY,
				)
			}
			if done {
				rt.Log.Info("offline exit test completed",
					"escape_presses", escapePresses,
					"save_exit_clicks", boolCount(saveClicked),
				)
				return nil
			}
		}
	}
}

func (m offlineExitMode) String() string {
	switch m {
	case offlineExitVerifiedRogueTown:
		return "verified_rogue_town"
	case offlineExitCurrentArea:
		return "memory_gated_current_area"
	default:
		return fmt.Sprintf("unknown_%d", m)
	}
}

func offlineExitLogArgs(machine *offlineExitMachine, state world.State, now time.Time, escapePresses int, saveClicked bool) []any {
	startedElapsed := int64(-1)
	if !machine.startedAt.IsZero() {
		startedElapsed = now.Sub(machine.startedAt).Milliseconds()
	}
	stageElapsed := int64(-1)
	if !machine.stageAt.IsZero() {
		stageElapsed = now.Sub(machine.stageAt).Milliseconds()
	}
	safeTownElapsed := int64(-1)
	if !machine.safeTownSince.IsZero() {
		safeTownElapsed = now.Sub(machine.safeTownSince).Milliseconds()
	}
	return []any{
		"mode", machine.mode.String(),
		"stage", machine.stage.String(),
		"elapsed_ms", startedElapsed,
		"stage_elapsed_ms", stageElapsed,
		"safe_town_elapsed_ms", safeTownElapsed,
		"valid", state.Valid,
		"phase", state.Phase,
		"area_id", state.Area.ID,
		"area", state.Area.Name,
		"identity_valid", state.Identity.Valid,
		"player_x", state.Player.Position.X,
		"player_y", state.Player.Position.Y,
		"inventory_open", state.UI.InventoryOpen,
		"npc_interact_open", state.UI.NPCInteractOpen,
		"npc_shop_open", state.UI.NPCShopOpen,
		"waypoint_open", state.UI.WaypointOpen,
		"stash_open", state.UI.StashOpen,
		"quit_menu_open", state.UI.QuitMenuOpen,
		"escape_presses", escapePresses,
		"save_exit_clicked", saveClicked,
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
