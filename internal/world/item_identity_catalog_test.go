package world

import "testing"

func TestGeneratedItemIdentityCatalogCoverage(t *testing.T) {
	if ItemIdentityCatalogVersion() != "3.2.92777" {
		t.Fatalf("catalog version = %q", ItemIdentityCatalogVersion())
	}
	entries := ItemIdentityCatalogEntries()
	sets, uniques := 0, 0
	talRasha := map[string]string{}
	for _, entry := range entries {
		switch entry.Kind {
		case ItemIdentitySet:
			sets++
		case ItemIdentityUnique:
			uniques++
		default:
			t.Fatalf("unknown identity kind %q", entry.Kind)
		}
		if entry.SetKey == "Tal Rasha's Wrappings" {
			talRasha[entry.Key] = entry.BaseCode
		}
	}
	if sets != 140 || uniques != 433 {
		t.Fatalf("catalog sets=%d uniques=%d", sets, uniques)
	}
	if len(talRasha) != 5 || talRasha["Tal Rasha's Adjudication"] != "amu" {
		t.Fatalf("Tal Rasha catalog = %+v", talRasha)
	}
}

func TestItemIdentityCatalogLookupsAreKindSafeAndDefensive(t *testing.T) {
	entry, ok := LookupItemIdentity(ItemIdentitySet, 77)
	if !ok || entry.Key != "Tal Rasha's Adjudication" || entry.DisplayName != "Tal Rasha's Adjudication" || entry.BaseCode != "amu" {
		t.Fatalf("set identity = %+v ok=%t", entry, ok)
	}
	if _, uniqueFound := LookupItemIdentity(ItemIdentityUnique, 77); !uniqueFound {
		t.Fatal("overlapping unique ID space was not independently addressable")
	}
	byKey, ok := LookupItemIdentityKey(ItemIdentitySet, "tal rasha's adjudication")
	if !ok || byKey.RawID != 77 {
		t.Fatalf("key lookup = %+v ok=%t", byKey, ok)
	}
	unique, ok := LookupItemIdentityKey(ItemIdentityUnique, "Harlequin Crest")
	if !ok || unique.DisplayName != "Harlequin Crest" || unique.BaseCode == "" {
		t.Fatalf("selected unique lookup = %+v ok=%t", unique, ok)
	}
	setAmulets := 0
	for _, candidate := range ItemIdentityCatalogEntries() {
		if candidate.Kind == ItemIdentitySet && candidate.BaseCode == "amu" {
			setAmulets++
		}
	}
	if setAmulets < 2 {
		t.Fatalf("set amulet coverage = %d, want multiple exact identities", setAmulets)
	}
	entries := ItemIdentityCatalogEntries()
	entries[0].Key = "mutated"
	again := ItemIdentityCatalogEntries()
	if again[0].Key == "mutated" {
		t.Fatal("catalog copy mutated generated authority")
	}
}
