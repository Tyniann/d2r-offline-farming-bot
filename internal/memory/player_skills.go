package memory

import "fmt"

const (
	// MaxPlayerSkillListNodes is the hard walk budget for the learned-skill linked list.
	MaxPlayerSkillListNodes = 128
	// SkillListIncompleteRead marks a mid-walk memory read failure.
	SkillListIncompleteRead = "skill_list_read_failed"
	// SkillListLimitExceeded marks a list that continues past MaxPlayerSkillListNodes.
	SkillListLimitExceeded = "skill_list_limit_exceeded"
)

// PlayerSkills holds learned skills and current mouse skill selection.
// SkillsKnown is authoritative for Missing-Skill decisions only when Complete is true.
type PlayerSkills struct {
	LeftSkill        uint16
	RightSkill       uint16
	SkillsKnown      map[uint16]bool
	Complete         bool
	IncompleteReason string
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

	known, complete, reason := p.collectKnownSkills(uintptr(skillListPtr))
	out.SkillsKnown = known
	out.Complete = complete
	out.IncompleteReason = reason
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

// collectKnownSkills walks the learned-skill linked list until a nil next pointer.
// Any mid-walk read failure or an unterminated 128-node list yields Complete=false
// so callers must not treat SkillsKnown as a Missing-Skill verdict.
func (p *ProbeReader) collectKnownSkills(skillListPtr uintptr) (map[uint16]bool, bool, string) {
	known := make(map[uint16]bool)
	skillPtr, err := p.reader.ReadUint64(skillListPtr)
	if err != nil {
		return known, false, SkillListIncompleteRead
	}
	if skillPtr == 0 {
		return known, true, ""
	}

	ptr := uintptr(skillPtr)
	for i := 0; i < MaxPlayerSkillListNodes; i++ {
		skillTxt, err := p.reader.ReadUint64(ptr)
		if err != nil {
			return known, false, SkillListIncompleteRead
		}
		if skillTxt == 0 {
			return known, false, SkillListIncompleteRead
		}
		skillID, err := p.reader.ReadUint16(uintptr(skillTxt))
		if err != nil {
			return known, false, SkillListIncompleteRead
		}
		if skillID != 0 {
			known[skillID] = true
		}
		next, err := p.reader.ReadUint64(ptr + 0x08)
		if err != nil {
			return known, false, SkillListIncompleteRead
		}
		if next == 0 {
			return known, true, ""
		}
		ptr = uintptr(next)
	}
	return known, false, SkillListLimitExceeded
}

// HasSkill reports whether a complete skill list contains the given skill ID.
// Incomplete lists always return false so callers cannot invent Missing results.
func (ps PlayerSkills) HasSkill(skillID uint16) bool {
	if !ps.Complete || ps.SkillsKnown == nil {
		return false
	}
	return ps.SkillsKnown[skillID]
}
