package world

// Player holds interpreted main-player vitals without memory pointers.
type Player struct {
	Position Position
	HP       uint32 // Current life points.
	MaxHP    uint32 // Maximum life points for percentage helpers.
	Mana     uint32 // Current mana.
	MaxMana  uint32 // Maximum mana for percentage helpers.
}

// HPPercent returns current HP as an integer percentage of MaxHP.
// Returns 0 when MaxHP is zero; clamps values above 100 to 100.
func (p Player) HPPercent() uint8 {
	if p.MaxHP == 0 {
		return 0
	}
	pct := p.HP * 100 / p.MaxHP
	if pct > 100 {
		return 100
	}
	return uint8(pct)
}

// ManaPercent returns current mana as an integer percentage of MaxMana.
// Returns 0 when MaxMana is zero; clamps values above 100 to 100.
func (p Player) ManaPercent() uint8 {
	if p.MaxMana == 0 {
		return 0
	}
	pct := p.Mana * 100 / p.MaxMana
	if pct > 100 {
		return 100
	}
	return uint8(pct)
}
