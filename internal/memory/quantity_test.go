package memory

import "testing"

func TestDecodeItemQuantityPrefersBase(t *testing.T) {
	present := func(v int32) SocketStatEvidence {
		return SocketStatEvidence{ListReadable: true, Present: true, Value: v}
	}
	absent := SocketStatEvidence{ListReadable: true}
	unreadable := SocketStatEvidence{}

	cases := []struct {
		name     string
		active   SocketStatEvidence
		base     SocketStatEvidence
		quantity int
		known    bool
	}{
		{name: "missing both lists", active: unreadable, base: unreadable},
		{name: "empty lists", active: absent, base: absent},
		{name: "live key base only", active: absent, base: present(5), quantity: 5, known: true},
		{name: "base preferred over active", active: present(1), base: present(1), quantity: 1, known: true},
		{name: "active only", active: present(12), base: absent, quantity: 12, known: true},
		{name: "active base conflict", active: present(5), base: present(4)},
		{name: "negative", active: absent, base: present(-1)},
		{name: "explicit zero", active: absent, base: present(0), quantity: 0, known: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quantity, known := decodeItemQuantity(tc.active, tc.base)
			if quantity != tc.quantity || known != tc.known {
				t.Fatalf("got quantity=%d known=%t, want %d/%t", quantity, known, tc.quantity, tc.known)
			}
		})
	}
}

func TestProbeSnapshotDecodesBaseOnlyQuantityStat(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		itemUnit    = uintptr(0x69000)
		itemData    = uintptr(0x6A000)
		itemPath    = uintptr(0x6B000)
		statsListEx = uintptr(0x6C000)
		activeArray = uintptr(0x6D000)
		baseArray   = uintptr(0x6E000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	setupItemUnit(access, itemUnit, itemData, itemPath, statsListEx, 0, 558, 44, 2, itemFlagIdentified, itemRawLocationInventory, 1, 0, 9, 3)

	activeHeader := statsListEx + off.Unit.StatsListActive
	baseHeader := statsListEx + off.Unit.StatsListBase
	writeU64(access, activeHeader+off.Stats.ListPtr, uint64(activeArray))
	writeU64(access, activeHeader+off.Stats.Count, 0)
	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 1)
	writeStatEntry(access, baseArray, 0, StatQuantity, 5)

	snap := probe.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("Items = %+v, want one item", snap.Items)
	}
	got := snap.Items[0]
	if !got.QuantityKnown || got.Quantity != 5 {
		t.Fatalf("quantity = known=%t value=%d, want 5", got.QuantityKnown, got.Quantity)
	}
	if len(got.Stats) != 0 {
		t.Fatalf("productive stats = %+v, want empty Active list", got.Stats)
	}
}
