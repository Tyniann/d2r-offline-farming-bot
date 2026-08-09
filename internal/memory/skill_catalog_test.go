package memory

import "testing"

func TestGeneratedSkillCatalogResolvesProductSkills(t *testing.T) {
	if SkillCatalogVersion() != "3.2.92777" {
		t.Fatalf("catalog version = %q", SkillCatalogVersion())
	}
	cases := map[string]uint16{
		"teleport":         SkillTeleport,
		"amplify_damage":   SkillAmplifyDamage,
		"bone_armor":       SkillBoneArmor,
		"corpse_explosion": SkillCorpseExplosion,
		"bone_wall":        SkillBoneWall,
		"bone_spear":       SkillBoneSpear,
		"bone_prison":      SkillBonePrison,
		"town_portal":      SkillTownPortal,
	}
	for key, want := range cases {
		entry, ok := LookupSkillByKey(key)
		if !ok || entry.ID != want {
			t.Fatalf("LookupSkillByKey(%q) = %+v ok=%v, want id %d", key, entry, ok, want)
		}
		byID, ok := LookupSkillByID(want)
		if !ok || byID.Key != key {
			t.Fatalf("LookupSkillByID(%d) = %+v ok=%v", want, byID, ok)
		}
		if SkillName(want) != key {
			t.Fatalf("SkillName(%d) = %q, want %q", want, SkillName(want), key)
		}
	}
	if _, ok := LookupSkillByKey("missing_skill"); ok {
		t.Fatal("missing skill unexpectedly found")
	}
}

func TestParseSkillTestNameUsesCatalogAndProductAliases(t *testing.T) {
	cases := map[string]uint16{
		"teleport":         SkillTeleport,
		"tp":               SkillTownPortal,
		"town_portal":      SkillTownPortal,
		"ce":               SkillCorpseExplosion,
		"ad":               SkillAmplifyDamage,
		"bone spear":       SkillBoneSpear,
		"corpse_explosion": SkillCorpseExplosion,
	}
	for name, want := range cases {
		got, err := ParseSkillTestName(name)
		if err != nil || got != want {
			t.Fatalf("ParseSkillTestName(%q) = %d (%v), want %d", name, got, err, want)
		}
	}
	if _, err := ParseSkillTestName("not_a_skill"); err == nil {
		t.Fatal("expected unknown skill error")
	}
}

func TestGeneratedCatalogHasNoRuntimeTXTDependency(t *testing.T) {
	if len(skillCatalogByKey) == 0 || len(skillCatalogByID) == 0 {
		t.Fatal("embedded skill catalog is empty")
	}
	if len(skillCatalogByKey) != len(skillCatalogByID) {
		t.Fatalf("catalog key/id size mismatch: %d vs %d", len(skillCatalogByKey), len(skillCatalogByID))
	}
}
