package app

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func validWorldState(hp uint32) world.State {
	return world.FromSnapshot(validSnapshot(hp))
}

func TestWorldShouldLogEntityFingerprintChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 1, ID: 119}}

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on entity fingerprint change")
	}
}

func TestWorldShouldLogUIStateChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.UI.StashOpen = true
	cur.UI.InventoryOpen = true
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on UI state change")
	}
	attrs := worldLogAttrs(cur, false)
	found := map[string]bool{}
	for _, attr := range attrs {
		found[attr.Key] = true
	}
	if !found["ui_stash_open"] || !found["ui_inventory_open"] {
		t.Fatalf("missing UI attrs: %v", found)
	}
}

func TestWorldShouldLogMercenaryTransition(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Mercenary = world.Mercenary{
		HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true,
		UnitID: 1, NPCID: memory.HirelingClassRogueScout, HP: 11, MaxHP: 90,
	}
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on mercenary transition")
	}
	attrs := worldLogAttrs(cur, false)
	found := map[string]bool{}
	for _, attr := range attrs {
		found[attr.Key] = true
	}
	for _, key := range []string{"mercenary_hired_known", "mercenary_alive", "mercenary_hp", "mercenary_max_hp", "mercenary_hp_pct"} {
		if !found[key] {
			t.Fatalf("missing %s in Mercenary attrs: %v", key, found)
		}
	}
}

func TestWorldShouldLogGroundItemFingerprintChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Items = []world.Item{{TxtFileNo: 625, UnitID: 4001, Location: world.ItemLocationGround}}

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on ground item fingerprint change")
	}
}

func TestWorldShouldNotLogNonGroundItemFingerprintChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Items = []world.Item{{TxtFileNo: 625, UnitID: 4001, Location: world.ItemLocationInventory}}

	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("non-ground item change should not trigger probe log")
	}
}

func TestWorldShouldNotLogPositionOnlyWithSameFingerprint(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++

	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("position-only change with same fingerprint should not log")
	}
}

func TestWorldLogAttrsIncludesEntityCounts(t *testing.T) {
	st := validWorldState(100)
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 1}}
	attrs := worldLogAttrs(st, false)
	found := map[string]bool{}
	for _, a := range attrs {
		found[a.Key] = true
	}
	for _, key := range []string{"object_count", "entrance_count", "monster_count", "item_count", "ground_item_count"} {
		if !found[key] {
			t.Fatalf("missing log attr %q", key)
		}
	}
}

func TestWorldLogIsHeartbeat(t *testing.T) {
	if worldLogIsHeartbeat(time.Time{}, worldHeartbeat) {
		t.Fatal("zero lastLog must not be heartbeat (first invalid log uses Info)")
	}
	if !worldLogIsHeartbeat(time.Now().Add(-6*time.Second), worldHeartbeat) {
		t.Fatal("expected heartbeat after interval elapsed")
	}
	if worldLogIsHeartbeat(time.Now(), worldHeartbeat) {
		t.Fatal("unexpected heartbeat before interval elapsed")
	}
}

func TestWorldShouldLogOnValueChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(90)
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on HP change")
	}
}

func TestWorldShouldLogManaChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Mana = 40

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on mana change")
	}
}

func TestWorldShouldLogMaxHPChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.MaxHP = 120

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on max HP change")
	}
}

func TestWorldShouldNotLogOnlyPositionChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++

	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("unexpected log for position-only change within heartbeat")
	}
}

func TestWorldShouldLogPositionChangeInVerbose(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, true) {
		t.Fatal("expected log for position-only change in verbose mode")
	}
}

func TestWorldShouldLogAreaChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Area = world.LookupArea(world.AreaID(prev.Area.ID + 1))

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on area change")
	}
}

func TestWorldShouldLogPhaseChange(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Phase = world.GamePhaseLoading

	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log on phase change")
	}
}

func TestWorldShouldLogHeartbeat(t *testing.T) {
	st := validWorldState(100)
	last := time.Now().Add(-6 * time.Second)
	if !worldShouldLog(st, st, last, worldHeartbeat, false, false) {
		t.Fatal("expected log after heartbeat interval")
	}
}

func TestWorldShouldNotLogUnchanged(t *testing.T) {
	st := validWorldState(100)
	last := time.Now()
	if worldShouldLog(st, st, last, worldHeartbeat, false, false) {
		t.Fatal("unexpected log for unchanged state within heartbeat")
	}
}

func TestWorldShouldIgnoreAtChange(t *testing.T) {
	prev := validWorldState(100)
	cur := prev
	cur.At = prev.At.Add(time.Minute)
	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("At change alone must not trigger log")
	}
}

