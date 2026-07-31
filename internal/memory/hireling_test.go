package memory

import (
	"encoding/binary"
	"testing"
)

func TestIsHirelingClassIDMatchesLocalExcel(t *testing.T) {
	want := map[uint32]string{
		HirelingClassRogueScout:      "rogue_scout",
		HirelingClassDesertMercenary: "desert_mercenary",
		HirelingClassEasternSorceror: "eastern_sorceror",
		HirelingClassBarbarianA:      "barbarian",
		HirelingClassBarbarianB:      "barbarian",
	}
	for id, name := range want {
		if !IsHirelingClassID(id) {
			t.Fatalf("IsHirelingClassID(%d) = false", id)
		}
		if got := HirelingClassName(id); got != name {
			t.Fatalf("HirelingClassName(%d) = %q, want %q", id, got, name)
		}
	}
	for _, id := range []uint32{0, 148, 150, 198, 242, 265, 40, 56, 131} {
		if IsHirelingClassID(id) {
			t.Fatalf("IsHirelingClassID(%d) unexpectedly true", id)
		}
	}
}

func TestCollectHirelingEvidenceIncludesCorpseAndRawLife(t *testing.T) {
	access := newMockAccess()
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)

	const (
		moduleBase  = 0x100000
		unitAddr    = 0x50000
		unitData    = 0x51000
		pathAddr    = 0x52000
		statsEx     = 0x53000
		baseArray   = 0x54000
		activeArray = 0x55000
	)
	access.moduleBase = moduleBase
	setupMonsterUnit(access, unitAddr, unitData, pathAddr, HirelingClassDesertMercenary, 4242, false)
	unitBuf := make([]byte, 0x200)
	copy(unitBuf, mustReadMock(t, access, unitAddr, 0x200))
	unitBuf[unitOffsetCorpse] = 1
	binary.LittleEndian.PutUint32(unitBuf[unitOffsetMode:], 12)
	binary.LittleEndian.PutUint64(unitBuf[off.Unit.StatsListEx:], uint64(statsEx))
	access.setBytes(unitAddr, unitBuf)
	pathBuf := make([]byte, 0x10)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetMonsterX:], 111)
	binary.LittleEndian.PutUint16(pathBuf[pathOffsetMonsterY:], 222)
	access.setBytes(pathAddr, pathBuf)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, unitAddr)

	baseHeader := statsEx + off.Unit.StatsListBase
	activeHeader := statsEx + off.Unit.StatsListActive
	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 2)
	writeStatEntry(access, baseArray+0, 0, StatLife, 16384)
	writeStatEntry(access, baseArray+8, 0, StatMaxLife, 32768)
	writeU64(access, activeHeader+off.Stats.ListPtr, uint64(activeArray))
	writeU64(access, activeHeader+off.Stats.Count, 2)
	writeStatEntry(access, activeArray+0, 0, StatLife, 8192)
	writeStatEntry(access, activeArray+8, 0, StatMaxLife, 32768)

	got, err := probe.CollectHirelingEvidence()
	if err != nil {
		t.Fatalf("CollectHirelingEvidence() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	ev := got[0]
	if ev.NPCID != HirelingClassDesertMercenary || ev.UnitID != 4242 || ev.Corpse != 1 {
		t.Fatalf("identity = %+v", ev)
	}
	if !ev.ModeKnown || ev.Mode != 12 {
		t.Fatalf("mode = %+v", ev)
	}
	if !ev.PositionKnown || ev.PosX != 111 || ev.PosY != 222 {
		t.Fatalf("position = %+v", ev)
	}
	if ev.BaseLifeRaw == nil || *ev.BaseLifeRaw != 16384 || ev.ActiveLifeRaw == nil || *ev.ActiveLifeRaw != 8192 {
		t.Fatalf("raw life = %+v", ev)
	}
	if ev.BaseLifeShift8 == nil || *ev.BaseLifeShift8 != 64 {
		t.Fatalf("shift8 = %+v", ev)
	}
	if ev.BaseLifeFrac32768 == nil || *ev.BaseLifeFrac32768 != 0.5 {
		t.Fatalf("frac = %+v", ev)
	}
}

func TestCollectHirelingEvidenceSkipsNonHirelings(t *testing.T) {
	access := newMockAccess()
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)

	const (
		moduleBase = 0x100000
		unitAddr   = 0x50000
		unitData   = 0x51000
		pathAddr   = 0x52000
	)
	access.moduleBase = moduleBase
	setupMonsterUnit(access, unitAddr, unitData, pathAddr, 56, 7, false)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, unitAddr)

	got, err := probe.CollectHirelingEvidence()
	if err != nil {
		t.Fatalf("CollectHirelingEvidence() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func mustReadMock(t *testing.T, access *mockAccess, addr uintptr, size int) []byte {
	t.Helper()
	buf := make([]byte, size)
	if err := access.ReadAt(addr, buf); err != nil {
		t.Fatalf("ReadAt(%#x): %v", addr, err)
	}
	return buf
}

func TestIsRuntimeMonsterCandidateExcludesHirelingsByDefault(t *testing.T) {
	for _, id := range []uint32{
		HirelingClassRogueScout,
		HirelingClassDesertMercenary,
		HirelingClassEasternSorceror,
		HirelingClassBarbarianA,
		HirelingClassBarbarianB,
	} {
		if IsRuntimeMonsterCandidate(id, 0) {
			t.Fatalf("hireling class %d must stay outside hostile runtime candidates", id)
		}
	}
}

func TestDecodeMercenaryVitalsUsesFractionalLifeAndScaledMax(t *testing.T) {
	tests := []struct {
		name       string
		rawLife    int32
		rawMaxLife int32
		wantHP     uint32
		wantMaxHP  uint32
		wantOK     bool
	}{
		{name: "healthy live sample", rawLife: 32768, rawMaxLife: 23040, wantHP: 90, wantMaxHP: 90, wantOK: true},
		{name: "injured live sample", rawLife: 4096, rawMaxLife: 23040, wantHP: 11, wantMaxHP: 90, wantOK: true},
		{name: "life clamps", rawLife: 65536, rawMaxLife: 23040, wantHP: 90, wantMaxHP: 90, wantOK: true},
		{name: "zero max", rawLife: 4096, rawMaxLife: 0, wantOK: false},
		{name: "negative life", rawLife: -1, rawMaxLife: 23040, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hp, maxHP, ok := decodeMercenaryVitals(tt.rawLife, tt.rawMaxLife)
			if hp != tt.wantHP || maxHP != tt.wantMaxHP || ok != tt.wantOK {
				t.Fatalf("decodeMercenaryVitals() = %d/%d/%v, want %d/%d/%v", hp, maxHP, ok, tt.wantHP, tt.wantMaxHP, tt.wantOK)
			}
		})
	}
}

