package memory

import (
	"encoding/binary"
	"testing"
	"time"
)

func testOffsetSet() OffsetSet {
	off := DefaultOffsetSet()
	off.UnitTable = 0x2000
	off.UI = 0x3000
	off.Expansion = 0x4000
	return off
}

func writeU16(m *mockAccess, addr uintptr, v uint16) {
	m.setBytes(addr, []byte{byte(v), byte(v >> 8)})
}

func writeU32(m *mockAccess, addr uintptr, v uint32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	m.setBytes(addr, buf)
}

func writeU64(m *mockAccess, addr uintptr, v uint64) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	m.setBytes(addr, buf)
}

func writeStatEntry(m *mockAccess, addr uintptr, layer, id uint16, value int32) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint16(buf[0:], layer)
	binary.LittleEndian.PutUint16(buf[2:], id)
	binary.LittleEndian.PutUint32(buf[4:], uint32(value))
	m.setBytes(addr, buf)
}

func setupProbeMock(t *testing.T) (*mockAccess, *ProbeReader, uintptr) {
	t.Helper()

	const moduleBase = uintptr(0x10000000)
	off := testOffsetSet()

	access := newMockAccess()
	access.moduleBase = moduleBase

	// UI buffer includes the inventory flag, in-game gate, stash flag, and loading flag.
	uiBase := moduleBase + off.UI - uiBufferBefore
	uiBuf := make([]byte, uiBufferSize)
	uiBuf[uiGateIndex] = 1
	access.setBytes(uiBase, uiBuf)

	// Expansion inactive.
	writeU64(access, moduleBase+off.Expansion, 0x50000)
	writeU16(access, 0x50000+off.Unit.ExpansionCharFlag, 0)

	// Player unit segment: list head 0 points to main player (full 128*8 segment buffer).
	const playerUnit = uintptr(0x20000)
	playerSeg := make([]byte, unitTableListHeads*unitTableHeadStride)
	binary.LittleEndian.PutUint64(playerSeg[0:], uint64(playerUnit))
	access.setBytes(moduleBase+off.UnitTable, playerSeg)
	writeU32(access, playerUnit+off.Unit.UnitID, 9001)

	// Inventory + main-player flag.
	const inventory = uintptr(0x21000)
	writeU64(access, playerUnit+off.Unit.Inventory, uint64(inventory))
	writeU16(access, inventory+off.Unit.MainPlayerNormal, 1)

	// Path and position.
	const path = uintptr(0x22000)
	writeU64(access, playerUnit+off.Unit.Path, uint64(path))
	writeU16(access, path+off.Unit.PositionX, 1234)
	writeU16(access, path+off.Unit.PositionY, 5678)

	// Area chain.
	const room1 = uintptr(0x23000)
	const room2 = uintptr(0x24000)
	const level = uintptr(0x25000)
	writeU64(access, path+off.Unit.PathRoom1, uint64(room1))
	writeU64(access, room1+off.Unit.Room2, uint64(room2))
	writeU64(access, room2+off.Unit.Level, uint64(level))
	writeU32(access, level+off.Unit.Area, 40)

	// Stats list: header lives at statsListEx + StatsListActive (d2go getStatsList).
	const statsListEx = uintptr(0x26000)
	statsHeader := statsListEx + off.Unit.StatsListActive
	const statsArray = uintptr(0x28000)
	writeU64(access, playerUnit+off.Unit.StatsListEx, uint64(statsListEx))
	writeU64(access, statsHeader+off.Stats.ListPtr, uint64(statsArray))
	writeU64(access, statsHeader+off.Stats.Count, 4)
	writeStatEntry(access, statsArray+0, 0, StatLife, 25600)
	writeStatEntry(access, statsArray+8, 0, StatMaxLife, 32000)
	writeStatEntry(access, statsArray+16, 0, StatMana, 12800)
	writeStatEntry(access, statsArray+24, 0, StatMaxMana, 19200)

	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, off)
	// Tests lay out unit-table segments at off.UnitTable; skip signature scan so
	// Snapshot uses the same offsets as writeSegmentHead / setup*Unit helpers.
	probe.offsetsResolved = true
	probe.activeOffsets = off
	probe.lastModuleBase = moduleBase
	return access, probe, moduleBase
}

