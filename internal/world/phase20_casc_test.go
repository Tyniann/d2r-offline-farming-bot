package world

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestPhase20IDsMatchLocalCASCFixtures(t *testing.T) {
	levels := readPhase20Fixture(t, "levels.txt")
	assertFixtureUint(t, levels, "*StringName", "Stony Field", "Id", uint64(StonyField))
	assertFixtureUint(t, levels, "*StringName", "Tristram", "Id", uint64(Tristram))
	assertFixtureUint(t, levels, "*StringName", "Moo Moo Farm", "Id", uint64(MooMooFarm))

	objects := readPhase20Fixture(t, "objects.txt")
	assertFixtureUint(t, objects, "Class", "PortalPermanent", "*ID", uint64(PermanentPortalID))
	assertFixtureUint(t, objects, "Class", "Wirt", "*ID", uint64(WirtsBodyID))

	skills := readPhase20Fixture(t, "skills.txt")
	assertFixtureUint(t, skills, "skill", "Teleport", "*Id", uint64(memory.SkillTeleport))
	assertFixtureUint(t, skills, "skill", "Amplify Damage", "*Id", uint64(memory.SkillAmplifyDamage))
	assertFixtureUint(t, skills, "skill", "Corpse Explosion", "*Id", uint64(memory.SkillCorpseExplosion))
	assertFixtureUint(t, skills, "skill", "Bone Spear", "*Id", uint64(memory.SkillBoneSpear))
	assertFixtureUint(t, skills, "skill", "TownPortal", "*Id", uint64(memory.SkillTownPortal))
	if row := findPhase20FixtureRow(t, skills, "skill", "Corpse Explosion"); row["TargetCorpse"] != "1" {
		t.Fatalf("skills.txt Corpse Explosion TargetCorpse=%q, want 1", row["TargetCorpse"])
	}

	monstats := readPhase20Fixture(t, "monstats.txt")
	assertFixtureUint(t, monstats, "Id", "fallen2", "*hcIdx", uint64(Rakanishu))
	assertFixtureUint(t, monstats, "Id", "hellbovine", "*hcIdx", uint64(HellBovine))
	assertFixtureUint(t, monstats, "Id", "cowking", "*hcIdx", uint64(CowKing))
	superuniques := readPhase20Fixture(t, "superuniques.txt")
	if row := findPhase20FixtureRow(t, superuniques, "Name", "Rakanishu"); row["Class"] != "fallen2" {
		t.Fatalf("superuniques.txt Rakanishu Class=%q, want fallen2", row["Class"])
	}
	if row := findPhase20FixtureRow(t, superuniques, "Name", "The Cow King"); row["Class"] != "cowking" || row["hcIdx"] != "39" {
		t.Fatalf("superuniques.txt Cow King row=%v, want Class=cowking hcIdx=39 (not NPC ID)", row)
	}

	items := readPhase20Fixture(t, "items-derived.txt")
	for code, wantID := range map[string]uint32{"leg": 88, "tbk": 533, "box": 564} {
		row := findPhase20FixtureRow(t, items, "Code", code)
		assertFixtureUint(t, items, "Code", code, "TxtFileNo", uint64(wantID))
		if got := LookupItemCode(wantID); got != code {
			t.Fatalf("generated item catalog id=%d code=%q, want %q from %s", wantID, got, code, row["Source"])
		}
		width, height := LookupItemDimensions(wantID)
		wantWidth, _ := strconv.Atoi(row["InvWidth"])
		wantHeight, _ := strconv.Atoi(row["InvHeight"])
		if width != wantWidth || height != wantHeight {
			t.Fatalf("generated item catalog %s size=%dx%d, want %dx%d from %s", code, width, height, wantWidth, wantHeight, row["Source"])
		}
	}
}

func readPhase20Fixture(t *testing.T, name string) []map[string]string {
	t.Helper()
	path := filepath.Join("testdata", "phase20", name)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.FieldsPerRecord = '\t', -1
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		t.Fatalf("read %s: records=%d err=%v", path, len(records), err)
	}
	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(header))
		for i, column := range header {
			if i < len(record) {
				row[column] = strings.TrimSpace(record[i])
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func findPhase20FixtureRow(t *testing.T, rows []map[string]string, key, value string) map[string]string {
	t.Helper()
	for _, row := range rows {
		if row[key] == value {
			return row
		}
	}
	t.Fatalf("fixture row %s=%q missing", key, value)
	return nil
}

func assertFixtureUint(t *testing.T, rows []map[string]string, key, value, field string, want uint64) {
	t.Helper()
	row := findPhase20FixtureRow(t, rows, key, value)
	got, err := strconv.ParseUint(row[field], 10, 64)
	if err != nil {
		t.Fatalf("fixture %s=%q field %s=%q: %v", key, value, field, row[field], err)
	}
	if got != want {
		t.Fatalf("fixture %s=%q field %s=%d, want compile-time %d", key, value, field, got, want)
	}
}
