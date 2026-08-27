package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func egressTestStateForAct(t *testing.T, act town.OriginAct) (world.State, pathing.LayoutFingerprint) {
	t.Helper()
	area, ok := town.TownAreaForAct(act)
	if !ok {
		t.Fatalf("unsupported act %s", act)
	}
	state := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(area), Player: world.Player{Position: world.Position{X: 5100, Y: 5100}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 42}, Objects: []world.Object{
		{ID: 59, UnitID: 6, Kind: world.ObjectKindTownPortal, Position: world.Position{X: 5102, Y: 5100}},
		{ID: 237, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 5120, Y: 5080}},
	}}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		t.Fatal(err)
	}
	return state, fingerprint
}

func egressTestState(t *testing.T) (world.State, pathing.LayoutFingerprint) {
	return egressTestStateForAct(t, town.OriginAct3)
}

func saveEgressTestRouteForAct(t *testing.T, directory string, act town.OriginAct, state world.State, fingerprint pathing.LayoutFingerprint) {
	t.Helper()
	route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: act, TownArea: state.Area.ID, GameVersion: "3.2.92777", LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash}, From: town.AnchorPortalArrival, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []town.SystemEgressPoint{{X: 5100, Y: 5100}, {X: 5110, Y: 5090}}}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressFilename), route); err != nil {
		t.Fatal(err)
	}
}

func saveEgressTestRoute(t *testing.T, directory string, state world.State, fingerprint pathing.LayoutFingerprint) {
	saveEgressTestRouteForAct(t, directory, town.OriginAct3, state, fingerprint)
}

func TestTownEgressAdapterSupportsGlobalRoutesForAllForeignActs(t *testing.T) {
	for _, act := range []town.OriginAct{town.OriginAct2, town.OriginAct3, town.OriginAct4, town.OriginAct5} {
		t.Run(string(act), func(t *testing.T) {
			cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			egress := cfg.Town.Egress[act]
			egress.RoutesDirectory = directory
			cfg.Town.Egress[act] = egress
			state, fingerprint := egressTestStateForAct(t, act)
			saveEgressTestRouteForAct(t, directory, act, state, fingerprint)
			adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
			if err := adapter.Start(act, state); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			adapter.Reset()
			otherCharacter := state
			otherCharacter.Identity.CharacterName = "Other"
			otherCharacter.Identity.MapSeed = 999
			if err := adapter.Start(act, otherCharacter); err != nil {
				t.Fatalf("global route rejected character/seed: %v", err)
			}
		})
	}
}

func TestTownEgressAdapterSupportsSpawnStartProofWithoutPortal(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress
	state := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LutGholein), Player: world.Player{Position: world.Position{X: 5100, Y: 5100}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 42}, Objects: []world.Object{{ID: 237, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 5120, Y: 5080}}}}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		t.Fatal(err)
	}
	proofPoint := 0
	route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: town.OriginAct2, TownArea: state.Area.ID, GameVersion: "3.2.92777", LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: fingerprint.Anchors}, LayoutProofPointIndex: &proofPoint, From: town.AnchorSpawn, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []town.SystemEgressPoint{{X: 5100, Y: 5100}, {X: 5110, Y: 5090}}}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressSpawnFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, state); err != nil {
		t.Fatalf("spawn StartFrom error = %v", err)
	}
	adapter.Reset()
	far := state
	far.Player.Position = world.Position{X: 5200, Y: 5200}
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, far); !errors.Is(err, pathing.ErrRouteStartMismatch) {
		t.Fatalf("far spawn error = %v", err)
	}
}

func TestTownEgressSpawnReportsTransientIdentityUntilConfirmed(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress
	confirmed := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LutGholein),
		Player:   world.Player{Position: world.Position{X: 5153, Y: 5203}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrHammer", Class: world.CharacterClassPaladin},
		Objects:  []world.Object{{ID: 156, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 5069, Y: 5084}}},
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(confirmed)
	if err != nil {
		t.Fatal(err)
	}
	proofPoint := 1
	route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: fingerprint.Anchors}, LayoutProofPointIndex: &proofPoint, From: town.AnchorSpawn, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 15}, SampleDistanceTiles: 4, Points: []town.SystemEgressPoint{{X: 5153, Y: 5203}, {X: 5149, Y: 5199}}}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressSpawnFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	unconfirmed := confirmed
	unconfirmed.Identity.Valid = false
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, unconfirmed); !errors.Is(err, pathing.ErrGameIdentityUnavailable) {
		t.Fatalf("unconfirmed identity error=%v, want transient identity error", err)
	}
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, confirmed); err != nil {
		t.Fatalf("confirmed identity start failed: %v", err)
	}
}