func TestParseVitalStatsProbeLayout(t *testing.T) {
	access, _, _ := setupProbeMock(t)
	off := testOffsetSet()
	reader := newTestReader(access)
	reader.Bind(access)

	const statsListEx = uintptr(0x26000)
	vitals, err := parseVitalStats(reader, statsListEx+off.Unit.StatsListActive, off.Stats)
	if err != nil {
		t.Fatalf("parseVitalStats() error = %v", err)
	}
	if vitals.HP != 100 {
		t.Fatalf("HP = %d, want 100", vitals.HP)
	}
}

func TestDefaultOffsetSetMetadata(t *testing.T) {
	off := DefaultOffsetSet()
	if off.Name == "" || off.Source == "" || off.SourceCommit == "" {
		t.Fatal("offset set metadata must not be empty")
	}
	if off.GameData == 0 || off.UnitTable == 0 {
		t.Fatal("GameData and UnitTable must be non-zero")
	}
	if off.Unit.NextUnit == 0 || off.Unit.StatsListEx == 0 {
		t.Fatal("unit struct offsets must be populated")
	}
}

func TestProbeReaderFindsMainPlayer(t *testing.T) {
	_, probe, _ := setupProbeMock(t)
	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("Snapshot() invalid, reason=%q phase=%s", snap.Reason, snap.Phase)
	}
	if snap.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %s, want in_game", snap.Phase)
	}
	if snap.HP != 100 || snap.MaxHP != 125 || snap.Mana != 50 || snap.MaxMana != 75 {
		t.Fatalf("vitals = %+v, want hp=100 max_hp=125 mana=50 max_mana=75", snap)
	}
	if snap.AreaID != 40 || snap.PosX != 1234 || snap.PosY != 5678 {
		t.Fatalf("area/pos = area %d (%d,%d)", snap.AreaID, snap.PosX, snap.PosY)
	}
	if snap.PlayerPtr != 0x20000 {
		t.Fatalf("PlayerPtr = %#x, want 0x20000", snap.PlayerPtr)
	}
}

func TestProbeReaderEarlyExitAfterMainPlayer(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	// Second player in same bucket after main — would fail if scanned incorrectly.
	const otherUnit = uintptr(0x30000)
	writeU64(access, 0x20000+off.Unit.NextUnit, uint64(otherUnit))
	writeU64(access, otherUnit+off.Unit.Inventory, 0) // corrupt follower

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("Snapshot() invalid after chained units, reason=%q", snap.Reason)
	}
	_ = moduleBase
}

func TestProbeNotInGame(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	uiBase := moduleBase + off.UI - uiBufferBefore
	uiBuf := make([]byte, uiBufferSize) // gate byte 0
	access.setBytes(uiBase, uiBuf)
	access.setBytes(moduleBase+off.UnitTable, make([]byte, unitTableListHeads*unitTableHeadStride))

	snap := probe.Snapshot()
	if snap.Valid {
		t.Fatal("expected invalid snapshot when not in game")
	}
	if snap.Reason != ReasonNotInGame {
		t.Fatalf("Reason = %q, want %q", snap.Reason, ReasonNotInGame)
	}
	if snap.Phase != GamePhaseMenu {
		t.Fatalf("Phase = %s, want menu", snap.Phase)
	}
}

func TestProbeCanReadPlayerWhenInGameGateIsZero(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	uiBase := moduleBase + off.UI - uiBufferBefore
	uiBuf := make([]byte, uiBufferSize) // gate disabled semantics: byte 0 but player readable
	access.setBytes(uiBase, uiBuf)

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid snapshot despite gate=0, reason=%q", snap.Reason)
	}
}

func TestProbeStatsUnavailable(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	writeU64(access, 0x20000+testOffsetSet().Unit.StatsListEx, 0)

	snap := probe.Snapshot()
	if snap.Valid {
		t.Fatal("expected invalid snapshot when stats missing")
	}
	if snap.Reason != ReasonStatsUnavailable {
		t.Fatalf("Reason = %q, want %q", snap.Reason, ReasonStatsUnavailable)
	}
}

