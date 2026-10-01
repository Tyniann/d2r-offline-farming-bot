package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type storageResearchMock struct {
	buffer []byte
	stats  []memory.ItemStatListEvidence
	err    error
}

func (m *storageResearchMock) CaptureUIBuffer() (memory.UIBufferCapture, error) {
	return memory.UIBufferCapture{At: time.Now(), Anchor: 1, Bytes: m.buffer}, nil
}
func (m *storageResearchMock) CaptureUIResearchBuffer() (memory.UIBufferCapture, error) {
	return memory.UIBufferCapture{At: time.Now(), Anchor: 1, Bytes: m.buffer}, m.err
}
func (m *storageResearchMock) CollectItemStatListEvidence() ([]memory.ItemStatListEvidence, error) {
	return m.stats, m.err
}
func (m *storageResearchMock) CollectObjectInspectEvidence() ([]memory.ObjectInspectEvidence, error) {
	panic("object walk must not run")
}

func TestStorageInspectOptionsAreIsolated(t *testing.T) {
	cfg := fullCountessConfig(t)
	cfg.Runs.Active = "countess"
	for _, label := range []string{"gems-cube-closed", "char-b-runes-99"} {
		opts := Options{StorageInspect: label}
		if err := ValidateStorageInspectOptions(opts); err != nil {
			t.Fatal(err)
		}
		if resolveActiveRun(opts, cfg) != "" || SessionExecutionRequested(opts) || CharacterLoadoutRequired(opts) {
			t.Fatal("diagnostic selected a run, queue, or loadout")
		}
		if err := validateRunMode(resolveRunSelection(opts, cfg), cfg, opts, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
			t.Fatal(err)
		}
	}
	for _, label := range []string{"../gems", "Gems", "", "gems_99", strings.Repeat("x", 65)} {
		if label == "" {
			continue
		} // An absent flag means this mode was not selected.
		if err := ValidateStorageInspectOptions(Options{StorageInspect: label}); err == nil {
			t.Fatalf("accepted label %q", label)
		}
	}
	for _, conflicting := range []Options{
		{Desktop: true}, {Probe: true}, {Run: "cows"}, {RunPhase: "stash-personal"}, {InputTest: "center-click"},
		{PathingTest: "hover:watch"}, {OfflineDifficulty: "hell"}, {OfflineCharacter: "test"}, {OfflineExitTest: true},
		{UIStateProbe: "gems"}, {CowProbe: "gems"}, {MercenaryProbe: "alive"}, {WeaponSetProbe: "primary-secondary"},
		{ObjectInspect: "closed"}, {ScreenAnchorCapture: "gems"}, {RuntimeTraceCapture: "gems"}, {ReplayRuntimeTrace: "trace.gz"},
		{SessionInspect: true}, {RunsInspect: true}, {WaypointTargetsInspect: true}, {SessionMaxRuns: 1},
		{Route: "list"}, {RouteName: "test"}, {RouteDifficulty: "hell"}, {TownInspect: true}, {TownTest: "stash"},
		{StorageInspectTimeoutMs: -1},
	} {
		conflicting.StorageInspect = "gems"
		if err := ValidateStorageInspectOptions(conflicting); err == nil {
			t.Fatalf("accepted conflict %+v", conflicting)
		}
	}
}

