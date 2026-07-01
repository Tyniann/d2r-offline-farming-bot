package memory

// OffsetSet holds versioned D2R module and struct offsets derived from d2go/Koolo research.
// Module-relative values (GameData, UnitTable, UI, Expansion) are patch-sensitive and must be
// re-validated after game updates; struct field offsets follow d2go player/stat layout.
type OffsetSet struct {
	Name         string
	D2RVersion   string
	Source       string
	SourceCommit string
	VerifiedAt   string
	ModuleName   string

	// Module-relative static offsets (moduleBase + offset).
	// GameData is retained for future gates; probe step 3 uses UI-based InGameGateOffset.
	GameData  uintptr
	UnitTable uintptr
	UI        uintptr
	Expansion uintptr

	Unit  UnitOffsets
	Stats StatOffsets
}

// UnitOffsets are field displacements within a player unit and related structures.
// Main-player detection follows d2go GetRawPlayerUnits: inventory flag at +0x30 or +0x70
// when expansion is active (inventory+Expansion.MainPlayerExpansion).
type UnitOffsets struct {
	UnitType    uintptr // 0x00 — player buckets use type 0; not filtered in d2go player scan
	UnitID      uintptr // 0x08
	UnitData    uintptr // 0x10
	Path        uintptr // 0x38 — pointer to path/position structure
	StatsListEx uintptr // 0x88 — pointer to extended stat list header
	Inventory   uintptr // 0x90 — pointer to inventory struct
	NextUnit    uintptr // 0x158 — next unit in bucket linked list
	SkillsList  uintptr // 0x100 — pointer to player skill list header

	// Inventory-relative main-player marker (uint16 > 0).
	MainPlayerNormal    uintptr // inventory + 0x30
	MainPlayerExpansion uintptr // inventory + 0x70

	// Expansion module chain: *(moduleBase+Expansion) then +ExpansionCharFlag.
	ExpansionCharFlag uintptr // 0x5C

	// Path-relative raw position (uint16).
	PositionX uintptr // path + 0x02
	PositionY uintptr // path + 0x06

	// Area ID pointer chain from path (all uint64 pointers unless noted).
	PathRoom1 uintptr // path + 0x20
	Room2     uintptr // room1 + 0x18
	Level     uintptr // room2 + 0x90
	Area      uintptr // level + 0x1F8 (uint32 area ID)

	// StatsListEx-relative offsets to stat list headers.
	StatsListBase   uintptr // statsListEx + 0x30
	StatsListActive uintptr // statsListEx + 0xA8 — current stats used for Life/Mana
}

// StatOffsets describe the flat stat array referenced by a stat list header.
// Layout per d2go getStatsList: header[+0]=array ptr, header[+8]=count; entries are 8 bytes.
type StatOffsets struct {
	ListPtr     uintptr // 0x00 within stat list header
	Count       uintptr // 0x08 within stat list header
	EntryStride uintptr // 8 bytes per entry
	Layer       uintptr // entry + 0x00, uint16
	ID          uintptr // entry + 0x02, uint16
	Value       uintptr // entry + 0x04, int32
	Next        uintptr // unused — d2go uses a flat stat array, not a linked list
}

// InGameGateOffset returns the module-relative address of the in-game byte gate.
// d2go IsIngame: ReadUInt(moduleBase+UI-0xA, 1) == 1.
func (o OffsetSet) InGameGateOffset() uintptr {
	if o.UI < 0xA {
		return 0
	}
	return o.UI - 0xA
}

const (
	d2goSource       = "github.com/hectorgimenez/d2go"
	d2goSourceCommit = "16d248a53591"
)

// DefaultOffsetSet returns the active offset set for Phase-1 state probing.
// Module offsets are static fallbacks; the probe scans patch-sensitive offsets at runtime.
// Validated manually with offline D2R player-state probing on the version below.
func DefaultOffsetSet() OffsetSet {
	return OffsetSet{
		Name:         "d2go-16d248a",
		D2RVersion:   "3.2.92777",
		Source:       d2goSource,
		SourceCommit: d2goSourceCommit,
		VerifiedAt:   "2026-06-25",
		ModuleName:   "D2R.exe",

		// Patch-sensitive fallbacks. Runtime scanning resolved UnitTable=0x1EB9430
		// and UI=0x1EC9134 for D2R 3.2.92777 during validation.
		GameData:  0x29C7C38,
		UnitTable: 0x22C6090,
		UI:        0x22D5D82,
		Expansion: 0x21E1D88,

		Unit: UnitOffsets{
			UnitType:            0x00,
			UnitID:              0x08,
			UnitData:            0x10,
			Path:                0x38,
			StatsListEx:         0x88,
			Inventory:           0x90,
			NextUnit:            0x158,
			SkillsList:          0x100,
			MainPlayerNormal:    0x30,
			MainPlayerExpansion: 0x70,
			ExpansionCharFlag:   0x5C,
			PositionX:           0x02,
			PositionY:           0x06,
			PathRoom1:           0x20,
			Room2:               0x18,
			Level:               0x90,
			Area:                0x1F8,
			StatsListBase:       0x30,
			StatsListActive:     0xA8,
		},
		Stats: StatOffsets{
			ListPtr:     0x00,
			Count:       0x08,
			EntryStride: 8,
			Layer:       0x00,
			ID:          0x02,
			Value:       0x04,
		},
	}
}
