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
