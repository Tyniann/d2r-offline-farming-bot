package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	offlineDifficultyClientWidth  = 1280
	offlineDifficultyClientHeight = 720
	offlineStartTimeout           = 45 * time.Second
	offlineStartStageTimeout      = 15 * time.Second
	offlineCharacterSettleDelay   = 1200 * time.Millisecond
	offlinePlayX                  = 640
	offlinePlayY                  = 648
)

var offlineCharacterNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type offlineDifficulty string

const (
	offlineDifficultyNormal    offlineDifficulty = "normal"
	offlineDifficultyNightmare offlineDifficulty = "nightmare"
	offlineDifficultyHell      offlineDifficulty = "hell"
)

type offlineDifficultyController interface {
	inputController
	Click(button input.MouseButton) error
	CaptureClient() (*image.RGBA, error)
}

type offlineStartStage uint8

const (
	offlineStartAwaitCharacter offlineStartStage = iota
	offlineStartAwaitDifficulty
	offlineStartAwaitGame
	offlineStartComplete
)

type offlineStartAction uint8

const (
	offlineStartNoAction offlineStartAction = iota
	offlineStartVerifyCharacter
	offlineStartVerifyDifficulty
)

type offlineStartMachine struct {
	stage         offlineStartStage
	stableTicks   int
	startedAt     time.Time
	stageAt       time.Time
	character     string
	expectedClass world.CharacterClass
	verifyClass   bool
}

type screenAnchorMismatchError struct {
	name       string
	difference float64
	maximum    float64
}

func (e *screenAnchorMismatchError) Error() string {
	return fmt.Sprintf("%s screen anchor mismatch: mean_difference=%.4f maximum=%.4f", e.name, e.difference, e.maximum)
}

func (m *offlineStartMachine) tick(now time.Time, state world.State) (offlineStartAction, bool, error) {
	if m.startedAt.IsZero() {
		m.startedAt, m.stageAt = now, now
	}
	if now.Sub(m.startedAt) >= offlineStartTimeout {
		return offlineStartNoAction, false, fmt.Errorf("offline game start timeout")
	}
	if now.Sub(m.stageAt) >= offlineStartStageTimeout {
		return offlineStartNoAction, false, fmt.Errorf("offline game start timeout in stage %d", m.stage)
	}
	switch m.stage {
	case offlineStartAwaitCharacter, offlineStartAwaitDifficulty:
		if state.Phase == world.GamePhaseLoading {
			m.stableTicks = 0
			return offlineStartNoAction, false, nil
		}
		if state.Phase != world.GamePhaseMenu || state.Valid {
			m.stableTicks = 0
			return offlineStartNoAction, false, nil
		}
		m.stableTicks++
		if m.stableTicks < offlineExitStableTicks {
			return offlineStartNoAction, false, nil
		}
		if m.stage == offlineStartAwaitCharacter {
			// Memory reaches `menu` before D2R has necessarily finished painting
			// the character screen after Save & Exit. Input remains fail-closed
			// until both the state and this render-settle window have elapsed.
			if now.Sub(m.stageAt) < offlineCharacterSettleDelay {
				return offlineStartNoAction, false, nil
			}
			return offlineStartVerifyCharacter, false, nil
		}
		return offlineStartVerifyDifficulty, false, nil
	case offlineStartAwaitGame:
		if state.Phase == world.GamePhaseLoading || !state.Valid {
			m.stableTicks = 0
			return offlineStartNoAction, false, nil
		}
		if state.Phase != world.GamePhaseInGame {
			return offlineStartNoAction, false, fmt.Errorf("offline game start expected in_game, got %s", state.Phase)
		}
		if !state.Identity.Valid || !strings.EqualFold(state.Identity.CharacterName, m.character) {
			m.stableTicks = 0
			return offlineStartNoAction, false, nil
		}
		if m.verifyClass && state.Identity.Class != m.expectedClass {
			return offlineStartNoAction, false, fmt.Errorf("offline game start expected class %s, got %s", m.expectedClass, state.Identity.Class)
		}
		if state.Area.ID != world.RogueEncampment {
			return offlineStartNoAction, false, fmt.Errorf("offline game start expected Rogue Encampment, got %s", state.Area.Name)
		}
		m.stableTicks++
		if m.stableTicks >= offlineExitStableTicks {
			m.stage = offlineStartComplete
			return offlineStartNoAction, true, nil
		}
		return offlineStartNoAction, false, nil
	case offlineStartComplete:
		return offlineStartNoAction, true, nil
	default:
		return offlineStartNoAction, false, fmt.Errorf("offline game start unknown stage %d", m.stage)
	}
}

