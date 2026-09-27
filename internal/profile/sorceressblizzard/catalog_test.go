package sorceressblizzard

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestSorceressSkillsMatchLocalCASCFixture(t *testing.T) {
	// Unveränderte Spaltenauswahl aus .tmp/d2r-excel/skills.txt,
	// D2R 3.2.92777. Der stabile Zeilenschlüssel ist skill, die ID-Spalte *Id.
	f, err := os.Open("testdata/skills.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"Frozen Armor": "frozen_armor", "Static Field": "static_field", "Teleport": "teleport", "Blizzard": "blizzard", "Ice Blast": "ice_blast", "TownPortal": "town_portal"}
	if len(rows) != len(keys)+1 {
		t.Fatalf("fixture rows=%d", len(rows))
	}
	for _, row := range rows[1:] {
		skill, ok := memory.LookupSkillByKey(keys[row[0]])
		id, err := strconv.Atoi(row[1])
		if err != nil || !ok || int(skill.ID) != id || skill.SourceName != row[0] || skill.CharClass != row[2] || skill.LeftSkill != (row[3] == "1") || skill.RightSkill != (row[4] == "1") || skill.InTown != (row[5] == "1") {
			t.Fatalf("catalog=%+v source=%v err=%v", skill, row, err)
		}
		if row[0] == "Ice Blast" && row[6] != "" {
			t.Fatalf("Ice Blast gained a cast delay: %v", row)
		}
		if row[0] == "Blizzard" && row[6] != "45" {
			t.Fatalf("Blizzard delay changed: %v", row)
		}
		if row[0] == "Static Field" && (row[7] != "ln12" || row[8] != "5" || row[9] != "1") {
			t.Fatalf("Static Field minimum radius changed: %v", row)
		}
	}
}
