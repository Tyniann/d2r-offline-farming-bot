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
	offlineExitOverallTimeout    = 30 * time.Second
	offlineExitQuitMenuTimeout   = 5 * time.Second
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
	stage       offlineExitStage
	stableTicks int
	startedAt   time.Time
	stageAt     time.Time
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
		if state.Phase == world.GamePhaseLoading || state.Phase == world.GamePhaseUnknown {
			m.stableTicks = 0
			return offlineExitNoAction, false, nil
		}
		if state.Phase != world.GamePhaseInGame {
			return offlineExitNoAction, false, fmt.Errorf("offline exit requires in_game, got %s", state.Phase)
		}
		if !state.Valid {
			m.stableTicks = 0
			return offlineExitNoAction, false, nil
		}
		if state.Area.ID != world.RogueEncampment {
			return offlineExitNoAction, false, fmt.Errorf("offline exit requires Rogue Encampment, got %s", state.Area.Name)
		}
		if !state.Identity.Valid {
			m.stableTicks = 0
			return offlineExitNoAction, false, nil
		}
		if state.UI.InventoryOpen || state.UI.StashOpen {
			return offlineExitNoAction, false, fmt.Errorf("offline exit blocked by open inventory or stash")
		}
		if state.UI.QuitMenuOpen {
			return offlineExitNoAction, false, fmt.Errorf("offline exit requires initially closed quit menu")
		}
		m.stableTicks++
		if m.stableTicks < offlineExitStableTicks {
			return offlineExitNoAction, false, nil
		}
		m.advance(offlineExitAwaitQuitMenu, now)
		return offlineExitPressEscape, false, nil

	case offlineExitAwaitQuitMenu:
		if now.Sub(m.stageAt) >= offlineExitQuitMenuTimeout {
			return offlineExitNoAction, false, fmt.Errorf("offline exit timeout waiting for quit menu")
		}
		if !state.Valid || state.Phase == world.GamePhaseLoading {
			m.stableTicks = 0
			return offlineExitNoAction, false, nil
		}
		if state.Phase != world.GamePhaseInGame || state.Area.ID != world.RogueEncampment {
			return offlineExitNoAction, false, fmt.Errorf("offline exit context changed before quit menu confirmation")
		}
		if state.UI.InventoryOpen || state.UI.StashOpen {
			return offlineExitNoAction, false, fmt.Errorf("offline exit blocked by open inventory or stash")
		}
		if !state.UI.QuitMenuOpen {
			m.stableTicks = 0
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

func (m *offlineExitMachine) advance(stage offlineExitStage, now time.Time) {
	m.stage = stage
	m.stageAt = now
	m.stableTicks = 0
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
	ctrl, ok := rt.Input.(offlineExitController)
	if !ok {
		return fmt.Errorf("offline exit: controller lacks click support")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer rt.Process.Detach()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	machine := &offlineExitMachine{}
	escapePressed := false
	saveClicked := false

	rt.Log.Info("offline exit test waiting for safe town state",
		"required_area", world.LookupArea(world.RogueEncampment).Name,
		"required_client_width", offlineExitClientWidth,
		"required_client_height", offlineExitClientHeight,
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("offline exit poll: %w", err)
			}
			action, done, err := machine.tick(time.Now(), rt.World.Current())
			if err != nil {
				return err
			}
			switch action {
			case offlineExitPressEscape:
				if escapePressed {
					return fmt.Errorf("offline exit invariant: escape already pressed")
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
				escapePressed = true
				rt.Log.Info("offline exit quit menu requested")
			case offlineExitClickSave:
				if !escapePressed || saveClicked {
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
					"escape_presses", boolCount(escapePressed),
					"save_exit_clicks", boolCount(saveClicked),
				)
				return nil
			}
		}
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
