package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	characterSelectionSettle    = 350 * time.Millisecond
	characterScreenAnchorMargin = 0.04
)

type characterNavigationAction uint8

const (
	characterNavigationNone characterNavigationAction = iota
	characterNavigationHome
	characterNavigationDown
	characterNavigationComplete
)

type characterNavigationMachine struct {
	characterCount int
	homeSent       bool
	downSteps      int
	stableMatches  int
}

func (m *characterNavigationMachine) tick(targetMatched bool) (characterNavigationAction, error) {
	if !m.homeSent {
		m.homeSent = true
		return characterNavigationHome, nil
	}
	if targetMatched {
		m.stableMatches++
		if m.stableMatches >= offlineExitStableTicks {
			return characterNavigationComplete, nil
		}
		return characterNavigationNone, nil
	}
	m.stableMatches = 0
	if m.downSteps >= m.characterCount-1 {
		return characterNavigationNone, fmt.Errorf("character_selection_unconfirmed: target anchor not found after Home and %d Down steps", m.downSteps)
	}
	m.downSteps++
	return characterNavigationDown, nil
}

// CharacterSelectionRequest identifies one catalog-revision-bound UI selection.
type CharacterSelectionRequest struct {
	Character       string
	Difficulty      string
	CatalogRevision uint64
	CharacterCount  int
	AnchorPath      string
	ExpectedClass   string
}

// ApplyCharacterSelection selects a catalog character via bounded Home/Down
// navigation, then reuses the verified Play, difficulty and Memory flow.
func (rt *Runtime) ApplyCharacterSelection(ctx context.Context, request CharacterSelectionRequest) error {
	character, err := validateOfflineCharacter(request.Character)
	if err != nil {
		return err
	}
	difficulty, err := parseOfflineDifficulty(request.Difficulty)
	if err != nil {
		return err
	}
	if request.CatalogRevision != 1 {
		return fmt.Errorf("selection catalog revision changed")
	}
	if request.CharacterCount <= 0 || request.AnchorPath == "" {
		return fmt.Errorf("character selection request is incomplete")
	}
	expectedClass, ok := mapProfileClass(request.ExpectedClass)
	if !ok {
		return fmt.Errorf("character class %q is unsupported", request.ExpectedClass)
	}
	if err := rt.navigateOfflineCharacter(ctx, character, request.AnchorPath, request.CharacterCount); err != nil {
		return err
	}
	return rt.runOfflineDifficultyForCharacter(ctx, difficulty, character, expectedClass, true)
}

func (rt *Runtime) navigateOfflineCharacter(ctx context.Context, character, anchorPath string, characterCount int) error {
	ctrl, ok := rt.Input.(offlineDifficultyController)
	if !ok {
		return fmt.Errorf("character selection: controller lacks click or screenshot support")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	state := &runState{}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		rt.Input.Unbind()
		_ = rt.Process.Detach()
	}()
	target := screenAnchor{name: "selected_character", path: anchorPath, rect: image.Rect(1035, 48, 1245, 108)}
	play := screenAnchor{name: "active_play", path: rt.Config.ResolvePath("ui/character-play.png"), rect: image.Rect(538, 624, 741, 671)}
	dialog := screenAnchor{name: "difficulty_dialog", path: rt.Config.ResolvePath("ui/difficulty-dialog.png"), rect: image.Rect(550, 245, 730, 420)}
	// Config.ResolvePath uses paths.config_dir, matching the production anchor root.
	machine := &characterNavigationMachine{characterCount: characterCount}
	nextCapture := time.Time{}
	startedAt := time.Now()
	focused := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if time.Since(startedAt) >= offlineStartTimeout {
				return fmt.Errorf("character selection timeout")
			}
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("character selection poll: %w", err)
			}
			current := rt.World.Current()
			if current.Phase != world.GamePhaseMenu || current.Valid || !rt.Input.Bound() || time.Now().Before(nextCapture) {
				continue
			}
			if err := validateOfflineExitWindow(ctrl); err != nil {
				return fmt.Errorf("character selection: %w", err)
			}
			status := ctrl.Status()
			if !status.Enabled || status.Paused || status.Stopped {
				return fmt.Errorf("character selection requires enabled, unpaused input")
			}
			if !focused {
				if err := ctrl.Focus(); err != nil {
					return fmt.Errorf("focus before character screen verification: %w", err)
				}
				focused = true
				nextCapture = time.Now().Add(characterSelectionSettle)
				continue
			}
			capture, err := ctrl.CaptureClient()
			if err != nil {
				return fmt.Errorf("character selection capture: %w", err)
			}
			if verifyErr := verifyCharacterScreenCapture(capture, play, dialog); verifyErr != nil {
				return verifyErr
			}
			targetMatched := false
			if machine.homeSent {
				targetScore, targetErr := matchScreenAnchor(capture, target)
				if targetErr != nil {
					return targetErr
				}
				targetMatched = targetScore <= screenAnchorMaxMeanDifference
			}
			action, actionErr := machine.tick(targetMatched)
			if actionErr != nil {
				return actionErr
			}
			switch action {
			case characterNavigationHome:
				if err := ctrl.PressKey("home"); err != nil {
					return fmt.Errorf("select first offline character: %w", err)
				}
				nextCapture = time.Now().Add(characterSelectionSettle)
			case characterNavigationDown:
				if err := ctrl.PressKey("down"); err != nil {
					return fmt.Errorf("select next offline character: %w", err)
				}
				nextCapture = time.Now().Add(characterSelectionSettle)
			case characterNavigationComplete:
				rt.Log.Info("offline character selected", "character", character, "home", true, "down_steps", machine.downSteps)
				return nil
			}
		}
	}
}

func verifyCharacterScreenCapture(capture image.Image, play, dialog screenAnchor) error {
	playScore, err := matchScreenAnchor(capture, play)
	if err != nil {
		return err
	}
	dialogScore, err := matchScreenAnchor(capture, dialog)
	if err != nil {
		return err
	}

	// The general positive-match threshold deliberately tolerates animation and
	// rendering variance, so both dark frontend anchors can fall below it on the
	// wrong screen. Screen classification must therefore compare the two positive
	// anchors and require a clear winner instead of interpreting one threshold as
	// proof that the other screen is present or absent.
	if playScore <= screenAnchorMaxMeanDifference && playScore+characterScreenAnchorMargin <= dialogScore {
		return nil
	}
	if dialogScore <= screenAnchorMaxMeanDifference && dialogScore+characterScreenAnchorMargin <= playScore {
		return fmt.Errorf("character_screen_unconfirmed: difficulty dialog is open (play=%.4f dialog=%.4f)", playScore, dialogScore)
	}
	return fmt.Errorf("character_screen_unconfirmed: screen anchors are ambiguous (play=%.4f dialog=%.4f)", playScore, dialogScore)
}
