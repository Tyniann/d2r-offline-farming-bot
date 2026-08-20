package memory

// ObjectUnit is a minimal object entity from the unit table object segment.
// Mode is UnitAny+0x0C, the same field object-inspect and hirelings use.
// ModeKnown is false when that read failed; Mode 0 is then not "closed".
type ObjectUnit struct {
	TxtFileNo uint32
	UnitID    uint32
	PosX      uint32
	PosY      uint32
	Mode      uint32
	ModeKnown bool
}

// EntranceUnit is a minimal entrance entity from the unit table entrance segment.
type EntranceUnit struct {
	TxtFileNo uint32
	UnitID    uint32
	PosX      uint32
	PosY      uint32
}

// MonsterUnit is a minimal living monster entity from the unit table monster segment.
type MonsterUnit struct {
	NPCID           uint32
	UnitID          uint32
	PosX            uint32
	PosY            uint32
	MonsterTypeFlag uint8
}

// CowCorpseUnit is one directly observed Hell Bovine or Cow King corpse from
// the current monster walk. ConsumptionKnown is false when the two live-
// validated Corpse Explosion state bits cannot be read consistently.
type CowCorpseUnit struct {
	NPCID            uint32
	UnitID           uint32
	PosX             uint32
	PosY             uint32
	MonsterTypeFlag  uint8
	Consumed         bool
	ConsumptionKnown bool
}

// CowRawEvidence is read-only Gate-20.0 evidence captured directly during the
// existing monster-unit walk. It is diagnostic only and does not authorize a
// Corpse Explosion cast.
type CowRawEvidence struct {
	NPCID           uint32
	UnitID          uint32
	Corpse          uint8
	Mode            uint32
	PosX            uint32
	PosY            uint32
	MonsterTypeFlag uint8
	// StateWindowOffset is relative to the unit's StatsListEx pointer.
	StateWindowOffset uint32
	// StateWindowHex preserves raw state words around both locally researched
	// StateFlags candidates. It is diagnostic and has no gameplay authority.
	StateWindowHex string
	// StateWindowComplete reports whether the complete raw window was readable.
	StateWindowComplete bool
	// Consumed is true only when both live-validated CE-consumption bits are set.
	Consumed bool
	// ConsumptionKnown reports that both consumption bits agreed in a complete
	// state window. A mismatch is treated as inconsistent and fail-closed.
	ConsumptionKnown bool
}

// RawStat is a raw D2R stat entry as read from a stat array.
type RawStat struct {
	ID    uint16
	Layer uint16
	Value int32
}

// SocketStatEvidence captures StatNumSockets from one item stat list for Gate 19.0.
// ListReadable means parseRawStats succeeded; Present means layer-0 Stat 194 was found.
// A readable list without Present is "stat missing", not an assumed zero sockets value.
type SocketStatEvidence struct {
	ListReadable bool
	Present      bool
	Value        int32
}

// ItemUnit is a primitive item entity from the unit table item segment.
type ItemUnit struct {
	TxtFileNo            uint32
	UnitID               uint32
	Quality              uint32
	UniqueSetID          int32
	UniqueSetIDAvailable bool
	RawLocation          uint32
	OwnerID              uint32
	PlayerOwned          bool
	Page                 uint32
	GridX                uint32
	GridY                uint32
	PosX                 uint32
	PosY                 uint32
	Flags                uint32
	Identified           bool
	Ethereal             bool
	Stats                []RawStat
	// SocketStatActive and SocketStatBase remain Gate diagnosis; productive
	// Sockets/SocketsAvailable/Socketed come only from decodeItemSockets.
	SocketStatActive SocketStatEvidence
	SocketStatBase   SocketStatEvidence
	Sockets          int
	SocketsAvailable bool
	Socketed         bool
	// Quantity and QuantityKnown are the fail-closed Gate-23.1 stack size.
	// Live keys keep Stat 70 on Base while Active is empty; the decoder is
	// Base-first and never treats an empty Active list as quantity 0.
	Quantity      int
	QuantityKnown bool
}
