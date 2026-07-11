package pathing

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const testLayoutHash = "c233f9b137a09e07e3b8d0d2fc02c74103bbc54e42ff89e57d9592a6024fb51c"

func validRoute() Route {
	seed := uint32(466817790)
	return Route{
		Version: RouteVersion, ID: "test-navigation-route", Name: "Test Navigation", Kind: RouteKindNavigation,
		Tags:      []string{"act1", "test"},
		Binding:   RouteBinding{CharacterName: "MrBones", CharacterClass: "necromancer", Difficulty: RouteDifficultyNightmare, MapSeed: &seed, GameVersion: "3.2.92777", LayoutFingerprint: RouteLayoutFingerprint{Version: 1, AreaID: world.BlackMarsh, AnchorCount: 2, Hash: testLayoutHash}},
		Recording: RouteRecording{RecordedAt: time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC), SampleDistanceTiles: 4},
		Playback:  RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments: []RouteSegment{
			{ID: "black-marsh", FromAreaID: world.BlackMarsh, ToAreaID: world.ForgottenTower, Movement: RouteMovementTeleport, Points: []RoutePoint{{X: 14858, Y: 5068}, {X: 14820, Y: 5065}}, Transition: RouteTransition{Type: "entrance", EntranceKind: "wilderness_to_tower"}},
			{ID: "forgotten-tower", FromAreaID: world.ForgottenTower, ToAreaID: world.TowerCellarLevel1, Movement: RouteMovementTeleport, Points: []RoutePoint{{X: 1000, Y: 1000}}, Transition: RouteTransition{Type: "entrance", EntranceKind: "unknown_antechamber"}},
		},
	}
}

func TestRouteValidate(t *testing.T) {
	if err := validRoute().Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRouteValidateRejectsContractViolations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Route)
		want   string
	}{
		{"version", func(r *Route) { r.Version = 2 }, "version"},
		{"duplicate tag", func(r *Route) { r.Tags = []string{"test", "test"} }, "duplicate"},
		{"unknown class", func(r *Route) { r.Binding.CharacterClass = "wizard" }, "character_class"},
		{"bad layout", func(r *Route) { r.Binding.LayoutFingerprint.Hash = "bad" }, "layout_fingerprint"},
		{"segment chain", func(r *Route) { r.Segments[1].FromAreaID = world.RogueEncampment }, "previous to_area_id"},
		{"duplicate segment", func(r *Route) { r.Segments[1].ID = r.Segments[0].ID }, "duplicate"},
		{"unknown movement", func(r *Route) { r.Segments[0].Movement = "fly" }, "movement"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := validRoute()
			tc.mutate(&r)
			if err := r.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestSaveLoadRouteAllowsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "route.yaml")
	route := validRoute()
	if err := SaveRoute(path, route); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("future_field: ignored\n")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRoute(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != route.ID || got.Binding.LayoutFingerprint.Hash != testLayoutHash {
		t.Fatalf("loaded route = %+v", got)
	}
}

func TestRouteRegistryBlocksDuplicateIDsAndReportsInvalid(t *testing.T) {
	dir := t.TempDir()
	first, second := validRoute(), validRoute()
	if err := SaveRoute(filepath.Join(dir, "a.yaml"), first); err != nil {
		t.Fatal(err)
	}
	if err := SaveRoute(filepath.Join(dir, "b.yaml"), second); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte("version: 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := LoadRouteRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Entries()) != 3 {
		t.Fatalf("entries = %d", len(registry.Entries()))
	}
	if _, err := registry.Get(first.ID); !errors.Is(err, ErrRouteDuplicateID) {
		t.Fatalf("Get error = %v", err)
	}
	duplicate, invalid := 0, 0
	for _, entry := range registry.Entries() {
		if entry.Status == RouteRegistryDuplicate {
			duplicate++
		}
		if entry.Status == RouteRegistryInvalid {
			invalid++
		}
	}
	if duplicate != 2 || invalid != 1 {
		t.Fatalf("duplicate=%d invalid=%d", duplicate, invalid)
	}
}

func TestValidateRoutePrecheck(t *testing.T) {
	route := validRoute()
	input := RoutePrecheckInput{
		Identity:    world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
		GameVersion: "3.2.92777",
		Layout:      LayoutFingerprint{Version: 1, AreaID: world.BlackMarsh, AnchorCount: 2, Hash: testLayoutHash},
		World:       world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.BlackMarsh), Player: world.Player{Position: world.Position{X: 14858, Y: 5068}}},
	}
	if err := ValidateRoutePrecheck(route, input); err != nil {
		t.Fatalf("precheck error = %v", err)
	}
	input.Identity.CharacterName = "MrHammer"
	if err := ValidateRoutePrecheck(route, input); !errors.Is(err, ErrRouteCharacterMismatch) {
		t.Fatalf("character error = %v", err)
	}
	input.Identity.CharacterName = "MrBones"
	input.Layout.Hash = strings.Repeat("0", 64)
	if err := ValidateRoutePrecheck(route, input); !errors.Is(err, ErrRouteLayoutMismatch) {
		t.Fatalf("layout error = %v", err)
	}
}
