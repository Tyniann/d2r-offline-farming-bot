package app

import "github.com/Tyniann/d2r-offline-farming-bot/internal/memory"

// Town Portal has three live identities that represent the same configured
// requirement and the same castable RMB action:
//   - TownPortal (359): catalog action / binding key
//   - Book of Townportal (220): tome-granted list evidence
//   - Townportal O Skill (411): Sling-granted list and hotbar selection
var skillTownPortalEquivalentIDs = [...]uint16{
	memory.SkillTownPortal,
	memory.MustSkillID("book_of_townportal"),
	memory.MustSkillID("townportal_o_skill"),
}

func isTownPortalEquivalent(skillID uint16) bool {
	for _, id := range skillTownPortalEquivalentIDs {
		if skillID == id {
			return true
		}
	}
	return false
}

// rightSkillMatches reports whether the live RMB skill satisfies the wanted
// configured skill identity. Town Portal accepts any of its equivalent IDs.
func rightSkillMatches(wanted, live uint16) bool {
	if wanted == 0 || live == 0 {
		return false
	}
	if wanted == live {
		return true
	}
	return isTownPortalEquivalent(wanted) && isTownPortalEquivalent(live)
}
