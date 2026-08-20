package world

import "testing"

func TestLookupObjectKindMapsLiveLowerKurastClasses(t *testing.T) {
	if got := LookupObjectKind(JungleChestID); got != ObjectKindSuperChest || got.String() != "super_chest" {
		t.Fatalf("JungleChest = %s, want super_chest", got)
	}
	if got := LookupObjectKind(JungleChest2ID); got != ObjectKindSuperChest {
		t.Fatalf("JungleChest2 = %s, want super_chest", got)
	}
	if got := LookupObjectKind(ArmorStand1ID); got != ObjectKindRack || LookupObjectName(ArmorStand1ID) != "Armor Stand" {
		t.Fatalf("ArmorStand1 kind/name = %s/%q", LookupObjectKind(ArmorStand1ID), LookupObjectName(ArmorStand1ID))
	}
	if got := LookupObjectKind(WeaponRack2ID); got != ObjectKindRack || LookupObjectName(WeaponRack2ID) != "Weapon Rack" {
		t.Fatalf("WeaponRack2 kind/name = %s/%q", LookupObjectKind(WeaponRack2ID), LookupObjectName(WeaponRack2ID))
	}
}

func TestLookupObjectKindKeepsUnknownIDsUnknown(t *testing.T) {
	for _, id := range []uint32{105, 106, 240, 241, 242, 455, 548, 551} {
		if got := LookupObjectKind(id); got != ObjectKindUnknown {
			t.Fatalf("id %d = %s, want unknown", id, got)
		}
	}
}

func TestParseObjectKindRoundTripsNewKinds(t *testing.T) {
	for _, kind := range []ObjectKind{ObjectKindSuperChest, ObjectKindRack, ObjectKindWirtsBody} {
		got, ok := ParseObjectKind(kind.String())
		if !ok || got != kind {
			t.Fatalf("ParseObjectKind(%q) = %s ok=%t", kind.String(), got, ok)
		}
	}
	if _, ok := ParseObjectKind("not_a_kind"); ok {
		t.Fatal("ParseObjectKind accepted an unknown label")
	}
}
