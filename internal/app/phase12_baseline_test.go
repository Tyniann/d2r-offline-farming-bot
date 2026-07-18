package app

import (
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPhase12BaselineProductiveRouteFiles(t *testing.T) {
	tests := []struct {
		path         string
		sha256       string
		routeID      string
		segments     int
		startArea    world.AreaID
		terminalArea world.AreaID
	}{
		{
			path:   filepath.Join("..", "..", "configs", "routes", "farming", "mrbones", "nightmare", "black-marsh-cellar5-nightmare-mrbones.yaml"),
			sha256: "8d3dbd3bfc693689739aab56dbb9086d606d9eb9691629dfef5abad701ee0d1f", routeID: "black-marsh-cellar5-nightmare-mrbones", segments: 7,
			startArea: world.BlackMarsh, terminalArea: world.TowerCellarLevel5,
		},
		{
			path:   filepath.Join("..", "..", "configs", "routes", "farming", "mrbones", "nightmare", "countess-mrbones-fd1756c208.yaml"),
			sha256: "c521a23e8b4fa72a59f48ab897961b9e6b57beedd04ccbcd30b7b565147b161e", routeID: "countess-mrbones-fd1756c208", segments: 7,
			startArea: world.BlackMarsh, terminalArea: world.TowerCellarLevel5,
		},
		{
			path:   filepath.Join("..", "..", "configs", "routes", "farming", "mrbones", "nightmare", "durance-2-mephisto-nightmare-mrbones.yaml"),
			sha256: "58752d884dfdf3db06aa546c9371de8326cd053994dd8e5a92393a130085b1f4", routeID: "durance-2-mephisto-nightmare-mrbones", segments: 2,
			startArea: world.DuranceOfHateLevel2, terminalArea: world.DuranceOfHateLevel3,
		},
	}
	for _, test := range tests {
		t.Run(test.routeID, func(t *testing.T) {
			fingerprint, err := fileSHA256(test.path)
			if err != nil {
				t.Fatal(err)
			}
			if fingerprint != test.sha256 {
				t.Fatalf("route file SHA-256 = %s, want %s", fingerprint, test.sha256)
			}
			route, err := pathing.LoadRoute(test.path)
			if err != nil {
				t.Fatal(err)
			}
			if route.ID != test.routeID || len(route.Segments) != test.segments || route.Segments[0].FromAreaID != test.startArea || route.Segments[len(route.Segments)-1].ToAreaID != test.terminalArea || route.Segments[len(route.Segments)-1].Transition.Type != "terminal" {
				t.Fatalf("productive route contract changed: %+v", route)
			}
		})
	}
}

func TestPhase12BaselineTownAssets(t *testing.T) {
	tests := map[string]string{
		filepath.Join("..", "..", "configs", "routes", "town", "act1", "graph", "graph.yaml"):            "806d48adabd46c29493228fe44adbbf34e70ed844674545e8a01f21d35ae079f",
		filepath.Join("..", "..", "configs", "routes", "town", "act3", "egress", "portal-waypoint.yaml"): "715e012f8fd77a9ea5ba2c405d422c15b85a3caf151df6cb3c1c348778652a02",
	}
	for path, want := range tests {
		got, err := fileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s SHA-256 = %s, want %s", path, got, want)
		}
	}
	route, err := town.LoadSystemEgressRoute(filepath.Join("..", "..", "configs", "routes", "town", "act3", "egress", town.SystemEgressFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := route.Validate(); err != nil {
		t.Fatalf("productive Act-3 Egress contract: %v", err)
	}
}

func TestPhase12BaselineAvailabilityAndQueue(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "countess-mrbones-fd1756c208", tasks.RunIDMephisto: "durance-2-mephisto-nightmare-mrbones"})

	store, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, catalog, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) != 3 {
		t.Fatalf("Farming catalog entries = %d, want 3", len(catalog.Entries))
	}
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{Character: "MrBones", CharacterClass: "necromancer", Difficulty: "nightmare", GameVersion: cfg.Memory.GameVersion})
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []tasks.RunID{tasks.RunIDCountess, tasks.RunIDMephisto} {
		availability, ok := findRunAvailability(report.Runs, runID)
		if !ok || availability.Status != tasks.RunAvailabilityRuntimeValidationRequired || len(availability.Reasons) != 1 || availability.Reasons[0] != tasks.RunReasonRouteRuntimeValidation {
			t.Fatalf("%s availability = %+v", runID, availability)
		}
	}
	plan, err := ValidateFarmQueue(cfg, FarmQueueValidationRequest{
		RunIDs: []string{"countess", "mephisto"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: catalog.Revision,
	}, FarmQueueValidationContext{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: manifest.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.RunIDs) != 2 || plan.RunIDs[0] != "countess" || plan.RunIDs[1] != "mephisto" {
		t.Fatalf("queue plan = %+v", plan)
	}
}