func TestProbeUsesBaseStatsWhenActiveStatsMissing(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	off := testOffsetSet()

	const statsListEx = uintptr(0x26000)
	activeHeader := statsListEx + off.Unit.StatsListActive
	baseHeader := statsListEx + off.Unit.StatsListBase
	const baseArray = uintptr(0x29000)

	writeU64(access, activeHeader+off.Stats.ListPtr, 0)
	writeU64(access, activeHeader+off.Stats.Count, 0)

	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 4)
	writeStatEntry(access, baseArray+0, 0, StatLife, 51200)
	writeStatEntry(access, baseArray+8, 0, StatMaxLife, 64000)
	writeStatEntry(access, baseArray+16, 0, StatMana, 25600)
	writeStatEntry(access, baseArray+24, 0, StatMaxMana, 38400)

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid snapshot from base stats, reason=%q", snap.Reason)
	}
	if snap.StatsSource != "base" {
		t.Fatalf("StatsSource = %q, want base", snap.StatsSource)
	}
	if snap.HP != 200 || snap.MaxHP != 250 || snap.Mana != 100 || snap.MaxMana != 150 {
		t.Fatalf("vitals = hp=%d max=%d mana=%d max_mana=%d", snap.HP, snap.MaxHP, snap.Mana, snap.MaxMana)
	}
}

func TestProbePrefersBaseStatsOverActiveStats(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	off := testOffsetSet()

	const statsListEx = uintptr(0x26000)
	baseHeader := statsListEx + off.Unit.StatsListBase
	const baseArray = uintptr(0x2A000)

	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 4)
	writeStatEntry(access, baseArray+0, 0, StatLife, 102400)
	writeStatEntry(access, baseArray+8, 0, StatMaxLife, 128000)
	writeStatEntry(access, baseArray+16, 0, StatMana, 51200)
	writeStatEntry(access, baseArray+24, 0, StatMaxMana, 76800)

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid snapshot, reason=%q", snap.Reason)
	}
	if snap.StatsSource != "base" {
		t.Fatalf("StatsSource = %q, want base", snap.StatsSource)
	}
	if snap.HP != 400 || snap.MaxHP != 500 || snap.Mana != 200 || snap.MaxMana != 300 {
		t.Fatalf("vitals = hp=%d max=%d mana=%d max_mana=%d", snap.HP, snap.MaxHP, snap.Mana, snap.MaxMana)
	}
}

func TestProbeNormalizesMaxVitalsFromObservedCurrent(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	off := testOffsetSet()

	const statsListEx = uintptr(0x26000)
	baseHeader := statsListEx + off.Unit.StatsListBase
	const baseArray = uintptr(0x2B000)

	writeU64(access, baseHeader+off.Stats.ListPtr, uint64(baseArray))
	writeU64(access, baseHeader+off.Stats.Count, 4)
	writeStatEntry(access, baseArray+0, 0, StatLife, int32(1254<<8))
	writeStatEntry(access, baseArray+8, 0, StatMaxLife, int32(949<<8))
	writeStatEntry(access, baseArray+16, 0, StatMana, int32(484<<8))
	writeStatEntry(access, baseArray+24, 0, StatMaxMana, int32(191<<8))

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid snapshot, reason=%q", snap.Reason)
	}
	if snap.HP != 1254 || snap.MaxHP != 1254 {
		t.Fatalf("hp/max_hp = %d/%d, want 1254/1254", snap.HP, snap.MaxHP)
	}
	if snap.Mana != 484 || snap.MaxMana != 484 {
		t.Fatalf("mana/max_mana = %d/%d, want 484/484", snap.Mana, snap.MaxMana)
	}

	writeStatEntry(access, baseArray+0, 0, StatLife, int32(1000<<8))
	writeStatEntry(access, baseArray+16, 0, StatMana, int32(300<<8))

	snap = probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid second snapshot, reason=%q", snap.Reason)
	}
	if snap.HP != 1000 || snap.MaxHP != 1254 {
		t.Fatalf("hp/max_hp after damage = %d/%d, want 1000/1254", snap.HP, snap.MaxHP)
	}
	if snap.Mana != 300 || snap.MaxMana != 484 {
		t.Fatalf("mana/max_mana after spend = %d/%d, want 300/484", snap.Mana, snap.MaxMana)
	}

	writeStatEntry(access, baseArray+0, 0, StatLife, int32(800<<8))
	writeStatEntry(access, baseArray+16, 0, StatMana, int32(100<<8))

	snap = probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid snapshot below stated cap, reason=%q", snap.Reason)
	}
	if snap.HP != 800 || snap.MaxHP != 1254 {
		t.Fatalf("hp/max_hp below stated cap = %d/%d, want 800/1254", snap.HP, snap.MaxHP)
	}
	if snap.Mana != 100 || snap.MaxMana != 484 {
		t.Fatalf("mana/max_mana below stated cap = %d/%d, want 100/484", snap.Mana, snap.MaxMana)
	}

	writeStatEntry(access, baseArray+0, 0, StatLife, int32(949<<8))
	writeStatEntry(access, baseArray+8, 0, StatMaxLife, int32(949<<8))
	writeStatEntry(access, baseArray+16, 0, StatMana, int32(191<<8))
	writeStatEntry(access, baseArray+24, 0, StatMaxMana, int32(191<<8))

	snap = probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid third snapshot, reason=%q", snap.Reason)
	}
	if snap.HP != 949 || snap.MaxHP != 949 {
		t.Fatalf("hp/max_hp after buff expiry = %d/%d, want 949/949", snap.HP, snap.MaxHP)
	}
	if snap.Mana != 191 || snap.MaxMana != 191 {
		t.Fatalf("mana/max_mana after buff expiry = %d/%d, want 191/191", snap.Mana, snap.MaxMana)
	}
}

