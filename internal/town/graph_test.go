package town

import (
	"os"
	"path/filepath"
	"testing"
)

func loadCommittedGraph(t *testing.T) ServiceGraph {
	t.Helper()
	graph, err := LoadServiceGraph(filepath.Join("..", "..", "configs", "routes", "town", "act1", "graph", "graph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func TestGraphSelectsOnlyRequiredCainAndAkaraRoute(t *testing.T) {
	graph := loadCommittedGraph(t)
	route, err := graph.Route(AnchorPortalArrival, []Anchor{AnchorCain, AnchorAkara}, AnchorWaypoint)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id      string
		reverse bool
	}{{"portal-cain", false}, {"akara-cain", true}, {"akara-waypoint", false}}
	if len(route) != len(want) {
		t.Fatalf("route = %+v", route)
	}
	for i := range want {
		if route[i].Edge.ID != want[i].id || route[i].Reverse != want[i].reverse {
			t.Fatalf("route[%d] = %+v, want %+v", i, route[i], want[i])
		}
	}
}

func TestGraphCombinedAcceptanceRouteVisitsEveryServiceOnce(t *testing.T) {
	graph := loadCommittedGraph(t)
	route, err := graph.Route(AnchorSpawn, []Anchor{AnchorStash, AnchorAkara, AnchorCain, AnchorCharsi}, AnchorWaypoint)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"stash-akara", "akara-cain", "cain-charsi", "charsi-waypoint"}
	if len(route) != len(want) {
		t.Fatalf("route = %+v", route)
	}
	for i, id := range want {
		if route[i].Edge.ID != id || route[i].Reverse {
			t.Fatalf("route[%d] = %+v, want %s forward", i, route[i], id)
		}
	}
}

func TestGraphTreatsSpawnAsStashWithoutNavigation(t *testing.T) {
	graph := loadCommittedGraph(t)
	route, err := graph.Route(AnchorSpawn, []Anchor{AnchorStash}, AnchorWaypoint)
	if err != nil {
		t.Fatal(err)
	}
	if len(route) != 1 || route[0].Edge.ID != "stash-waypoint" {
		t.Fatalf("route = %+v, want direct stash-waypoint edge", route)
	}
}

func TestGraphRouteForLayoutIgnoresLegacyAndSelectsExactVariants(t *testing.T) {
	graph := loadCommittedGraph(t)
	const layout = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := graph.RouteForLayout(layout, AnchorStash, nil, AnchorWaypoint); err == nil || err.Error() != string(ReasonTownLayoutRouteMissing) {
		t.Fatalf("legacy route was selected or wrong error: %v", err)
	}
	for i := range graph.Edges {
		route := graph.Edges[i].Route
		if route == "" && len(graph.Edges[i].Variants) > 0 {
			route = graph.Edges[i].Variants[0].Route
		}
		graph.Edges[i].Variants = []GraphRouteVariant{{Layout: layout, Route: route}}
	}
	route, err := graph.RouteForLayout(layout, AnchorStash, nil, AnchorWaypoint)
	if err != nil || len(route) != 1 || route[0].Edge.ID != "stash-waypoint" {
		t.Fatalf("layout route=%+v err=%v", route, err)
	}
	const other = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := graph.RouteForLayout(other, AnchorStash, nil, AnchorWaypoint); err == nil || err.Error() != string(ReasonTownLayoutRouteMissing) {
		t.Fatalf("unknown layout error=%v", err)
	}
}

func TestGraphOrderedLayoutRoutePreservesCainBeforeAkara(t *testing.T) {
	graph := loadCommittedGraph(t)
	const layout = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for i := range graph.Edges {
		route := graph.Edges[i].Route
		if route == "" && len(graph.Edges[i].Variants) > 0 {
			route = graph.Edges[i].Variants[0].Route
		}
		graph.Edges[i].Variants = []GraphRouteVariant{{Layout: layout, Route: route}}
	}
	route, err := graph.RouteOrderedForLayout(layout, AnchorStash, []Anchor{AnchorCain, AnchorAkara}, AnchorWaypoint)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id      string
		reverse bool
	}{{"stash-akara", false}, {"akara-cain", false}, {"akara-cain", true}, {"akara-waypoint", false}}
	if len(route) != len(want) {
		t.Fatalf("route=%+v", route)
	}
	for index := range want {
		if route[index].Edge.ID != want[index].id || route[index].Reverse != want[index].reverse {
			t.Fatalf("route[%d]=%+v want=%+v", index, route[index], want[index])
		}
	}
}

func TestGraphRejectsUnsafeRouteAndNonReversibleReverse(t *testing.T) {
	graph := loadCommittedGraph(t)
	graph.Edges[0].Route = "../escape.yaml"
	if err := graph.Validate(); err == nil {
		t.Fatal("escaping route accepted")
	}
	graph = ServiceGraph{Version: ServiceGraphVersion, AreaID: 1, Edges: []GraphEdge{{ID: "one-way", From: AnchorSpawn, To: AnchorWaypoint, Route: "one.yaml", Cost: 1}}}
	if _, err := graph.Route(AnchorWaypoint, nil, AnchorSpawn); err == nil {
		t.Fatal("non-reversible edge replayed backwards")
	}
}

func TestGraphKashyaRouteUsesActivatedLayoutVariants(t *testing.T) {
	graph := loadCommittedGraph(t)
	const layout = "76876927307330686a85da717370ee70ec4e0c2a47dcb7fa4bdba91cc9017381"
	route, err := graph.RouteForLayout(layout, AnchorStash, []Anchor{AnchorKashya}, AnchorWaypoint)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id      string
		reverse bool
	}{{"stash-waypoint", false}, {"waypoint-kashya", false}, {"waypoint-kashya", true}}
	if len(route) != len(want) {
		t.Fatalf("route = %+v", route)
	}
	for i := range want {
		if route[i].Edge.ID != want[i].id || route[i].Reverse != want[i].reverse {
			t.Fatalf("route[%d] = %+v, want %+v", i, route[i], want[i])
		}
	}
	ordered, err := graph.RouteOrderedForLayout(layout, AnchorWaypoint, []Anchor{AnchorKashya}, AnchorWaypoint)
	if err != nil || len(ordered) != 2 || ordered[0].Edge.ID != "waypoint-kashya" || ordered[0].Reverse || !ordered[1].Reverse {
		t.Fatalf("ordered waypoint-kashya roundtrip = %+v err=%v", ordered, err)
	}
}

func TestGraphKashyaMissingOnUnknownLayout(t *testing.T) {
	graph := loadCommittedGraph(t)
	const unknown = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := graph.RouteForLayout(unknown, AnchorStash, []Anchor{AnchorKashya}, AnchorWaypoint); err == nil || err.Error() != string(ReasonTownLayoutRouteMissing) {
		t.Fatalf("unknown layout must fail closed: %v", err)
	}
}

func TestEgressFormatRejectsServiceFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "egress.yaml")
	body := "act: act3\narea: kurast_docks\nroute_id: act3-egress\nanchors: [portal_arrival, waypoint]\nservices: [akara]\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEgressRoute(path); err == nil {
		t.Fatal("egress services field accepted")
	}
}
