package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const storageInspectSamples = 12

type storageInspectArtifact struct {
	SchemaVersion         int                    `json:"schema_version"`
	CapturedAt            time.Time              `json:"captured_at"`
	Label                 string                 `json:"label"`
	ProcessID             uint32                 `json:"process_id"`
	GameVersion           string                 `json:"game_version"`
	VersionError          string                 `json:"version_error,omitempty"`
	ConfiguredGameVersion string                 `json:"configured_game_version"`
	Completion            string                 `json:"completion"`
	Samples               []storageInspectSample `json:"samples"`
	Notes                 []string               `json:"notes"`
}

type storageInspectWindow struct {
	At     time.Time `json:"at"`
	Anchor int       `json:"anchor_index"`
	Size   int       `json:"size"`
	Hex    string    `json:"hex,omitempty"`
	Error  string    `json:"error,omitempty"`
}

type storageInspectSample struct {
	At              time.Time                     `json:"at"`
	Generation      uint64                        `json:"snapshot_generation"`
	ReadEndedAt     time.Time                     `json:"read_ended_at"`
	PlayerUnitID    uint32                        `json:"player_unit_id"`
	Identity        memory.IdentityProbe          `json:"identity"`
	AreaID          uint32                        `json:"area_id"`
	UI              memory.UIState                `json:"ui"`
	Collection      memory.CollectionSnapshot     `json:"collection"`
	WorldCollection world.CollectionState         `json:"world_collection"`
	RawItems        []memory.ItemUnit             `json:"raw_items"`
	Items           []world.Item                  `json:"items"`
	ItemStats       []memory.ItemStatListEvidence `json:"item_stats"`
	ItemStatsAt     time.Time                     `json:"item_stats_at"`
	ItemStatsError  string                        `json:"item_stats_error,omitempty"`
	UIBefore        storageInspectWindow          `json:"ui_before"`
	Research        storageInspectWindow          `json:"ui_research"`
	UIAfter         storageInspectWindow          `json:"ui_after"`
	UIUnchanged     bool                          `json:"ui_unchanged"`
}

// ValidateStorageInspectOptions rejects unsafe mode combinations before runtime
// construction, including modes that otherwise return before app.New.
func ValidateStorageInspectOptions(opts Options) error {
	if opts.StorageInspect == "" {
		return nil
	}
	if !objectInspectLabelPattern.MatchString(opts.StorageInspect) {
		return fmt.Errorf("--storage-inspect: Label muss %s entsprechen", objectInspectLabelPattern.String())
	}
	if opts.StorageInspectTimeoutMs < 0 {
		return fmt.Errorf("--storage-inspect-timeout-ms darf nicht negativ sein")
	}
	if opts.Desktop || opts.Probe || opts.InputTest != "" || opts.Run != "" || opts.RunPhase != "" || opts.RuntimeTraceCapture != "" || opts.ReplayRuntimeTrace != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.ObjectInspect != "" || opts.ScreenAnchorCapture != "" || opts.SessionInspect || opts.RunsInspect || opts.WaypointTargetsInspect || opts.SessionMaxRuns != 0 || opts.Route != "" || opts.RouteName != "" || opts.RouteDifficulty != "" || opts.TownInspect || opts.TownTest != "" {
		return fmt.Errorf("--storage-inspect ist mit anderen Lauf- und Diagnosemodi nicht kombinierbar")
	}
	return nil
}

