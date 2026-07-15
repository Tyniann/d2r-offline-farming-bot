package app

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func egressTestState(t *testing.T) (world.State, pathing.LayoutFingerprint) {
	t.Helper()
	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.KurastDocks),
		Player:   world.Player{Position: world.Position{X: 5100, Y: 5100}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 42},
		Objects:  []world.Object{{ID: 237, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 5120, Y: 5080}}},
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		t.Fatal(err)
	}
	return state, fingerprint
}

func saveEgressTestRoute(t *testing.T, directory string, state world.State, fingerprint pathing.LayoutFingerprint) {
	t.Helper()
	seed := state.Identity.MapSeed
	route := pathing.Route{
		Version: pathing.RouteVersion, ID: "act3-egress", Name: "Kurast-Docks-Egress", Kind: pathing.RouteKindNavigation,
		Binding:   pathing.RouteBinding{CharacterName: "MrBones", CharacterClass: "necromancer", Difficulty: pathing.RouteDifficultyNightmare, MapSeed: &seed, GameVersion: "3.2.92777", LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash}},
		Recording: pathing.RouteRecording{RecordedAt: time.Now().UTC(), SampleDistanceTiles: 4},
		Playback:  pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments:  []pathing.RouteSegment{{ID: "kurast-docks-egress", FromAreaID: world.KurastDocks, ToAreaID: world.KurastDocks, Movement: pathing.RouteMovementWalk, Points: []pathing.RoutePoint{{X: 5100, Y: 5100}, {X: 5110, Y: 5090}}, Transition: pathing.RouteTransition{Type: "terminal"}}},
	}
	if err := pathing.SaveRoute(filepath.Join(directory, "act3-egress.yaml"), route); err != nil {
		t.Fatal(err)
	}
}

func TestTownEgressAdapterRequiresBoundRouteAndStartAnchor(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct3]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct3] = egress
	state, fingerprint := egressTestState(t)
	saveEgressTestRoute(t, directory, state, fingerprint)
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.Start(town.OriginAct3, state); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	adapter.Reset()

	wrongCharacter := state
	wrongCharacter.Identity.CharacterName = "Other"
	if err := adapter.Start(town.OriginAct3, wrongCharacter); !errors.Is(err, pathing.ErrRouteCharacterMismatch) {
		t.Fatalf("character mismatch error = %v", err)
	}
	far := state
	far.Player.Position = world.Position{X: 5200, Y: 5200}
	if err := adapter.Start(town.OriginAct3, far); !errors.Is(err, pathing.ErrRouteStartMismatch) {
		t.Fatalf("start mismatch error = %v", err)
	}
}

func TestTownEgressAdapterMissingRouteStopsBeforeNavigatorInput(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	egress := cfg.Town.Egress[town.OriginAct3]
	egress.RoutesDirectory = t.TempDir()
	cfg.Town.Egress[town.OriginAct3] = egress
	state, _ := egressTestState(t)
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.Start(town.OriginAct3, state); !errors.Is(err, pathing.ErrRouteNotFound) {
		t.Fatalf("missing route error = %v", err)
	}
}

func TestAct3EgressRouteRejectsTeleportPlayback(t *testing.T) {
	state, fingerprint := egressTestState(t)
	directory := t.TempDir()
	saveEgressTestRoute(t, directory, state, fingerprint)
	registry, err := pathing.LoadRouteRegistry(directory)
	if err != nil {
		t.Fatal(err)
	}
	route, err := registry.Get("act3-egress")
	if err != nil {
		t.Fatal(err)
	}
	route.Segments[0].Movement = pathing.RouteMovementTeleport
	if err := validateAct3EgressRoute(route); !errors.Is(err, pathing.ErrRouteStartMismatch) {
		t.Fatalf("teleport egress error = %v", err)
	}
}
