package world

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func validSnapshot() memory.Snapshot {
	return memory.Snapshot{
		At:      time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		Valid:   true,
		Phase:   memory.GamePhaseInGame,
		AreaID:  uint32(BlackMarsh),
		PosX:    1234,
		PosY:    5678,
		HP:      100,
		MaxHP:   125,
		Mana:    50,
		MaxMana: 75,
	}
}

func TestFromSnapshotValid(t *testing.T) {
	snap := validSnapshot()
	snap.UI = memory.UIState{InventoryOpen: true, StashOpen: true, QuitMenuOpen: true}
	state := FromSnapshot(snap)

	if !state.Valid {
		t.Fatal("Valid = false, want true")
	}
	if state.Reason != "" {
		t.Fatalf("Reason = %q, want empty", state.Reason)
	}
	if state.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %v, want GamePhaseInGame", state.Phase)
	}
	if state.Objects == nil || state.Entrances == nil || state.Monsters == nil || state.Items == nil {
		t.Fatal("valid snapshot should have non-nil entity/item slices")
	}
	if !state.At.Equal(snap.At) {
		t.Fatalf("At = %v, want %v", state.At, snap.At)
	}
	if state.Area.Name != "Black Marsh" {
		t.Fatalf("Area.Name = %q, want Black Marsh", state.Area.Name)
	}
	if state.Area.Act != Act1 {
		t.Fatalf("Area.Act = %v, want Act1", state.Area.Act)
	}
	if state.Area.Kind != AreaKindOutdoor {
		t.Fatalf("Area.Kind = %v, want AreaKindOutdoor", state.Area.Kind)
	}
	if state.Player.Position.X != snap.PosX || state.Player.Position.Y != snap.PosY {
		t.Fatalf("Position = (%d,%d), want (%d,%d)",
			state.Player.Position.X, state.Player.Position.Y, snap.PosX, snap.PosY)
	}
	if state.Player.HP != snap.HP || state.Player.MaxHP != snap.MaxHP {
		t.Fatalf("HP = %d/%d, want %d/%d", state.Player.HP, state.Player.MaxHP, snap.HP, snap.MaxHP)
	}
	if state.Player.Mana != snap.Mana || state.Player.MaxMana != snap.MaxMana {
		t.Fatalf("Mana = %d/%d, want %d/%d", state.Player.Mana, state.Player.MaxMana, snap.Mana, snap.MaxMana)
	}
	if !state.UI.InventoryOpen || !state.UI.StashOpen || !state.UI.QuitMenuOpen {
		t.Fatalf("UI = %+v, want inventory, stash, and quit menu open", state.UI)
	}
}

func TestFromSnapshotMapsOnlyConfirmedIdentity(t *testing.T) {
	snap := validSnapshot()
	snap.Identity = memory.IdentityProbe{Valid: true, Confirmed: true, CharacterName: "MrBones", ClassID: 2, MapSeed: 123}
	state := FromSnapshot(snap)
	if !state.Identity.Valid || state.Identity.CharacterName != "MrBones" || state.Identity.Class != CharacterClassNecromancer || state.Identity.MapSeed != 123 {
		t.Fatalf("Identity = %+v", state.Identity)
	}
	snap.Identity.Confirmed = false
	if got := FromSnapshot(snap).Identity; got != (GameIdentity{}) {
		t.Fatalf("unconfirmed identity mapped as %+v", got)
	}
}

func TestFromSnapshotGamePhaseFromSnapshot(t *testing.T) {
	snap := validSnapshot()
	snap.Phase = memory.GamePhaseInGame
	state := FromSnapshot(snap)
	if state.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %v, want GamePhaseInGame", state.Phase)
	}
}

