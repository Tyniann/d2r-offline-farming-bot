package memory

import (
	"encoding/binary"
	"testing"
)

func writeSegmentHead(access *mockAccess, moduleBase uintptr, unitTable uintptr, unitType int, headUnit uintptr) {
	seg := make([]byte, unitTableListHeads*unitTableHeadStride)
	binary.LittleEndian.PutUint64(seg[0:], uint64(headUnit))
	access.setBytes(unitSegmentBase(moduleBase, unitTable, unitType), seg)
}

func setupObjectUnit(access *mockAccess, unitAddr, unitData, path uintptr, txtFileNo, unitID uint32) {
	buf := make([]byte, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetUnitType:], unitTypeObject)
	binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], txtFileNo)
	binary.LittleEndian.PutUint32(buf[0x08:], unitID)
	binary.LittleEndian.PutUint64(buf[unitOffsetUnitData:], uint64(unitData))
	binary.LittleEndian.PutUint64(buf[0x38:], uint64(path))
	access.setBytes(unitAddr, buf)

	pathBuf := make([]byte, 0x20)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetObjectX:], 100)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetObjectY:], 200)
	access.setBytes(path, pathBuf)

	access.setBytes(unitData, []byte{1}) // non-nil unitData
}

func setupEntranceUnit(access *mockAccess, unitAddr, unitData, path uintptr, txtFileNo, unitID uint32) {
	buf := make([]byte, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetUnitType:], unitTypeEntrance)
	binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], txtFileNo)
	binary.LittleEndian.PutUint32(buf[0x08:], unitID)
	binary.LittleEndian.PutUint64(buf[unitOffsetUnitData:], uint64(unitData))
	binary.LittleEndian.PutUint64(buf[0x38:], uint64(path))
	access.setBytes(unitAddr, buf)

	pathBuf := make([]byte, 0x20)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetObjectX:], 300)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetObjectY:], 400)
	access.setBytes(path, pathBuf)

	if unitData != 0 {
		access.setBytes(unitData, []byte{1})
	}
}

func setupEntranceUnitNoUnitData(access *mockAccess, unitAddr, path uintptr, txtFileNo, unitID uint32) {
	setupEntranceUnit(access, unitAddr, 0, path, txtFileNo, unitID)
}

// writeJunkObjectChain builds a linked list of count object units with non-allowlisted txt IDs.
func writeJunkObjectChain(access *mockAccess, moduleBase uintptr, off OffsetSet, count int) uintptr {
	if count < 1 {
		return 0
	}
	units := make([]uintptr, count)
	for i := range units {
		units[i] = uintptr(0xA00000 + uintptr(i*0x200))
	}
	for i, addr := range units {
		buf := make([]byte, 0x200)
		binary.LittleEndian.PutUint32(buf[unitOffsetUnitType:], unitTypeObject)
		binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], 9999)
		if i+1 < len(units) {
			binary.LittleEndian.PutUint64(buf[off.Unit.NextUnit:], uint64(units[i+1]))
		}
		access.setBytes(addr, buf)
	}
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentObject, units[0])
	return units[0]
}

func setupMonsterUnit(access *mockAccess, unitAddr, unitData, path uintptr, npcID, unitID uint32, superUnique bool) {
	buf := make([]byte, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], npcID)
	binary.LittleEndian.PutUint32(buf[0x08:], unitID)
	buf[unitOffsetCorpse] = 0
	binary.LittleEndian.PutUint64(buf[unitOffsetUnitData:], uint64(unitData))
	binary.LittleEndian.PutUint64(buf[0x38:], uint64(path))
	access.setBytes(unitAddr, buf)

	flag := uint8(0)
	if superUnique {
		flag = 10
	}
	access.setBytes(unitData+unitDataMonsterFlag, []byte{flag})

	pathBuf := make([]byte, 0x10)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetMonsterX:], 500)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetMonsterY:], 600)
	access.setBytes(path, pathBuf)
}

func TestProbeSnapshotEnumeratesCountessEntitiesWhenInGame(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		objUnit = uintptr(0x60000)
		objData = uintptr(0x61000)
		objPath = uintptr(0x62000)
		entUnit = uintptr(0x63000)
		entData = uintptr(0x64000)
		entPath = uintptr(0x65000)
		monUnit = uintptr(0x66000)
		monData = uintptr(0x67000)
		monPath = uintptr(0x68000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentObject, objUnit)
	setupObjectUnit(access, objUnit, objData, objPath, 157, 1001)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnit(access, entUnit, 0, entPath, 10, 2001)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, monUnit)
	setupMonsterUnit(access, monUnit, monData, monPath, 51, 3001, true)

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("Snapshot() invalid, reason=%q phase=%s", snap.Reason, snap.Phase)
	}
	if snap.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %s, want in_game", snap.Phase)
	}
	if len(snap.Objects) != 1 || snap.Objects[0].TxtFileNo != 157 || snap.Objects[0].UnitID != 1001 {
		t.Fatalf("Objects = %+v, want waypoint 157", snap.Objects)
	}
	if len(snap.Entrances) != 1 || snap.Entrances[0].TxtFileNo != 10 {
		t.Fatalf("Entrances = %+v, want tower entrance 10", snap.Entrances)
	}
	if len(snap.Monsters) != 1 || snap.Monsters[0].NPCID != 51 || snap.Monsters[0].MonsterTypeFlag != 10 {
		t.Fatalf("Monsters = %+v, want living Dark Stalker super-unique", snap.Monsters)
	}
}

