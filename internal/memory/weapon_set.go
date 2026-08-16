package memory

import "fmt"

// WeaponSetSnapshot is fail-closed evidence for the active weapon set.
// Value `0` is primary and `1` is secondary only when Available is true.
type WeaponSetSnapshot struct {
	Value     uint8
	Available bool
}

// ConfigureWeaponSetSkillEvidence configures the two CASC-backed CTA skills
// whose joint presence distinguishes the explicit Hammerdin secondary set.
func (p *ProbeReader) ConfigureWeaponSetSkillEvidence(first, second uint16) error {
	if first == 0 || second == 0 || first == second {
		return fmt.Errorf("weapon-set skill evidence requires two distinct non-zero skill IDs")
	}
	p.weaponSetSecondarySkills = [2]uint16{first, second}
	return nil
}

func (p *ProbeReader) readActiveWeaponSetFromSkills(skills PlayerSkills) WeaponSetSnapshot {
	first, second := p.weaponSetSecondarySkills[0], p.weaponSetSecondarySkills[1]
	if first == 0 || second == 0 || !skills.Complete || skills.SkillsKnown == nil {
		return WeaponSetSnapshot{}
	}
	firstKnown := skills.SkillsKnown[first]
	secondKnown := skills.SkillsKnown[second]
	if firstKnown != secondKnown {
		return WeaponSetSnapshot{}
	}
	if firstKnown {
		return WeaponSetSnapshot{Value: 1, Available: true}
	}
	return WeaponSetSnapshot{Value: 0, Available: true}
}
