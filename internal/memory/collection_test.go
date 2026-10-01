package memory

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
)

func collectionWriteU32(a *mockAccess, addr uintptr, value uint32) {
	for base, data := range a.memory {
		if addr >= base && addr+4 <= base+uintptr(len(data)) {
			binary.LittleEndian.PutUint32(data[addr-base:], value)
			return
		}
	}
	writeU32(a, addr, value)
}

func collectionWriteU64(a *mockAccess, addr uintptr, value uint64) {
	for base, data := range a.memory {
		if addr >= base && addr+8 <= base+uintptr(len(data)) {
			binary.LittleEndian.PutUint64(data[addr-base:], value)
			return
		}
	}
	writeU64(a, addr, value)
}

func collectionFixture(t *testing.T) (*ProbeReader, *mockAccess, OffsetSet, Snapshot) {
	t.Helper()
	var fixture struct {
		Inventory string `json:"inventory_hex"`
		Owner     string `json:"owner_hex"`
		Item      string `json:"item_hex"`
		Data      string `json:"data_hex"`
	}
	data, err := os.ReadFile("testdata/collection-gems-stock.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	access := newMockAccess()
	access.moduleBase = 0x100000
	off := DefaultOffsetSet()
	for addr, value := range map[uintptr]string{0x20000: fixture.Inventory, 0x30000: fixture.Owner, 0x40000: fixture.Item, 0x50000: fixture.Data} {
		bytes, err := hex.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		access.setBytes(addr, bytes)
	}
	writeSegmentHead(access, access.moduleBase, off.UnitTable, unitSegmentPlayer, 0x30000)
	// The owner fixture ends before NextUnit; only the captured zero link is needed.
	collectionWriteU64(access, 0x30000+off.Unit.NextUnit, 0)
	ui := make([]byte, uiBufferSize)
	ui[uiInventoryIndex] = 1
	ui[uiStashIndex] = 1
	ui[uiGateIndex] = 1
	access.setBytes(access.moduleBase+off.UI-uiBufferBefore, ui)
	r := NewReader(testLogger())
	r.Bind(access)
	snap := Snapshot{Valid: true, Phase: GamePhaseInGame, Generation: 1, UI: UIState{InventoryOpen: true, StashOpen: true, CubeOpenKnown: true}}
	return NewProbeReader(r, off), access, off, snap
}

func TestCollectionStorageCapturedStockAndBounds(t *testing.T) {
	for _, count := range []uint32{22, 0, 1, 98, 99} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			p, a, off, snap := collectionFixture(t)
			collectionWriteU32(a, 0x50000+collectionCountAt, count)
			got := p.readCollectionStorage(a.moduleBase, off, snap)
			if !got.Available || got.ScopeID == "" || got.Generation != snap.Generation || len(got.Counts) != 1 || got.Counts[0].Count != count || got.Counts[0].UnitID != 142 {
				t.Fatalf("collection = %+v, want captured bound count %d", got, count)
			}
		})
	}
}

func TestCollectionStorageRejectsIncompleteEvidence(t *testing.T) {
	cases := []struct {
		name   string
		change func(*mockAccess, *OffsetSet, *Snapshot)
	}{
		{"out_of_range", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) { collectionWriteU32(a, 0x50000+collectionCountAt, 100) }},
		{"wrong_parent", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) {
			collectionWriteU64(a, 0x50000+collectionParentAt, 0x60000)
		}},
		{"cycle", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) {
			collectionWriteU64(a, 0x50000+collectionNextAt, 0x40000)
		}},
		{"bad_reverse_link", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) {
			collectionWriteU64(a, 0x50000+collectionPreviousAt, 0x40000)
		}},
		{"wrong_endpoint", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) {
			collectionWriteU64(a, 0x20000+collectionLastAt, 0x60000)
		}},
		{"missing_node", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) { collectionWriteU32(a, 0x20000+collectionLengthAt, 2) }},
		{"owner_changed", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) { collectionWriteU64(a, 0x30000+0x90, 0) }},
		{"unreadable_data", func(a *mockAccess, _ *OffsetSet, _ *Snapshot) { a.partialAt = 0x50000 }},
		{"stash_closed", func(_ *mockAccess, _ *OffsetSet, s *Snapshot) { s.UI.StashOpen = false }},
		{"stale", func(_ *mockAccess, _ *OffsetSet, s *Snapshot) { s.Generation = 0 }},
		{"version", func(_ *mockAccess, o *OffsetSet, _ *Snapshot) { o.D2RVersion = "unknown" }},
		{"ui_changed", func(a *mockAccess, o *OffsetSet, _ *Snapshot) {
			a.setBytes(a.moduleBase+o.UI-uiBufferBefore, make([]byte, uiBufferSize))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, a, off, snap := collectionFixture(t)
			tc.change(a, &off, &snap)
			got := p.readCollectionStorage(a.moduleBase, off, snap)
			if got.Available || got.ScopeID != "" || len(got.Counts) != 0 {
				t.Fatalf("unsafe collection = %+v", got)
			}
		})
	}
}

type changingCollectionAccess struct {
	*mockAccess
	dataReads int
}

type collectionVersionAccess struct {
	*mockAccess
	version string
}

func (a collectionVersionAccess) Status() process.Status {
	return process.Status{State: process.StateAttached, FileVersion: a.version}
}

func TestCollectionStorageRequiresBoundVersion(t *testing.T) {
	p, a, off, snap := collectionFixture(t)
	p.reader.Bind(collectionVersionAccess{mockAccess: a, version: "unsupported"})
	if p.readCollectionStorage(a.moduleBase, off, snap).Available {
		t.Fatal("configured version replaced unsupported actual version")
	}
}

func (a *changingCollectionAccess) ReadAt(addr uintptr, buf []byte) error {
	if addr == 0x50000 && len(buf) == collectionDataSize {
		a.dataReads++
		if a.dataReads == 2 {
			collectionWriteU32(a.mockAccess, addr+collectionCountAt, 23)
		}
	}
	return a.mockAccess.ReadAt(addr, buf)
}

func TestCollectionStorageRejectsCountDriftAndChangesScope(t *testing.T) {
	p, a, off, snap := collectionFixture(t)
	p.reader.Bind(&changingCollectionAccess{mockAccess: a})
	if got := p.readCollectionStorage(a.moduleBase, off, snap); got.Available {
		t.Fatalf("changing count accepted: %+v", got)
	}
	p.reader.Bind(a)
	before := p.readCollectionStorage(a.moduleBase, off, snap)
	a.setBytes(0x60000, a.memory[0x20000])
	collectionWriteU64(a, 0x30000+off.Unit.Inventory, 0x60000)
	collectionWriteU64(a, 0x50000+collectionParentAt, 0x60000)
	after := p.readCollectionStorage(a.moduleBase, off, snap)
	if !before.Available || !after.Available || before.ScopeID == after.ScopeID {
		t.Fatalf("scope change = %+v -> %+v", before, after)
	}
	// The fixture's txt ID remains the captured value, not a guessed catalog ID.
	if before.Counts[0].TxtFileNo != binary.LittleEndian.Uint32(a.memory[0x40000][4:]) {
		t.Fatal("txt binding lost")
	}
}