// RunStorageInspect captures twelve read-only samples of a manually prepared
// storage state. It uses neither the task loop nor the input controller.
// Ctrl+C cancels the bounded capture; incomplete research remains explicit.
func (rt *Runtime) RunStorageInspect(label string) error {
	opts := rt.Options
	opts.StorageInspect = label
	if err := ValidateStorageInspectOptions(opts); err != nil {
		return err
	}
	timeout := time.Duration(opts.StorageInspectTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ctx, stop := context.WithTimeout(ctx, timeout)
	defer stop()
	return rt.runStorageInspect(ctx, label, rt.Config.ResolvePath(filepath.Join("diagnostics", "storage")))
}

func (rt *Runtime) runStorageInspect(ctx context.Context, label, directory string) error {
	if rt.Process == nil || rt.Probe == nil || rt.UIProbe == nil || rt.ObjectInspect == nil || rt.World == nil {
		return fmt.Errorf("Materialtruhe-Diagnose: Reader nicht verfügbar")
	}
	defer func() {
		if err := rt.Process.Detach(); err != nil {
			rt.Log.Warn("process detach failed", "error", err)
		}
	}()
	artifact := storageInspectArtifact{
		SchemaVersion: 2, CapturedAt: time.Now().UTC(), Label: label,
		ConfiguredGameVersion: rt.Config.Memory.GameVersion,
		Samples:               make([]storageInspectSample, 0, storageInspectSamples),
		Notes: []string{
			"Read-only Gate 25.0. Keine Tastatur-, Maus-, Task- oder Bindungsaktionen.",
			"Item-, Stat- und UI-Reads folgen nacheinander; ui_unchanged belegt ausschließlich gleiche UI-Bytes, keinen atomaren Collection-Snapshot.",
			"UI-Forschungsfenster: UI-0x8000 bis UI+0x7fff. Unvalidierte Bytes autorisieren keinen Input.",
			"Gate 25.2: Gem-/Skull-Zähler stammen aus der zweimal gelesenen Collection-Inventarinstanz. Aktiver Tab und Runen-Counts bleiben unbekannt. Das Würfelraster gehört zur offenen Truhe, wenn ein Cube im persönlichen Inventar liegt.",
			"Der vorhandene Item-Walk ist begrenzt und überspringt unlesbare Units. Eine leere Itemliste beweist keinen leeren Materialslot.",
		},
	}
	ticker := time.NewTicker(time.Duration(max(1, rt.Config.Runtime.PollIntervalMs)) * time.Millisecond)
	defer ticker.Stop()
	attached := false
	rt.Log.Info("storage inspect started", "label", label, "input", "disabled")
	for len(artifact.Samples) < storageInspectSamples {
		select {
		case <-ctx.Done():
			if len(artifact.Samples) == 0 {
				return fmt.Errorf("Materialtruhe-Diagnose ohne In-Game-Sample: %w", ctx.Err())
			}
			artifact.Completion = "partial_cancelled"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				artifact.Completion = "partial_timeout"
			}
			return rt.publishStorageInspect(directory, artifact)
		case <-ticker.C:
			if !attached {
				if err := rt.Process.Attach(ctx); err != nil {
					if process.IsFatal(err) {
						return fmt.Errorf("Materialtruhe-Diagnose Attach: %w", err)
					}
					continue
				}
				attached = true
			}
			status := rt.Process.Poll()
			if status.State != process.StateAttached {
				return fmt.Errorf("Materialtruhe-Diagnose: Prozessbindung verloren")
			}
			if len(artifact.Samples) > 0 && artifact.ProcessID != status.PID {
				return fmt.Errorf("Materialtruhe-Diagnose: Prozess gewechselt")
			}
			artifact.ProcessID, artifact.GameVersion, artifact.VersionError = status.PID, status.FileVersion, status.VersionError
			snap := rt.Probe.Snapshot()
			if !snap.Valid || snap.Phase != memory.GamePhaseInGame {
				continue
			}
			rt.World.Update(snap)
			artifact.Samples = append(artifact.Samples, rt.collectStorageInspectSample(snap))
		}
	}
	artifact.Completion = "complete"
	return rt.publishStorageInspect(directory, artifact)
}

func storageWindow(capture memory.UIBufferCapture, err error) storageInspectWindow {
	if err != nil {
		return storageInspectWindow{Error: err.Error()}
	}
	return storageInspectWindow{At: capture.At, Anchor: capture.Anchor, Size: len(capture.Bytes), Hex: hex.EncodeToString(capture.Bytes)}
}

