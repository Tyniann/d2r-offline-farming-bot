package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var phase16CharacterAnchorRect = image.Rect(1035, 48, 1245, 108)

const characterCaptureMinimumGoldPixels = 80

// CaptureCharacterSelectionAnchor fokussiert ausschließlich das kompatible D2R-Fenster, prüft den Charakterbildschirm und schreibt den markierten 210×60-Beleg.
func (rt *Runtime) CaptureCharacterSelectionAnchor(parent context.Context, targetPath string) error {
	if validPNGSize(targetPath, phase16CharacterAnchorSize) {
		return &CharacterSetupError{Code: string(Phase16ReasonCharacterAnchorExists), Err: fmt.Errorf("valid character anchor already exists")}
	}
	ctrl, ok := rt.Input.(offlineDifficultyController)
	if !ok {
		return fmt.Errorf("character capture controller lacks screenshot support")
	}
	ctx, cancel := context.WithTimeout(parent, screenAnchorCaptureTimeout)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	defer func() {
		rt.Input.Unbind()
		_ = rt.Process.Detach()
	}()
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	stableMenuTicks := 0
	focused := false
	captureAfter := time.Time{}
	play := screenAnchor{name: "active_play", path: rt.Config.ResolvePath("ui/character-play.png"), rect: image.Rect(538, 624, 741, 671)}
	dialog := screenAnchor{name: "difficulty_dialog", path: rt.Config.ResolvePath("ui/difficulty-dialog.png"), rect: image.Rect(550, 245, 730, 420)}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("character capture poll: %w", err)
			}
			current := rt.World.Current()
			if current.Phase != world.GamePhaseMenu || current.Valid || !rt.Input.Bound() {
				stableMenuTicks = 0
				continue
			}
			stableMenuTicks++
			if stableMenuTicks < offlineExitStableTicks {
				continue
			}
			if err := validateOfflineExitWindow(ctrl); err != nil {
				return fmt.Errorf("character capture window: %w", err)
			}
			status := ctrl.Status()
			if !status.Enabled || status.Paused || status.Stopped {
				return fmt.Errorf("character capture requires enabled, unpaused input")
			}
			if !focused {
				rt.Log.Info("character selection anchor focus requested", "reason", "capture_marked_offline_character")
				if err := ctrl.Focus(); err != nil {
					return fmt.Errorf("character capture focus: %w", err)
				}
				focused = true
				captureAfter = time.Now().Add(characterSelectionSettle)
				continue
			}
			if time.Now().Before(captureAfter) {
				continue
			}
			capture, err := ctrl.CaptureClient()
			if err != nil {
				return fmt.Errorf("character capture screenshot: %w", err)
			}
			if err := verifyCharacterScreenCapture(capture, play, dialog); err != nil {
				return err
			}
			selectedRect, selectionErr := selectedCharacterCaptureRect(capture)
			if selectionErr != nil {
				return selectionErr
			}
			crop := capture.SubImage(selectedRect)
			if err := writeCharacterAnchorPNG(targetPath, crop); err != nil {
				return err
			}
			if !validPNGSize(targetPath, phase16CharacterAnchorSize) {
				return fmt.Errorf("character capture PNG re-read failed")
			}
			rt.Log.Info("character selection anchor published", "file", filepath.Base(targetPath), "width", 210, "height", 60)
			return nil
		}
	}
}

func selectedCharacterCaptureRect(capture image.Image) (image.Rectangle, error) {
	selected, mostGold := image.Rectangle{}, 0
	for row := 0; row < characterSelectionVisibleRows; row++ {
		rect := characterSelectionRowRect(row)
		if rect.In(capture.Bounds()) {
			if gold := countCharacterBorderGold(capture, rect); gold > mostGold {
				selected, mostGold = rect, gold
			}
		}
	}
	if selected.Empty() || mostGold < characterCaptureMinimumGoldPixels {
		return image.Rectangle{}, fmt.Errorf("der markierte Charakter konnte nicht eindeutig gefunden werden; markiere ihn in D2R und versuche es erneut")
	}
	return selected, nil
}

func writeCharacterAnchorPNG(path string, value image.Image) error {
	if value == nil || value.Bounds().Dx() != phase16CharacterAnchorSize.X || value.Bounds().Dy() != phase16CharacterAnchorSize.Y {
		return fmt.Errorf("character anchor must be exactly 210x60")
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, value); err != nil {
		return fmt.Errorf("encode character anchor: %w", err)
	}
	if err := writeAtomicYAML(path, encoded.Bytes(), "character-anchor"); err != nil {
		return fmt.Errorf("publish character anchor: %w", err)
	}
	return nil
}
