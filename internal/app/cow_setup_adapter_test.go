package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type cowWirtApproachMock struct {
	active bool
	goals  []pathing.Goal
	result pathing.NavTickResult
}

func (m *cowWirtApproachMock) Active() bool { return m.active }
func (m *cowWirtApproachMock) Start(goal pathing.Goal) error {
	m.goals = append(m.goals, goal)
	m.active = true
	return nil
}
func (m *cowWirtApproachMock) Tick(context.Context, world.State) pathing.NavTickResult {
	if m.result.Done {
		m.active = false
	}
	return m.result
}
func (m *cowWirtApproachMock) Reset() { m.active = false }

func TestCowWirtInteractionPinsFreshNearbyLeg(t *testing.T) {
	in := &preparationInputMock{}
	pathCfg := pathing.DefaultConfig()
	clickCfg := pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	adapter := &cowSetupAdapter{
		log: config.NewLogger("error"), controller: in, pathCfg: pathCfg,
		wirtClicker: pathing.NewEntityClicker(config.NewLogger("error"), in, pathCfg.Projector(), clickCfg),
	}
	now := time.Now()
	state := world.State{
		At: now, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.Tristram),
		Player:  world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{{ID: world.WirtsBodyID, Kind: world.ObjectKindWirtsBody, UnitID: 2680, Position: world.Position{X: 103, Y: 103}, Name: "Wirt's Body"}},
		Items:   []world.Item{{UnitID: 50, Code: "leg", Location: world.ItemLocationGround, Position: world.Position{X: 130, Y: 130}}},
	}
	if result := adapter.TickWirt(context.Background(), state); result.Done || in.moves != 1 || in.clicks != 0 {
		t.Fatalf("first Wirt tick=%+v input moves/clicks=%d/%d", result, in.moves, in.clicks)
	}
	state.At = now.Add(100 * time.Millisecond)
	state.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 2680}
	if result := adapter.TickWirt(context.Background(), state); result.Done || in.clicks != 1 {
		t.Fatalf("hover-confirmed Wirt tick=%+v clicks=%d", result, in.clicks)
	}
	state.At = now.Add(200 * time.Millisecond)
	state.Hover = world.HoverInfo{}
	state.Items = append(state.Items, world.Item{UnitID: 77, TxtFileNo: 88, Code: "leg", Name: "Wirt's Leg", Location: world.ItemLocationGround, Position: world.Position{X: 104, Y: 103}})
	if result := adapter.TickWirt(context.Background(), state); !result.Done || result.Reason != "" || result.UnitID != 77 || in.clicks != 1 {
		t.Fatalf("fresh leg result=%+v clicks=%d", result, in.clicks)
	}
}

func TestCowWirtInteractionApproachesBeforeHoverClick(t *testing.T) {
	in := &preparationInputMock{}
	pathCfg := pathing.DefaultConfig()
	clickCfg := pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	approach := &cowWirtApproachMock{}
	adapter := &cowSetupAdapter{
		log: config.NewLogger("error"), controller: in, pathCfg: pathCfg, wirtApproach: approach,
		wirtClicker: pathing.NewEntityClicker(config.NewLogger("error"), in, pathCfg.Projector(), clickCfg),
	}
	now := time.Now()
	state := world.State{
		At: now, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.Tristram),
		Player:  world.Player{Position: world.Position{X: 100, Y: 100}},
		Objects: []world.Object{{ID: world.WirtsBodyID, Kind: world.ObjectKindWirtsBody, UnitID: 2680, Position: world.Position{X: 112, Y: 100}, Name: "Wirt's Body"}},
	}
	if result := adapter.TickWirt(context.Background(), state); result.Done || len(approach.goals) != 1 || in.moves != 0 {
		t.Fatalf("approach start=%+v goals=%+v mouse_moves=%d", result, approach.goals, in.moves)
	}
	if goal := approach.goals[0]; goal.TargetPos != state.Objects[0].Position || goal.ArrivalDistance != cowWirtClickDistance {
		t.Fatalf("approach goal=%+v", goal)
	}

	state.At = now.Add(time.Second)
	state.Player.Position = world.Position{X: 105, Y: 100}
	approach.result = pathing.NavTickResult{Status: pathing.NavArrived, Done: true}
	if result := adapter.TickWirt(context.Background(), state); result.Done || in.moves != 0 {
		t.Fatalf("arrival=%+v mouse_moves=%d", result, in.moves)
	}
	state.At = now.Add(1100 * time.Millisecond)
	if result := adapter.TickWirt(context.Background(), state); result.Done || in.moves != 1 || in.clicks != 0 {
		t.Fatalf("post-arrival probe=%+v input moves/clicks=%d/%d", result, in.moves, in.clicks)
	}
}

