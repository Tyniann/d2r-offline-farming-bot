package memory

import (
	"fmt"
	"strings"
)

// Skill IDs from d2go pkg/data/skill @ 16d248a53591 (subset for bot precheck and casting).
const (
	SkillAttack        uint16 = 0
	SkillThrow         uint16 = 2
	SkillTeleport      uint16 = 54
	SkillAmplifyDamage uint16 = 66
	SkillBoneArmor     uint16 = 68
	SkillBoneWall      uint16 = 78
	SkillBoneSpear     uint16 = 84
	SkillBonePrison    uint16 = 88
	SkillTownPortal    uint16 = 359
)

// SkillName returns a stable label for known skill IDs used in logs.
func SkillName(id uint16) string {
	switch id {
	case SkillAttack:
		return "attack"
	case SkillThrow:
		return "throw"
	case SkillTeleport:
		return "teleport"
	case SkillAmplifyDamage:
		return "amplify_damage"
	case SkillBoneArmor:
		return "bone_armor"
	case SkillBoneWall:
		return "bone_wall"
	case SkillBoneSpear:
		return "bone_spear"
	case SkillBonePrison:
		return "bone_prison"
	case SkillTownPortal:
		return "town_portal"
	default:
		return "unknown"
	}
}

// IsBasicLeftSkill reports whether id is the default attack or throw skill.
func IsBasicLeftSkill(id uint16) bool {
	return id == SkillAttack || id == SkillThrow
}

// ParseSkillTestName maps CLI skill names used by --input-test to skill IDs.
func ParseSkillTestName(name string) (uint16, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "teleport":
		return SkillTeleport, nil
	case "amplify_damage", "amplifydamage", "amplify damage", "ad":
		return SkillAmplifyDamage, nil
	case "bone_armor", "bonearmor", "bone armor":
		return SkillBoneArmor, nil
	case "bone_wall", "bonewall", "bone wall":
		return SkillBoneWall, nil
	case "bone_spear", "bonespear", "bone spear":
		return SkillBoneSpear, nil
	case "bone_prison", "boneprison", "bone prison":
		return SkillBonePrison, nil
	case "town_portal", "townportal", "tp":
		return SkillTownPortal, nil
	default:
		return 0, fmt.Errorf("unknown skill %q (use teleport, amplify_damage, bone_armor, bone_wall, bone_prison, bone_spear, or town_portal)", name)
	}
}
