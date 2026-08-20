package town

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAkaraVendorCostsMatchExtractedD2RTables(t *testing.T) {
	for code, want := range map[string]int{"hp1": 30, "hp5": 500, "mp1": 60, "mp5": 1000, "tsc": 100, "isc": 80, "key": 45} {
		got, ok := AkaraVendorCost(code)
		if !ok || got != want {
			t.Fatalf("AkaraVendorCost(%q) = %d/%v, want %d/true", code, got, ok, want)
		}
	}
	if _, ok := AkaraVendorCost("r01"); ok {
		t.Fatal("unsupported item received an Akara cost")
	}
}

func TestMaximumAkaraUnitCostCoversRestockResources(t *testing.T) {
	for resource, want := range map[RestockResource]int{RestockHealing: 500, RestockMana: 1000, RestockTownPortalScroll: 100, RestockIdentifyScroll: 80, RestockKey: 45} {
		got, ok := MaximumAkaraUnitCost(resource)
		if !ok || got != want {
			t.Fatalf("MaximumAkaraUnitCost(%q) = %d/%v, want %d/true", resource, got, ok, want)
		}
	}
	if _, ok := MaximumAkaraUnitCost("rejuvenation"); ok {
		t.Fatal("rejuvenation received a vendor cost")
	}
}

func TestAkaraCityKeyCostMatchesLocalMiscAndNpcTables(t *testing.T) {
	miscPath := filepath.Join("..", "..", ".tmp", "d2r-excel", "misc.txt")
	misc, err := os.ReadFile(miscPath)
	if err != nil {
		t.Fatalf("CASC-Datei fehlt: %s (%v). Bitte misc.txt unter .tmp/d2r-excel nachreichen.", miscPath, err)
	}
	costCol, codeCol := -1, -1
	found := false
	for i, line := range strings.Split(string(misc), "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if i == 0 {
			for col, name := range fields {
				switch name {
				case "cost":
					costCol = col
				case "code":
					codeCol = col
				}
			}
			if costCol < 0 || codeCol < 0 {
				t.Fatal("misc.txt header missing cost or code")
			}
			continue
		}
		if codeCol >= len(fields) || costCol >= len(fields) || fields[codeCol] != KeyItemCode {
			continue
		}
		if fields[0] != "Key" {
			continue
		}
		found = true
		if fields[costCol] != "45" {
			t.Fatalf("misc.txt Key cost=%q, want 45", fields[costCol])
		}
		break
	}
	if !found {
		t.Fatal("misc.txt Key row with code=key missing")
	}

	npcPath := filepath.Join("..", "..", ".tmp", "d2r-excel", "npc.txt")
	npc, err := os.ReadFile(npcPath)
	if err != nil {
		t.Fatalf("CASC-Datei fehlt: %s (%v). Bitte npc.txt unter .tmp/d2r-excel nachreichen.", npcPath, err)
	}
	sellCol := -1
	akaraFound := false
	for i, line := range strings.Split(string(npc), "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if i == 0 {
			for col, name := range fields {
				if name == "sell mult" {
					sellCol = col
				}
			}
			if sellCol < 0 {
				t.Fatal("npc.txt header missing sell mult")
			}
			continue
		}
		if len(fields) == 0 || fields[0] != "akara" {
			continue
		}
		akaraFound = true
		if sellCol >= len(fields) || fields[sellCol] != "1024" {
			t.Fatalf("npc.txt akara sell mult=%q, want 1024", fields[sellCol])
		}
		break
	}
	if !akaraFound {
		t.Fatal("npc.txt akara row missing")
	}
}
