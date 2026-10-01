package replay

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestCollectionStorageRoundTripAndUnknownEvidence(t *testing.T) {
	now := time.Unix(100, 0)
	state := world.State{At: now, Generation: 9, Valid: true, Phase: world.GamePhaseInGame, UI: world.UIState{StashOpen: true}, Collection: world.CollectionState{ScopeID: "opaque-live-instance", ScopeKnown: true, Generation: 9, ObservedAt: now, Counts: []world.MaterialCount{{Code: "gsr", Count: 22, Known: true}, {Code: "r01", Known: false}}}}
	projection := NormalizeWorld(state)
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateSafeJSON(encoded); err != nil {
		t.Fatal(err)
	}
	var decoded WorldFrame
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	result := worldStateFromFrame(Frame{Generation: 9, World: decoded}, now)
	if !reflect.DeepEqual(result.Collection, state.Collection) {
		t.Fatalf("roundtrip = %+v, want %+v", result.Collection, state.Collection)
	}
	projection.Collection.Counts[0].Count = 99
	if state.Collection.Counts[0].Count != 22 {
		t.Fatal("projection alias")
	}
	if _, known := result.CollectionCount("r01"); known || result.Collection.TabKnown {
		t.Fatal("unknown evidence became known")
	}
	for _, tab := range []world.StorageTab{"basic", "materials", world.StorageTabGems, world.StorageTabRunes} {
		state.Collection.Tab, state.Collection.TabKnown = tab, true
		decoded := worldStateFromFrame(Frame{Generation: 9, World: NormalizeWorld(state)}, now)
		if !decoded.Collection.TabKnown || decoded.Collection.Tab != tab {
			t.Fatalf("known navigation tab lost in replay: %+v", decoded.Collection)
		}
	}
	for _, change := range []func(*Frame){func(f *Frame) { f.World.Collection = CollectionFrame{} }, func(f *Frame) { f.Generation++ }, func(f *Frame) { f.World.UI.StashOpen = false }, func(f *Frame) { f.World.Collection.Counts[0].Count = 100 }} {
		f := Frame{Generation: 9, World: NormalizeWorld(state)}
		change(&f)
		if worldStateFromFrame(f, now).CollectionAvailable() {
			t.Fatal("invalid trace evidence available")
		}
	}
	state.Generation++
	if NormalizeWorld(state).Collection.ScopeKnown {
		t.Fatal("stale state normalized as known")
	}
}

func TestCollectionStorageSchemaTwoAndOldTraceUnknown(t *testing.T) {
	bundle := Bundle{SchemaVersion: 1, Contract: ContractSnapshot{RunID: "mephisto"}, Frames: []Frame{{Tick: 1, World: WorldFrame{Phase: "in_game", Valid: true}}}}
	if err := bundle.Validate(); err != nil {
		t.Fatal(err)
	}
	if worldStateFromFrame(bundle.Frames[0], time.Now()).CollectionAvailable() {
		t.Fatal("old trace invented storage")
	}
	bundle.Frames[0].World.Collection.ScopeKnown = true
	if err := bundle.Validate(); err == nil {
		t.Fatal("schema 1 accepted collection")
	}
	bundle.SchemaVersion = SchemaVersion
	if err := bundle.Validate(); err != nil {
		t.Fatal(err)
	}
	bundle.SchemaVersion = SchemaVersion + 1
	if err := bundle.Validate(); err == nil {
		t.Fatal("future schema accepted")
	}
}