func TestTownEgressSpawnPinsDelayedLayoutByRecordedProofPoint(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress
	input := &preparationInputMock{}
	spawn := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LutGholein),
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
	}
	proofState := spawn
	proofState.Player.Position = world.Position{X: 104, Y: 100}
	proofState.Objects = []world.Object{{ID: 156, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 120, Y: 100}}}
	fingerprint, err := pathing.BuildLayoutFingerprint(proofState)
	if err != nil {
		t.Fatal(err)
	}
	proofPoint := 1
	route := town.SystemEgressRoute{
		SchemaVersion: town.SystemEgressSchemaVersion,
		Contract: town.SystemEgressContract{
			Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777",
			LayoutFingerprint:     town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: fingerprint.Anchors},
			LayoutProofPointIndex: &proofPoint, From: town.AnchorSpawn, To: town.AnchorWaypoint,
			Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 3,
		},
		SampleDistanceTiles: 4,
		Points:              []town.SystemEgressPoint{{X: 100, Y: 100}, {X: 104, Y: 100}, {X: 108, Y: 100}},
	}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressSpawnFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", input, pathing.DefaultConfig(), nil)
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, spawn); err != nil {
		t.Fatalf("delayed spawn start rejected: %v", err)
	}
	if _, err := adapter.Tick(t.Context(), spawn); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Tick(t.Context(), proofState); err != nil {
		t.Fatalf("recorded proof point rejected: %v", err)
	}
	if !adapter.layoutMatched {
		t.Fatal("layout was not pinned at the recorded proof point")
	}
}

func TestTownEgressSpawnFailsClosedBeforeInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*world.State)
	}{
		{name: "layout mismatch", mutate: func(state *world.State) {
			state.Objects = []world.Object{{ID: 267, UnitID: 9, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 130, Y: 130}}}
		}},
		{name: "wrong area", mutate: func(state *world.State) { state.Area = world.LookupArea(world.KurastDocks) }},
		{name: "blocked ui", mutate: func(state *world.State) { state.UI.StashOpen = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			egress := cfg.Town.Egress[town.OriginAct2]
			egress.RoutesDirectory = directory
			cfg.Town.Egress[town.OriginAct2] = egress
			input := &preparationInputMock{}
			spawn := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LutGholein), Player: world.Player{Position: world.Position{X: 100, Y: 100}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}}
			boundState := spawn
			boundState.Objects = []world.Object{{ID: 156, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 120, Y: 100}}}
			fingerprint, err := pathing.BuildLayoutFingerprint(boundState)
			if err != nil {
				t.Fatal(err)
			}
			proofPoint := 1
			route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: fingerprint.Anchors}, LayoutProofPointIndex: &proofPoint, From: town.AnchorSpawn, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []town.SystemEgressPoint{{X: 100, Y: 100}, {X: 104, Y: 100}, {X: 108, Y: 100}}}
			if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressSpawnFilename), route); err != nil {
				t.Fatal(err)
			}
			adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", input, pathing.DefaultConfig(), nil)
			if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, spawn); err != nil {
				t.Fatal(err)
			}
			invalid := spawn
			tc.mutate(&invalid)
			if _, err := adapter.Tick(t.Context(), invalid); err == nil {
				t.Fatal("negative start tick succeeded")
			}
			if input.moves != 0 || input.keys != 0 || input.clicks != 0 {
				t.Fatalf("input before rejection: moves=%d keys=%d clicks=%d", input.moves, input.keys, input.clicks)
			}
		})
	}
}