func TestFromSnapshotEffectiveMaxValues(t *testing.T) {
	snap := validSnapshot()
	snap.HP = 100
	snap.MaxHP = 125
	snap.Mana = 50
	snap.MaxMana = 75

	state := FromSnapshot(snap)
	if state.Player.HP != 100 || state.Player.MaxHP != 125 {
		t.Fatalf("HP = %d/%d, want 100/125", state.Player.HP, state.Player.MaxHP)
	}
	if state.Player.Mana != 50 || state.Player.MaxMana != 75 {
		t.Fatalf("Mana = %d/%d, want 50/75", state.Player.Mana, state.Player.MaxMana)
	}
	if state.Player.HPPercent() != 80 {
		t.Fatalf("HPPercent = %d, want 80", state.Player.HPPercent())
	}
	if state.Player.ManaPercent() != 66 {
		t.Fatalf("ManaPercent = %d, want 66", state.Player.ManaPercent())
	}
}

func TestFromSnapshotUnknownArea(t *testing.T) {
	snap := validSnapshot()
	snap.AreaID = 9999

	state := FromSnapshot(snap)
	if !state.Valid {
		t.Fatal("Valid = false, want true for unknown area")
	}
	if state.Area.Name != "Unknown Area 9999" {
		t.Fatalf("Area.Name = %q, want Unknown Area 9999", state.Area.Name)
	}
	if state.Area.Kind != AreaKindUnknown {
		t.Fatalf("Area.Kind = %v, want AreaKindUnknown", state.Area.Kind)
	}
	if state.Player.HP != snap.HP {
		t.Fatalf("Player.HP = %d, want %d", state.Player.HP, snap.HP)
	}
}

func TestFromSnapshotValidAreaIDZero(t *testing.T) {
	snap := validSnapshot()
	snap.AreaID = 0

	state := FromSnapshot(snap)
	if !state.Valid {
		t.Fatal("Valid = false, want true")
	}
	if state.Phase != GamePhaseInGame {
		t.Fatalf("Phase = %v, want GamePhaseInGame", state.Phase)
	}
	if state.Area.Name != "Unknown Area 0" {
		t.Fatalf("Area.Name = %q, want Unknown Area 0", state.Area.Name)
	}
	if state.Area.Act != ActUnknown {
		t.Fatalf("Area.Act = %v, want ActUnknown", state.Area.Act)
	}
	if state.Area.Kind != AreaKindUnknown {
		t.Fatalf("Area.Kind = %v, want AreaKindUnknown", state.Area.Kind)
	}
	if state.Player.HP != snap.HP || state.Player.Position.X != snap.PosX {
		t.Fatal("Player should remain populated for valid snapshot with AreaID 0")
	}
}

func TestFromSnapshotNamedConstantWithoutCatalogName(t *testing.T) {
	snap := validSnapshot()
	snap.AreaID = uint32(MapsAncientTemple)

	state := FromSnapshot(snap)
	if !state.Valid {
		t.Fatal("Valid = false, want true for unnamed catalog constant")
	}
	if state.Area.Name != "Unknown Area 137" {
		t.Fatalf("Area.Name = %q, want Unknown Area 137", state.Area.Name)
	}
	if state.Area.Kind != AreaKindUnknown {
		t.Fatalf("Area.Kind = %v, want AreaKindUnknown", state.Area.Kind)
	}
	if state.Area.Act != Act5 {
		t.Fatalf("Area.Act = %v, want Act5", state.Area.Act)
	}
}

func TestFromSnapshotInvalid(t *testing.T) {
	at := time.Date(2026, 6, 25, 13, 0, 0, 0, time.UTC)
	snap := memory.Snapshot{
		At:     at,
		Valid:  false,
		Reason: memory.ReasonStatsUnavailable,
		AreaID: 6,
		HP:     999,
	}

	state := FromSnapshot(snap)
	if state.Valid {
		t.Fatal("Valid = true, want false")
	}
	if !state.At.Equal(at) {
		t.Fatalf("At = %v, want %v", state.At, at)
	}
	if state.Reason != memory.ReasonStatsUnavailable {
		t.Fatalf("Reason = %q, want %q", state.Reason, memory.ReasonStatsUnavailable)
	}
	if state.Phase != GamePhaseUnknown {
		t.Fatalf("Phase = %v, want GamePhaseUnknown", state.Phase)
	}
	if state.Area != (Area{}) {
		t.Fatalf("Area = %+v, want zero value", state.Area)
	}
	if state.Player != (Player{}) {
		t.Fatalf("Player = %+v, want zero value", state.Player)
	}
}

