package memory

// ObjectUnit is a minimal object entity from the unit table object segment.
type ObjectUnit struct {
	TxtFileNo uint32
	UnitID    uint32
	PosX      uint32
	PosY      uint32
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
}