func TestTownEgressSpawnLayoutDeadlineStopsBeforeFurtherInput(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress
	input := &preparationInputMock{}
	spawn := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LutGholein), Player: world.Player{Position: world.Position{X: 100, Y: 100}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}}
	boundState := spawn
	boundState.Objects = []world.Object{{ID: 156, UnitID: 7, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 120, Y: 100}}}
	fingerprint, err := pathing.BuildLayoutFingerprint(boundState)
	if err != nil {
		t.Fatal(err)
	}
	proofPoint := 1
	route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: fingerprint.Anchors}, LayoutProofPointIndex: &proofPoint, From: town.AnchorSpawn, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []town.SystemEgressPoint{{X: 100, Y: 100}, {X: 104, Y: 100}, {X: 108, Y: 100}}}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressSpawnFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", input, pathing.DefaultConfig(), nil)
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, spawn); err != nil {
		t.Fatal(err)
	}
	for _, position := range []world.Position{{X: 100, Y: 100}, {X: 104, Y: 100}} {
		state := spawn
		state.Player.Position = position
		state.At = state.At.Add(time.Millisecond)
		if _, err := adapter.Tick(t.Context(), state); err != nil {
			t.Fatal(err)
		}
	}
	state := spawn
	state.Player.Position = world.Position{X: 108, Y: 100}
	if _, err := adapter.Tick(t.Context(), state); err != nil {
		t.Fatalf("final point settle start error=%v", err)
	}
	state.At = state.At.Add(time.Second)
	if _, err := adapter.Tick(t.Context(), state); err == nil || !errors.Is(err, pathing.ErrLayoutAnchorsUnavailable) {
		t.Fatalf("deadline error=%v", err)
	}
	if input.moves != 0 || input.keys != 0 || input.clicks != 0 {
		t.Fatalf("deadline sent input: moves=%d keys=%d clicks=%d", input.moves, input.keys, input.clicks)
	}
}

func TestTownEgressSpawnLayoutDeadlineAccountsForWalkerArrivalTolerance(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress
	spawn := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.LutGholein), Player: world.Player{Position: world.Position{X: 100, Y: 100}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}}
	proofState := spawn
	proofState.Player.Position = world.Position{X: 104, Y: 100}
	proofState.Objects = []world.Object{{ID: 24, Kind: world.ObjectKindPermanentPortal, Position: world.Position{X: 80, Y: 100}}}
	fingerprint, err := pathing.BuildLayoutFingerprint(proofState)
	if err != nil {
		t.Fatal(err)
	}
	proofPoint := 1
	route := town.SystemEgressRoute{SchemaVersion: town.SystemEgressSchemaVersion, Contract: town.SystemEgressContract{Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: town.SystemEgressLayoutFingerprint{Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount, Hash: fingerprint.Hash, Anchors: fingerprint.Anchors}, LayoutProofPointIndex: &proofPoint, From: town.AnchorSpawn, To: town.AnchorWaypoint, Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 15}, SampleDistanceTiles: 4, Points: []town.SystemEgressPoint{{X: 100, Y: 100}, {X: 104, Y: 100}, {X: 108, Y: 100}, {X: 112, Y: 100}, {X: 116, Y: 100}}}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressSpawnFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.StartFrom(town.OriginAct2, town.AnchorSpawn, spawn); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Tick(t.Context(), spawn); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Tick(t.Context(), spawn); err != nil {
		t.Fatal(err)
	}
	approachingProof := spawn
	approachingProof.Player.Position = world.Position{X: 101, Y: 100}
	if _, err := adapter.Tick(t.Context(), approachingProof); err != nil {
		t.Fatalf("walker index reached the deadline before the recorded proof position: %v", err)
	}
	if _, err := adapter.Tick(t.Context(), proofState); err != nil {
		t.Fatalf("layout at recorded proof position rejected: %v", err)
	}
	if !adapter.layoutMatched {
		t.Fatal("layout was not pinned at the recorded proof position")
	}
}