func TestFromSnapshotInvalidNotInGameMapsMenu(t *testing.T) {
	snap := memory.Snapshot{
		At:     time.Now(),
		Valid:  false,
		Reason: memory.ReasonNotInGame,
		Phase:  memory.GamePhaseMenu,
	}

	state := FromSnapshot(snap)
	if state.Phase != GamePhaseMenu {
		t.Fatalf("Phase = %v, want GamePhaseMenu", state.Phase)
	}
	if state.Objects == nil || state.Entrances == nil || state.Monsters == nil || state.Items == nil {
		t.Fatal("invalid snapshot should have non-nil empty entity/item slices")
	}
}

func TestFromSnapshotMapsItems(t *testing.T) {
	snap := validSnapshot()
	snap.Hover = memory.HoverState{IsHovered: true, UnitType: memory.HoverUnitTypeItem, UnitID: 4001}
	snap.Items = []memory.ItemUnit{{
		TxtFileNo:   625,
		UnitID:      4001,
		Quality:     2,
		RawLocation: 3,
		PlayerOwned: false,
		PosX:        700,
		PosY:        800,
		Flags:       0x10,
		Identified:  true,
		Stats:       []memory.RawStat{{ID: 123, Layer: 2, Value: 456}},
	}}

	state := FromSnapshot(snap)
	if len(state.Items) != 1 {
		t.Fatalf("Items = %+v, want one item", state.Items)
	}
	got := state.Items[0]
	if got.Code != "r01" || got.Name != "El Rune" || got.Type != "rune" {
		t.Fatalf("Code/Name/Type = %q/%q/%q, want r01/El Rune/rune", got.Code, got.Name, got.Type)
	}
	if got.NormalCode != "" || got.UberCode != "" || got.UltraCode != "" {
		t.Fatalf("Rune tier codes = %q/%q/%q, want empty", got.NormalCode, got.UberCode, got.UltraCode)
	}
	if got.Quality != ItemQualityNormal || got.Quality.String() != "normal" {
		t.Fatalf("Quality = %v (%s), want normal", got.Quality, got.Quality.String())
	}
	if got.Location != ItemLocationGround || got.RawLocation != 3 {
		t.Fatalf("Location = %q raw=%d, want ground raw=3", got.Location, got.RawLocation)
	}
	if got.Width != 1 || got.Height != 1 {
		t.Fatalf("Dimensions = %dx%d, want rune 1x1", got.Width, got.Height)
	}
	if got.Position.X != 700 || got.Position.Y != 800 || !got.IsHovered || !got.Identified {
		t.Fatalf("Item = %+v, want hovered identified at 700,800", got)
	}
	if len(got.Stats) != 1 || got.Stats[0].ID != 123 || got.Stats[0].Layer != 2 || got.Stats[0].Value != 456 {
		t.Fatalf("Stats = %+v, want raw stat 123/2/456", got.Stats)
	}
}

func TestItemLocationAndQualityStrings(t *testing.T) {
	if ItemLocation("").String() != "unknown" {
		t.Fatalf("empty location = %q, want unknown", ItemLocation("").String())
	}
	if ItemLocationCube.String() != "cube" {
		t.Fatalf("cube location = %q, want cube", ItemLocationCube.String())
	}
	if ItemLocationSharedStash1.String() != "shared_stash_1" {
		t.Fatalf("shared stash = %q", ItemLocationSharedStash1.String())
	}
	if ItemQualityMagic.String() != "magic" {
		t.Fatalf("magic quality = %q", ItemQualityMagic.String())
	}
	if ItemQuality(99).String() != "unknown" {
		t.Fatalf("unknown quality = %q", ItemQuality(99).String())
	}
}

