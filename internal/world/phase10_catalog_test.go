package world

import "testing"

func TestPhase10MephistoAndDuranceCatalog(t *testing.T) {
	if Mephisto != 242 || LookupNPCName(Mephisto) != "Mephisto" {
		t.Fatalf("Mephisto catalog = id %d name %q", Mephisto, LookupNPCName(Mephisto))
	}
	for _, areaID := range []AreaID{DuranceOfHateLevel2, DuranceOfHateLevel3} {
		if area := LookupArea(areaID); !area.IsDungeon() || area.Act != Act3 {
			t.Fatalf("area %d = %+v, want Act-3 dungeon", areaID, area)
		}
	}
	for _, entry := range []struct {
		id   uint32
		kind EntranceKind
	}{
		{EntranceDuranceUpLeft, EntranceKindDuranceUp},
		{EntranceDuranceUpRight, EntranceKindDuranceUp},
		{EntranceDuranceDownLeft, EntranceKindDuranceDown},
		{EntranceDuranceDownRight, EntranceKindDuranceDown},
	} {
		if got := LookupEntranceKind(entry.id); got != entry.kind || LookupEntranceName(entry.id) == "" {
			t.Fatalf("entrance %d = %s/%q, want %s with name", entry.id, got, LookupEntranceName(entry.id), entry.kind)
		}
	}
}

func TestGeneratedBaseTierCatalog(t *testing.T) {
	want := map[string]BaseTier{"cap": BaseTierNormal, "xap": BaseTierExceptional, "uap": BaseTierElite, "hax": BaseTierNormal, "9ha": BaseTierExceptional, "7ha": BaseTierElite}
	found := map[string]BaseTier{}
	for _, entry := range itemCatalog {
		if _, ok := want[entry.Code]; ok {
			found[entry.Code] = entry.BaseTier
		}
	}
	for code, tier := range want {
		if found[code] != tier {
			t.Fatalf("base tier for %q = %q, want %q", code, found[code], tier)
		}
	}
	if got := lookupItemCatalog(615).BaseTier; got != BaseTierUnknown {
		t.Fatalf("gem tier = %q, want unknown", got)
	}
}