func (rt *Runtime) collectStorageInspectSample(snap memory.Snapshot) storageInspectSample {
	before, beforeErr := rt.UIProbe.CaptureUIBuffer()
	research, researchErr := rt.UIProbe.CaptureUIResearchBuffer()
	stats, statsErr := rt.ObjectInspect.CollectItemStatListEvidence()
	statsAt := time.Now().UTC()
	after, afterErr := rt.UIProbe.CaptureUIBuffer()
	sample := storageInspectSample{
		At: snap.At, Generation: snap.Generation, ReadEndedAt: time.Now().UTC(),
		PlayerUnitID: snap.PlayerUnitID, Identity: snap.Identity, AreaID: snap.AreaID,
		UI: snap.UI, RawItems: snap.Items, Items: rt.World.Current().Items,
		Collection: snap.Collection, WorldCollection: rt.World.Current().Collection,
		ItemStats: stats, ItemStatsAt: statsAt,
		UIBefore: storageWindow(before, beforeErr), Research: storageWindow(research, researchErr), UIAfter: storageWindow(after, afterErr),
		UIUnchanged: beforeErr == nil && afterErr == nil && len(before.Bytes) > 0 && before.Anchor == after.Anchor && bytes.Equal(before.Bytes, after.Bytes),
	}
	if statsErr != nil {
		sample.ItemStatsError = statsErr.Error()
	}
	// Freeze diagnostic data now; the readers may reuse buffers on the next poll.
	return cloneStorageInspectSample(sample)
}

func cloneStorageInspectSample(sample storageInspectSample) storageInspectSample {
	// JSON contains only serializable value types and owns nested stats/maps too.
	// Explicit copying below avoids introducing a generic artifact framework.
	sample.RawItems = append([]memory.ItemUnit(nil), sample.RawItems...)
	sample.Collection.Counts = append([]memory.CollectionCount(nil), sample.Collection.Counts...)
	sample.WorldCollection.Counts = append([]world.MaterialCount(nil), sample.WorldCollection.Counts...)
	for i := range sample.RawItems {
		sample.RawItems[i].Stats = append([]memory.RawStat(nil), sample.RawItems[i].Stats...)
	}
	sample.Items = append([]world.Item(nil), sample.Items...)
	for i := range sample.Items {
		sample.Items[i].Stats = append([]world.ItemStat(nil), sample.Items[i].Stats...)
	}
	sample.ItemStats = append([]memory.ItemStatListEvidence(nil), sample.ItemStats...)
	for i := range sample.ItemStats {
		sample.ItemStats[i].Active = append([]memory.RawStat(nil), sample.ItemStats[i].Active...)
		sample.ItemStats[i].Base = append([]memory.RawStat(nil), sample.ItemStats[i].Base...)
	}
	return sample
}

func (rt *Runtime) publishStorageInspect(directory string, artifact storageInspectArtifact) error {
	path, err := saveStorageInspectArtifact(directory, artifact)
	if err != nil {
		return err
	}
	rt.Log.Info("storage inspect written", "path", path, "label", artifact.Label, "samples", len(artifact.Samples), "completion", artifact.Completion, "game_version", artifact.GameVersion)
	return nil
}

func saveStorageInspectArtifact(directory string, artifact storageInspectArtifact) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("Materialtruhe-Diagnose Verzeichnis: %w", err)
	}
	path := filepath.Join(directory, artifact.CapturedAt.Format("20060102T150405.000000000Z")+"-"+artifact.Label+".json")
	tmp, err := os.CreateTemp(directory, ".storage-inspect-*.tmp")
	if err != nil {
		return "", fmt.Errorf("Materialtruhe-Diagnose Staging: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		return "", fmt.Errorf("Materialtruhe-Diagnose JSON: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("Materialtruhe-Diagnose Flush: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("Materialtruhe-Diagnose Close: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", fmt.Errorf("Materialtruhe-Diagnose Publish: %w", err)
	}
	return path, nil
}
