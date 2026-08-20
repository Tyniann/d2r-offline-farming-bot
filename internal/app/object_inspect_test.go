package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestValidateObjectInspectLabel(t *testing.T) {
	for _, label := range []string{"closed", "opened", "locked-with-key", "keys-in-town"} {
		if err := validateObjectInspectLabel(label); err != nil {
			t.Fatalf("label %q: %v", label, err)
		}
	}
	for _, label := range []string{"", "Closed", "has space", "../escape"} {
		if err := validateObjectInspectLabel(label); err == nil {
			t.Fatalf("label %q should fail", label)
		}
	}
}

func TestBuildObjectInspectArtifactKeepsUnknownIDsAndKeyQuantity(t *testing.T) {
	quantity := int32(7)
	state := world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.LookupArea(world.LowerKurast),
		Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Items: []world.Item{
			{Code: "key", Name: "Key", TxtFileNo: 558, UnitID: 3, Location: world.ItemLocationInventory, GridX: 0, GridY: 0, Stats: []world.ItemStat{{ID: 70, Value: quantity}}},
			{Code: "r21", Name: "Pul Rune", UnitID: 4},
		},
	}
	objects := []memory.ObjectInspectEvidence{
		{TxtFileNo: 267, UnitID: 1, PosX: 110, PosY: 100, PositionKnown: true, Mode: 1, ModeKnown: true},
		{TxtFileNo: 181, UnitID: 2, PosX: 130, PosY: 120, PositionKnown: true, Mode: 0, ModeKnown: true},
	}
	catalog := map[uint32]objectInspectCatalogEntry{
		181: {Class: "JungleChest", Name: "jungle chest"},
	}
	got := buildObjectInspectArtifact("closed", "3.2.92777", state, objects, catalog, "fixture")
	if got.AreaID != 79 || got.AreaName != "Lower Kurast" || got.ObjectCount != 2 {
		t.Fatalf("area/count = %+v", got)
	}
	if got.Objects[0].TxtFileNo != 267 || got.Objects[0].RuntimeKind != "personal_stash" || got.Objects[0].CatalogName != "Personal Stash" {
		t.Fatalf("nearest object = %+v", got.Objects[0])
	}
	if got.Objects[1].TxtFileNo != 181 || got.Objects[1].RuntimeKind != "unknown" || got.Objects[1].CatalogClass != "JungleChest" || got.Objects[1].CatalogName != "jungle chest" {
		t.Fatalf("unknown object = %+v", got.Objects[1])
	}
	if len(got.KeyStacks) != 1 || !got.KeyStacks[0].QuantityKnown || got.KeyStacks[0].QuantityStat == nil || *got.KeyStacks[0].QuantityStat != 7 {
		t.Fatalf("key stacks = %+v", got.KeyStacks)
	}
}

func TestLoadObjectInspectCatalogReadsClassAndName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "objects.txt")
	data := "Class\tName\t*Description\t*ID\nJungleChest\tjungle chest\tchest\t181\nArmorStand\tarmor stand\track\t104\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, source := loadObjectInspectCatalog(path)
	if source != path || catalog[181].Class != "JungleChest" || catalog[104].Name != "armor stand" {
		t.Fatalf("catalog = %+v source=%q", catalog, source)
	}
}

func TestSaveObjectInspectArtifactPublishesJSON(t *testing.T) {
	artifact := objectInspectArtifact{
		SchemaVersion: 1, CapturedAt: time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC),
		Label: "closed", GameVersion: "3.2.92777", AreaID: 79, ObjectCount: 1,
		Objects: []objectInspectObject{{TxtFileNo: 181, UnitID: 9, ModeKnown: true}},
	}
	path, err := saveObjectInspectArtifact(t.TempDir(), artifact)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got objectInspectArtifact
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Label != "closed" || got.AreaID != 79 || filepath.Ext(path) != ".json" {
		t.Fatalf("artifact = %+v path=%q", got, path)
	}
}
