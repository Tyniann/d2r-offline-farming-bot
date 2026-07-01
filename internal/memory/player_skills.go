package memory

import "fmt"

// PlayerSkills holds learned skills and current mouse skill selection.
type PlayerSkills struct {
	LeftSkill   uint16
	RightSkill  uint16
	SkillsKnown map[uint16]bool
}

// readPlayerSkills reads skill list and LMB/RMB selection from the main player unit.
func (p *ProbeReader) readPlayerSkills(playerPtr uintptr, off OffsetSet) (PlayerSkills, error) {
	var out PlayerSkills
	if playerPtr == 0 {
		return out, fmt.Errorf("player pointer is null")
	}

	skillListPtr, err := p.reader.ReadUint64(playerPtr + off.Unit.SkillsList)
	if err != nil || skillListPtr == 0 {
		return out, fmt.Errorf("skill list pointer unavailable")
	}

	leftID, rightID, err := p.readMouseSkills(uintptr(skillListPtr))
	if err != nil {
		return out, err
	}
	out.LeftSkill = leftID
	out.RightSkill = rightID

	known, err := p.collectKnownSkills(uintptr(skillListPtr))
	if err != nil {
		return out, err
	}
	out.SkillsKnown = known
	return out, nil
}

func (p *ProbeReader) readMouseSkills(skillListPtr uintptr) (left, right uint16, err error) {
	leftSkillPtr, err := p.reader.ReadUint64(skillListPtr + 0x08)
	if err != nil || leftSkillPtr == 0 {
		return 0, 0, fmt.Errorf("left skill pointer unavailable")
	}
	leftTxt, err := p.reader.ReadUint64(uintptr(leftSkillPtr))
	if err != nil || leftTxt == 0 {
		return 0, 0, fmt.Errorf("left skill txt unavailable")
	}
	leftVal, err := p.reader.ReadUint16(uintptr(leftTxt))
	if err != nil {
		return 0, 0, err
	}

	rightSkillPtr, err := p.reader.ReadUint64(skillListPtr + 0x10)
	if err != nil || rightSkillPtr == 0 {
		return leftVal, 0, fmt.Errorf("right skill pointer unavailable")
	}
	rightTxt, err := p.reader.ReadUint64(uintptr(rightSkillPtr))
	if err != nil || rightTxt == 0 {
		return leftVal, 0, fmt.Errorf("right skill txt unavailable")
	}
	rightVal, err := p.reader.ReadUint16(uintptr(rightTxt))
	if err != nil {
		return leftVal, 0, err
	}
	return leftVal, rightVal, nil
}

func (p *ProbeReader) collectKnownSkills(skillListPtr uintptr) (map[uint16]bool, error) {
	known := make(map[uint16]bool)
	skillPtr, err := p.reader.ReadUint64(skillListPtr)
	if err != nil || skillPtr == 0 {
		return known, nil
	}

	const maxSkills = 128
	ptr := uintptr(skillPtr)
	for i := 0; i < maxSkills && ptr != 0; i++ {
		skillTxt, err := p.reader.ReadUint64(ptr)
		if err != nil || skillTxt == 0 {
			break
		}
		skillID, err := p.reader.ReadUint16(uintptr(skillTxt))
		if err != nil {
			break
		}
		if skillID != 0 {
			known[skillID] = true
		}
		next, err := p.reader.ReadUint64(ptr + 0x08)
		if err != nil {
			break
		}
		ptr = uintptr(next)
	}
	return known, nil
}

// HasSkill reports whether the player has learned the given skill.
func (ps PlayerSkills) HasSkill(skillID uint16) bool {
	if ps.SkillsKnown == nil {
		return false
	}
	return ps.SkillsKnown[skillID]
}
