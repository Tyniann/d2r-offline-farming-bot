package memory

import (
	"fmt"
	"strings"
)

// SkillName returns the canonical catalog key for known skill IDs used in logs.
func SkillName(id uint16) string {
	if entry, ok := LookupSkillByID(id); ok {
		return entry.Key
	}
	return "unknown"
}

// IsBasicLeftSkill reports whether id is the default attack or throw skill.
func IsBasicLeftSkill(id uint16) bool {
	return id == SkillAttack || id == SkillThrow
}

// ParseSkillTestName maps CLI and config skill names onto catalog IDs.
// Explicit product aliases such as `ce` and `tp` remain first-class aliases;
// all other values resolve through the generated CASC catalog.
func ParseSkillTestName(name string) (uint16, error) {
	key, ok := resolveSkillAlias(name)
	if !ok {
		return 0, fmt.Errorf("unknown skill %q (use teleport, amplify_damage, bone_armor, corpse_explosion, bone_wall, bone_prison, bone_spear, or town_portal)", name)
	}
	entry, found := LookupSkillByKey(key)
	if !found {
		return 0, fmt.Errorf("skill catalog missing key %q", key)
	}
	return entry.ID, nil
}

func resolveSkillAlias(name string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.Join(strings.Fields(normalized), " ")
	switch normalized {
	case "teleport":
		return "teleport", true
	case "amplify_damage", "amplifydamage", "amplify damage", "ad":
		return "amplify_damage", true
	case "bone_armor", "bonearmor", "bone armor":
		return "bone_armor", true
	case "corpse_explosion", "corpseexplosion", "corpse explosion", "ce":
		return "corpse_explosion", true
	case "bone_wall", "bonewall", "bone wall":
		return "bone_wall", true
	case "bone_spear", "bonespear", "bone spear":
		return "bone_spear", true
	case "bone_prison", "boneprison", "bone prison":
		return "bone_prison", true
	case "town_portal", "townportal", "tp":
		return "town_portal", true
	default:
		compact := strings.ReplaceAll(normalized, " ", "_")
		if _, ok := LookupSkillByKey(compact); ok {
			return compact, true
		}
		return "", false
	}
}