func TestWorldShouldLogInvalidReasonChange(t *testing.T) {
	prev := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	cur := world.State{Valid: false, Reason: memory.ReasonStatsUnavailable}
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log when invalid reason changes")
	}
}

func TestWorldShouldNotLogSameInvalidReason(t *testing.T) {
	prev := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	cur := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	if worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("unexpected log for same invalid reason within heartbeat")
	}
}

func TestWorldShouldLogInvalidOnHeartbeat(t *testing.T) {
	prev := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	cur := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	last := time.Now().Add(-6 * time.Second)
	if !worldShouldLog(prev, cur, last, worldHeartbeat, false, false) {
		t.Fatal("expected heartbeat log for unchanged invalid state")
	}
}

func TestWorldShouldNotLogPositionOnlyOnHeartbeat(t *testing.T) {
	prev := validWorldState(100)
	cur := validWorldState(100)
	cur.Player.Position.X++
	cur.Player.Position.Y++
	last := time.Now().Add(-6 * time.Second)

	if worldShouldLog(prev, cur, last, worldHeartbeat, false, false) {
		t.Fatal("unexpected heartbeat Info log for position-only change without verbose")
	}
	if !worldShouldLog(prev, cur, last, worldHeartbeat, false, true) {
		t.Fatal("expected heartbeat log for position-only change in verbose mode")
	}
}

func TestWorldShouldLogOnForce(t *testing.T) {
	st := validWorldState(100)
	if !worldShouldLog(st, st, time.Now(), worldHeartbeat, true, false) {
		t.Fatal("expected log when force=true after re-attach")
	}
}

func TestWorldShouldLogValidToInvalid(t *testing.T) {
	prev := validWorldState(100)
	cur := world.State{Valid: false, Reason: memory.ReasonNotInGame}
	if !worldShouldLog(prev, cur, time.Now(), worldHeartbeat, false, false) {
		t.Fatal("expected log when state becomes invalid")
	}
}

func TestWorldLogStateAfterProcessLostAndReattach(t *testing.T) {
	var lastLogged world.State
	var lastLog time.Time

	st := validWorldState(100)
	if !worldShouldLog(lastLogged, st, lastLog, worldHeartbeat, true, false) {
		t.Fatal("expected forced log after re-attach")
	}
}

func TestWorldLogAttrsValidState(t *testing.T) {
	st := validWorldState(100)
	attrs := worldLogAttrs(st, false)
	if len(attrs) == 0 {
		t.Fatal("expected attrs for valid state")
	}
	found := map[string]bool{}
	for _, a := range attrs {
		found[a.Key] = true
	}
	for _, key := range []string{"phase", "area_name", "area_id", "act", "area_kind", "hp", "max_hp", "hp_pct", "mana", "max_mana", "mana_pct", "pos_x", "pos_y", "item_count", "ground_item_count"} {
		if !found[key] {
			t.Fatalf("missing log attr %q", key)
		}
	}
}

func TestWorldLogAttrsInvalidState(t *testing.T) {
	st := world.State{Valid: false, Reason: "test_reason"}
	attrs := worldLogAttrs(st, false)
	if len(attrs) != 1 || attrs[0].Key != "reason" {
		t.Fatalf("unexpected invalid attrs: %+v", attrs)
	}
}

func TestWorldLogAttrsGroundItemsHintOnlyVerbose(t *testing.T) {
	st := validWorldState(100)
	st.Items = []world.Item{{
		TxtFileNo: 625,
		UnitID:    4001,
		Code:      "r01",
		Name:      "El Rune",
		Quality:   world.ItemQualityNormal,
		Location:  world.ItemLocationGround,
		Position:  world.Position{X: 10, Y: 20},
	}}

	for _, a := range worldLogAttrs(st, false) {
		if a.Key == "ground_items_hint" {
			t.Fatalf("%s should be absent when verbose=false", a.Key)
		}
	}

	foundHint := false
	for _, a := range worldLogAttrs(st, true) {
		if a.Key == "ground_items_hint" {
			foundHint = true
		}
	}
	if !foundHint {
		t.Fatal("ground_items_hint should be present when verbose=true")
	}
}

