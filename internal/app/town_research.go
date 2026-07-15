package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

// RunTownInspect writes one read-only Town research report after a valid World snapshot.
// It waits for the same authoritative Stash/Waypoint anchors used by routing and
// never substitutes the short-lived diagnostic layout pin for direct observation.
func (rt *Runtime) RunTownInspect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("town inspect timeout waiting for unique Stash and Waypoint anchors")
			}
			return nil
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("town inspect poll: %w", err)
			}
			current := rt.World.Current()
			if !state.attached || !current.Valid || !current.Identity.Valid {
				continue
			}
			layout, layoutReason := town.InspectTownLayout(current)
			if layoutReason != "" {
				continue
			}
			artifact := struct {
				CapturedAt  time.Time                  `json:"captured_at"`
				GameVersion string                     `json:"game_version"`
				TownLayout  town.TownLayoutFingerprint `json:"town_layout"`
				Report      town.ResearchReport        `json:"report"`
			}{time.Now().UTC(), rt.Config.Memory.GameVersion, layout, town.Research(current)}
			data, err := json.MarshalIndent(artifact, "", "  ")
			if err != nil {
				return fmt.Errorf("encode town research: %w", err)
			}
			if err := os.MkdirAll(filepath.Join("diagnostics", "town"), 0o755); err != nil {
				return fmt.Errorf("create town diagnostics directory: %w", err)
			}
			path := filepath.Join("diagnostics", "town", "research-"+artifact.CapturedAt.Format("20060102T150405Z")+".json")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return fmt.Errorf("write town research report: %w", err)
			}
			pinPath := filepath.Join("diagnostics", "town", "layout-pin.json")
			if err := saveTownLayoutTestPin(pinPath, rt.Process.Status().PID, current, layout, time.Now()); err != nil {
				return fmt.Errorf("write Town layout test pin: %w", err)
			}
			rt.Log.Info("town research report written", "path", path, "town_layout", layout.Hash, "waypoint_dx", layout.WaypointDeltaX, "waypoint_dy", layout.WaypointDeltaY, "bulk_purchase_safe", artifact.Report.BulkPurchaseSafe, "bulk_reason", artifact.Report.BulkReason)
			rt.Log.Info("Town layout test pin saved", "town_layout", layout.Hash, "path", pinPath)
			return nil
		}
	}
}
