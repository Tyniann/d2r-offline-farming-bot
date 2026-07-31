package world

// Mercenary is the semantic, fail-closed state of the player's hired
// hireling. Unknown is the zero value; absence must never be interpreted as
// Dead or NotHired by downstream consumers.
type Mercenary struct {
	HiredKnown  bool
	Hired       bool
	Alive       bool
	Dead        bool
	VitalsKnown bool
	UnitID      uint32
	NPCID       uint32
	HP          uint32
	MaxHP       uint32
}

// HPPercent returns integer Merc HP in the range 0..100. It returns zero when
// vitals are unknown or MaxHP is zero.
func (m Mercenary) HPPercent() uint8 {
	if !m.VitalsKnown || m.MaxHP == 0 {
		return 0
	}
	if m.HP >= m.MaxHP {
		return 100
	}
	return uint8(uint64(m.HP) * 100 / uint64(m.MaxHP))
}
