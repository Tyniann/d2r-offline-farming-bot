package memory

import "testing"

func TestDecodeItemSocketsTruthTable(t *testing.T) {
	basePresent := func(v int32) SocketStatEvidence {
		return SocketStatEvidence{ListReadable: true, Present: true, Value: v}
	}
	baseAbsent := SocketStatEvidence{ListReadable: true}
	unreadable := SocketStatEvidence{}

	cases := []struct {
		name      string
		flags     uint32
		active    SocketStatEvidence
		base      SocketStatEvidence
		sockets   int
		available bool
		socketed  bool
	}{
		{
			name:  "missing both lists",
			flags: itemFlagSocketed, active: unreadable, base: unreadable,
		},
		{
			name:  "unreadable lists flag off",
			flags: 0, active: unreadable, base: unreadable,
		},
		{
			name:  "absent base no flag",
			flags: 0, active: baseAbsent, base: baseAbsent,
			available: true,
		},
		{
			name:  "absent base with flag",
			flags: itemFlagSocketed, active: baseAbsent, base: baseAbsent,
		},
		{
			name:   "live thresher base only",
			flags:  itemFlagSocketed | itemFlagIdentified | 0x800000,
			active: baseAbsent, base: basePresent(4),
			sockets: 4, available: true, socketed: true,
		},
		{
			name:   "live elegant blade 1os",
			flags:  itemFlagSocketed | itemFlagIdentified | 0x800000,
			active: baseAbsent, base: basePresent(1),
			sockets: 1, available: true, socketed: true,
		},
		{
			name:   "live bone wand white 2os",
			flags:  itemFlagSocketed | itemFlagIdentified | 0x800000 | 0x4000000,
			active: baseAbsent, base: basePresent(2),
			sockets: 2, available: true, socketed: true,
		},
		{
			name:   "active preferred when base absent",
			flags:  itemFlagSocketed,
			active: basePresent(3), base: baseAbsent,
			sockets: 3, available: true, socketed: true,
		},
		{
			name:   "active absent base present still works",
			flags:  itemFlagSocketed,
			active: baseAbsent, base: basePresent(6),
			sockets: 6, available: true, socketed: true,
		},
		{
			name:   "active and base agree",
			flags:  itemFlagSocketed,
			active: basePresent(4), base: basePresent(4),
			sockets: 4, available: true, socketed: true,
		},
		{
			name:   "active base value conflict",
			flags:  itemFlagSocketed,
			active: basePresent(2), base: basePresent(4),
		},
		{
			name:  "flag on value 0",
			flags: itemFlagSocketed, active: unreadable, base: basePresent(0),
		},
		{
			name:  "flag off value positive",
			flags: 0, active: unreadable, base: basePresent(4),
		},
		{
			name:  "explicit zero no flag",
			flags: 0, active: unreadable, base: basePresent(0),
			sockets: 0, available: true, socketed: false,
		},
		{
			name:  "value out of range high",
			flags: itemFlagSocketed, active: unreadable, base: basePresent(7),
		},
		{
			name:  "value negative",
			flags: itemFlagSocketed, active: unreadable, base: basePresent(-1),
		},
		{
			name:   "skullders unsocketed",
			flags:  itemFlagIdentified | 0x800000,
			active: baseAbsent, base: baseAbsent,
			available: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sockets, available, socketed := decodeItemSockets(tc.flags, tc.active, tc.base)
			if sockets != tc.sockets || available != tc.available || socketed != tc.socketed {
				t.Fatalf("got sockets=%d available=%t socketed=%t, want %d/%t/%t",
					sockets, available, socketed, tc.sockets, tc.available, tc.socketed)
			}
		})
	}
}

func TestProbeSnapshotDecodesBaseOnlySocketStat(t *testing.T) {
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
	setupGroundItemUnit(access, itemUnit, itemData, itemPath, statsListEx, 0, 255, 162, 2, itemFlagSocketed|itemFlagIdentified|0x800000)

	activeHeader := statsListEx + off.Unit.StatsListActive
	baseHeader := statsListEx + off.Unit.StatsListBase
	writeU64(access, activeHeader+off.Stats.ListPtr, uint64(activeArray))
	writeU64(access, activeHeader+off.Stats.Count, 1)
	writeStatEntry(access, activeArray, 0, 17, 10)
	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 1)
	writeStatEntry(access, baseArray, 0, StatNumSockets, 4)

	snap := probe.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("Items = %+v, want one item", snap.Items)
	}
	got := snap.Items[0]
	if !got.SocketsAvailable || !got.Socketed || got.Sockets != 4 {
		t.Fatalf("socket decode = sockets=%d available=%t socketed=%t, want 4/true/true", got.Sockets, got.SocketsAvailable, got.Socketed)
	}
}