func TestStorageInspectRunsWithoutInputOrTasksAndPublishesActualVersion(t *testing.T) {
	cfg := fullCountessConfig(t)
	cfg.Runtime.PollIntervalMs = 1
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	snap := validSnapshot(100)
	snap.At, snap.Generation = time.Now(), 7
	snap.Items = []memory.ItemUnit{{UnitID: 809, RawLocation: 0, PlayerOwned: true, GridX: 9, GridY: 2, Quantity: 99, QuantityKnown: true}}
	research := &storageResearchMock{buffer: []byte{0, 1, 0}, err: errors.New("research unreadable")}
	rt := &Runtime{Config: cfg, Log: log, World: world.NewModel(log), Probe: &mockProbe{snap: snap}, UIProbe: research, ObjectInspect: research,
		Process: &mockProcess{pollStatus: process.Status{State: process.StateAttached, PID: 42, FileVersion: "actual-version"}}}
	// Nil Input and Tasks would panic if the productive tick or binding path ran.
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rt.runStorageInspect(ctx, "gems-99", dir); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("artifacts=%v err=%v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var artifact storageInspectArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != 2 || artifact.ProcessID != 42 || artifact.GameVersion != "actual-version" || artifact.ConfiguredGameVersion != cfg.Memory.GameVersion || artifact.Completion != "complete" || len(artifact.Samples) != 12 {
		t.Fatalf("metadata=%+v", artifact)
	}
	first := artifact.Samples[0]
	if first.Generation != 7 || first.RawItems[0].UnitID != 809 || first.RawItems[0].Quantity != 99 || first.Research.Error == "" || first.Research.Hex != "" || first.ItemStatsError == "" || !first.UIUnchanged {
		t.Fatalf("sample=%+v", first)
	}
	for _, forbidden := range []string{"collection_counts", "storage_scope", "active_material_tab"} {
		if strings.Contains(string(data), "\""+forbidden+"\"") {
			t.Fatalf("invented collection evidence %s", forbidden)
		}
	}
	temps, _ := filepath.Glob(filepath.Join(dir, ".storage-inspect-*.tmp"))
	if len(temps) != 0 {
		t.Fatalf("unpublished temporary files=%v", temps)
	}
}

func TestStorageInspectFreezesNestedEvidence(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	research := &storageResearchMock{buffer: []byte{1}, stats: []memory.ItemStatListEvidence{{UnitID: 9, BaseReadable: true, Base: []memory.RawStat{{ID: 70, Value: 1}}}}}
	snap := validSnapshot(100)
	snap.Items = []memory.ItemUnit{{UnitID: 9, Stats: []memory.RawStat{{ID: 70, Value: 1}}}}
	snap.Generation = 1
	snap.UI.StashOpen = true
	for _, entry := range world.ItemCatalogEntries() {
		if entry.Code == "gsr" {
			snap.Collection = memory.CollectionSnapshot{ScopeID: "fixture", Available: true, Generation: 1, Counts: []memory.CollectionCount{{TxtFileNo: entry.TxtFileNo, UnitID: 142, Count: 22}}}
			break
		}
	}
	rt := &Runtime{World: world.NewModel(log), UIProbe: research, ObjectInspect: research}
	rt.World.Update(snap)
	got := rt.collectStorageInspectSample(snap)
	research.buffer[0] = 2
	research.stats[0].Base[0].Value = 2
	snap.Items[0].Stats[0].Value = 2
	snap.Collection.Counts[0].Count = 99
	if got.Collection.Counts[0].Count != 22 || got.WorldCollection.Counts[0].Count != 22 {
		t.Fatal("collection sample alias")
	}
	if got.UIBefore.Hex != "01" || got.Research.Hex != "01" || got.ItemStats[0].Base[0].Value != 1 || got.RawItems[0].Stats[0].Value != 1 {
		t.Fatalf("mutated sample=%+v", got)
	}
	if w := storageWindow(memory.UIBufferCapture{}, errors.New("unreadable")); w.Error == "" || w.Size != 0 || w.Hex != "" {
		t.Fatalf("unreadable window=%+v", w)
	}
}

func TestStorageInspectCancellationWithoutSampleFails(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := fullCountessConfig(t)
	research := &storageResearchMock{}
	rt := &Runtime{Config: cfg, Log: log, World: world.NewModel(log), Probe: &mockProbe{}, UIProbe: research, ObjectInspect: research, Process: &mockProcess{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if err := rt.runStorageInspect(ctx, "closed", dir); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatal("published a capture with no samples")
	}
}
