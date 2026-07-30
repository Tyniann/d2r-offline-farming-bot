package memory

import (
	"encoding/binary"
	"fmt"
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

func setupGroundItemUnit(access *mockAccess, unitAddr, unitData, path, statsListEx, statsArray uintptr, txtFileNo, unitID, quality, flags uint32) {
	setupItemUnit(access, unitAddr, unitData, path, statsListEx, statsArray, txtFileNo, unitID, quality, flags, itemRawLocationGround, 0, 0, 700, 800)
}

func setupItemUnit(access *mockAccess, unitAddr, unitData, path, statsListEx, statsArray uintptr, txtFileNo, unitID, quality, flags, rawLocation, ownerID uint32, page uint8, x, y uint16) {
	buf := make([]byte, 0x200)
	binary.LittleEndian.PutUint32(buf[unitOffsetUnitType:], itemUnitType)
	binary.LittleEndian.PutUint32(buf[unitOffsetTxtFileNo:], txtFileNo)
	binary.LittleEndian.PutUint32(buf[0x08:], unitID)
	binary.LittleEndian.PutUint32(buf[itemOffsetRawLocation:], rawLocation)
	binary.LittleEndian.PutUint64(buf[unitOffsetUnitData:], uint64(unitData))
	binary.LittleEndian.PutUint64(buf[0x38:], uint64(path))
	if statsListEx != 0 {
		binary.LittleEndian.PutUint64(buf[0x88:], uint64(statsListEx))
	}
	access.setBytes(unitAddr, buf)

	dataBuf := make([]byte, 0x60)
	binary.LittleEndian.PutUint32(dataBuf[itemDataOffsetQuality:], quality)
	binary.LittleEndian.PutUint32(dataBuf[itemDataOffsetOwnerID:], ownerID)
	binary.LittleEndian.PutUint32(dataBuf[itemDataOffsetFlags:], flags)
	binary.LittleEndian.PutUint32(dataBuf[itemDataOffsetUniqueSetID:], ^uint32(0))
	dataBuf[itemDataOffsetPage] = page
	access.setBytes(unitData, dataBuf)

	if path != 0 {
		pathBuf := make([]byte, 0x20)
		binary.LittleEndian.PutUint16(pathBuf[pathOffsetObjectX:], x)
		binary.LittleEndian.PutUint16(pathBuf[pathOffsetObjectY:], y)
		access.setBytes(path, pathBuf)
	}

	if statsListEx != 0 && statsArray != 0 {
		off := testOffsetSet()
		activeHeader := statsListEx + off.Unit.StatsListActive
		writeU64(access, activeHeader+off.Stats.ListPtr, uint64(statsArray))
		writeU64(access, activeHeader+off.Stats.Count, 1)
		writeStatEntry(access, statsArray, 2, 123, 456)
	}
}

func writeJunkItemChain(access *mockAccess, moduleBase uintptr, off OffsetSet, count int) uintptr {
	if count < 1 {
		return 0
	}
	units := make([]uintptr, count)
	for i := range units {
		units[i] = uintptr(0xB00000 + uintptr(i*0x200))
	}
	for i, addr := range units {
		buf := make([]byte, 0x200)
		binary.LittleEndian.PutUint32(buf[unitOffsetUnitType:], 99)
		if i+1 < len(units) {
			binary.LittleEndian.PutUint64(buf[off.Unit.NextUnit:], uint64(units[i+1]))
		}
		access.setBytes(addr, buf)
	}
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, units[0])
	return units[0]
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

func TestAppendRuntimeMonsterNeverStarvesBossAndKeepsNearestCleanupTargets(t *testing.T) {
	snap := Snapshot{PosX: 100, PosY: 100, Monsters: make([]MonsterUnit, 0)}
	for i := 0; i < Phase17MaxRuntimeMonsters; i++ {
		appendRuntimeMonster(&snap, MonsterUnit{
			NPCID: 131, UnitID: uint32(i + 1), PosX: uint32(200 + i), PosY: 100,
		})
	}

	boss := MonsterUnit{NPCID: 250, UnitID: 500, PosX: 110, PosY: 100}
	appendRuntimeMonster(&snap, boss)
	nearby := MonsterUnit{NPCID: 56, UnitID: 501, PosX: 101, PosY: 100}
	appendRuntimeMonster(&snap, nearby)

	finalizeMonsterCoverage(&snap)
	if len(snap.Monsters) != Phase17MaxRuntimeMonsters+1 {
		t.Fatalf("monster count=%d, want bounded cleanup reservoir plus priority boss", len(snap.Monsters))
	}
	var bossFound, nearbyFound bool
	for _, monster := range snap.Monsters {
		bossFound = bossFound || monster.UnitID == boss.UnitID
		nearbyFound = nearbyFound || monster.UnitID == nearby.UnitID
	}
	if !bossFound || !nearbyFound {
		t.Fatalf("boss_found=%t nearby_found=%t monsters=%+v", bossFound, nearbyFound, snap.Monsters)
	}
	if snap.MonsterCoverage.EligibleMonsterCount != Phase17MaxRuntimeMonsters+2 ||
		!snap.MonsterCoverage.MonstersTruncated ||
		snap.MonsterCoverage.MonsterCoverageRadiusTiles != 610 {
		t.Fatalf("coverage = %+v", snap.MonsterCoverage)
	}
}

func TestRuntimeMonsterCoverageBoundaries(t *testing.T) {
	for _, count := range []int{32, 33, Phase17MaxRuntimeMonsters, Phase17MaxRuntimeMonsters + 1} {
		t.Run(fmt.Sprintf("%d", count), func(t *testing.T) {
			snap := Snapshot{PosX: 100, PosY: 100, Monsters: make([]MonsterUnit, 0)}
			for i := 0; i < count; i++ {
				appendRuntimeMonster(&snap, MonsterUnit{NPCID: 40, UnitID: uint32(i + 1), PosX: uint32(101 + i), PosY: 100})
			}
			finalizeMonsterCoverage(&snap)
			wantRetained := count
			if wantRetained > Phase17MaxRuntimeMonsters {
				wantRetained = Phase17MaxRuntimeMonsters
			}
			if len(snap.Monsters) != wantRetained || snap.MonsterCoverage.EligibleMonsterCount != count {
				t.Fatalf("count=%d retained=%d coverage=%+v", count, len(snap.Monsters), snap.MonsterCoverage)
			}
			if got, want := snap.MonsterCoverage.MonstersTruncated, count > Phase17MaxRuntimeMonsters; got != want {
				t.Fatalf("count=%d truncated=%t want=%t", count, got, want)
			}
		})
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

func TestUnknownEntranceEnumerated(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		entUnit = uintptr(0x63000)
		entPath = uintptr(0x65000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnitNoUnitData(access, entUnit, entPath, 999, 2003)

	snap := probe.Snapshot()
	if len(snap.Entrances) != 1 || snap.Entrances[0].TxtFileNo != 999 || snap.Entrances[0].UnitID != 2003 {
		t.Fatalf("Entrances = %+v, want unknown entrance 999", snap.Entrances)
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

func TestProbeSnapshotEnumeratesGroundItems(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		itemUnit    = uintptr(0x69000)
		itemData    = uintptr(0x6A000)
		itemPath    = uintptr(0x6B000)
		statsListEx = uintptr(0x6C000)
		statsArray  = uintptr(0x6D000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	setupGroundItemUnit(access, itemUnit, itemData, itemPath, statsListEx, statsArray, 610, 4001, 2, itemFlagIdentified|itemFlagEthereal)
	writeU32(access, itemData+itemDataOffsetUniqueSetID, 77)

	snap := probe.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("Items = %+v, want one ground item", snap.Items)
	}
	got := snap.Items[0]
	if got.TxtFileNo != 610 || got.UnitID != 4001 || got.Quality != 2 || got.RawLocation != itemRawLocationGround {
		t.Fatalf("Item = %+v, want txt=610 unit=4001 quality=2 ground", got)
	}
	if got.PosX != 700 || got.PosY != 800 || !got.Identified || !got.Ethereal {
		t.Fatalf("Item flags/position = %+v, want identified ethereal at 700,800", got)
	}
	if !got.UniqueSetIDAvailable || got.UniqueSetID != 77 {
		t.Fatalf("UniqueSetID = %d available=%t, want 77/true", got.UniqueSetID, got.UniqueSetIDAvailable)
	}
	if len(got.Stats) != 1 || got.Stats[0].ID != 123 || got.Stats[0].Layer != 2 || got.Stats[0].Value != 456 {
		t.Fatalf("Stats = %+v, want raw stat 123/2/456", got.Stats)
	}
}

func TestItemIdentityReadFailureKeepsItem(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		itemUnit = uintptr(0x69000)
		itemData = uintptr(0x6A000)
		itemPath = uintptr(0x6B000)
	)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	setupGroundItemUnit(access, itemUnit, itemData, itemPath, 0, 0, 610, 4001, 5, 0)
	access.partialAt = itemData + itemDataOffsetUniqueSetID

	snap := probe.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("Items = %+v, want item despite unavailable identity", snap.Items)
	}
	if snap.Items[0].UniqueSetIDAvailable {
		t.Fatalf("UniqueSetIDAvailable = true, want false: %+v", snap.Items[0])
	}
}

func TestProbeSnapshotEnumeratesInventoryItemsWithGridFields(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		itemUnit = uintptr(0x69000)
		itemData = uintptr(0x6A000)
		itemPath = uintptr(0x6B000)
	)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	setupItemUnit(access, itemUnit, itemData, itemPath, 0, 0, 625, 4001, 2, 0, itemRawLocationInventory, 9001, 0, 4, 2)

	snap := probe.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("Items = %+v, want one inventory item", snap.Items)
	}
	got := snap.Items[0]
	if got.RawLocation != itemRawLocationInventory || got.OwnerID != 9001 || !got.PlayerOwned {
		t.Fatalf("Item ownership/location = %+v, want player-owned inventory", got)
	}
	if got.Page != 0 || got.GridX != 4 || got.GridY != 2 {
		t.Fatalf("Item grid = page %d (%d,%d), want page 0 (4,2)", got.Page, got.GridX, got.GridY)
	}
}

func TestIsPlayerOwnedItem(t *testing.T) {
	if !isPlayerOwnedItem(9001, 9001) {
		t.Fatal("main player owner should be player-owned")
	}
	if !isPlayerOwnedItem(itemOwnerPlayerSentinel, 9001) {
		t.Fatal("player sentinel owner should be player-owned")
	}
	if isPlayerOwnedItem(2, 9001) {
		t.Fatal("stash-like owner should not be player-owned")
	}
	if isPlayerOwnedItem(0, 9001) {
		t.Fatal("unknown owner should not be player-owned")
	}
}

func TestItemUnitSegmentBaseOffset(t *testing.T) {
	const moduleBase = uintptr(0x10000000)
	const unitTable = uintptr(0x2000)
	if got, want := unitSegmentBase(moduleBase, unitTable, unitSegmentItem), moduleBase+unitTable+4096; got != want {
		t.Fatalf("item segment base = %#x, want %#x", got, want)
	}
}

func TestGroundItemWithNilPathSkipped(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		itemUnit = uintptr(0x69000)
		itemData = uintptr(0x6A000)
	)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	setupGroundItemUnit(access, itemUnit, itemData, 0, 0, 0, 610, 4001, 2, 0)

	snap := probe.Snapshot()
	if len(snap.Items) != 0 {
		t.Fatalf("Items = %+v, want nil-path ground item skipped", snap.Items)
	}
}

func TestItemStatsFailureKeepsItem(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		itemUnit    = uintptr(0x69000)
		itemData    = uintptr(0x6A000)
		itemPath    = uintptr(0x6B000)
		statsListEx = uintptr(0x6C000)
	)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	setupGroundItemUnit(access, itemUnit, itemData, itemPath, statsListEx, 0, 610, 4001, 2, 0)

	snap := probe.Snapshot()
	if len(snap.Items) != 1 {
		t.Fatalf("Items = %+v, want item without stats", snap.Items)
	}
	if len(snap.Items[0].Stats) != 0 {
		t.Fatalf("Stats = %+v, want empty on stat failure", snap.Items[0].Stats)
	}
}

