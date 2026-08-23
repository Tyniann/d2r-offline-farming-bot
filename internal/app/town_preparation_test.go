package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type preparationInputMock struct {
	moves, keys, clicks, modified int
	pressed                       []string
}

func (m *preparationInputMock) MoveTo(int, int) error         { m.moves++; return nil }
func (m *preparationInputMock) Click(input.MouseButton) error { m.clicks++; return nil }
func (m *preparationInputMock) ClickWithModifier(string, input.MouseButton) error {
	m.modified++
	return nil
}
func (m *preparationInputMock) PressKey(key string) error {
	m.keys++
	m.pressed = append(m.pressed, key)
	return nil
}
func (*preparationInputMock) CastSkillAt(input.BindingSource, uint16, int, int) error { return nil }
func (*preparationInputMock) Window() (input.WindowInfo, bool) {
	return input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}, true
}
func (*preparationInputMock) Status() input.Status { return input.Status{Enabled: true} }

func preparationState(pos world.Position, at time.Time, fullBelt bool) world.State {
	stash := world.Position{X: 100, Y: 100}
	state := world.State{
		At: at, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Player:    world.Player{Position: pos},
		Mercenary: world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, UnitID: 9, NPCID: 271, HP: 90, MaxHP: 90},
		Objects:   []world.Object{{UnitID: 1, Kind: world.ObjectKindPersonalStash, Position: stash}, {UnitID: 2, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 80, Y: 70}}},
		Monsters:  []world.Monster{{NPCID: world.Akara, UnitID: 3, Position: world.Position{X: 110, Y: 90}}, {NPCID: world.DeckardCain, UnitID: 4, Position: world.Position{X: 120, Y: 80}}, {NPCID: world.Charsi, UnitID: 5, Position: world.Position{X: 130, Y: 70}}, {NPCID: world.Kashya, UnitID: 6, Position: world.Position{X: 90, Y: 80}}},
	}
	if fullBelt {
		for i := 0; i < 4; i++ {
			state.Items = append(state.Items, world.Item{UnitID: uint32(i + 1), Type: "hpot", Location: world.ItemLocationBelt})
		}
		for i := 0; i < 8; i++ {
			state.Items = append(state.Items, world.Item{UnitID: uint32(i + 10), Type: "mpot", Location: world.ItemLocationBelt})
		}
	}
	return state
}

func TestLayoutTownWaypointWalkerReusesConfirmedWaypointHandoff(t *testing.T) {
	in := &preparationInputMock{}
	adapter := &townPreparationAdapter{log: config.NewLogger("error"), driver: in, pathCfg: pathing.DefaultConfig()}
	state := preparationState(world.Position{X: 80, Y: 70}, time.Now(), true)
	result := (&layoutTownWaypointWalker{adapter: adapter}).TickAct1Waypoint(context.Background(), state)
	if result.Status != pathing.TownWalkWaypointVisible || !result.Done {
		t.Fatalf("waypoint handoff result = %+v", result)
	}
	if adapter.started || in.moves != 0 || in.keys != 0 || in.clicks != 0 {
		t.Fatalf("waypoint handoff started route or input: started=%t input=%d/%d/%d", adapter.started, in.moves, in.keys, in.clicks)
	}
}

func TestLayoutTownWaypointWalkerDoesNotReuseHandoffAfterWalkStarted(t *testing.T) {
	// Once town walking has started, proximity alone must not short-circuit
	// acquire: that caused open/select while Force Move was still carrying the
	// character past the waypoint.
	adapter := &townPreparationAdapter{
		log:     config.NewLogger("error"),
		pathCfg: pathing.DefaultConfig(),
		started: true,
	}
	state := preparationState(world.Position{X: 80, Y: 70}, time.Now(), true)
	result := (&layoutTownWaypointWalker{adapter: adapter}).TickAct1Waypoint(context.Background(), state)
	if result.Status == pathing.TownWalkWaypointVisible {
		t.Fatalf("mid-walk handoff reused: %+v", result)
	}
	if result.Status != pathing.TownWalkRouteMissing || result.Reason != "town_preparation_state_invalid" || !result.Done {
		t.Fatalf("expected Tick failure after started walk, got %+v", result)
	}
}

