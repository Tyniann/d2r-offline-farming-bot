package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const screenAnchorCaptureTimeout = 30 * time.Second

type screenCaptureController interface {
	inputController
	CaptureClient() (*image.RGBA, error)
}

// RunScreenAnchorCapture stores one named, read-only 1280x720 D2R client
// screenshot for narrow Phase-7.3 frontend-anchor calibration.
func (rt *Runtime) RunScreenAnchorCapture(label string) error {
	if err := validateUIStateProbeLabel(label); err != nil {
		return fmt.Errorf("screen anchor label: %w", err)
	}
	ctrl, ok := rt.Input.(screenCaptureController)
	if !ok {
		return fmt.Errorf("screen anchor capture: controller lacks screenshot support")
	}
	ctx, cancel := context.WithTimeout(context.Background(), screenAnchorCaptureTimeout)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer rt.Process.Detach()
	defer rt.Input.Unbind()
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	stableMenuTicks := 0

	rt.Log.Info("screen anchor capture waiting for stable frontend",
		"label", label,
		"required_client_width", offlineDifficultyClientWidth,
		"required_client_height", offlineDifficultyClientHeight,
	)
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("screen anchor capture timeout waiting for stable frontend")
			}
			return nil
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("screen anchor capture poll: %w", err)
			}
			cur := rt.World.Current()
			if !state.attached || !rt.Input.Bound() || cur.Phase != world.GamePhaseMenu || cur.Valid {
				stableMenuTicks = 0
				continue
			}
			stableMenuTicks++
			if stableMenuTicks < offlineExitStableTicks {
				continue
			}
			if err := validateOfflineExitWindow(ctrl); err != nil {
				return err
			}
			if err := ctrl.Focus(); err != nil {
				return fmt.Errorf("screen anchor capture focus: %w", err)
			}
			img, err := ctrl.CaptureClient()
			if err != nil {
				return fmt.Errorf("screen anchor capture: %w", err)
			}
			path, err := saveScreenAnchorPNG(filepath.Join("diagnostics", "screen-anchors"), label, time.Now().UTC(), img)
			if err != nil {
				return err
			}
			rt.Log.Info("screen anchor capture published", "label", label, "path", path, "width", img.Bounds().Dx(), "height", img.Bounds().Dy())
			return nil
		}
	}
}

func saveScreenAnchorPNG(directory, label string, at time.Time, img image.Image) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create screen anchor directory: %w", err)
	}
	path := filepath.Join(directory, fmt.Sprintf("%s-%s.png", at.Format("20060102T150405.000000000Z"), label))
	tmp, err := os.CreateTemp(directory, ".screen-anchor-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary screen anchor: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := png.Encode(tmp, img); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("encode screen anchor: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush screen anchor: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close screen anchor: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish screen anchor: %w", err)
	}
	return path, nil
}
