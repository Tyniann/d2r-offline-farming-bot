package memory

import (
	"encoding/binary"
	"testing"
)

func TestUnitSegmentBaseOffsets(t *testing.T) {
	const moduleBase = uintptr(0x10000000)
	const unitTable = uintptr(0x2000)

	cases := []struct {
		unitType int
		want     uintptr
	}{
		{unitSegmentMonster, moduleBase + unitTable + 1024},
		{unitSegmentObject, moduleBase + unitTable + 2048},
		{unitSegmentEntrance, moduleBase + unitTable + 5120},
	}
	for _, tc := range cases {
		got := unitSegmentBase(moduleBase, unitTable, tc.unitType)
		if got != tc.want {
			t.Fatalf("unitSegmentBase(type=%d) = %#x, want %#x", tc.unitType, got, tc.want)
		}
	}
}

func TestReadUnitTableSegmentBuffer(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const head0 = uintptr(0x60000)
	segmentBase := unitSegmentBase(moduleBase, off.UnitTable, unitSegmentObject)
	segBuf := make([]byte, unitTableListHeads*unitTableHeadStride)
	writeU64ToBuf(segBuf, 0, uint64(head0))
	access.setBytes(segmentBase, segBuf)

	buf, err := probe.readUnitTableSegment(moduleBase, off, unitSegmentObject)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != unitTableListHeads*unitTableHeadStride {
		t.Fatalf("buffer len = %d, want %d", len(buf), unitTableListHeads*unitTableHeadStride)
	}
}

func writeU64ToBuf(buf []byte, offset int, v uint64) {
	binary.LittleEndian.PutUint64(buf[offset:], v)
}

func TestWalkUnitSegmentRespectsGlobalVisitCap(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	segmentBase := unitSegmentBase(moduleBase, off.UnitTable, unitSegmentMonster)
	const first = uintptr(0x70000)
	const second = uintptr(0x71000)
	segBuf := make([]byte, unitTableListHeads*unitTableHeadStride)
	writeU64ToBuf(segBuf, 0, uint64(first))
	access.setBytes(segmentBase, segBuf)
	writeU64(access, first+off.Unit.NextUnit, uint64(second))
	writeU64(access, second+off.Unit.NextUnit, uint64(first)) // cycle

	visited := 0
	calls := 0
	_ = probe.walkUnitSegment(moduleBase, off, unitSegmentMonster, &visited, 0, func(_ uintptr) (unitWalkAction, error) {
		calls++
		return unitWalkContinue, nil
	})
	if visited > maxTotalUnitVisits {
		t.Fatalf("visited = %d, exceeds cap %d", visited, maxTotalUnitVisits)
	}
	if calls == 0 {
		t.Fatal("expected at least one visit")
	}
}

func TestWalkUnitSegmentRespectsPerBucketCap(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	segmentBase := unitSegmentBase(moduleBase, off.UnitTable, unitSegmentMonster)
	var prev uintptr = 0x80000
	segBuf := make([]byte, unitTableListHeads*unitTableHeadStride)
	writeU64ToBuf(segBuf, 0, uint64(prev))
	access.setBytes(segmentBase, segBuf)
	for i := 0; i < maxUnitsPerBucket+5; i++ {
		next := uintptr(0x90000 + uintptr(i*0x100))
		writeU64(access, prev+off.Unit.NextUnit, uint64(next))
		prev = next
	}

	visited := 0
	perBucket := 0
	_ = probe.walkUnitSegment(moduleBase, off, unitSegmentMonster, &visited, 0, func(_ uintptr) (unitWalkAction, error) {
		perBucket++
		return unitWalkContinue, nil
	})
	if perBucket > maxUnitsPerBucket {
		t.Fatalf("per-bucket visits = %d, want <= %d", perBucket, maxUnitsPerBucket)
	}
}

func TestWalkUnitSegmentRespectsSegmentLimit(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	segmentBase := unitSegmentBase(moduleBase, off.UnitTable, unitSegmentMonster)
	var prev uintptr = 0x80000
	segBuf := make([]byte, unitTableListHeads*unitTableHeadStride)
	writeU64ToBuf(segBuf, 0, uint64(prev))
	access.setBytes(segmentBase, segBuf)
	for i := 0; i < 20; i++ {
		next := uintptr(0x90000 + uintptr(i*0x100))
		writeU64(access, prev+off.Unit.NextUnit, uint64(next))
		prev = next
	}

	visited := 0
	calls := 0
	const segmentCap = 5
	_ = probe.walkUnitSegment(moduleBase, off, unitSegmentMonster, &visited, segmentCap, func(_ uintptr) (unitWalkAction, error) {
		calls++
		return unitWalkContinue, nil
	})
	if calls != segmentCap {
		t.Fatalf("segment visits = %d, want %d", calls, segmentCap)
	}
}