func TestTownPreparationPortalStartUsesNearbyPortalProof(t *testing.T) {
	cfg := pathing.DefaultConfig()
	adapter := &townPreparationAdapter{pathCfg: cfg, startAnchor: town.AnchorPortalArrival}
	state := preparationState(world.Position{X: 100, Y: 100}, time.Now(), true)
	state.Objects = append(state.Objects, world.Object{UnitID: 9, Kind: world.ObjectKindTownPortal, Position: world.Position{X: 110, Y: 100}})

	if !adapter.externalStartConfirmed(state, world.Position{X: 120, Y: 100}) {
		t.Fatal("nearby Memory-confirmed portal arrival was rejected")
	}
	state.Objects[len(state.Objects)-1].Position = world.Position{X: 130, Y: 100}
	if adapter.externalStartConfirmed(state, world.Position{X: 120, Y: 100}) {
		t.Fatal("distant portal must not confirm portal arrival")
	}
}

func TestTownPreparationPlansPortalArrivalToAkara(t *testing.T) {
	directory := filepath.Join("..", "..", "configs", "routes", "town", "act1", "graph")
	graph, err := town.LoadServiceGraph(filepath.Join(directory, "graph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	const layout = "4ad7f3b4018ecb39ef311d808dcf0b0b9b5991d8b548b8d3ac1688a657c33f30"
	a := &townPreparationAdapter{
		log: config.NewLogger("error"), graph: graph, layout: layout,
		startAnchor: town.AnchorPortalArrival, targetAnchor: town.AnchorAkara,
	}
	if reason := a.start(preparationState(world.Position{X: 1, Y: 1}, time.Now(), true)); reason != "" {
		t.Fatal(reason)
	}
	if len(a.traversals) != 2 || a.traversals[0].Edge.ID != "portal-cain" ||
		a.traversals[1].Edge.ID != "akara-cain" || !a.traversals[1].Reverse {
		t.Fatalf("portal-to-Akara traversals = %+v", a.traversals)
	}
	if a.handoffAnchor() != town.AnchorAkara {
		t.Fatalf("handoff anchor = %q", a.handoffAnchor())
	}
}

func TestTownPreparationAkaraHandoffRequiresNearbyLiveNPC(t *testing.T) {
	state := preparationState(world.Position{X: 110, Y: 90}, time.Now(), true)
	if !townPreparationHandoffReady(state, town.AnchorAkara, 15) {
		t.Fatal("nearby live Akara was rejected")
	}
	state.Player.Position = world.Position{X: 140, Y: 90}
	if townPreparationHandoffReady(state, town.AnchorAkara, 15) {
		t.Fatal("distant Akara must not confirm the handoff")
	}
	state.Monsters = nil
	if townPreparationHandoffReady(state, town.AnchorAkara, 15) {
		t.Fatal("missing Akara must revoke the handoff")
	}
}

func TestTownPreparationNoServiceFromWaypointCompletesWithoutStashEdge(t *testing.T) {
	directory := filepath.Join("..", "..", "configs", "routes", "town", "act1", "graph")
	graph, err := town.LoadServiceGraph(filepath.Join(directory, "graph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := pathing.DefaultConfig()
	in := &preparationInputMock{}
	a := &townPreparationAdapter{
		log: config.NewLogger("error"), driver: in, pathCfg: cfg, graph: graph, directory: directory,
		layoutPin: &townLayoutPin{}, startAnchor: town.AnchorStash, nextRunID: "cows",
	}
	state := preparationState(world.Position{X: 80, Y: 70}, time.Now(), true)
	got := a.Tick(context.Background(), state)
	if !got.Done || got.Status != "complete" || in.moves != 0 {
		t.Fatalf("result=%+v moves=%d resolved=%q traversals=%d", got, in.moves, a.resolvedStart, len(a.traversals))
	}
	if a.resolvedStart != town.AnchorWaypoint {
		t.Fatalf("resolved=%q", a.resolvedStart)
	}
}

func TestTownPreparationPotionPlanFromWaypointUsesAkaraRoundtrip(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	in := &preparationInputMock{}
	a, err := newTownPreparationAdapter(config.NewLogger("error"), in, pathing.DefaultConfig(), cfg, "cows", &townLayoutPin{}, &preparationTelemetryMock{}, true)
	if err != nil {
		t.Fatal(err)
	}
	a.layout = "911703945495707c9e6578c2db467e76ed70cf0548f119ac1b397368a8af5a53"
	a.layoutOrigin = world.Position{X: 5466, Y: 4709}
	state := preparationState(world.Position{X: 80, Y: 70}, time.Now(), false)
	state.Player.Gold, state.Player.GoldKnown = 50938, true
	if reason := a.start(state); reason != "" {
		t.Fatal(reason)
	}
	if a.resolvedStart != town.AnchorWaypoint || a.handler == nil || a.handler.anchor != town.AnchorWaypoint {
		t.Fatalf("resolved=%q handler_anchor=%q", a.resolvedStart, a.handler.anchor)
	}
	if len(a.handler.traversals) != 2 {
		t.Fatalf("traversals=%+v", a.handler.traversals)
	}
	first := a.handler.traversals[0]
	if first.Edge.ID != "akara-waypoint" || !first.Reverse {
		t.Fatalf("expected reverse akara-waypoint first, got %+v", first)
	}
	if a.handler.traversals[1].Edge.ID != "akara-waypoint" || a.handler.traversals[1].Reverse {
		t.Fatalf("expected forward akara-waypoint second, got %+v", a.handler.traversals[1])
	}
	if !a.externalStartConfirmed(state, world.Position{}) {
		t.Fatal("waypoint origin must confirm without stash proximity")
	}
}

func TestTownPreparationStashStartUsesNearbyStashProof(t *testing.T) {
	cfg := pathing.DefaultConfig()
	adapter := &townPreparationAdapter{pathCfg: cfg, startAnchor: town.AnchorStash}
	// Player is within stash click range (15) but farther than TownWalk.ArrivalDistance (8)
	// from the recorded first walk sample — the live post-stash failure mode.
	state := preparationState(world.Position{X: 112, Y: 100}, time.Now(), true)
	if !adapter.externalStartConfirmed(state, world.Position{X: 100, Y: 100}) {
		t.Fatal("nearby Memory-confirmed stash origin was rejected")
	}
	state.Player.Position = world.Position{X: 140, Y: 100}
	if adapter.externalStartConfirmed(state, world.Position{X: 100, Y: 100}) {
		t.Fatal("distant stash must not confirm stash origin")
	}
}

func TestTownPreparationFailsBeforeInputWhenPotionGoldUnavailable(t *testing.T) {
	in := &preparationInputMock{}
	a := &townPreparationAdapter{
		log: config.NewLogger("error"), driver: in, pathCfg: pathing.DefaultConfig(), services: true,
		thresholds: town.Thresholds{Healing: 2, Mana: 2},
		profile: config.ProfileResourcesConfig{
			Healing: config.ResourceRuleConfig{BeltSlots: []int{1}}, Mana: config.ResourceRuleConfig{BeltSlots: []int{2, 3}}, Rejuvenation: config.ResourceRuleConfig{BeltSlots: []int{4}},
		},
	}
	got := a.Tick(context.Background(), preparationState(world.Position{X: 1, Y: 1}, time.Now(), false))
	if !got.Done || got.Reason != string(town.ReasonGoldUnavailable) || in.moves != 0 || in.keys != 0 {
		t.Fatalf("result=%+v input=%d/%d", got, in.moves, in.keys)
	}
}

func TestTownPreparationIgnoresUnboundLegacyRoutes(t *testing.T) {
	directory := filepath.Join("..", "..", "configs", "routes", "town", "act1", "graph")
	graph, err := town.LoadServiceGraph(filepath.Join(directory, "graph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	filtered := graph.Edges[:0]
	for _, edge := range graph.Edges {
		if edge.ID == "stash-waypoint" && len(edge.Variants) > 0 {
			edge.Route = edge.Variants[0].Route
			filtered = append(filtered, edge)
		}
	}
	graph.Edges = filtered
	in := &preparationInputMock{}
	a := &townPreparationAdapter{log: config.NewLogger("error"), driver: in, pathCfg: pathing.DefaultConfig(), graph: graph, directory: directory, thresholds: town.Thresholds{}}
	got := a.Tick(context.Background(), preparationState(world.Position{X: 100, Y: 100}, time.Now(), true))
	if !got.Done || got.Reason != string(town.ReasonTownLayoutRouteMissing) || in.moves != 0 || in.keys != 0 {
		t.Fatalf("legacy result=%+v input=%d/%d", got, in.moves, in.keys)
	}
}

func TestTownPreparationPlaysMinimalGraphToWaypointAndResets(t *testing.T) {
	directory := filepath.Join("..", "..", "configs", "routes", "town", "act1", "graph")
	graph, err := town.LoadServiceGraph(filepath.Join(directory, "graph.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	filtered := graph.Edges[:0]
	for _, edge := range graph.Edges {
		if edge.ID == "stash-waypoint" && len(edge.Variants) > 0 {
			edge.Route = edge.Variants[0].Route
			filtered = append(filtered, edge)
		}
	}
	graph.Edges = filtered
	fingerprint, reason := town.InspectTownLayout(preparationState(world.Position{X: 100, Y: 100}, time.Now(), true))
	if reason != "" {
		t.Fatal(reason)
	}
	boundDirectory := t.TempDir()
	for i := range graph.Edges {
		points, loadErr := pathing.LoadNamedTownRoute(filepath.Join(directory, graph.Edges[i].Route), graph.Edges[i].ID)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if saveErr := pathing.SaveLayoutBoundTownRoute(filepath.Join(boundDirectory, graph.Edges[i].Route), graph.Edges[i].ID, fingerprint.Hash, world.Position{X: fingerprint.StashX, Y: fingerprint.StashY}, 8, points); saveErr != nil {
			t.Fatal(saveErr)
		}
		graph.Edges[i].Variants = []town.GraphRouteVariant{{Layout: fingerprint.Hash, Route: graph.Edges[i].Route}}
	}
	traversals, err := graph.RouteForLayout(fingerprint.Hash, town.AnchorStash, nil, town.AnchorWaypoint)
	if err != nil {
		t.Fatal(err)
	}
	in := &preparationInputMock{}
	cfg := pathing.DefaultConfig()
	cfg.TownWalk.MoveInterval = time.Millisecond
	cfg.TownWalk.SettleTimeout = time.Millisecond
	cfg.TownWalk.StuckTimeout = time.Second
	a := &townPreparationAdapter{log: config.NewLogger("error"), driver: in, pathCfg: cfg, graph: graph, directory: boundDirectory, thresholds: town.Thresholds{Healing: 2, Mana: 4}, startAnchor: town.AnchorStash}
	now := time.Now()
	// Seed planning from the live Stash object before walking recorded samples.
	if got := a.Tick(context.Background(), preparationState(world.Position{X: 100, Y: 100}, now, true)); got.Done && got.Status == "failed" {
		t.Fatalf("stash seed result=%+v", got)
	}
	for _, traversal := range traversals {
		points, loadErr := pathing.LoadLayoutBoundTownRoute(filepath.Join(boundDirectory, traversal.Edge.Route), traversal.Edge.ID, fingerprint.Hash, world.Position{X: fingerprint.StashX, Y: fingerprint.StashY})
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if traversal.Reverse {
			reversePositions(points)
		}
		for _, point := range points {
			now = now.Add(10 * time.Millisecond)
			_ = a.Tick(context.Background(), preparationState(point, now, true))
		}
		now = now.Add(10 * time.Millisecond)
		_ = a.Tick(context.Background(), preparationState(points[len(points)-1], now, true))
	}
	final := preparationState(world.Position{X: 80, Y: 70}, now.Add(time.Second), true)
	got := a.Tick(context.Background(), final)
	if !got.Done || got.Status != "complete" {
		t.Fatalf("result=%+v keys=%d edge=%d/%d pressed=%v", got, in.keys, a.index, len(traversals), in.pressed)
	}
	a.Reset()
	if a.started || a.done || a.index != 0 || a.walker != nil || a.resolvedStart != "" {
		t.Fatalf("reset adapter=%+v", a)
	}
}

type preparationTelemetryMock struct{ events []town.ExecutorEvent }

func (m *preparationTelemetryMock) EmitTown(event town.ExecutorEvent) error {
	m.events = append(m.events, event)
	return nil
}

func TestTownPreparationBuildsProductivePotionPlanWithLiveGold(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	in := &preparationInputMock{}
	a, err := newTownPreparationAdapter(config.NewLogger("error"), in, pathing.DefaultConfig(), cfg, "countess", &townLayoutPin{}, &preparationTelemetryMock{}, true)
	if err != nil {
		t.Fatal(err)
	}
	a.layout = "911703945495707c9e6578c2db467e76ed70cf0548f119ac1b397368a8af5a53"
	a.layoutOrigin = world.Position{X: 5466, Y: 4709}
	state := preparationState(a.layoutOrigin, time.Now(), false)
	state.Player.Gold, state.Player.GoldKnown = 50938, true
	if reason := a.start(state); reason != "" {
		t.Fatal(reason)
	}
	if a.executor == nil || a.handler == nil || len(a.handler.orders) != 2 {
		t.Fatalf("executor=%v handler=%+v", a.executor != nil, a.handler)
	}
	for _, traversal := range a.handler.traversals {
		if traversal.Edge.From == town.AnchorStash && traversal.Edge.To == town.AnchorWaypoint {
			t.Fatal("potion demand incorrectly selected direct Stash-to-Waypoint route")
		}
	}
	if len(a.handler.traversals) != 2 || a.handler.traversals[0].Edge.To != town.AnchorAkara || a.handler.traversals[1].Edge.To != town.AnchorWaypoint {
		t.Fatalf("traversals=%+v", a.handler.traversals)
	}
}

func TestCowReadinessRequiresRejuvenationAndFillsAssignedColumns(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	in := &preparationInputMock{}
	a, err := newTownPreparationAdapter(config.NewLogger("error"), in, pathing.DefaultConfig(), cfg, "cows", &townLayoutPin{}, &preparationTelemetryMock{}, true)
	if err != nil {
		t.Fatal(err)
	}
	a.requireFullBuyableBelt, a.minimumRejuvenation = true, 1
	a.layout = "911703945495707c9e6578c2db467e76ed70cf0548f119ac1b397368a8af5a53"
	a.layoutOrigin = world.Position{X: 5466, Y: 4709}
	state := preparationState(a.layoutOrigin, time.Now(), false)
	state.Player.Gold, state.Player.GoldKnown = 50938, true
	for i := 0; i < 3; i++ {
		state.Items = append(state.Items, world.Item{UnitID: uint32(10 + i), Type: "hpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 0, GridY: i})
	}
	for i := 0; i < 7; i++ {
		state.Items = append(state.Items, world.Item{UnitID: uint32(20 + i), Type: "mpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 1 + i%2, GridY: i / 2})
	}
	if reason := a.start(state); reason != "cow_rejuvenation_reserve_missing" {
		t.Fatalf("missing rejuvenation reason=%q", reason)
	}
	state.Items = append(state.Items, world.Item{UnitID: 40, Type: "rpot", Location: world.ItemLocationBelt, PlayerOwned: true, GridX: 3})
	if reason := a.start(state); reason != "" {
		t.Fatal(reason)
	}
	if a.handler == nil || len(a.handler.orders) != 2 {
		t.Fatalf("orders=%+v", a.handler)
	}
	if a.handler.orders[0].Target != 4 || a.handler.orders[1].Target != 8 {
		t.Fatalf("orders=%+v", a.handler.orders)
	}
}

func TestCountProfilePotionSuppliesDecodesFlattenedBeltRows(t *testing.T) {
	profile := config.ProfileResourcesConfig{
		Healing:      config.ResourceRuleConfig{BeltSlots: []int{1}},
		Mana:         config.ResourceRuleConfig{BeltSlots: []int{2, 3}},
		Rejuvenation: config.ResourceRuleConfig{BeltSlots: []int{4}},
	}
	state := world.State{Items: []world.Item{
		{Type: "hpot", Location: world.ItemLocationBelt, GridX: 0}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 1}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 2}, {Type: "rpot", Location: world.ItemLocationBelt, GridX: 3},
		{Type: "hpot", Location: world.ItemLocationBelt, GridX: 4}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 5}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 6},
		{Type: "hpot", Location: world.ItemLocationBelt, GridX: 8}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 9}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 10},
		{Type: "hpot", Location: world.ItemLocationBelt, GridX: 12}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 13}, {Type: "mpot", Location: world.ItemLocationBelt, GridX: 14},
	}}
	healing, mana, rejuvenation := countProfilePotionSupplies(state, profile)
	if healing != 4 || mana != 8 || rejuvenation != 1 {
		t.Fatalf("supplies=%d/%d/%d, want 4/8/1", healing, mana, rejuvenation)
	}
}

func TestPurchaseCostUsesActualNightmareVendorOffer(t *testing.T) {
	state := world.State{Items: []world.Item{{TxtFileNo: 1, Code: "hp4", Type: "hpot", Location: world.ItemLocationVendor}, {TxtFileNo: 2, Code: "mp4", Type: "mpot", Location: world.ItemLocationVendor}}}
	profile := config.ProfileResourcesConfig{Mana: config.ResourceRuleConfig{BeltSlots: []int{2, 3}}}
	code, cost, ok := purchaseCostForState(state, profile, town.RestockOrder{Resource: town.RestockMana, Mode: town.BuyModeBulk, Target: 8})
	if !ok || code != "mp4" || cost != 4000 {
		t.Fatalf("purchase=%q/%d/%v", code, cost, ok)
	}
}

func TestCountInventoryKeysSumsKnownStacksAndTreatsUnknownAsZero(t *testing.T) {
	state := world.State{Items: []world.Item{
		{Code: "key", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Quantity: 5, QuantityKnown: true},
		{Code: "key", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Quantity: 4, QuantityKnown: true},
		{Code: "key", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, QuantityKnown: false},
		{Code: "pk1", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Quantity: 1, QuantityKnown: true},
		{Code: "key", Location: world.ItemLocationStash, PlayerOwned: true, Page: 0, Quantity: 12, QuantityKnown: true},
	}}
	if got := state.InventoryQuantityByCode(town.KeyItemCode); got != 9 {
		t.Fatalf("keys=%d, want 9", got)
	}
}

func TestPurchaseCostForCityKeyIgnoresOtherKeyTypes(t *testing.T) {
	state := world.State{Items: []world.Item{
		{TxtFileNo: 559, Code: "luv", Type: "key", Location: world.ItemLocationVendor},
		{TxtFileNo: 558, Code: "key", Type: "key", Location: world.ItemLocationVendor},
	}}
	code, cost, ok := purchaseCostForState(state, config.ProfileResourcesConfig{}, town.RestockOrder{Resource: town.RestockKey, Mode: town.BuyModeSingle, Target: town.KeyRestockTarget})
	if !ok || code != "key" || cost != 45 {
		t.Fatalf("purchase=%q/%d/%v", code, cost, ok)
	}
}

func TestTownPotionHandlerBuysOnceAndVerifiesTarget(t *testing.T) {
	in := &preparationInputMock{}
	a := &townPreparationAdapter{
		log: config.NewLogger("error"), driver: in, controller: in, thresholds: town.Thresholds{Healing: 2},
		profile: config.ProfileResourcesConfig{Healing: config.ResourceRuleConfig{BeltSlots: []int{1}}},
	}
	h := &townPreparationStepHandler{adapter: a, stage: "orders", anchor: town.AnchorAkara, orders: []town.RestockOrder{{Resource: town.RestockHealing, Mode: town.BuyModeBulk, Before: 0, Target: 4, Clicks: 1}}}
	state := world.State{Valid: true, UI: world.UIState{NPCShopOpen: true}, Player: world.Player{Gold: 50938, GoldKnown: true}, Items: []world.Item{{TxtFileNo: 10, UnitID: 99, Code: "hp4", Type: "hpot", Location: world.ItemLocationVendor, GridX: 1, GridY: 2}}}
	step := town.PlanStep{Kind: town.StepService, Service: town.ServicePotions}
	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionPending || h.buyer == nil {
		t.Fatalf("authorize=%+v buyer=%v", got, h.buyer != nil)
	}
	if got := h.Tick(context.Background(), step, state); got.Action != "vendor_move" || got.Cost != 1000 {
		t.Fatalf("move=%+v", got)
	}
	if got := h.Tick(context.Background(), step, state); got.Action != "vendor_buy_bulk" || in.modified != 1 || got.Cost != 1000 {
		t.Fatalf("buy=%+v modified=%d", got, in.modified)
	}
	h.settleUntil = time.Time{}
	for i := 0; i < 4; i++ {
		state.Items = append(state.Items, world.Item{UnitID: uint32(200 + i), Type: "hpot", Location: world.ItemLocationBelt})
	}
	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionPending || h.buyer != nil || got.Reason != "" {
		t.Fatalf("post-purchase=%+v buyer=%v", got, h.buyer != nil)
	}
	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionPending || h.order != 1 {
		t.Fatalf("verify=%+v order=%d", got, h.order)
	}
	_ = h.Tick(context.Background(), step, state)
	if got := h.Tick(context.Background(), step, state); got.Action != "shop_close" {
		t.Fatalf("close=%+v", got)
	}
	state.UI.NPCShopOpen = false
	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionComplete || in.modified != 1 {
		t.Fatalf("complete=%+v modified=%d", got, in.modified)
	}
}

type npcClickerStub struct {
	results []town.NPCClickResult
}

func (s *npcClickerStub) TickNPC(world.State, town.NPCClickTarget, float64) (town.NPCClickResult, error) {
	if len(s.results) == 0 {
		return town.NPCClickResult{}, nil
	}
	r := s.results[0]
	s.results = s.results[1:]
	return r, nil
}
func (*npcClickerStub) Reset() {}

func TestMercenaryHealCompletesWithoutClosingAkaraDialog(t *testing.T) {
	trace := &preparationTelemetryMock{}
	adapter := &townPreparationAdapter{
		log:     config.NewLogger("error"),
		pathCfg: pathing.DefaultConfig(),
		profile: config.ProfileResourcesConfig{
			Mercenary: config.MercenaryResourceConfig{UseBelowPercent: 50, BeltSlots: []int{1}, CooldownMs: 4000},
		},
		telemetry: trace,
	}
	h := newTownPreparationStepHandler(adapter, nil, nil, nil)
	h.stage = "interact"
	h.npc = town.NewNPCInteractor(&npcClickerStub{results: []town.NPCClickResult{{Clicked: true, Done: true}}}, world.Akara, 15, time.Second)

	state := preparationState(world.Position{X: 110, Y: 90}, time.Now(), true)
	state.Mercenary = world.Mercenary{HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, UnitID: 9, NPCID: 271, HP: 40, MaxHP: 100}
	step := town.PlanStep{Phase: town.PlanPhaseServices, Kind: town.StepService, Service: town.ServiceMercenaryHeal}

	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionAction || got.Action != "npc_click" {
		t.Fatalf("click=%+v", got)
	}
	state.UI.NPCInteractOpen = true
	if got := h.Tick(context.Background(), step, state); got.Reason == "npc_ui_preopened" {
		t.Fatalf("own Akara dialog rejected as preopened: %+v", got)
	}
	if h.stage != "verify" {
		t.Fatalf("stage=%q after dialog open, want verify", h.stage)
	}
	if len(trace.events) == 0 || trace.events[0].Event != "town_mercenary_heal_requested" {
		t.Fatalf("heal requested telemetry=%+v", trace.events)
	}
	state.Mercenary.HP = state.Mercenary.MaxHP
	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionComplete || !got.Done {
		t.Fatalf("confirmed heal=%+v", got)
	}
	if !state.UI.NPCInteractOpen {
		t.Fatal("test setup unexpectedly closed Akara dialog")
	}
}

func TestPotionNPCClickDoesNotRejectOwnAkaraDialog(t *testing.T) {
	adapter := &townPreparationAdapter{
		log:     config.NewLogger("error"),
		pathCfg: pathing.DefaultConfig(),
		profile: config.ProfileResourcesConfig{Healing: config.ResourceRuleConfig{BeltSlots: []int{1}}},
	}
	h := newTownPreparationStepHandler(adapter, nil, nil, nil)
	h.stage = "npc"
	h.npc = town.NewNPCInteractor(&npcClickerStub{results: []town.NPCClickResult{{Clicked: true, Done: true}}}, world.Akara, 15, time.Second)
	state := preparationState(world.Position{X: 110, Y: 90}, time.Now(), true)
	state.Monsters = []world.Monster{{NPCID: world.Akara, UnitID: 14, Position: world.Position{X: 110, Y: 90}}}
	step := town.PlanStep{Kind: town.StepService, Service: town.ServicePotions}

	if got := h.Tick(context.Background(), step, state); got.Status != town.InteractionAction || got.Action != "npc_click" {
		t.Fatalf("click=%+v", got)
	}
	if !h.authorizedAkaraDialog {
		t.Fatal("own Akara click must authorize the following dialog tick")
	}
	state.UI.NPCInteractOpen = true
	if got := h.Tick(context.Background(), step, state); got.Reason == "npc_ui_preopened" {
		t.Fatalf("own Akara dialog rejected as preopened: %+v", got)
	}
	if h.stage != "shop" {
		t.Fatalf("stage=%q after dialog open, want shop", h.stage)
	}
}