func TestEnumerateMonstersMapsLivingHirelingOutsideHostiles(t *testing.T) {
	access, probe, off, moduleBase := setupMercenaryEnumeration(t, 0, 2, 4096, 23040)
	snap := Snapshot{
		PosX:     100,
		PosY:     100,
		Identity: IdentityProbe{Valid: true, Confirmed: true, CharacterName: "MrHammer"},
	}
	visited := 0
	if err := probe.enumerateMonsters(moduleBase, off, &visited, &snap); err != nil {
		t.Fatalf("enumerateMonsters() error = %v", err)
	}
	if len(snap.Monsters) != 0 || snap.MonsterCoverage.EligibleMonsterCount != 0 {
		t.Fatalf("hireling leaked into hostiles: monsters=%+v coverage=%+v", snap.Monsters, snap.MonsterCoverage)
	}
	if !snap.Mercenary.HiredKnown || !snap.Mercenary.Hired || !snap.Mercenary.Alive || snap.Mercenary.Dead ||
		!snap.Mercenary.VitalsKnown || snap.Mercenary.HP != 11 || snap.Mercenary.MaxHP != 90 {
		t.Fatalf("Mercenary = %+v", snap.Mercenary)
	}
	if access.readAtCalls == 0 {
		t.Fatal("expected memory reads")
	}
}

type segmentCountingAccess struct {
	*mockAccess
	segmentAddr  uintptr
	segmentReads int
}

func (a *segmentCountingAccess) ReadAt(addr uintptr, buf []byte) error {
	if addr == a.segmentAddr && len(buf) == unitTableSegmentBytes {
		a.segmentReads++
	}
	return a.mockAccess.ReadAt(addr, buf)
}

func TestEnumerateMonstersWalksMonsterSegmentOnceForHireling(t *testing.T) {
	access, _, off, moduleBase := setupMercenaryEnumeration(t, 0, 1, 32768, 23040)
	counting := &segmentCountingAccess{
		mockAccess:  access,
		segmentAddr: unitSegmentBase(moduleBase, off.UnitTable, unitSegmentMonster),
	}
	reader := NewReader(testLogger())
	reader.Bind(counting)
	probe := NewProbeReader(reader, off)
	snap := Snapshot{Identity: IdentityProbe{Valid: true, Confirmed: true, CharacterName: "MrHammer"}}
	visited := 0
	if err := probe.enumerateMonsters(moduleBase, off, &visited, &snap); err != nil {
		t.Fatalf("enumerateMonsters() error = %v", err)
	}
	if counting.segmentReads != 1 {
		t.Fatalf("monster segment reads = %d, want exactly 1", counting.segmentReads)
	}
}

func TestEnumerateMonstersMapsDeadHirelingWithoutVitals(t *testing.T) {
	_, probe, off, moduleBase := setupMercenaryEnumeration(t, 1, 12, 0, 23040)
	snap := Snapshot{Identity: IdentityProbe{Valid: true, Confirmed: true, CharacterName: "MrHammer"}}
	visited := 0
	if err := probe.enumerateMonsters(moduleBase, off, &visited, &snap); err != nil {
		t.Fatalf("enumerateMonsters() error = %v", err)
	}
	if !snap.Mercenary.HiredKnown || !snap.Mercenary.Hired || snap.Mercenary.Alive || !snap.Mercenary.Dead ||
		snap.Mercenary.VitalsKnown || snap.Mercenary.HP != 0 || snap.Mercenary.MaxHP != 0 {
		t.Fatalf("Mercenary = %+v", snap.Mercenary)
	}
}

