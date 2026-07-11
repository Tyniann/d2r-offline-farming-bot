package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	offlineDifficultyClientWidth  = 1280
	offlineDifficultyClientHeight = 720
	offlineDifficultyTimeout      = 30 * time.Second
)

type offlineDifficulty string

const (
	offlineDifficultyNormal    offlineDifficulty = "normal"
	offlineDifficultyNightmare offlineDifficulty = "nightmare"
	offlineDifficultyHell      offlineDifficulty = "hell"
)

type offlineDifficultyController interface {
	inputController
	Click(button input.MouseButton) error
}

func parseOfflineDifficulty(raw string) (offlineDifficulty, error) {
	switch difficulty := offlineDifficulty(strings.ToLower(strings.TrimSpace(raw))); difficulty {
	case offlineDifficultyNormal, offlineDifficultyNightmare, offlineDifficultyHell:
		return difficulty, nil
	default:
		return "", fmt.Errorf("offline difficulty must be normal, nightmare, or hell, got %q", raw)
	}
}

func offlineDifficultyPoint(difficulty offlineDifficulty) (int, int) {
	switch difficulty {
	case offlineDifficultyNormal:
		return 640, 311
	case offlineDifficultyNightmare:
		return 640, 355
	case offlineDifficultyHell:
		return 640, 403
	default:
		return 0, 0
	}
}

func selectOfflineDifficulty(ctrl offlineDifficultyController, difficulty offlineDifficulty) error {
	win, ok := ctrl.Window()
	if !ok {
		return fmt.Errorf("offline difficulty selection: input window not bound")
	}
	if win.ClientWidth != offlineDifficultyClientWidth || win.ClientHeight != offlineDifficultyClientHeight {
		return fmt.Errorf("offline difficulty selection requires %dx%d, got %dx%d", offlineDifficultyClientWidth, offlineDifficultyClientHeight, win.ClientWidth, win.ClientHeight)
	}
	x, y := offlineDifficultyPoint(difficulty)
	if x == 0 || y == 0 {
		return fmt.Errorf("offline difficulty selection: unsupported difficulty %q", difficulty)
	}
	if err := ctrl.MoveTo(x, y); err != nil {
		return fmt.Errorf("move to %s difficulty: %w", difficulty, err)
	}
	if err := ctrl.Click(input.MouseLeft); err != nil {
		return fmt.Errorf("click %s difficulty: %w", difficulty, err)
	}
	return nil
}

// RunOfflineDifficultyTest performs one explicit difficulty click from a manually prepared
// offline character screen and verifies in-game arrival plus confirmed character identity.
func (rt *Runtime) RunOfflineDifficultyTest(rawDifficulty string) error {
	difficulty, err := parseOfflineDifficulty(rawDifficulty)
	if err != nil {
		return err
	}
	ctrl, ok := rt.Input.(offlineDifficultyController)
	if !ok {
		return fmt.Errorf("offline difficulty selection: controller lacks click support")
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
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	deadline := time.Now().Add(offlineDifficultyTimeout)

	rt.Log.Info("offline difficulty test waiting for prepared offline character screen",
		"difficulty", difficulty,
		"required_client_width", offlineDifficultyClientWidth,
		"required_client_height", offlineDifficultyClientHeight,
	)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			cur := rt.World.Current()
			if state.attached && rt.Input.Bound() && cur.Phase == world.GamePhaseMenu {
				if err := selectOfflineDifficulty(ctrl, difficulty); err != nil {
					return err
				}
				rt.Log.Info("offline difficulty selected", "difficulty", difficulty)
				return rt.waitOfflineGameIdentity(ctx, state, hotkeys, ticker, deadline, cancel, difficulty)
			}
		}
	}
	return fmt.Errorf("offline difficulty test timeout waiting for menu state")
}

func (rt *Runtime) waitOfflineGameIdentity(ctx context.Context, state *runState, hotkeys <-chan input.HotkeyEvent, ticker *time.Ticker, deadline time.Time, cancel context.CancelFunc, difficulty offlineDifficulty) error {
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			cur := rt.World.Current()
			if cur.Valid && cur.Phase == world.GamePhaseInGame && cur.Identity.Valid {
				rt.Log.Info("offline difficulty test completed",
					"selected_difficulty", difficulty,
					"character_name", cur.Identity.CharacterName,
					"character_class", cur.Identity.Class.String(),
				)
				return nil
			}
		}
	}
	return fmt.Errorf("offline difficulty test timeout waiting for confirmed in-game character")
}
