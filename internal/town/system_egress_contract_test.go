package town

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

func TestSystemEgressContractIsGlobalAndWalkOnly(t *testing.T) {
	for _, act := range []OriginAct{OriginAct2, OriginAct3, OriginAct4, OriginAct5} {
		area, _ := TownAreaForAct(act)
		for _, from := range []Anchor{AnchorPortalArrival, AnchorSpawn} {
			contract := SystemEgressContract{Act: act, TownArea: area, GameVersion: "3.2.92777", LayoutFingerprint: SystemEgressLayoutFingerprint{Version: 1, AreaID: area, AnchorCount: 2, Hash: "fingerprint"}, From: from, To: AnchorWaypoint, Movement: SystemEgressMovementWalk, ArrivalToleranceTiles: 3}
			if err := contract.Validate(); err != nil {
				t.Fatalf("%s/%s: %v", act, from, err)
			}
			contract.Movement = "teleport"
			if err := contract.Validate(); err == nil {
				t.Fatalf("%s/%s teleport system Egress accepted", act, from)
			}
		}
	}
}

func TestSystemEgressFilenameMatchesStartAnchor(t *testing.T) {
	for _, test := range []struct {
		anchor Anchor
		name   string
	}{{AnchorPortalArrival, "portal-waypoint.yaml"}, {AnchorSpawn, "spawn-waypoint.yaml"}} {
		if got, err := SystemEgressFilenameForAnchor(test.anchor); err != nil || got != test.name {
			t.Fatalf("anchor %s filename=%q err=%v", test.anchor, got, err)
		}
	}
	if _, err := SystemEgressFilenameForAnchor(AnchorStash); err == nil {
		t.Fatal("stash start filename accepted")
	}
}

func TestLoadSystemEgressRouteRejectsFilenameContractMismatch(t *testing.T) {
	directory := t.TempDir()
	proofPoint := 1
	route := SystemEgressRoute{SchemaVersion: 1, Contract: SystemEgressContract{Act: OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: SystemEgressLayoutFingerprint{Version: 1, AreaID: world.LutGholein, AnchorCount: 1, Hash: "layout"}, LayoutProofPointIndex: &proofPoint, From: AnchorSpawn, To: AnchorWaypoint, Movement: SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []SystemEgressPoint{{X: 1, Y: 1}, {X: 2, Y: 2}}}
	wrongPath := filepath.Join(directory, SystemEgressFilename)
	data, err := yaml.Marshal(route)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrongPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSystemEgressRoute(wrongPath); err == nil || !strings.Contains(err.Error(), "filename") {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestSpawnSystemEgressRequiresBoundedLayoutProofPoint(t *testing.T) {
	base := SystemEgressRoute{SchemaVersion: 1, Contract: SystemEgressContract{Act: OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: SystemEgressLayoutFingerprint{Version: 1, AreaID: world.LutGholein, AnchorCount: 1, Hash: "layout"}, From: AnchorSpawn, To: AnchorWaypoint, Movement: SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []SystemEgressPoint{{X: 1, Y: 1}, {X: 2, Y: 2}}}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "layout proof point") {
		t.Fatalf("missing proof point error=%v", err)
	}
	negative := -1
	base.Contract.LayoutProofPointIndex = &negative
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "layout proof point") {
		t.Fatalf("negative proof point error=%v", err)
	}
	beyond := len(base.Points)
	base.Contract.LayoutProofPointIndex = &beyond
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "layout proof point") {
		t.Fatalf("out-of-range proof point error=%v", err)
	}
	valid := 1
	base.Contract.LayoutProofPointIndex = &valid
	if err := base.Validate(); err != nil {
		t.Fatalf("valid proof point rejected: %v", err)
	}
}

func TestSystemEgressPersistenceContainsNoFarmingBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), SystemEgressFilename)
	route := SystemEgressRoute{SchemaVersion: 1, Contract: SystemEgressContract{Act: OriginAct5, TownArea: world.Harrogath, GameVersion: "3.2.92777", LayoutFingerprint: SystemEgressLayoutFingerprint{Version: 1, AreaID: world.Harrogath, AnchorCount: 1, Hash: "layout"}, From: AnchorPortalArrival, To: AnchorWaypoint, Movement: SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []SystemEgressPoint{{X: 1, Y: 1}, {X: 2, Y: 2}}}
	if err := SaveSystemEgressRoute(path, route); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"character", "class", "difficulty", "map_seed", "route_id"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("global Egress contains forbidden field %q:\n%s", forbidden, data)
		}
	}
}

func TestPublishedSpawnSystemEgressBundle(t *testing.T) {
	for _, test := range []struct {
		act   OriginAct
		area  world.AreaID
		start SystemEgressPoint
	}{
		{act: OriginAct2, area: world.LutGholein, start: SystemEgressPoint{X: 5153, Y: 5203}},
		{act: OriginAct3, area: world.KurastDocks, start: SystemEgressPoint{X: 5118, Y: 5168}},
		{act: OriginAct4, area: world.ThePandemoniumFortress, start: SystemEgressPoint{X: 5048, Y: 5043}},
		{act: OriginAct5, area: world.Harrogath, start: SystemEgressPoint{X: 5098, Y: 5023}},
	} {
		t.Run(string(test.act), func(t *testing.T) {
			path := filepath.Join("..", "..", "configs", "routes", "town", string(test.act), "egress", SystemEgressSpawnFilename)
			route, err := LoadSystemEgressRoute(path)
			if err != nil {
				t.Fatalf("load published spawn Egress route: %v", err)
			}
			if route.Contract.Act != test.act || route.Contract.TownArea != test.area {
				t.Fatalf("contract act/area = %s/%d, want %s/%d", route.Contract.Act, route.Contract.TownArea, test.act, test.area)
			}
			if route.Contract.GameVersion != "3.2.92777" {
				t.Fatalf("game version = %q, want 3.2.92777", route.Contract.GameVersion)
			}
			if route.Contract.From != AnchorSpawn || route.Contract.To != AnchorWaypoint || route.Contract.Movement != SystemEgressMovementWalk {
				t.Fatalf("route contract = %s -> %s via %s, want spawn -> waypoint via walk", route.Contract.From, route.Contract.To, route.Contract.Movement)
			}
			if len(route.Points) == 0 {
				t.Fatal("route has no points")
			}
			if route.Points[0] != test.start {
				t.Fatalf("start point = %+v, want %+v", route.Points[0], test.start)
			}
		})
	}
}

func TestSaveSystemEgressRouteAtomicallyReplacesInvalidDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), SystemEgressFilename)
	if err := os.WriteFile(path, []byte("invalid: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	route := SystemEgressRoute{SchemaVersion: 1, Contract: SystemEgressContract{Act: OriginAct2, TownArea: world.LutGholein, GameVersion: "3.2.92777", LayoutFingerprint: SystemEgressLayoutFingerprint{Version: 1, AreaID: world.LutGholein, AnchorCount: 1, Hash: "layout"}, From: AnchorPortalArrival, To: AnchorWaypoint, Movement: SystemEgressMovementWalk, ArrivalToleranceTiles: 3}, SampleDistanceTiles: 4, Points: []SystemEgressPoint{{X: 1, Y: 1}, {X: 2, Y: 2}}}
	if err := SaveSystemEgressRoute(path, route); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSystemEgressRoute(path)
	if err != nil || loaded.Contract.Act != OriginAct2 {
		t.Fatalf("replacement route=%+v err=%v", loaded, err)
	}
}