func TestLocalCatalogAllGemsHaveOneByOneInventoryDimensions(t *testing.T) {
	gemCount := 0
	for id, item := range itemCatalog {
		if !strings.HasPrefix(item.Type, "gem") {
			continue
		}
		gemCount++
		if item.Width != 1 || item.Height != 1 {
			t.Fatalf("gem %d (%s) dimensions = %dx%d, want 1x1 from local misc.txt", id, item.Code, item.Width, item.Height)
		}
	}
	if gemCount == 0 {
		t.Fatal("local item catalog contains no gems")
	}
}

func TestItemCatalogLookupIncludesBaseItems(t *testing.T) {
	if LookupItemCode(27) != "sbr" || LookupItemName(27) != "Saber" || LookupItemType(27) != "swor" {
		t.Fatalf("Saber lookup = %q/%q/%q", LookupItemCode(27), LookupItemName(27), LookupItemType(27))
	}
	if w, h := LookupItemDimensions(27); w != 1 || h != 3 {
		t.Fatalf("Saber dimensions = %dx%d, want 1x3", w, h)
	}
	if LookupItemCode(316) != "stu" || LookupItemName(316) != "Studded Leather" || LookupItemType(316) != "tors" {
		t.Fatalf("Studded Leather lookup = %q/%q/%q", LookupItemCode(316), LookupItemName(316), LookupItemType(316))
	}
	if w, h := LookupItemDimensions(316); w != 2 || h != 3 {
		t.Fatalf("Studded Leather dimensions = %dx%d, want 2x3", w, h)
	}
}

func TestItemCatalogLookupIncludesDummyTypeForDiagnostics(t *testing.T) {
	if LookupItemCode(553) != "fng" || LookupItemName(553) != "Fang" || LookupItemType(553) != "body" {
		t.Fatalf("Fang lookup = %q/%q/%q", LookupItemCode(553), LookupItemName(553), LookupItemType(553))
	}
	if LookupItemCode(662) != "pk1" || LookupItemName(662) != "Key of Terror" || LookupItemType(662) != "ques" {
		t.Fatalf("Key of Terror lookup = %q/%q/%q", LookupItemCode(662), LookupItemName(662), LookupItemType(662))
	}
	if LookupItemCode(634) != "r10" || LookupItemName(634) != "Thul Rune" || LookupItemType(634) != "rune" {
		t.Fatalf("Thul lookup = %q/%q/%q", LookupItemCode(634), LookupItemName(634), LookupItemType(634))
	}
}

func TestItemCatalogLookupIncludesInventoryDimensions(t *testing.T) {
	cases := []struct {
		id     uint32
		width  int
		height int
	}{
		{533, 1, 2}, // Tome of Town Portal
		{534, 1, 2}, // Tome of Identify
		{564, 2, 2}, // Horadric Cube
		{618, 1, 1}, // Small Charm
		{619, 1, 2}, // Large Charm
		{620, 1, 3}, // Grand Charm
		{625, 1, 1}, // El Rune
		{628, 1, 1}, // Nef Rune
		{629, 1, 1}, // Eth Rune
	}
	for _, tc := range cases {
		w, h := LookupItemDimensions(tc.id)
		if w != tc.width || h != tc.height {
			t.Fatalf("LookupItemDimensions(%d) = %dx%d, want %dx%d", tc.id, w, h, tc.width, tc.height)
		}
	}
}

func TestHoverUnitTypeItemString(t *testing.T) {
	if HoverUnitType(memory.HoverUnitTypeItem).String() != "item" {
		t.Fatalf("HoverUnitTypeItem string = %q, want item", HoverUnitType(memory.HoverUnitTypeItem).String())
	}
}

