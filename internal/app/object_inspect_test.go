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
		{TxtFileNo: 240, UnitID: 2, PosX: 130, PosY: 120, PositionKnown: true, Mode: 0, ModeKnown: true},
		{TxtFileNo: 181, UnitID: 3, PosX: 125, PosY: 118, PositionKnown: true, Mode: 0, ModeKnown: true},
	}
	catalog := map[uint32]objectInspectCatalogEntry{
		181: {Class: "JungleChest", Name: "jungle chest"},
		240: {Class: "Chest5", Name: "chest"},
	}
	statLists := []memory.ItemStatListEvidence{{
		TxtFileNo: 558, UnitID: 3, StatsListExPresent: true,
		Active: []memory.RawStat{{ID: 70, Value: quantity}}, ActiveReadable: true,
		BaseReadable: true, Base: []memory.RawStat{},
	}}
	got := buildObjectInspectArtifact("closed", "3.2.92777", state, objects, statLists, catalog, "fixture")
	if got.AreaID != 79 || got.AreaName != "Lower Kurast" || got.ObjectCount != 3 {
		t.Fatalf("area/count = %+v", got)
	}
	if got.Objects[0].TxtFileNo != 267 || got.Objects[0].RuntimeKind != "personal_stash" || got.Objects[0].CatalogName != "Personal Stash" {
		t.Fatalf("nearest object = %+v", got.Objects[0])
	}
	if got.Objects[1].TxtFileNo != 181 || got.Objects[1].RuntimeKind != "super_chest" || got.Objects[1].CatalogClass != "JungleChest" {
		t.Fatalf("catalog super chest = %+v", got.Objects[1])
	}
	if got.Objects[2].TxtFileNo != 240 || got.Objects[2].RuntimeKind != "unknown" || got.Objects[2].CatalogClass != "Chest5" {
		t.Fatalf("unknown object = %+v", got.Objects[2])
	}
	if len(got.KeyStacks) != 1 || !got.KeyStacks[0].QuantityKnown || got.KeyStacks[0].QuantityStat == nil || *got.KeyStacks[0].QuantityStat != 7 || got.KeyStacks[0].QuantitySource != "active" {
		t.Fatalf("key stacks = %+v", got.KeyStacks)
	}
}

func TestBuildObjectInspectArtifactReadsQuantityFromBaseWhenActiveEmpty(t *testing.T) {
	quantity := int32(6)
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Items: []world.Item{{Code: "key", Name: "Key", TxtFileNo: 558, UnitID: 9, Location: world.ItemLocationInventory}},
	}
	statLists := []memory.ItemStatListEvidence{{
		TxtFileNo: 558, UnitID: 9, StatsListExPresent: true,
		ActiveReadable: true, Active: []memory.RawStat{},
		BaseReadable: true, Base: []memory.RawStat{{ID: 70, Layer: 0, Value: quantity}},
	}}
	got := buildObjectInspectArtifact("keys-in-town", "3.2.92777", state, nil, statLists, nil, "fixture")
	if len(got.KeyStacks) != 1 {
		t.Fatalf("key stacks = %+v", got.KeyStacks)
	}
	key := got.KeyStacks[0]
	if !key.StatsActiveReadable || len(key.StatsActive) != 0 {
		t.Fatalf("active = readable=%t stats=%+v", key.StatsActiveReadable, key.StatsActive)
	}
	if !key.QuantityKnown || key.QuantityStat == nil || *key.QuantityStat != quantity || key.QuantitySource != "base" {
		t.Fatalf("quantity = known=%t source=%q stat=%v", key.QuantityKnown, key.QuantitySource, key.QuantityStat)
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
