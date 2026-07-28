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
	characterSelectionSettle           = 350 * time.Millisecond
	characterScreenAnchorMargin        = 0.04
	characterNameAnchorMaxDifference   = 0.25
	characterNameAnchorBrightThreshold = 140
	characterNameAnchorShiftRadius     = 3
	characterSelectionRowStride        = 60
	characterSelectionVisibleRows      = 9
	characterSelectionBorderWidth      = 4
	characterSelectionBorderRatio      = 3
)

// characterNameAnchorRegion excludes the mutable title, level and class lines
// from the 210x60 evidence. The save name is the stable, catalog-unique UI
// identity; Memory still confirms both name and class after game entry.
var characterNameAnchorRegion = image.Rect(4, 17, 154, 36)

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
	if request.CatalogRevision == 0 {
		return fmt.Errorf("selection catalog revision is required")
	}
	if request.CharacterCount <= 0 || request.AnchorPath == "" {
		return fmt.Errorf("character selection request is incomplete")
	}
	expectedClass, ok := mapProfileClass(request.ExpectedClass)
	if !ok {
		return fmt.Errorf("character class %q is unsupported", request.ExpectedClass)
	}
	selectedRect, err := rt.navigateOfflineCharacter(ctx, character, request.AnchorPath, request.CharacterCount)
	if err != nil {
		return err
	}
	return rt.runOfflineDifficultyForCharacter(ctx, difficulty, character, expectedClass, true, selectedRect)
}

func (rt *Runtime) navigateOfflineCharacter(ctx context.Context, character, anchorPath string, characterCount int) (image.Rectangle, error) {
	ctrl, ok := rt.Input.(offlineDifficultyController)
	if !ok {
		return image.Rectangle{}, fmt.Errorf("character selection: controller lacks click or screenshot support")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return image.Rectangle{}, err
	}
	defer rt.stopHotkeys(cancel)
	state := &runState{}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		rt.Input.Unbind()
		_ = rt.Process.Detach()
	}()
	target := screenAnchor{
		name: "selected_character", path: anchorPath, rect: phase16CharacterAnchorRect,
		comparisonRegion: characterNameAnchorRegion, maxMeanDifference: characterNameAnchorMaxDifference,
		brightThreshold: characterNameAnchorBrightThreshold, brightShiftRadius: characterNameAnchorShiftRadius,
	}
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
			return image.Rectangle{}, canceledInputOperationError(ctx, ctrl.Status(), rt.Config.Input.StopHotkey)
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if time.Since(startedAt) >= offlineStartTimeout {
				return image.Rectangle{}, fmt.Errorf("character selection timeout")
			}
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return image.Rectangle{}, fmt.Errorf("character selection poll: %w", err)
			}
			current := rt.World.Current()
			if current.Phase != world.GamePhaseMenu || current.Valid || !rt.Input.Bound() || time.Now().Before(nextCapture) {
				continue
			}
			if err := validateOfflineExitWindow(ctrl); err != nil {
				return image.Rectangle{}, fmt.Errorf("character selection: %w", err)
			}
			status := ctrl.Status()
			if !status.Enabled || status.Paused || status.Stopped {
				return image.Rectangle{}, fmt.Errorf("character selection requires enabled, unpaused input")
			}
			if !focused {
				if err := ctrl.Focus(); err != nil {
					return image.Rectangle{}, fmt.Errorf("focus before character screen verification: %w", err)
				}
				focused = true
				nextCapture = time.Now().Add(characterSelectionSettle)
				continue
			}
			capture, err := ctrl.CaptureClient()
			if err != nil {
				return image.Rectangle{}, fmt.Errorf("character selection capture: %w", err)
			}
			if verifyErr := verifyCharacterScreenCapture(capture, play, dialog); verifyErr != nil {
				return image.Rectangle{}, verifyErr
			}
			targetMatched := false
			if machine.homeSent {
				target.rect = characterSelectionRowRect(machine.downSteps)
				targetScore, targetErr := matchScreenAnchor(capture, target)
				if targetErr != nil {
					return image.Rectangle{}, targetErr
				}
				borderMatched, borderErr := matchSelectedCharacterBorder(capture, target)
				if borderErr != nil {
					return image.Rectangle{}, borderErr
				}
				targetMatched = targetScore <= target.maximumMeanDifference() && borderMatched
			}
			action, actionErr := machine.tick(targetMatched)
			if actionErr != nil {
				return image.Rectangle{}, actionErr
			}
			switch action {
			case characterNavigationHome:
				if err := ctrl.PressKey("home"); err != nil {
					return image.Rectangle{}, fmt.Errorf("select first offline character: %w", err)
				}
				nextCapture = time.Now().Add(characterSelectionSettle)
			case characterNavigationDown:
				if err := ctrl.PressKey("down"); err != nil {
					return image.Rectangle{}, fmt.Errorf("select next offline character: %w", err)
				}
				nextCapture = time.Now().Add(characterSelectionSettle)
			case characterNavigationComplete:
				rt.Log.Info("offline character selected", "character", character, "home", true, "down_steps", machine.downSteps)
				return target.rect, nil
			}
		}
	}
}

func characterSelectionRowRect(downSteps int) image.Rectangle {
	if downSteps < 0 {
		downSteps = 0
	}
	if downSteps >= characterSelectionVisibleRows {
		downSteps = characterSelectionVisibleRows - 1
	}
	offsetY := downSteps * characterSelectionRowStride
	return phase16CharacterAnchorRect.Add(image.Pt(0, offsetY))
}

func matchSelectedCharacterBorder(actual image.Image, anchor screenAnchor) (bool, error) {
	expected, err := loadScreenAnchor(actual, anchor, image.Rect(0, 0, anchor.rect.Dx(), anchor.rect.Dy()))
	if err != nil {
		return false, err
	}
	expectedGold := countCharacterBorderGold(expected, expected.Bounds())
	actualGold := countCharacterBorderGold(actual, anchor.rect)
	if expectedGold == 0 {
		return false, fmt.Errorf("%s screen anchor contains no selected border", anchor.name)
	}
	return actualGold*characterSelectionBorderRatio >= expectedGold, nil
}

func countCharacterBorderGold(value image.Image, rect image.Rectangle) int {
	count := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			localX, localY := x-rect.Min.X, y-rect.Min.Y
			if localX >= characterSelectionBorderWidth && localX < rect.Dx()-characterSelectionBorderWidth &&
				localY >= characterSelectionBorderWidth && localY < rect.Dy()-characterSelectionBorderWidth {
				continue
			}
			r, g, b, _ := value.At(x, y).RGBA()
			if r >= 120*0x101 && g >= 80*0x101 && r*10 > b*13 && g*10 > b*11 {
				count++
			}
		}
	}
	return count
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