func (m *offlineStartMachine) advance(stage offlineStartStage, now time.Time) {
	m.stage, m.stageAt, m.stableTicks = stage, now, 0
}

func parseOfflineDifficulty(raw string) (offlineDifficulty, error) {
	switch difficulty := offlineDifficulty(strings.ToLower(strings.TrimSpace(raw))); difficulty {
	case offlineDifficultyNormal, offlineDifficultyNightmare, offlineDifficultyHell:
		return difficulty, nil
	default:
		return "", fmt.Errorf("offline difficulty must be normal, nightmare, or hell, got %q", raw)
	}
}

func validateOfflineCharacter(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if !offlineCharacterNamePattern.MatchString(name) {
		return "", fmt.Errorf("offline character must contain only letters, digits, underscore, or hyphen, got %q", raw)
	}
	return name, nil
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
	x, y := offlineDifficultyPoint(difficulty)
	if x == 0 || y == 0 {
		return fmt.Errorf("offline difficulty selection: unsupported difficulty %q", difficulty)
	}
	return clickOfflinePoint(ctrl, x, y, string(difficulty)+" difficulty")
}

func clickOfflinePoint(ctrl offlineDifficultyController, x, y int, label string) error {
	if err := validateOfflineExitWindow(ctrl); err != nil {
		return err
	}
	if err := ctrl.Focus(); err != nil {
		return fmt.Errorf("focus before %s: %w", label, err)
	}
	if err := ctrl.MoveTo(x, y); err != nil {
		return fmt.Errorf("move to %s: %w", label, err)
	}
	if err := ctrl.Click(input.MouseLeft); err != nil {
		return fmt.Errorf("click %s: %w", label, err)
	}
	return nil
}

func verifyOfflineAnchor(ctrl offlineDifficultyController, anchors ...screenAnchor) (map[string]float64, error) {
	if err := validateOfflineExitWindow(ctrl); err != nil {
		return nil, err
	}
	if err := ctrl.Focus(); err != nil {
		return nil, fmt.Errorf("focus before screen verification: %w", err)
	}
	img, err := ctrl.CaptureClient()
	if err != nil {
		return nil, fmt.Errorf("capture before screen verification: %w", err)
	}
	scores := make(map[string]float64, len(anchors))
	for _, anchor := range anchors {
		score, err := matchScreenAnchor(img, anchor)
		if err != nil {
			return nil, err
		}
		scores[anchor.name] = score
		maximum := anchor.maximumMeanDifference()
		if score > maximum {
			return scores, &screenAnchorMismatchError{name: anchor.name, difference: score, maximum: maximum}
		}
		if anchor.requireSelectedBorder {
			borderMatched, borderErr := matchSelectedCharacterBorder(img, anchor)
			if borderErr != nil {
				return scores, borderErr
			}
			if !borderMatched {
				return scores, &screenAnchorMismatchError{name: anchor.name + "_selected_border", difference: 1, maximum: 0}
			}
		}
	}
	return scores, nil
}

func canceledInputOperationError(ctx context.Context, status input.Status, stopHotkey string) error {
	if status.Stopped {
		return fmt.Errorf("mit Not-Aus (%s) abgebrochen; starte die App neu und versuche es erneut", strings.ToUpper(stopHotkey))
	}
	return ctx.Err()
}

