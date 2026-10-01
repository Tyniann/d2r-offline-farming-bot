package world

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func capturedRubyID(t *testing.T) uint32 {
	t.Helper()
	data, err := os.ReadFile("../memory/testdata/collection-gems-stock.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Item string `json:"item_hex"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	unit, err := hex.DecodeString(fixture.Item)
	if err != nil {
		t.Fatal(err)
	}
	id := binary.LittleEndian.Uint32(unit[4:])
	if LookupItemCode(id) != "gsr" {
		t.Fatal("captured Ruby must match local CASC catalog")
	}
	return id
}

func collectionSnapshot(t *testing.T) memory.Snapshot {
	t.Helper()
	return memory.Snapshot{At: time.Unix(100, 0), Generation: 8, Valid: true, Phase: memory.GamePhaseInGame, UI: memory.UIState{StashOpen: true}, Collection: memory.CollectionSnapshot{Available: true, ScopeID: "live-instance", Generation: 8, Counts: []memory.CollectionCount{{TxtFileNo: capturedRubyID(t), UnitID: 142, Count: 22}}}}
}

func TestCollectionStorageMapsCapturedStockAndUnknownSlots(t *testing.T) {
	for _, value := range []uint32{22, 0, 1, 98, 99} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			snap := collectionSnapshot(t)
			snap.Collection.Counts[0].Count = value
			state := FromSnapshot(snap)
			count, known := state.CollectionCount("gsr")
			if !known || count != int(value) {
				t.Fatalf("Ruby = %d/%t, want %d/true", count, known, value)
			}
			if _, known = state.CollectionCount("glr"); known {
				t.Fatal("missing slot became known zero")
			}
			if state.Collection.TabKnown {
				t.Fatal("unproved UI became known")
			}
		})
	}
	// The concrete Gate 25.4 capture covers all seven gem/skull families and
	// the observed rune source. Ordinary Item.Quantity remains unrelated.
	data, err := os.ReadFile("testdata/phase25/collection-stock.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Counts []struct {
			memory.CollectionCount
			Code string `json:"code"`
		}
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	snap := collectionSnapshot(t)
	snap.Collection.Tab, snap.Collection.TabKnown = "gems", true
	snap.Collection.Counts = nil
	for _, entry := range fixture.Counts {
		if LookupItemCode(entry.TxtFileNo) != entry.Code {
			t.Fatalf("capture code %s disagrees with local CASC catalog", entry.Code)
		}
		snap.Collection.Counts = append(snap.Collection.Counts, entry.CollectionCount)
		snap.Items = append(snap.Items, memory.ItemUnit{TxtFileNo: entry.TxtFileNo, Quantity: 99, QuantityKnown: true})
	}
	state := FromSnapshot(snap)
	if !state.Collection.TabKnown || state.Collection.Tab != StorageTabGems {
		t.Fatal("captured tab was not mapped")
	}
	for _, entry := range fixture.Counts {
		if got, known := state.CollectionCount(entry.Code); !known || got != int(entry.Count) {
			t.Fatalf("%s=%d/%t, want %d/true", entry.Code, got, known, entry.Count)
		}
	}
}

func TestCollectionStorageRevokesInvalidAndStaleSnapshots(t *testing.T) {
	cases := []struct {
		name   string
		change func(*memory.Snapshot)
	}{
		{"invalid", func(s *memory.Snapshot) { s.Valid = false }},
		{"loading", func(s *memory.Snapshot) { s.Phase = memory.GamePhaseLoading }},
		{"closed", func(s *memory.Snapshot) { s.UI.StashOpen = false }},
		{"missing", func(s *memory.Snapshot) { s.Collection = memory.CollectionSnapshot{} }},
		{"stale", func(s *memory.Snapshot) { s.Collection.Generation-- }},
		{"range", func(s *memory.Snapshot) { s.Collection.Counts[0].Count = 100 }},
		{"duplicate", func(s *memory.Snapshot) { s.Collection.Counts = append(s.Collection.Counts, s.Collection.Counts[0]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := collectionSnapshot(t)
			tc.change(&snap)
			state := FromSnapshot(snap)
			if state.CollectionAvailable() {
				t.Fatalf("unsafe collection available: %+v", state.Collection)
			}
			if _, known := state.CollectionCount("gsr"); known {
				t.Fatal("unsafe count available")
			}
		})
	}
	state := FromSnapshot(collectionSnapshot(t))
	state.Generation++
	if _, known := state.CollectionCount("gsr"); known {
		t.Fatal("reused world count available")
	}
}

func TestCollectionStorageModelCopiesResetsAndChangesScope(t *testing.T) {
	snap := collectionSnapshot(t)
	model := NewModel(slog.New(slog.NewTextHandler(io.Discard, nil)))
	first := model.Update(snap)
	first.Collection.Counts[0].Count = 99
	if got, _ := model.Current().CollectionCount("gsr"); got != 22 {
		t.Fatalf("model alias: %d", got)
	}
	snap.Collection.Counts[0].Count = 1
	if got, _ := model.Current().CollectionCount("gsr"); got != 22 {
		t.Fatal("snapshot alias")
	}
	snap.Collection.ScopeID = "replacement-instance"
	next := model.Update(snap)
	if next.Collection.ScopeID == first.Collection.ScopeID {
		t.Fatal("scope did not change")
	}
	if model.Reset(snap.At, "process_lost").CollectionAvailable() {
		t.Fatal("reset retained collection")
	}
}