func TestItemEnumerationFailureDoesNotClearEntities(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		entUnit  = uintptr(0x63000)
		entPath  = uintptr(0x65000)
		itemUnit = uintptr(0x69000)
	)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnitNoUnitData(access, entUnit, entPath, 10, 2001)

	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentItem, itemUnit)
	writeU64(access, itemUnit+off.Unit.NextUnit, uint64(0xDEADBEEF))

	snap := probe.Snapshot()
	if len(snap.Entrances) != 1 {
		t.Fatalf("Entrances = %+v, want entity preserved after item failure", snap.Entrances)
	}
	if len(snap.Items) != 0 {
		t.Fatalf("Items = %+v, want empty after item failure", snap.Items)
	}
}

func TestLargeItemSegmentDoesNotStarveCountessEntities(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	const (
		entUnit = uintptr(0x63000)
		entPath = uintptr(0x65000)
	)
	writeJunkItemChain(access, moduleBase, off, maxItemUnitVisits)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentEntrance, entUnit)
	setupEntranceUnitNoUnitData(access, entUnit, entPath, 10, 2001)

	snap := probe.Snapshot()
	if len(snap.Entrances) != 1 {
		t.Fatalf("Entrances = %+v, want entity preserved despite large item segment", snap.Entrances)
	}
}