func TestProbePlayerPointerUnavailable(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	access.setBytes(moduleBase+off.UnitTable, make([]byte, unitTableListHeads*unitTableHeadStride))

	snap := probe.Snapshot()
	if snap.Valid {
		t.Fatal("expected invalid snapshot without player")
	}
	if snap.Reason != ReasonPlayerPointerUnavailable {
		t.Fatalf("Reason = %q, want %q", snap.Reason, ReasonPlayerPointerUnavailable)
	}
}

func TestParseVitalStatsMissingStat(t *testing.T) {
	access := newMockAccess()
	const header = uintptr(0xA000)
	const array = uintptr(0xB000)
	off := DefaultOffsetSet().Stats

	writeU64(access, header+off.ListPtr, uint64(array))
	writeU64(access, header+off.Count, 1)
	writeStatEntry(access, array, 0, StatLife, 25600)

	reader := newTestReader(access)
	reader.Bind(access)

	_, err := parseVitalStats(reader, header, off)
	if err == nil {
		t.Fatal("expected error when vital stats incomplete")
	}
}

func TestScaleVitalStat(t *testing.T) {
	if got := scaleVitalStat(StatLife, 25600); got != 100 {
		t.Fatalf("scaleVitalStat() = %d, want 100", got)
	}
}

func TestSnapshotSetsTimestamp(t *testing.T) {
	_, probe, _ := setupProbeMock(t)
	before := time.Now()
	snap := probe.Snapshot()
	if snap.At.Before(before) {
		t.Fatal("snapshot timestamp should be current")
	}
}

func TestProbeNotAttached(t *testing.T) {
	probe := NewProbeReader(newTestReader(nil), DefaultOffsetSet())
	snap := probe.Snapshot()
	if snap.Valid || snap.Reason != ReasonNotAttached {
		t.Fatalf("Snapshot() = %+v, want invalid not_attached", snap)
	}
}

func TestUnitTableLoopProtection(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	seg := make([]byte, unitTableListHeads*unitTableHeadStride)
	binary.LittleEndian.PutUint64(seg[0:], 0x20000)
	access.setBytes(moduleBase+off.UnitTable, seg)
	writeU64(access, 0x21000+off.Unit.MainPlayerNormal, 0)
	writeU64(access, 0x21000+off.Unit.MainPlayerExpansion, 0)
	writeU64(access, 0x20000+off.Unit.NextUnit, 0x20000)

	snap := probe.Snapshot()
	_ = moduleBase
	if snap.Valid {
		t.Fatal("expected invalid snapshot without main player")
	}
	if snap.Reason != ReasonPlayerPointerUnavailable {
		t.Fatalf("Reason = %q, want %q", snap.Reason, ReasonPlayerPointerUnavailable)
	}
}

