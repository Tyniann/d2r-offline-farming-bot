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

// ItemUnit is a primitive item entity from the unit table item segment.
type ItemUnit struct {
	TxtFileNo   uint32
	UnitID      uint32
	Quality     uint32
	RawLocation uint32
	PosX        uint32
	PosY        uint32
	Flags       uint32
	Identified  bool
	Ethereal    bool
	Stats       []RawStat
}
