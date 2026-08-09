package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeSkillKeyTownPortalAndSpacedNames(t *testing.T) {
	cases := map[string]string{
		"TownPortal":       "town_portal",
		"Teleport":         "teleport",
		"Bone Spear":       "bone_spear",
		"Amplify Damage":   "amplify_damage",
		"Corpse Explosion": "corpse_explosion",
		"Attack":           "attack",
	}
	for source, want := range cases {
		got, err := canonicalizeSkillKey(source)
		if err != nil || got != want {
			t.Fatalf("%q -> %q (%v), want %q", source, got, err, want)
		}
	}
}

func TestReadSkillRowsRequiresHeaderAndRejectsBrokenRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.txt")
	if err := os.WriteFile(path, []byte("skill\tName\nAttack\t0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSkillRows(path); err == nil || !strings.Contains(err.Error(), "*id") {
		t.Fatalf("missing-column error = %v", err)
	}

	validHeader := "skill\t*Id\tcharclass\tskilldesc\tleftskill\trightskill\tInTown\tscroll\tpassive\n"
	if err := os.WriteFile(path, []byte(validHeader+"Attack\tx\t\t\t1\t1\t\t\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSkillRows(path); err == nil || !strings.Contains(err.Error(), "invalid *Id") {
		t.Fatalf("invalid-id error = %v", err)
	}

	if err := os.WriteFile(path, []byte(validHeader+
		"Attack\t0\t\t\t1\t1\t\t\t\n"+
		"Throw\t0\t\t\t1\t1\t\t\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSkillRows(path); err == nil || !strings.Contains(err.Error(), "duplicate skill id") {
		t.Fatalf("duplicate-id error = %v", err)
	}

	if err := os.WriteFile(path, []byte(validHeader+
		"Bone Spear\t84\tnec\t\t1\t1\t\t\t\n"+
		"Bone  Spear\t85\tnec\t\t1\t1\t\t\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSkillRows(path); err == nil || !strings.Contains(err.Error(), "duplicate skill key") {
		t.Fatalf("duplicate-key error = %v", err)
	}
}

func TestReadSkillRowsFixtureContainsProductIDs(t *testing.T) {
	path := filepath.Join("testdata", "skills_fixture.txt")
	rows, err := readSkillRows(path)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]skillRow{}
	for _, row := range rows {
		byKey[row.Key] = row
	}
	want := map[string]uint16{
		"teleport":         54,
		"amplify_damage":   66,
		"bone_armor":       68,
		"corpse_explosion": 74,
		"bone_wall":        78,
		"bone_spear":       84,
		"bone_prison":      88,
		"town_portal":      359,
	}
	for key, id := range want {
		row, ok := byKey[key]
		if !ok || row.ID != id {
			t.Fatalf("%s = %+v, want id %d", key, row, id)
		}
	}
	if err := validateProductAliases(rows); err != nil {
		t.Fatal(err)
	}
}

func TestRenderIsDeterministicAndVersionBound(t *testing.T) {
	rows := []skillRow{
		{SourceName: "Teleport", Key: "teleport", ID: 54, CharClass: "sor", RightSkill: true},
		{SourceName: "TownPortal", Key: "town_portal", ID: 359},
		{SourceName: "Attack", Key: "attack", ID: 0, LeftSkill: true, RightSkill: true},
		{SourceName: "Throw", Key: "throw", ID: 2, LeftSkill: true, RightSkill: true},
		{SourceName: "Amplify Damage", Key: "amplify_damage", ID: 66, CharClass: "nec", RightSkill: true},
		{SourceName: "Bone Armor", Key: "bone_armor", ID: 68, CharClass: "nec", RightSkill: true, InTown: true},
		{SourceName: "Corpse Explosion", Key: "corpse_explosion", ID: 74, CharClass: "nec", RightSkill: true},
		{SourceName: "Bone Wall", Key: "bone_wall", ID: 78, CharClass: "nec", RightSkill: true},
		{SourceName: "Bone Spear", Key: "bone_spear", ID: 84, CharClass: "nec", LeftSkill: true, RightSkill: true},
		{SourceName: "Bone Prison", Key: "bone_prison", ID: 88, CharClass: "nec", RightSkill: true},
	}
	first, err := render(supportedSourceVersion, rows)
	if err != nil {
		t.Fatal(err)
	}
	second, err := render(supportedSourceVersion, rows)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("render output is not deterministic")
	}
	text := string(first)
	for _, want := range []string{
		"D2R " + supportedSourceVersion,
		"SkillTeleport",
		"SkillTownPortal",
		`"teleport"`,
		`"town_portal"`,
		"54:",
		"359:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated source missing %q", want)
		}
	}
}

func TestValidateSourceVersionRejectsDrift(t *testing.T) {
	if err := validateSourceVersion("3.2.wrong"); err == nil {
		t.Fatal("expected unsupported version error")
	}
}