func TestTownEgressAdapterAcceptsSmallAnchorCoordinateJitter(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress

	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame,
		Area:     world.LookupArea(world.LutGholein),
		Player:   world.Player{Position: world.Position{X: 5100, Y: 5100}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 42},
		Objects: []world.Object{
			{ID: 59, UnitID: 6, Kind: world.ObjectKindTownPortal, Position: world.Position{X: 5105, Y: 5100}},
			{ID: 267, UnitID: 7, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 5121, Y: 5073}},
		},
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		t.Fatal(err)
	}
	bound := fingerprint
	bound.Hash = "different-exact-hash"
	route := town.SystemEgressRoute{
		SchemaVersion: town.SystemEgressSchemaVersion,
		Contract: town.SystemEgressContract{
			Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777",
			LayoutFingerprint: town.SystemEgressLayoutFingerprint{
				Version: bound.Version, AreaID: bound.AreaID, AnchorCount: 1, Hash: bound.Hash,
				Anchors: []string{"o:267:5121,5076"},
			},
			From: town.AnchorPortalArrival, To: town.AnchorWaypoint,
			Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 15,
		},
		SampleDistanceTiles: 4,
		Points:              []town.SystemEgressPoint{{X: 5100, Y: 5100}, {X: 5090, Y: 5090}},
	}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.Start(town.OriginAct2, state); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestTownEgressAdapterRejectsMissingVersionLayoutAndFarStart(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	egress := cfg.Town.Egress[town.OriginAct3]
	egress.RoutesDirectory = t.TempDir()
	cfg.Town.Egress[town.OriginAct3] = egress
	state, fingerprint := egressTestState(t)
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.Start(town.OriginAct3, state); !errors.Is(err, pathing.ErrRouteNotFound) {
		t.Fatalf("missing route error = %v", err)
	}
	saveEgressTestRoute(t, egress.RoutesDirectory, state, fingerprint)
	adapter.gameVersion = "other"
	if err := adapter.Start(town.OriginAct3, state); !errors.Is(err, pathing.ErrRouteGameVersionMismatch) {
		t.Fatalf("version error = %v", err)
	}
	adapter.gameVersion = "3.2.92777"
	layout := state
	layout.Objects = append([]world.Object(nil), state.Objects...)
	for i := range layout.Objects {
		if layout.Objects[i].Kind == world.ObjectKindWaypoint {
			layout.Objects[i].Position.X++
		}
	}
	if err := adapter.Start(town.OriginAct3, layout); !errors.Is(err, pathing.ErrRouteLayoutMismatch) {
		t.Fatalf("layout error = %v", err)
	}
	far := state
	far.Player.Position = world.Position{X: 5200, Y: 5200}
	if err := adapter.Start(town.OriginAct3, far); !errors.Is(err, pathing.ErrRouteStartMismatch) {
		t.Fatalf("start error = %v", err)
	}
}

func TestTownEgressAdapterAcceptsPortalArrivalAwayFromFirstWalkPoint(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct2]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct2] = egress

	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame,
		Area:     world.LookupArea(world.LutGholein),
		Player:   world.Player{Position: world.Position{X: 5168, Y: 5066}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 42},
		Objects: []world.Object{
			{ID: 59, UnitID: 6, Kind: world.ObjectKindTownPortal, Position: world.Position{X: 5170, Y: 5066}},
			{ID: 267, UnitID: 7, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 5121, Y: 5073}},
		},
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		t.Fatal(err)
	}
	route := town.SystemEgressRoute{
		SchemaVersion: town.SystemEgressSchemaVersion,
		Contract: town.SystemEgressContract{
			Act: town.OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777",
			LayoutFingerprint: town.SystemEgressLayoutFingerprint{
				Version: fingerprint.Version, AreaID: fingerprint.AreaID, AnchorCount: fingerprint.AnchorCount,
				Hash: fingerprint.Hash, Anchors: append([]string(nil), fingerprint.Anchors...),
			},
			From: town.AnchorPortalArrival, To: town.AnchorWaypoint,
			Movement: town.SystemEgressMovementWalk, ArrivalToleranceTiles: 15,
		},
		SampleDistanceTiles: 4,
		// First walk sample is farther than ArrivalToleranceTiles from the player,
		// matching the live Lut Gholein portal-vs-sample gap (~17 tiles).
		Points: []town.SystemEgressPoint{{X: 5183, Y: 5058}, {X: 5170, Y: 5059}},
	}
	if err := town.SaveSystemEgressRoute(filepath.Join(directory, town.SystemEgressFilename), route); err != nil {
		t.Fatal(err)
	}
	adapter := newTownEgressAdapter(config.NewLogger("error"), cfg, "3.2.92777", &preparationInputMock{}, pathing.DefaultConfig(), nil)
	if err := adapter.Start(town.OriginAct2, state); err != nil {
		t.Fatalf("portal-confirmed start away from Points[0] rejected: %v", err)
	}
}

func TestSystemEgressRouteRejectsTeleportAndFarmingFields(t *testing.T) {
	state, fingerprint := egressTestState(t)
	directory := t.TempDir()
	saveEgressTestRoute(t, directory, state, fingerprint)
	path := filepath.Join(directory, town.SystemEgressFilename)
	route, err := town.LoadSystemEgressRoute(path)
	if err != nil {
		t.Fatal(err)
	}
	route.Contract.Movement = town.SystemEgressMovement("teleport")
	if err := route.Validate(); err == nil {
		t.Fatal("teleport egress accepted")
	}
	data := []byte("schema_version: 1\ncharacter_name: MrBones\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := town.LoadSystemEgressRoute(path); err == nil {
		t.Fatal("farming binding field accepted")
	}
}
