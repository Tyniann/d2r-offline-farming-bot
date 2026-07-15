package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndSelectMephistoByHCIdx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monstats.txt")
	data := "Id\t*hcIdx\tNameStr\tenabled\nfallen\t0\tFallen\t1\nmephisto\t242\tMephisto\t1\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readMonsterRows(path)
	if err != nil {
		t.Fatal(err)
	}
	row, err := selectMephisto(rows)
	if err != nil || row.HCIdx != 242 || row.Name != "Mephisto" {
		t.Fatalf("row=%+v err=%v", row, err)
	}
}

func TestMonsterCatalogRejectsBrokenTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monstats.txt")
	if err := os.WriteFile(path, []byte("Id\tNameStr\tenabled\nmephisto\tMephisto\t1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readMonsterRows(path); err == nil || !strings.Contains(err.Error(), "*hcidx") {
		t.Fatalf("missing-column error = %v", err)
	}
	if _, err := selectMephisto([]monsterRow{{ID: "not_mephisto", HCIdx: 242, Name: "Wrong", Enabled: true}}); err == nil {
		t.Fatal("invalid Mephisto row accepted")
	}
}

func TestRenderedMonsterCatalogsAreVersionBoundAndSynchronized(t *testing.T) {
	row := monsterRow{ID: "mephisto", HCIdx: 242, Name: "Mephisto", Enabled: true}
	memorySource, worldSource := string(renderMemory("3.2.test", row)), string(renderWorld("3.2.test", row))
	for _, want := range []string{"3.2.test", "242", "Mephisto"} {
		if !strings.Contains(memorySource, want) || !strings.Contains(worldSource, want) {
			t.Fatalf("generated sources do not both contain %q", want)
		}
	}
}