func TestMainPlayerExpansionFlag(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	writeU16(access, 0x50000+off.Unit.ExpansionCharFlag, 1)
	writeU16(access, 0x21000+off.Unit.MainPlayerNormal, 0)
	writeU16(access, 0x21000+off.Unit.MainPlayerExpansion, 1)

	_ = moduleBase
	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expansion main player: invalid, reason=%q", snap.Reason)
	}
}

func TestMainPlayerExpansionFlagFallbackWhenExpansionOffsetUnknown(t *testing.T) {
	access, _, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	off.Expansion = 0
	writeU16(access, 0x21000+off.Unit.MainPlayerNormal, 0)
	writeU16(access, 0x21000+off.Unit.MainPlayerExpansion, 1)

	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, off)

	_ = moduleBase
	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("fallback expansion main player: invalid, reason=%q", snap.Reason)
	}
}

func TestProbeUnitTableOffsetZero(t *testing.T) {
	access, _, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	off.UnitTable = 0

	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, off)

	_ = moduleBase
	snap := probe.Snapshot()
	if snap.Valid {
		t.Fatal("expected invalid snapshot when UnitTable offset is zero")
	}
	if snap.Reason != ReasonUnitTableUnavailable {
		t.Fatalf("Reason = %q, want %q", snap.Reason, ReasonUnitTableUnavailable)
	}
}

func TestProbeReadFailedOnNullPath(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	writeU64(access, 0x20000+testOffsetSet().Unit.Path, 0)

	snap := probe.Snapshot()
	if snap.Valid {
		t.Fatal("expected invalid snapshot when path pointer is null")
	}
	if snap.Reason != ReasonReadFailed {
		t.Fatalf("Reason = %q, want %q", snap.Reason, ReasonReadFailed)
	}
}

func TestProbeReasonConstants(t *testing.T) {
	reasons := []string{
		ReasonNotAttached,
		ReasonNotInGame,
		ReasonUnitTableUnavailable,
		ReasonPlayerPointerUnavailable,
		ReasonStatsUnavailable,
		ReasonReadFailed,
	}
	seen := make(map[string]struct{}, len(reasons))
	for _, r := range reasons {
		if r == "" {
			t.Fatal("reason constant must not be empty")
		}
		if _, dup := seen[r]; dup {
			t.Fatalf("duplicate reason constant %q", r)
		}
		seen[r] = struct{}{}
	}
}

func TestProbeSnapshotEntitiesOnlyWhenInGame(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	uiBase := moduleBase + off.UI - uiBufferBefore
	buf := make([]byte, uiBufferSize)
	buf[uiGateIndex] = 1
	buf[uiLoadingIndex] = 1
	access.setBytes(uiBase, buf)

	snap := probe.Snapshot()
	if !snap.Valid {
		t.Fatalf("expected valid snapshot with readable player during loading, reason=%q", snap.Reason)
	}
	if snap.Phase != GamePhaseLoading {
		t.Fatalf("Phase = %s, want loading", snap.Phase)
	}
	if len(snap.Objects) != 0 || len(snap.Entrances) != 0 || len(snap.Monsters) != 0 {
		t.Fatal("loading snapshot should have empty entity slices")
	}
	if snap.Objects == nil || snap.Entrances == nil || snap.Monsters == nil {
		t.Fatal("entity slices should be non-nil")
	}
}

// TestProbePlayerBucketsLayout documents d2go behavior: player units live in buckets
// 0–127 at moduleBase+UnitTable+i*8; no per-unit type filter is applied in that scan.
func TestProbePlayerBucketsLayout(t *testing.T) {
	if unitTableBuckets != 128 {
		t.Fatalf("unitTableBuckets = %d, want 128 (d2go GetRawPlayerUnits)", unitTableBuckets)
	}
}

func TestProbeSnapshotGenerationAdvancesAcrossFreshReads(t *testing.T) {
	_, probe, _ := setupProbeMock(t)
	first := probe.Snapshot()
	second := probe.Snapshot()
	// Windows may return the same wall-clock value for two immediate reads;
	// Generation is the authoritative freshness signal.
	if first.Generation == 0 || second.Generation != first.Generation+1 || first.At.IsZero() || second.At.IsZero() {
		t.Fatalf("first generation/time=%d/%v second=%d/%v", first.Generation, first.At, second.Generation, second.At)
	}
}