func TestVerboseGroundItemsHintIncludesIdentityDiagnosis(t *testing.T) {
	st := validWorldState(100)
	st.Items = []world.Item{{
		TxtFileNo: 535, UnitID: 4001, Code: "amu", Name: "Amulet",
		Quality: world.ItemQualitySet, Ethereal: true, Location: world.ItemLocationGround,
		IdentityKind: world.ItemIdentitySet, IdentityRawID: 77, IdentityAvailable: true,
		IdentityName: "Tal Rasha's Adjudication", IdentityValid: true,
	}}

	hint := verboseGroundItemsHint(st)
	for _, want := range []string{`code="amu"`, "quality=set", "ethereal=true", `identity_kind="set"`, "identity_raw_id=77", "identity_available=true", `identity_name="Tal Rasha's Adjudication"`, "identity_consistent=true"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("ground hint %q missing %q", hint, want)
		}
	}
}

func TestWorldLogAttrsGroundItemsHintFiltersBodyItems(t *testing.T) {
	st := validWorldState(100)
	st.Items = []world.Item{
		{
			TxtFileNo: 538,
			UnitID:    4001,
			Code:      "fng",
			Type:      "body",
			Name:      "Fang",
			Quality:   world.ItemQualityNormal,
			Location:  world.ItemLocationGround,
			Position:  world.Position{X: 10, Y: 20},
		},
		{
			TxtFileNo: 530,
			UnitID:    4002,
			Code:      "isc",
			Type:      "scro",
			Name:      "Scroll of Identify",
			Quality:   world.ItemQualityNormal,
			Location:  world.ItemLocationGround,
			Position:  world.Position{X: 11, Y: 21},
		},
	}

	for _, a := range worldLogAttrs(st, true) {
		if a.Key != "ground_items_hint" {
			continue
		}
		hint := a.Value.String()
		if strings.Contains(hint, "Fang") {
			t.Fatalf("ground_items_hint should filter body item, got %q", hint)
		}
		if !strings.Contains(hint, "Scroll of Identify") || !strings.Contains(hint, `type="scro"`) {
			t.Fatalf("ground_items_hint missing visible item details, got %q", hint)
		}
		return
	}
	t.Fatal("ground_items_hint missing")
}

func TestRuntimeWorldLogAttrsInventoryVerbose(t *testing.T) {
	lock, err := loot.NewInventoryLock([][]int{
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		Loot: loot.NewFilter(slog.New(slog.NewTextHandler(io.Discard, nil)), lock, &loot.Pickit{}),
	}
	st := validWorldState(100)
	st.Items = []world.Item{{
		TxtFileNo:   625,
		UnitID:      4001,
		Code:        "r01",
		Name:        "El Rune",
		Location:    world.ItemLocationInventory,
		PlayerOwned: true,
		Page:        0,
		GridX:       2,
		GridY:       0,
		Width:       1,
		Height:      1,
	}}

	attrs := rt.runtimeWorldLogAttrs(st, true)
	found := map[string]bool{}
	for _, a := range attrs {
		found[a.Key] = true
	}
	for _, key := range []string{"inventory_item_count", "inventory_free_slots", "inventory_locked_slots", "inventory_capacity_unsafe", "inventory_items_hint"} {
		if !found[key] {
			t.Fatalf("missing runtime inventory attr %q", key)
		}
	}
}

func TestVerboseInventoryItemsHintIncludesIdentityDiagnosis(t *testing.T) {
	items := []world.Item{{
		TxtFileNo: 535, UnitID: 4001, Code: "amu", Name: "Amulet",
		Quality: world.ItemQualitySet, Location: world.ItemLocationInventory,
		IdentityKind: world.ItemIdentitySet, IdentityRawID: 77, IdentityAvailable: true,
		IdentityReason: world.ItemIdentityReasonBaseMismatch,
	}}

	hint := verboseInventoryItemsHint(items)
	for _, want := range []string{`code="amu"`, "quality=set", "ethereal=false", `identity_kind="set"`, "identity_raw_id=77", "identity_available=true", "identity_consistent=false", `identity_reason="item_identity_base_mismatch"`} {
		if !strings.Contains(hint, want) {
			t.Fatalf("inventory hint %q missing %q", hint, want)
		}
	}
}

func TestRuntimeWorldLogAttrsInventoryOnlyVerbose(t *testing.T) {
	lock, err := loot.NewInventoryLock([][]int{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		Loot: loot.NewFilter(slog.New(slog.NewTextHandler(io.Discard, nil)), lock, &loot.Pickit{}),
	}
	st := validWorldState(100)
	st.Items = []world.Item{{Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Width: 1, Height: 1}}

	for _, a := range rt.runtimeWorldLogAttrs(st, false) {
		if strings.HasPrefix(a.Key, "inventory_") {
			t.Fatalf("%s should be absent when verbose=false", a.Key)
		}
	}
}