func TestCowTomePurchaseBuysExactlyOneAndPreservesOperationalTome(t *testing.T) {
	in := &preparationInputMock{}
	pathCfg := pathing.DefaultConfig()
	adapter := &cowSetupAdapter{
		log: config.NewLogger("error"), controller: in, pathCfg: pathCfg,
		approach: &townPreparationAdapter{log: config.NewLogger("error"), driver: in, controller: in, pathCfg: pathCfg, done: true},
	}
	state := world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Monsters: []world.Monster{{NPCID: world.Akara, UnitID: 3, Position: world.Position{X: 105, Y: 105}}},
		Items:    []world.Item{{UnitID: 10, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Width: 1, Height: 2}},
	}
	ctx := context.Background()
	adapter.TickTome(ctx, state) // approach and baseline
	adapter.TickTome(ctx, state) // NPC hover probe
	state.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeMonster, UnitID: 3}
	adapter.TickTome(ctx, state) // NPC click
	state.Hover = world.HoverInfo{}
	state.UI.NPCInteractOpen = true
	adapter.TickTome(ctx, state) // NPC dialog confirmed
	adapter.TickTome(ctx, state) // home
	adapter.TickTome(ctx, state) // down
	adapter.TickTome(ctx, state) // enter
	state.UI.NPCInteractOpen = false
	state.UI.NPCShopOpen = true
	state.Items = append(state.Items, world.Item{UnitID: 90, Code: "tbk", Location: world.ItemLocationVendor, GridX: 2, GridY: 1})
	adapter.TickTome(ctx, state) // shop confirmed
	adapter.TickTome(ctx, state) // vendor move
	adapter.TickTome(ctx, state) // exactly one right click
	state.Items = append(state.Items, world.Item{UnitID: 11, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Width: 1, Height: 2})
	for range cowTomeVerifySnapshots {
		adapter.TickTome(ctx, state)
	}
	adapter.TickTome(ctx, state) // Esc
	state.UI.NPCShopOpen = false
	result := adapter.TickTome(ctx, state)
	if !result.Done || result.Reason != "" || result.UnitID != 11 {
		t.Fatalf("tome result=%+v", result)
	}
	if in.clicks != 2 || in.modified != 0 {
		t.Fatalf("clicks=%d modified=%d, want NPC LMB + one vendor RMB", in.clicks, in.modified)
	}
	if len(adapter.tomeExisting) != 1 || !adapter.tomeExisting[10] {
		t.Fatalf("operative tome baseline=%v", adapter.tomeExisting)
	}
}

func TestCowTomeVerificationRejectsMultipleNewUnitIDs(t *testing.T) {
	adapter := &cowSetupAdapter{controller: &preparationInputMock{}, tomeStage: "verify", tomeExisting: map[uint32]bool{10: true}}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Items: []world.Item{
			{UnitID: 10, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0},
			{UnitID: 11, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0},
			{UnitID: 12, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0},
		},
	}
	result := adapter.TickTome(context.Background(), state)
	if !result.Done || result.Reason != "cow_tome_purchase_ambiguous" {
		t.Fatalf("ambiguous result=%+v", result)
	}
}

var _ town.ShopInput = (*preparationInputMock)(nil)