func TestStateItemQueries(t *testing.T) {
	st := validSnapshot()
	st.Items = []memory.ItemUnit{
		{TxtFileNo: 625, UnitID: 4001, RawLocation: 3},
		{TxtFileNo: 611, UnitID: 4002, RawLocation: 1, PlayerOwned: true},
	}
	state := FromSnapshot(st)

	if got := state.GroundItems(); len(got) != 1 || got[0].UnitID != 4001 {
		t.Fatalf("GroundItems = %+v, want unit 4001", got)
	}
	if got := state.ItemsByLocation(ItemLocationEquipped); len(got) != 1 || got[0].UnitID != 4002 {
		t.Fatalf("ItemsByLocation(equipped) = %+v, want unit 4002", got)
	}
	if got, ok := state.FindItemByUnitID(4001); !ok || got.UnitID != 4001 {
		t.Fatalf("FindItemByUnitID = %+v/%v, want unit 4001", got, ok)
	}
	if _, ok := state.FindItemByUnitID(9999); ok {
		t.Fatal("FindItemByUnitID should not find missing unit")
	}
}

func TestStateInventoryItemsUsesValidatedPersonalInventoryOnly(t *testing.T) {
	st := validSnapshot()
	st.Items = []memory.ItemUnit{
		{TxtFileNo: 625, UnitID: 4001, RawLocation: 0, OwnerID: 9001, PlayerOwned: true, Page: 0, GridX: 4, GridY: 2},
		{TxtFileNo: 564, UnitID: 4002, RawLocation: 0, OwnerID: 9001, PlayerOwned: true, Page: 3, GridX: 0, GridY: 0},
		{TxtFileNo: 625, UnitID: 4003, RawLocation: 0, OwnerID: 0, PlayerOwned: false, Page: 0, GridX: 5, GridY: 2},
	}
	state := FromSnapshot(st)

	items := state.InventoryItems()
	if len(items) != 1 || items[0].UnitID != 4001 {
		t.Fatalf("InventoryItems = %+v, want only personal inventory unit 4001", items)
	}
	if items[0].GridX != 4 || items[0].GridY != 2 || items[0].Width != 1 || items[0].Height != 1 {
		t.Fatalf("Inventory item grid/dimensions = %+v, want col 4 row 2 size 1x1", items[0])
	}
	if got := state.ItemsByLocation(ItemLocationCube); len(got) != 1 || got[0].UnitID != 4002 {
		t.Fatalf("Cube items = %+v, want unit 4002", got)
	}
	if got := state.ItemsByLocation(ItemLocationUnknown); len(got) != 1 || got[0].UnitID != 4003 {
		t.Fatalf("Unknown items = %+v, want non-player inventory unit 4003", got)
	}
}

func TestFromSnapshotInvalidEmptyReason(t *testing.T) {
	snap := memory.Snapshot{
		At:    time.Now(),
		Valid: false,
	}

	state := FromSnapshot(snap)
	if state.Valid {
		t.Fatal("Valid = true, want false")
	}
	if state.Reason != "" {
		t.Fatalf("Reason = %q, want empty (no synthetic reason)", state.Reason)
	}
	if state.Phase != GamePhaseUnknown {
		t.Fatalf("Phase = %v, want GamePhaseUnknown", state.Phase)
	}
}

