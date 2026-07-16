package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const routeRecordSampleDistance = 4.0

// RunRouteRecord observes manual navigation until the Stop hotkey and publishes a valid route.
func (rt *Runtime) RunRouteRecord(id, name, difficultyLabel string) error {
	return rt.runRouteRecord(id, name, difficultyLabel, pathing.RouteMovementTeleport, "", 0)
}

func (rt *Runtime) runRouteRecord(id, name, difficultyLabel string, movement pathing.RouteMovement, directory string, expectedArea world.AreaID) error {
	if err := pathing.ValidateRouteID(id); err != nil {
		return err
	}
	difficulty, err := parseOfflineDifficulty(difficultyLabel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name = routeDisplayName(id)
	}
	if strings.TrimSpace(rt.Config.Memory.GameVersion) == "" {
		return fmt.Errorf("route recording requires memory.game_version")
	}
	recorder, err := pathing.NewRouteRecorder(pathing.RouteRecorderConfig{SampleDistanceTiles: routeRecordSampleDistance, Movement: movement})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("detach after route recording", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	var startFingerprint pathing.LayoutFingerprint
	recordedAt := time.Now().UTC()
	stopRequested := false

	rt.Log.Info("route recording waiting for confirmed in-game identity and stable start anchor",
		"route_id", id, "difficulty", difficulty, "sample_distance_tiles", routeRecordSampleDistance,
		"stop_hotkey", rt.Config.Input.StopHotkey,
	)
	for {
		select {
		case <-ctx.Done():
			if !stopRequested {
				return fmt.Errorf("route recording cancelled before operator Stop: %w", ctx.Err())
			}
			return rt.finishRouteRecording(recorder, id, name, pathing.RouteDifficulty(difficulty), recordedAt, startFingerprint, directory, expectedArea)
		case event := <-hotkeys:
			if event.Action == input.HotkeyActionStop {
				stopRequested = true
			}
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if state.hasEverAttached && !state.attached {
				return fmt.Errorf("route recording: process lost")
			}
			if rt.Input.Status().Paused {
				continue
			}
			cur := rt.World.Current()
			if startFingerprint.Hash == "" && cur.Valid && cur.Phase == world.GamePhaseInGame && cur.Identity.Valid {
				if expectedArea != 0 && cur.Area.ID != expectedArea {
					continue
				}
				fingerprint, err := pathing.BuildLayoutFingerprint(cur)
				if errors.Is(err, pathing.ErrLayoutAnchorsUnavailable) {
					continue
				}
				if err != nil {
					return fmt.Errorf("route recording start fingerprint: %w", err)
				}
				startFingerprint = fingerprint
				rt.Log.Info("route recording started", "route_id", id, "character", cur.Identity.CharacterName, "area_id", cur.Area.ID, "layout_fingerprint", fingerprint.Hash)
			}
			if startFingerprint.Hash == "" {
				continue
			}
			if expectedArea != 0 && cur.Area.ID != expectedArea {
				return fmt.Errorf("route recording left required area: got %d want %d", cur.Area.ID, expectedArea)
			}
			event, err := recorder.Observe(cur)
			if err != nil {
				return fmt.Errorf("route recording observe: %w", err)
			}
			if event.SampleAccepted {
				rt.Log.Info("route sample accepted", "route_id", id, "area_id", event.AreaID, "pos_x", event.Position.X, "pos_y", event.Position.Y)
			}
			if event.SegmentComplete {
				rt.Log.Info("route segment completed", "route_id", id, "segment_id", event.Segment.ID, "from_area_id", event.Segment.FromAreaID, "to_area_id", event.Segment.ToAreaID, "points", len(event.Segment.Points), "entrance_kind", event.Segment.Transition.EntranceKind)
			}
		}
	}
}

func (rt *Runtime) finishRouteRecording(recorder *pathing.RouteRecorder, id, name string, difficulty pathing.RouteDifficulty, recordedAt time.Time, fingerprint pathing.LayoutFingerprint, directory string, expectedArea world.AreaID) error {
	segments, err := recorder.Finish()
	if err != nil {
		return fmt.Errorf("route recording not published: %w", err)
	}
	identity := recorder.Identity()
	if expectedArea == 0 {
		directory, err = farmingRouteDirectory(rt.Config, identity.CharacterName, string(difficulty))
		if err != nil {
			return fmt.Errorf("route recording directory: %w", err)
		}
	}
	seed := identity.MapSeed
	route := pathing.Route{
		Version: pathing.RouteVersion, ID: id, Name: name, Kind: pathing.RouteKindNavigation,
		Binding:   pathing.RouteBinding{CharacterName: identity.CharacterName, CharacterClass: identity.Class.String(), Difficulty: difficulty, MapSeed: &seed, GameVersion: rt.Config.Memory.GameVersion, LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash}},
		Recording: pathing.RouteRecording{RecordedAt: recordedAt, SampleDistanceTiles: routeRecordSampleDistance},
		Playback:  pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments:  segments,
	}
	if expectedArea == world.KurastDocks {
		if err := validateAct3EgressRoute(route); err != nil {
			return fmt.Errorf("route recording not published: %w", err)
		}
	}
	path := filepath.Join(directory, id+".yaml")
	if err := pathing.SaveRoute(path, route); err != nil {
		return fmt.Errorf("route recording publish: %w", err)
	}
	if expectedArea == 0 {
		lifecycle, err := NewRouteLifecycleStore(rt.Config)
		if err != nil {
			return fmt.Errorf("route recording lifecycle: %w", err)
		}
		if _, err := lifecycle.RecordRoute(path); err != nil {
			return fmt.Errorf("route recording lifecycle commit: %w", err)
		}
	}
	rt.Log.Info("route recording published", "route_id", id, "path", path, "segments", len(segments), "character", identity.CharacterName, "difficulty", difficulty, "layout_fingerprint", fingerprint.Hash)
	return nil
}

func routeDisplayName(id string) string {
	words := strings.Fields(strings.ReplaceAll(id, "-", " "))
	for i := range words {
		words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
	}
	return strings.Join(words, " ")
}
