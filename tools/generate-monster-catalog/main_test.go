package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndSelectBossesByHCIdx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monstats.txt")
	data := "Id\t*hcIdx\tNameStr\tenabled\nskeleton1\t0\tSkeleton\t1\nskeleton2\t1\tReturned\t1\nzombie2\t6\tHungry Dead\t1\nfoulcrow1\t15\tFoul Crow\t1\nfallen2\t20\tCarver\t1\nbaboon4\t51\tDoom Ape\t1\ngoatman1\t53\tMoon Clan\t1\ngoatman2\t54\tNight Clan\t1\nfallenshaman2\t59\tCarver Shaman\t1\nsandleaper4\t81\tTree Lurker\t1\nvulture3\t112\tHell Buzzard\t1\ncr_archer1\t160\tDark Ranger\t1\nsk_archer1\t170\tSkeleton Archer\t1\ncrownest1\t206\tFoul Crow Nest\t1\nzealot1\t235\tZakarumite\t1\nmephisto\t242\tMephisto\t1\nsummoner\t250\tSummoner\t1\nhellbovine\t391\tHell Bovine\t1\nnihlathakboss\t526\tNihlathak\t1\ncowking\t735\tHell Bovine\t1\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readMonsterRows(path)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := selectBosses(rows, defaultBossTargets)
	if err != nil || len(selected) != 20 || selected[0].Row.HCIdx != 0 || selected[19].Row.HCIdx != 735 {
		t.Fatalf("selected=%+v err=%v", selected, err)
	}
}

func TestVerifySuperUniqueTargetsUsesClassNotHCIdxAsNPCID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "superuniques.txt")
	data := "Name\tClass\thcIdx\nRakanishu\tfallen2\t3\nThe Cow King\tcowking\t39\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	selected := []selectedBoss{
		{Target: bossTarget{HCIdx: 20, ID: "fallen2", ConstName: "Rakanishu", Phase20: true, SuperUniqueName: "Rakanishu"}, Row: monsterRow{ID: "fallen2", HCIdx: 20}},
		{Target: bossTarget{HCIdx: 735, ID: "cowking", ConstName: "CowKing", Phase20: true, SuperUniqueName: "The Cow King"}, Row: monsterRow{ID: "cowking", HCIdx: 735}},
	}
	if err := verifySuperUniqueTargets(path, selected); err != nil {
		t.Fatal(err)
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
		{Target: bossTarget{HCIdx: 51, ID: "baboon4", ConstName: "DoomApe", LowerKurast: true}, Row: monsterRow{ID: "baboon4", HCIdx: 51, Name: "Doom Ape", Enabled: true}},
		{Target: bossTarget{HCIdx: 81, ID: "sandleaper4", ConstName: "TreeLurker", LowerKurast: true}, Row: monsterRow{ID: "sandleaper4", HCIdx: 81, Name: "Tree Lurker", Enabled: true}},
		{Target: bossTarget{HCIdx: 112, ID: "vulture3", ConstName: "HellBuzzard", LowerKurast: true}, Row: monsterRow{ID: "vulture3", HCIdx: 112, Name: "Hell Buzzard", Enabled: true}},
		{Target: bossTarget{HCIdx: 235, ID: "zealot1", ConstName: "Zakarumite", LowerKurast: true}, Row: monsterRow{ID: "zealot1", HCIdx: 235, Name: "Zakarumite", Enabled: true}},
		{Target: bossTarget{HCIdx: 242, ID: "mephisto", ConstName: "Mephisto"}, Row: monsterRow{ID: "mephisto", HCIdx: 242, Name: "Mephisto", Enabled: true}},
		{Target: bossTarget{HCIdx: 250, ID: "summoner", ConstName: "Summoner"}, Row: monsterRow{ID: "summoner", HCIdx: 250, Name: "Summoner", Enabled: true}},
		{Target: bossTarget{HCIdx: 526, ID: "nihlathakboss", ConstName: "Nihlathak"}, Row: monsterRow{ID: "nihlathakboss", HCIdx: 526, Name: "Nihlathak", Enabled: true}},
		{Target: bossTarget{HCIdx: 20, ID: "fallen2", ConstName: "Rakanishu", Phase20: true, Priority: true}, Row: monsterRow{ID: "fallen2", HCIdx: 20, Name: "Carver", Enabled: true}},
		{Target: bossTarget{HCIdx: 391, ID: "hellbovine", ConstName: "HellBovine", Phase20: true}, Row: monsterRow{ID: "hellbovine", HCIdx: 391, Name: "Hell Bovine", Enabled: true}},
		{Target: bossTarget{HCIdx: 735, ID: "cowking", ConstName: "CowKing", Phase20: true, Priority: true}, Row: monsterRow{ID: "cowking", HCIdx: 735, Name: "Hell Bovine", Enabled: true}},
	}
	memorySource, worldSource := string(renderMemory("3.2.test", bosses)), string(renderWorld("3.2.test", bosses))
	for _, want := range []string{"3.2.test", "20", "51", "81", "112", "235", "242", "250", "391", "526", "735", "DoomApe", "TreeLurker", "HellBuzzard", "Zakarumite", "Rakanishu", "HellBovine", "CowKing"} {
		if !strings.Contains(memorySource, want) || !strings.Contains(worldSource, want) {
			t.Fatalf("generated sources do not both contain %q", want)
		}
	}
}
