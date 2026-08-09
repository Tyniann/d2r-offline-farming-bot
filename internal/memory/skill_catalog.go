package memory

import "fmt"

// SkillCatalogEntry is one static skill row from the CASC-backed skills.txt catalog.
type SkillCatalogEntry struct {
	Key        string
	ID         uint16
	CharClass  string
	SourceName string
	LeftSkill  bool
	RightSkill bool
	InTown     bool
	Scroll     bool
	Passive    bool
}

// LookupSkillByKey returns the catalog entry for a canonical skill key.
func LookupSkillByKey(key string) (SkillCatalogEntry, bool) {
	entry, ok := skillCatalogByKey[key]
	return entry, ok
}

// LookupSkillByID returns the catalog entry for a numeric skill ID.
func LookupSkillByID(id uint16) (SkillCatalogEntry, bool) {
	entry, ok := skillCatalogByID[id]
	return entry, ok
}

// MustSkillID resolves key against the generated catalog or panics.
// It exists for compile-time-like product wiring that cannot continue without
// a known skill identity.
func MustSkillID(key string) uint16 {
	entry, ok := LookupSkillByKey(key)
	if !ok {
		panic(fmt.Sprintf("skill catalog missing key %q", key))
	}
	return entry.ID
}

// SkillCatalogVersion returns the embedded D2R source version for the skill catalog.
func SkillCatalogVersion() string {
	return generatedSkillCatalogVersion
}
