package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestTownRecordingEdgeAllowsOnlyApprovedUnregisteredDraft(t *testing.T) {
	graph := town.ServiceGraph{Edges: []town.GraphEdge{{ID: "stash-akara", From: town.AnchorStash, To: town.AnchorAkara, Route: "stash-akara.yaml", Cost: 1}}}
	if edge, draft, ok := townRecordingEdge(graph, "stash-akara"); !ok || draft || edge.ID != "stash-akara" {
		t.Fatalf("registered edge = %+v draft=%v ok=%v", edge, draft, ok)
	}
	edge, draft, ok := townRecordingEdge(graph, "stash-waypoint")
	if !ok || !draft || edge.From != town.AnchorStash || edge.To != town.AnchorWaypoint || edge.Route != "stash-waypoint.yaml" || !edge.Reversible {
		t.Fatalf("draft edge = %+v draft=%v ok=%v", edge, draft, ok)
	}
	edge, draft, ok = townRecordingEdge(graph, "waypoint-kashya")
	if !ok || !draft || edge.From != town.AnchorWaypoint || edge.To != town.AnchorKashya || edge.Route != "waypoint-kashya.yaml" || !edge.Reversible {
		t.Fatalf("Kashya draft edge = %+v draft=%v ok=%v", edge, draft, ok)
	}
	if _, _, ok := townRecordingEdge(graph, "unknown-edge"); ok {
		t.Fatal("unknown recording draft accepted")
	}
}

func TestTownRecordingEndpointDistanceUsesDeclaredMemoryAnchor(t *testing.T) {
	state := world.State{
		Valid:    true,
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Monsters: []world.Monster{{UnitID: 1, NPCID: world.Akara, Position: world.Position{X: 109, Y: 112}}, {UnitID: 3, NPCID: world.Kashya, Position: world.Position{X: 112, Y: 100}}},
		Objects:  []world.Object{{UnitID: 2, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 110, Y: 100}}},
	}
	if distance, ok := townRecordingEndpointDistance(town.AnchorAkara, state); !ok || distance != 15 {
		t.Fatalf("Akara distance = %.1f ok=%t, want 15", distance, ok)
	}
	if distance, ok := townRecordingEndpointDistance(town.AnchorWaypoint, state); !ok || distance != 10 {
		t.Fatalf("Waypoint distance = %.1f ok=%t, want 10", distance, ok)
	}
	if distance, ok := townRecordingEndpointDistance(town.AnchorKashya, state); !ok || distance != 12 {
		t.Fatalf("Kashya distance = %.1f ok=%t, want 12", distance, ok)
	}
	if _, ok := townRecordingEndpointDistance(town.AnchorCain, state); ok {
		t.Fatal("missing Cain accepted")
	}
}

func TestTownRecordingObservationKeepsSamplingWhenLayoutAnchorsUnload(t *testing.T) {
	anchored := world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.LookupArea(world.RogueEncampment),
		Player: world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{
			{UnitID: 1, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 100, Y: 100}},
			{UnitID: 2, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 90, Y: 70}},
		},
	}
	pinned, sample, err := townRecordingObservation(anchored, town.TownLayoutFingerprint{})
	if err != nil || !sample || pinned.Hash == "" {
		t.Fatalf("initial observation = %+v sample=%t err=%v", pinned, sample, err)
	}

	unloaded := anchored
	unloaded.Player.Position = world.Position{X: 130, Y: 120}
	unloaded.Objects = nil
	got, sample, err := townRecordingObservation(unloaded, pinned)
	if err != nil || !sample || got.Hash != pinned.Hash {
		t.Fatalf("unloaded observation = %+v sample=%t err=%v", got, sample, err)
	}

	returned := anchored
	returned.Player.Position = world.Position{X: 160, Y: 130}
	got, sample, err = townRecordingObservation(returned, pinned)
	if err != nil || !sample || got.Hash != pinned.Hash {
		t.Fatalf("returned observation = %+v sample=%t err=%v", got, sample, err)
	}
}

func TestTownRecordingObservationBuffersBeforeFingerprintPin(t *testing.T) {
	state := world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.LookupArea(world.RogueEncampment),
		Player: world.Player{Position: world.Position{X: 150, Y: 120}},
	}
	observed, sample, err := townRecordingObservation(state, town.TownLayoutFingerprint{})
	if err != nil || !sample || observed.Hash != "" {
		t.Fatalf("pre-pin observation = %+v sample=%t err=%v", observed, sample, err)
	}
}

func TestTownRecordingObservationRejectsChangedReturnedLayout(t *testing.T) {
	state := world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.RogueEncampment),
		Objects: []world.Object{
			{UnitID: 1, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 100, Y: 100}},
			{UnitID: 2, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 90, Y: 70}},
		},
	}
	pinned, _, err := townRecordingObservation(state, town.TownLayoutFingerprint{})
	if err != nil {
		t.Fatal(err)
	}
	state.Objects[1].Position = world.Position{X: 130, Y: 80}
	if _, sample, err := townRecordingObservation(state, pinned); err == nil || sample {
		t.Fatalf("changed layout sample=%t err=%v", sample, err)
	}
}
