package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndSelectBossesByHCIdx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monstats.txt")
	data := "Id\t*hcIdx\tNameStr\tenabled\nfallen\t0\tFallen\t1\nmephisto\t242\tMephisto\t1\nsummoner\t250\tSummoner\t1\nnihlathakboss\t526\tNihlathak\t1\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readMonsterRows(path)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := selectBosses(rows, defaultBossTargets)
	if err != nil || len(selected) != 3 || selected[0].Row.HCIdx != 242 || selected[1].Row.HCIdx != 250 || selected[2].Row.HCIdx != 526 {
		t.Fatalf("selected=%+v err=%v", selected, err)
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
	if _, err := selectBosses([]monsterRow{{ID: "not_mephisto", HCIdx: 242, Name: "Wrong", Enabled: true}}, defaultBossTargets); err == nil {
		t.Fatal("incomplete boss set accepted")
	}
}

func TestRenderedMonsterCatalogsAreVersionBoundAndSynchronized(t *testing.T) {
	bosses := []selectedBoss{
		{Target: bossTarget{HCIdx: 242, ID: "mephisto", ConstName: "Mephisto"}, Row: monsterRow{ID: "mephisto", HCIdx: 242, Name: "Mephisto", Enabled: true}},
		{Target: bossTarget{HCIdx: 250, ID: "summoner", ConstName: "Summoner"}, Row: monsterRow{ID: "summoner", HCIdx: 250, Name: "Summoner", Enabled: true}},
		{Target: bossTarget{HCIdx: 526, ID: "nihlathakboss", ConstName: "Nihlathak"}, Row: monsterRow{ID: "nihlathakboss", HCIdx: 526, Name: "Nihlathak", Enabled: true}},
	}
	memorySource, worldSource := string(renderMemory("3.2.test", bosses)), string(renderWorld("3.2.test", bosses))
	for _, want := range []string{"3.2.test", "242", "250", "526", "Mephisto", "Summoner", "Nihlathak"} {
		if !strings.Contains(memorySource, want) || !strings.Contains(worldSource, want) {
			t.Fatalf("generated sources do not both contain %q", want)
		}
	}
}
