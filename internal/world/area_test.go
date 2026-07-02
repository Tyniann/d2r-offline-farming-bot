package world

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestLookupAreaBlackMarsh(t *testing.T) {
	a := LookupArea(BlackMarsh)
	if a.Name != "Black Marsh" {
		t.Fatalf("Name = %q, want Black Marsh", a.Name)
	}
	if a.Act != Act1 {
		t.Fatalf("Act = %v, want Act1", a.Act)
	}
	if a.Kind != AreaKindOutdoor {
		t.Fatalf("Kind = %v, want AreaKindOutdoor", a.Kind)
	}
	if a.IsTown() {
		t.Fatal("Black Marsh should not be a town")
	}
}

func TestLookupAreaBloodMoor(t *testing.T) {
	a := LookupArea(BloodMoor)
	if a.Kind != AreaKindOutdoor {
		t.Fatalf("Kind = %v, want AreaKindOutdoor", a.Kind)
	}
	if a.IsTown() || a.IsDungeon() {
		t.Fatal("Blood Moor should be outdoor only")
	}
}

func TestLookupAreaForgottenTower(t *testing.T) {
	a := LookupArea(ForgottenTower)
	if a.IsTown() {
		t.Fatal("Forgotten Tower should not be a town")
	}
	if a.IsDungeon() {
		t.Fatal("Forgotten Tower is special, not a dungeon")
	}
	if a.Kind != AreaKindSpecial {
		t.Fatalf("Kind = %v, want AreaKindSpecial", a.Kind)
	}
}

func TestLookupAreaRogueEncampment(t *testing.T) {
	a := LookupArea(RogueEncampment)
	if !a.IsTown() {
		t.Fatal("Rogue Encampment should be a town")
	}
}

func TestLookupAreaTowerCellarLevels(t *testing.T) {
	cases := []struct {
		id   AreaID
		name string
	}{
		{TowerCellarLevel1, "Tower Cellar Level 1"},
		{TowerCellarLevel2, "Tower Cellar Level 2"},
		{TowerCellarLevel3, "Tower Cellar Level 3"},
		{TowerCellarLevel4, "Tower Cellar Level 4"},
		{TowerCellarLevel5, "Tower Cellar Level 5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := LookupArea(tc.id)
			if a.Name != tc.name {
				t.Fatalf("Name = %q, want %q", a.Name, tc.name)
			}
			if !a.IsDungeon() {
				t.Fatalf("%s should be a dungeon", tc.name)
			}
			if a.Act != Act1 {
				t.Fatalf("Act = %v, want Act1", a.Act)
			}
		})
	}
}

func TestLookupAreaZero(t *testing.T) {
	a := LookupArea(AreaID(0))
	if a.Name != "Unknown Area 0" {
		t.Fatalf("Name = %q, want Unknown Area 0", a.Name)
	}
	if a.Act != ActUnknown {
		t.Fatalf("Act = %v, want ActUnknown", a.Act)
	}
	if a.Kind != AreaKindUnknown {
		t.Fatalf("Kind = %v, want AreaKindUnknown", a.Kind)
	}
	if a.IsKnown() {
		t.Fatal("area 0 should not be known")
	}
}

func TestLookupAreaConstantsWithoutNames(t *testing.T) {
	for _, id := range []AreaID{MapsAncientTemple, MapsDesecratedTemple, MapsFrigidPlateau, MapsInfernalTrial, MapsRuinedCitadel} {
		a := LookupArea(id)
		wantName := fmt.Sprintf("Unknown Area %d", id)
		if a.Name != wantName {
			t.Fatalf("LookupArea(%d).Name = %q, want %q", id, a.Name, wantName)
		}
		if a.IsKnown() {
			t.Fatalf("LookupArea(%d) should not be known", id)
		}
	}
	if LookupArea(MapsAncientTemple).Act != Act5 {
		t.Fatalf("Act for 137 = %v, want Act5", LookupArea(MapsAncientTemple).Act)
	}
}

func TestAreaIDIsTown(t *testing.T) {
	towns := []AreaID{RogueEncampment, LutGholein, KurastDocks, ThePandemoniumFortress, Harrogath}
	for _, id := range towns {
		if !id.IsTown() {
			t.Fatalf("AreaID(%d) should be a town", id)
		}
		if !LookupArea(id).IsTown() {
			t.Fatalf("LookupArea(%d) should be a town", id)
		}
	}
	if BloodMoor.IsTown() {
		t.Fatal("Blood Moor should not be a town")
	}
}