// RunOfflineDifficultyTest starts one offline game from the verified character
// screen and confirms the expected character in Rogue Encampment.
func (rt *Runtime) RunOfflineDifficultyTest(rawDifficulty string) error {
	err := rt.runOfflineDifficultyTest(context.Background(), rawDifficulty)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (rt *Runtime) runOfflineDifficultyTest(parent context.Context, rawDifficulty string) error {
	difficulty, err := parseOfflineDifficulty(rawDifficulty)
	if err != nil {
		return err
	}
	character, err := validateOfflineCharacter(rt.Options.OfflineCharacter)
	if err != nil {
		return err
	}
	return rt.runOfflineDifficultyForCharacter(parent, difficulty, character, 0, false, phase16CharacterAnchorRect)
}

func (rt *Runtime) runOfflineDifficultyForCharacter(parent context.Context, difficulty offlineDifficulty, character string, expectedClass world.CharacterClass, verifyClass bool, characterAnchorRect image.Rectangle) error {
	ctrl, ok := rt.Input.(offlineDifficultyController)
	if !ok {
		return fmt.Errorf("offline game start: controller lacks click or screenshot support")
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
	machine := &offlineStartMachine{character: character, expectedClass: expectedClass, verifyClass: verifyClass}
	playClicked, difficultyClicked := false, false
	characterSlug := strings.ToLower(character)
	characterAnchor := screenAnchor{
		name: "selected_character", path: rt.Config.ResolvePath(filepath.Join("ui", "characters", characterSlug+"-selected.png")), rect: characterAnchorRect,
		comparisonRegion: characterNameAnchorRegion, maxMeanDifference: characterNameAnchorMaxDifference,
		brightThreshold: characterNameAnchorBrightThreshold, brightShiftRadius: characterNameAnchorShiftRadius,
		requireSelectedBorder: true,
	}
	playAnchor := screenAnchor{name: "active_play", path: rt.Config.ResolvePath(filepath.Join("ui", "character-play.png")), rect: image.Rect(538, 624, 741, 671)}
	difficultyAnchor := screenAnchor{name: "difficulty_dialog", path: rt.Config.ResolvePath(filepath.Join("ui", "difficulty-dialog.png")), rect: image.Rect(550, 245, 730, 420)}

	rt.Log.Info("offline game start waiting for verified character screen", "difficulty", difficulty, "character", character)
	for {
		select {
		case <-ctx.Done():
			return canceledInputOperationError(ctx, ctrl.Status(), rt.Config.Input.StopHotkey)
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			// Selection only needs attach, window bind and world identity.
			// A full tick would run BindingsPrecheck against empty desktop dummy
			// bindings after the difficulty click enters InGame.
			if err := rt.pollQueueSnapshot(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("offline game start poll: %w", err)
			}
			rt.ensureVisibleInputWindow()
			action, done, err := machine.tick(time.Now(), rt.World.Current())
			if err != nil {
				return err
			}
			switch action {
			case offlineStartVerifyCharacter:
				if playClicked {
					return fmt.Errorf("offline game start invariant: Play already clicked")
				}
				scores, err := verifyOfflineAnchor(ctrl, characterAnchor, playAnchor)
				if err != nil {
					var mismatch *screenAnchorMismatchError
					if errors.As(err, &mismatch) {
						machine.stableTicks = 0
						rt.Log.Debug("offline character screen not visually stable yet", "error", err, "anchor_scores", scores)
						continue
					}
					return err
				}
				if err := clickOfflinePoint(ctrl, offlinePlayX, offlinePlayY, "Play"); err != nil {
					return err
				}
				playClicked = true
				machine.advance(offlineStartAwaitDifficulty, time.Now())
				rt.Log.Info("offline game start Play clicked", "anchor_scores", scores)
			case offlineStartVerifyDifficulty:
				if !playClicked || difficultyClicked {
					return fmt.Errorf("offline game start invariant: invalid difficulty click order")
				}
				scores, err := verifyOfflineAnchor(ctrl, difficultyAnchor)
				if err != nil {
					var mismatch *screenAnchorMismatchError
					if errors.As(err, &mismatch) {
						machine.stableTicks = 0
						rt.Log.Debug("offline difficulty dialog not visually stable yet", "error", err, "anchor_scores", scores)
						continue
					}
					return err
				}
				if err := selectOfflineDifficulty(ctrl, difficulty); err != nil {
					return err
				}
				difficultyClicked = true
				machine.advance(offlineStartAwaitGame, time.Now())
				rt.Log.Info("offline game start difficulty clicked", "difficulty", difficulty, "anchor_scores", scores)
			}
			if done {
				rt.Log.Info("offline game start completed", "difficulty", difficulty, "character", character, "play_clicks", boolCount(playClicked), "difficulty_clicks", boolCount(difficultyClicked))
				return nil
			}
		}
	}
}
