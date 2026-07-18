package town

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestSystemEgressContractIsGlobalAndWalkOnly(t *testing.T) {
	for _, act := range []OriginAct{OriginAct2, OriginAct3, OriginAct4, OriginAct5} {
		area, _ := TownAreaForAct(act)
		contract := SystemEgressContract{Act: act, TownArea: area, GameVersion: "3.2.92777", LayoutFingerprint: SystemEgressLayoutFingerprint{Version: 1, AreaID: area, AnchorCount: 2, Hash: "fingerprint"}, From: AnchorPortalArrival, To: AnchorWaypoint, Movement: SystemEgressMovementWalk, ArrivalToleranceTiles: 3}
		if err := contract.Validate(); err != nil {
			t.Fatalf("%s: %v", act, err)
		}
		contract.Movement = "teleport"
		if err := contract.Validate(); err == nil {
			t.Fatalf("%s teleport system Egress accepted", act)
		}
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
