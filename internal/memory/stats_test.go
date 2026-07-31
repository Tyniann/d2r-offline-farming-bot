package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatIDConstantsMatchD2go(t *testing.T) {
	// d2go pkg/data/stat: Strength=0 … Life=6, MaxLife=7, Mana=8, MaxMana=9.
	if StatLife != 6 || StatMaxLife != 7 || StatMana != 8 || StatMaxMana != 9 {
		t.Fatalf("stat IDs changed: life=%d max=%d mana=%d max_mana=%d", StatLife, StatMaxLife, StatMana, StatMaxMana)
	}
}

func TestStatNumSocketsMatchesLocalItemStatCost(t *testing.T) {
	if StatNumSockets != 194 {
		t.Fatalf("StatNumSockets = %d, want 194 from item_numsockets", StatNumSockets)
	}

	path := filepath.Join("..", "..", ".tmp", "d2r-excel", "itemstatcost.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("local excel extract unavailable: %v", err)
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] != "item_numsockets" {
			continue
		}
		found = true
		if fields[1] != "194" {
			t.Fatalf("item_numsockets *ID = %q, want 194", fields[1])
		}
		break
	}
	if !found {
		t.Fatal("item_numsockets row missing from itemstatcost.txt")
	}
}

func TestParseVitalStatsSkipsNonZeroLayer(t *testing.T) {
	access := newMockAccess()
	const header = uintptr(0xC000)
	const array = uintptr(0xD000)
	off := DefaultOffsetSet().Stats

	writeU64(access, header+off.ListPtr, uint64(array))
	writeU64(access, header+off.Count, 5)
	// Layer 1 entries must be ignored; layer 0 holds the vitals.
	writeStatEntry(access, array+0, 1, StatLife, 99999)
	writeStatEntry(access, array+8, 0, StatLife, 25600)
	writeStatEntry(access, array+16, 0, StatMaxLife, 32000)
	writeStatEntry(access, array+24, 0, StatMana, 12800)
	writeStatEntry(access, array+32, 0, StatMaxMana, 19200)

	reader := newTestReader(access)
	reader.Bind(access)

	vitals, err := parseVitalStats(reader, header, off)
	if err != nil {
		t.Fatalf("parseVitalStats() error = %v", err)
	}
	if vitals.HP != 100 {
		t.Fatalf("HP = %d, want 100 (layer 1 entry must be skipped)", vitals.HP)
	}
}

func TestParseVitalStatsAllFour(t *testing.T) {
	access := newMockAccess()
	const header = uintptr(0xE000)
	const array = uintptr(0xF000)
	off := DefaultOffsetSet().Stats

	writeU64(access, header+off.ListPtr, uint64(array))
	writeU64(access, header+off.Count, 4)
	writeStatEntry(access, array+0, 0, StatLife, 25600)
	writeStatEntry(access, array+8, 0, StatMaxLife, 32000)
	writeStatEntry(access, array+16, 0, StatMana, 12800)
	writeStatEntry(access, array+24, 0, StatMaxMana, 19200)

	reader := newTestReader(access)
	reader.Bind(access)

	vitals, err := parseVitalStats(reader, header, off)
	if err != nil {
		t.Fatalf("parseVitalStats() error = %v", err)
	}
	if vitals.HP != 100 || vitals.MaxHP != 125 || vitals.Mana != 50 || vitals.MaxMana != 75 {
		t.Fatalf("vitals = %+v", vitals)
	}
}

func TestParseGoldStatsReadsLayerZeroValues(t *testing.T) {
	access := newMockAccess()
	const header = uintptr(0x11000)
	const array = uintptr(0x12000)
	off := DefaultOffsetSet().Stats

	writeU64(access, header+off.ListPtr, uint64(array))
	writeU64(access, header+off.Count, 3)
	writeStatEntry(access, array+0, 0, StatGold, 50938)
	writeStatEntry(access, array+8, 0, StatGoldBank, 2401390)
	writeStatEntry(access, array+16, 1, StatGold, 1)

	reader := newTestReader(access)
	reader.Bind(access)
	got, err := parseGoldStats(reader, header, off)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CarriedKnown || !got.StashKnown || got.Carried != 50938 || got.PrivateStash != 2401390 {
		t.Fatalf("gold stats = %+v", got)
	}
}