func TestMaxItemsPerSnapshotCountsAcceptedItems(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	firstAccepted := uintptr(0xC00000)
	writeJunkItemChain(access, moduleBase, off, 10)
	writeU64(access, 0xB00000+uintptr(9*0x200)+off.Unit.NextUnit, uint64(firstAccepted))
	seg := make([]byte, unitTableListHeads*unitTableHeadStride)
	binary.LittleEndian.PutUint64(seg[0:], uint64(0xB00000))
	binary.LittleEndian.PutUint64(seg[unitTableHeadStride:], uint64(firstAccepted+uintptr(246*0x200)))
	binary.LittleEndian.PutUint64(seg[2*unitTableHeadStride:], uint64(firstAccepted+uintptr(502*0x200)))
	access.setBytes(unitSegmentBase(moduleBase, off.UnitTable, unitSegmentItem), seg)

	prev := firstAccepted
	for i := 0; i < maxItemsPerSnapshot+2; i++ {
		unitAddr := firstAccepted + uintptr(i*0x200)
		dataAddr := uintptr(0xD00000 + uintptr(i*0x200))
		pathAddr := uintptr(0xE00000 + uintptr(i*0x200))
		setupGroundItemUnit(access, unitAddr, dataAddr, pathAddr, 0, 0, 610, uint32(5000+i), 2, 0)
		if i+1 < maxItemsPerSnapshot+2 {
			writeU64(access, unitAddr+off.Unit.NextUnit, uint64(unitAddr+0x200))
		}
		prev = unitAddr
	}
	writeU64(access, prev+off.Unit.NextUnit, 0)

	snap := probe.Snapshot()
	if len(snap.Items) != maxItemsPerSnapshot {
		t.Fatalf("Items len = %d, want cap %d", len(snap.Items), maxItemsPerSnapshot)
	}
	if snap.Items[0].UnitID != 5000 {
		t.Fatalf("first accepted item unit = %d, want junk units ignored before cap", snap.Items[0].UnitID)
	}
}
