package memory

import (
	"encoding/binary"
	"testing"
)

func TestCollectObjectInspectEvidenceReadsModeOutsideAllowlist(t *testing.T) {
	access := newMockAccess()
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)

	const (
		moduleBase = 0x100000
		knownAddr  = 0x60000
		knownData  = 0x61000
		knownPath  = 0x62000
		junkAddr   = 0x63000
		junkData   = 0x64000
		junkPath   = 0x65000
	)
	access.moduleBase = moduleBase

	setupObjectUnit(access, knownAddr, knownData, knownPath, 267, 11)
	setupObjectUnit(access, junkAddr, junkData, junkPath, 181, 22)
	writeObjectMode(t, access, knownAddr, 1)
	writeObjectMode(t, access, junkAddr, 0)
	linkObjectUnits(t, access, off, knownAddr, junkAddr)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentObject, knownAddr)

	got, err := probe.CollectObjectInspectEvidence()
	if err != nil {
		t.Fatalf("CollectObjectInspectEvidence() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if !IsRuntimeObjectID(267) || IsRuntimeObjectID(181) {
		t.Fatal("fixture IDs no longer match allowlist vs inspect-only split")
	}
	byID := map[uint32]ObjectInspectEvidence{}
	for _, ev := range got {
		byID[ev.TxtFileNo] = ev
	}
	stash := byID[267]
	if stash.UnitID != 11 || !stash.ModeKnown || stash.Mode != 1 || !stash.PositionKnown || stash.PosX != 100 || stash.PosY != 200 {
		t.Fatalf("stash evidence = %+v", stash)
	}
	unknown := byID[181]
	if unknown.UnitID != 22 || !unknown.ModeKnown || unknown.Mode != 0 || !unknown.PositionKnown {
		t.Fatalf("unknown evidence = %+v", unknown)
	}
}

func TestCollectObjectInspectEvidenceSkipsNonObjectUnits(t *testing.T) {
	access := newMockAccess()
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)

	const (
		moduleBase = 0x100000
		unitAddr   = 0x70000
		pathAddr   = 0x71000
	)
	access.moduleBase = moduleBase
	setupEntranceUnit(access, unitAddr, 0x72000, pathAddr, 1, 9)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentObject, unitAddr)

	got, err := probe.CollectObjectInspectEvidence()
	if err != nil {
		t.Fatalf("CollectObjectInspectEvidence() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func writeObjectMode(t *testing.T, access *mockAccess, unitAddr uintptr, mode uint32) {
	t.Helper()
	buf := mustReadMock(t, access, unitAddr, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetMode:], mode)
	access.setBytes(unitAddr, buf)
}

func linkObjectUnits(t *testing.T, access *mockAccess, off OffsetSet, first, second uintptr) {
	t.Helper()
	buf := mustReadMock(t, access, first, 0x200)
	binary.LittleEndian.PutUint64(buf[off.Unit.NextUnit:], uint64(second))
	access.setBytes(first, buf)
}