func TestProbeSnapshotEnumeratesSuperUniqueRegardlessOfNPCID(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		monUnit = uintptr(0x66000)
		monData = uintptr(0x67000)
		monPath = uintptr(0x68000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, monUnit)
	setupMonsterUnit(access, monUnit, monData, monPath, 52, 3001, true)

	snap := probe.Snapshot()
	if len(snap.Monsters) != 1 || snap.Monsters[0].NPCID != 52 || snap.Monsters[0].MonsterTypeFlag != 10 {
		t.Fatalf("Monsters = %+v, want super-unique npc 52 (Black Rogue)", snap.Monsters)
	}

	if unitSegmentBase(moduleBase, off.UnitTable, unitSegmentMonster) != moduleBase+off.UnitTable+1024 {
		t.Fatal("monster segment offset mismatch")
	}
	if unitSegmentBase(moduleBase, off.UnitTable, unitSegmentObject) != moduleBase+off.UnitTable+2048 {
		t.Fatal("object segment offset mismatch")
	}
	if unitSegmentBase(moduleBase, off.UnitTable, unitSegmentEntrance) != moduleBase+off.UnitTable+5120 {
		t.Fatal("entrance segment offset mismatch")
	}
}

func TestProbeSnapshotSkipsEntityWithNilPath(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const objUnit = uintptr(0x60000)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentObject, objUnit)

	buf := make([]byte, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetUnitType:], unitTypeObject)
	binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], 157)
	binary.LittleEndian.PutUint32(buf[0x08:], 42)
	binary.LittleEndian.PutUint64(buf[unitOffsetUnitData:], 0x61000)
	// path left zero
	access.setBytes(objUnit, buf)
	access.setBytes(0x61000, []byte{1})

	snap := probe.Snapshot()
	if len(snap.Objects) != 0 {
		t.Fatalf("expected no objects with nil path, got %+v", snap.Objects)
	}
}

func TestProbeSnapshotSkipsCorpseMonster(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const monUnit = uintptr(0x66000)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, monUnit)

	buf := make([]byte, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], 51)
	buf[unitOffsetCorpse] = 1
	access.setBytes(monUnit, buf)

	snap := probe.Snapshot()
	if len(snap.Monsters) != 0 {
		t.Fatalf("expected no corpse monsters, got %+v", snap.Monsters)
	}
}

func TestReadEntranceSegmentHeadAfterSetup(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		entUnit = uintptr(0x63000)
		entPath = uintptr(0x65000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnitNoUnitData(access, entUnit, entPath, 10, 2001)

	buf, err := probe.readUnitTableSegment(moduleBase, off, unitSegmentEntrance)
	if err != nil {
		t.Fatal(err)
	}
	head := uintptr(binary.LittleEndian.Uint64(buf[0:8]))
	if head != entUnit {
		t.Fatalf("entrance list head = %#x, want %#x", head, entUnit)
	}

	unitType, err := probe.reader.ReadUint32(entUnit + unitOffsetUnitType)
	if err != nil {
		t.Fatal(err)
	}
	if unitType != unitTypeEntrance {
		t.Fatalf("unitType = %d, want %d", unitType, unitTypeEntrance)
	}
}

func TestEntranceEnumeratedWithoutUnitData(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		entUnit = uintptr(0x63000)
		entPath = uintptr(0x65000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnitNoUnitData(access, entUnit, entPath, 10, 2001)

	snap := probe.Snapshot()
	if len(snap.Entrances) != 1 || snap.Entrances[0].TxtFileNo != 10 {
		t.Fatalf("Entrances = %+v, want tower entrance 10 without unitData", snap.Entrances)
	}
}

func TestEntrancesEnumeratedDespiteLargeObjectSegment(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		entUnit = uintptr(0x63000)
		entPath = uintptr(0x65000)
	)

	writeJunkObjectChain(access, moduleBase, off, maxTotalUnitVisits+10)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnitNoUnitData(access, entUnit, entPath, 11, 2002)

	snap := probe.Snapshot()
	if len(snap.Entrances) != 1 || snap.Entrances[0].TxtFileNo != 11 {
		t.Fatalf("Entrances = %+v, want cellar stairs 11 despite object flood", snap.Entrances)
	}
}