func TestEnumerateMonstersConfirmsNotHiredAfterThreeStableSnapshots(t *testing.T) {
	access := newMockAccess()
	access.moduleBase = 0x100000
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)
	writeSegmentHead(access, access.moduleBase, off.UnitTable, unitSegmentMonster, 0)

	for tick := 1; tick <= notHiredConfirmationSnapshots; tick++ {
		snap := Snapshot{Identity: IdentityProbe{Valid: true, Confirmed: true, CharacterName: "MrBook"}}
		visited := 0
		if err := probe.enumerateMonsters(access.moduleBase, off, &visited, &snap); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
		if tick < notHiredConfirmationSnapshots && snap.Mercenary.HiredKnown {
			t.Fatalf("tick %d confirmed NotHired too early: %+v", tick, snap.Mercenary)
		}
		if tick == notHiredConfirmationSnapshots && (!snap.Mercenary.HiredKnown || snap.Mercenary.Hired) {
			t.Fatalf("tick %d did not confirm NotHired: %+v", tick, snap.Mercenary)
		}
	}
	probe.resetMercenaryStability()
	if probe.noHirelingStableTicks != 0 {
		t.Fatalf("reset ticks = %d", probe.noHirelingStableTicks)
	}
}

func TestEnumerateMonstersRejectsMultipleHirelingsAsUnknown(t *testing.T) {
	const (
		moduleBase = uintptr(0x100000)
		firstUnit  = uintptr(0x50000)
		secondUnit = uintptr(0x51000)
	)
	access := newMockAccess()
	access.moduleBase = moduleBase
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)

	setupMonsterUnit(access, firstUnit, 0x52000, 0x53000, HirelingClassRogueScout, 1, false)
	setupMonsterUnit(access, secondUnit, 0x54000, 0x55000, HirelingClassDesertMercenary, 2, false)
	firstBuf := mustReadMock(t, access, firstUnit, 0x200)
	binary.LittleEndian.PutUint32(firstBuf[unitOffsetMode:], 1)
	binary.LittleEndian.PutUint64(firstBuf[off.Unit.NextUnit:], uint64(secondUnit))
	access.setBytes(firstUnit, firstBuf)
	secondBuf := mustReadMock(t, access, secondUnit, 0x200)
	binary.LittleEndian.PutUint32(secondBuf[unitOffsetMode:], 1)
	access.setBytes(secondUnit, secondBuf)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, firstUnit)

	snap := Snapshot{Identity: IdentityProbe{Valid: true, Confirmed: true, CharacterName: "MrHammer"}}
	visited := 0
	if err := probe.enumerateMonsters(moduleBase, off, &visited, &snap); err != nil {
		t.Fatalf("enumerateMonsters() error = %v", err)
	}
	if snap.Mercenary != (MercenarySnapshot{}) {
		t.Fatalf("multiple hirelings must map Unknown, got %+v", snap.Mercenary)
	}
}

func setupMercenaryEnumeration(
	t *testing.T,
	corpse uint8,
	mode uint32,
	rawLife int32,
	rawMaxLife int32,
) (*mockAccess, *ProbeReader, OffsetSet, uintptr) {
	t.Helper()
	const (
		moduleBase = uintptr(0x100000)
		unitAddr   = uintptr(0x50000)
		unitData   = uintptr(0x51000)
		pathAddr   = uintptr(0x52000)
		statsEx    = uintptr(0x53000)
		baseArray  = uintptr(0x54000)
	)
	access := newMockAccess()
	access.moduleBase = moduleBase
	reader := NewReader(testLogger())
	reader.Bind(access)
	off := testOffsetSet()
	probe := NewProbeReader(reader, off)

	setupMonsterUnit(access, unitAddr, unitData, pathAddr, HirelingClassRogueScout, 1, false)
	unitBuf := mustReadMock(t, access, unitAddr, 0x200)
	unitBuf[unitOffsetCorpse] = corpse
	binary.LittleEndian.PutUint32(unitBuf[unitOffsetMode:], mode)
	binary.LittleEndian.PutUint64(unitBuf[off.Unit.StatsListEx:], uint64(statsEx))
	access.setBytes(unitAddr, unitBuf)
	writeSegmentHead(access, moduleBase, off.UnitTable, unitSegmentMonster, unitAddr)

	baseHeader := statsEx + off.Unit.StatsListBase
	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 2)
	writeStatEntry(access, baseArray, 0, StatLife, rawLife)
	writeStatEntry(access, baseArray+8, 0, StatMaxLife, rawMaxLife)
	return access, probe, off, moduleBase
}