func TestAreaCatalogComplete(t *testing.T) {
	for id := AreaID(1); id <= 136; id++ {
		a := LookupArea(id)
		if !a.IsKnown() {
			t.Fatalf("ID %d should be known", id)
		}
		if a.Name == "" {
			t.Fatalf("ID %d has empty name", id)
		}
	}
}

func TestLookupAreaUnknownID(t *testing.T) {
	a := LookupArea(AreaID(9999))
	if a.Name != "Unknown Area 9999" {
		t.Fatalf("Name = %q, want Unknown Area 9999", a.Name)
	}
}

func TestAreaIDActBoundaries(t *testing.T) {
	cases := []struct {
		id   AreaID
		want Act
	}{
		{0, ActUnknown},
		{39, Act1},
		{40, Act2},
		{74, Act2},
		{75, Act3},
		{102, Act3},
		{103, Act4},
		{108, Act4},
		{109, Act5},
	}
	for _, tc := range cases {
		if got := tc.id.Act(); got != tc.want {
			t.Fatalf("AreaID(%d).Act() = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestPlayerHPPercent(t *testing.T) {
	if got := (Player{HP: 50, MaxHP: 100}).HPPercent(); got != 50 {
		t.Fatalf("HPPercent = %d, want 50", got)
	}
	if got := (Player{HP: 10, MaxHP: 0}).HPPercent(); got != 0 {
		t.Fatalf("HPPercent with MaxHP=0 = %d, want 0", got)
	}
	if got := (Player{HP: 150, MaxHP: 100}).HPPercent(); got != 100 {
		t.Fatalf("HPPercent clamp = %d, want 100", got)
	}
}

func TestPlayerManaPercent(t *testing.T) {
	if got := (Player{Mana: 25, MaxMana: 50}).ManaPercent(); got != 50 {
		t.Fatalf("ManaPercent = %d, want 50", got)
	}
	if got := (Player{Mana: 10, MaxMana: 0}).ManaPercent(); got != 0 {
		t.Fatalf("ManaPercent with MaxMana=0 = %d, want 0", got)
	}
	if got := (Player{Mana: 200, MaxMana: 100}).ManaPercent(); got != 100 {
		t.Fatalf("ManaPercent clamp = %d, want 100", got)
	}
}

func TestStateValueSemantics(t *testing.T) {
	assertWorldValueFields(t, reflect.TypeOf(Player{}))
	assertWorldValueFields(t, reflect.TypeOf(Area{}))

	s1 := State{
		At:     time.Now(),
		Phase:  GamePhaseInGame,
		Valid:  true,
		Area:   LookupArea(BloodMoor),
		Player: Player{HP: 100, MaxHP: 100},
	}
	s2 := s1
	s2.Player.HP = 50
	if s1.Player.HP != 100 {
		t.Fatal("State copy should be independent (value semantics)")
	}
}

func TestInvalidStateZeroFields(t *testing.T) {
	s := State{Reason: "not_in_game"}
	if s.Valid {
		t.Fatal("state without Valid: true must be invalid")
	}
	if s.Reason != "not_in_game" {
		t.Fatalf("reason = %q, want not_in_game", s.Reason)
	}
	if s.Area != (Area{}) || s.Player != (Player{}) {
		t.Fatal("invalid state should have zero area and player")
	}
}

func TestGamePhaseString(t *testing.T) {
	if GamePhaseUnknown.String() != "unknown" {
		t.Fatalf("unknown = %q", GamePhaseUnknown.String())
	}
	if GamePhaseMenu.String() != "menu" {
		t.Fatalf("menu = %q", GamePhaseMenu.String())
	}
	if GamePhaseLoading.String() != "loading" {
		t.Fatalf("loading = %q", GamePhaseLoading.String())
	}
	if GamePhaseInGame.String() != "in_game" {
		t.Fatalf("in_game = %q", GamePhaseInGame.String())
	}
}

func assertWorldValueFields(t *testing.T, typ reflect.Type) {
	t.Helper()
	const worldPkg = "github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		switch field.Type.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice:
			t.Fatalf("%s.%s must not be pointer, map, or slice", typ.Name(), field.Name)
		case reflect.Struct:
			if field.Type.PkgPath() == worldPkg {
				assertWorldValueFields(t, field.Type)
			}
		}
	}
}