func testModel(t *testing.T) *Model {
	t.Helper()
	return NewModel(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestModelCurrentBeforeUpdate(t *testing.T) {
	m := testModel(t)
	state := m.Current()
	if state.Valid || state.Reason != "" || !state.At.IsZero() {
		t.Fatalf("Current before Update = %+v, want zero State", state)
	}
}

func TestModelUpdateStoresAndReturnsState(t *testing.T) {
	m := testModel(t)
	snap := validSnapshot()

	got := m.Update(snap)
	current := m.Current()

	if !got.Valid || !current.Valid {
		t.Fatal("Update should produce valid state")
	}
	if got.Area.Name != current.Area.Name {
		t.Fatalf("Update return Area = %q, Current Area = %q", got.Area.Name, current.Area.Name)
	}
	if got.Player.HP != current.Player.HP {
		t.Fatalf("Update return HP = %d, Current HP = %d", got.Player.HP, current.Player.HP)
	}
}

func TestModelUpdateReplacesPreviousState(t *testing.T) {
	m := testModel(t)

	first := m.Update(validSnapshot())
	secondSnap := validSnapshot()
	secondSnap.AreaID = uint32(ForgottenTower)
	secondSnap.HP = 42
	second := m.Update(secondSnap)

	if first.Area.Name == second.Area.Name {
		t.Fatal("second Update should replace area")
	}
	if second.Area.Name != "Forgotten Tower" {
		t.Fatalf("Area.Name = %q, want Forgotten Tower", second.Area.Name)
	}
	if m.Current().Player.HP != 42 {
		t.Fatalf("Current HP = %d, want 42", m.Current().Player.HP)
	}
}

func TestModelCurrentReturnsIndependentCopy(t *testing.T) {
	m := testModel(t)
	m.Update(validSnapshot())

	c1 := m.Current()
	c2 := m.Current()
	c1.Player.HP = 0

	if c2.Player.HP == 0 {
		t.Fatal("mutating c1 should not affect c2")
	}
	if m.Current().Player.HP != 100 {
		t.Fatalf("mutating copy should not affect model state, HP = %d", m.Current().Player.HP)
	}
}

func TestModelUpdateReturnMutationDoesNotAffectStoredState(t *testing.T) {
	m := testModel(t)
	got := m.Update(validSnapshot())
	got.Player.HP = 0

	if m.Current().Player.HP != 100 {
		t.Fatalf("mutating Update return should not affect stored state, HP = %d", m.Current().Player.HP)
	}
}

func TestModelReset(t *testing.T) {
	m := testModel(t)
	m.Update(validSnapshot())

	at := time.Now()
	got := m.Reset(at, "process_lost")
	cur := m.Current()

	if got.Valid || cur.Valid {
		t.Fatal("Reset should produce invalid state")
	}
	if got.Reason != "process_lost" || cur.Reason != "process_lost" {
		t.Fatalf("Reason = %q, want process_lost", cur.Reason)
	}
	if got.Phase != GamePhaseUnknown || cur.Phase != GamePhaseUnknown {
		t.Fatal("Reset should set GamePhaseUnknown")
	}
	if got.Area != (Area{}) || got.Player != (Player{}) {
		t.Fatalf("Reset return should zero Area/Player, got Area=%+v Player=%+v", got.Area, got.Player)
	}
	if cur.Area != (Area{}) || cur.Player != (Player{}) {
		t.Fatalf("Current after Reset should zero Area/Player, got Area=%+v Player=%+v", cur.Area, cur.Player)
	}
	if !got.At.Equal(at) {
		t.Fatal("Reset should preserve At timestamp")
	}
}

func TestMapPhaseAllValues(t *testing.T) {
	cases := []struct {
		in   memory.GamePhase
		want GamePhase
	}{
		{memory.GamePhaseUnknown, GamePhaseUnknown},
		{memory.GamePhaseMenu, GamePhaseMenu},
		{memory.GamePhaseLoading, GamePhaseLoading},
		{memory.GamePhaseInGame, GamePhaseInGame},
	}
	for _, tc := range cases {
		if got := mapPhase(tc.in); got != tc.want {
			t.Fatalf("mapPhase(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFromSnapshotLoadingInvalid(t *testing.T) {
	snap := memory.Snapshot{
		Valid:  false,
		Phase:  memory.GamePhaseLoading,
		Reason: memory.ReasonPlayerPointerUnavailable,
	}
	state := FromSnapshot(snap)
	if state.Phase != GamePhaseLoading {
		t.Fatalf("Phase = %v, want loading", state.Phase)
	}
	if state.Area != (Area{}) || state.Player != (Player{}) {
		t.Fatal("invalid loading state should zero Area/Player")
	}
}
