package loot

import (
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type cowRecipeInputMock struct {
	window       input.WindowInfo
	keys         []string
	moves        [][2]int
	clicks       []input.MouseButton
	modified     []string
	focuses      int
	portalTicks  int
	portalResult CowPortalClickResult
}

func (m *cowRecipeInputMock) Window() (input.WindowInfo, bool) { return m.window, true }
func (m *cowRecipeInputMock) Focus() error                     { m.focuses++; return nil }
func (m *cowRecipeInputMock) MoveTo(x, y int) error {
	m.moves = append(m.moves, [2]int{x, y})
	return nil
}
func (m *cowRecipeInputMock) Click(button input.MouseButton) error {
	m.clicks = append(m.clicks, button)
	return nil
}
func (m *cowRecipeInputMock) ClickWithModifier(modifier string, button input.MouseButton) error {
	m.modified = append(m.modified, modifier+":"+string(button))
	return nil
}
func (m *cowRecipeInputMock) PressKey(key string) error { m.keys = append(m.keys, key); return nil }
func (m *cowRecipeInputMock) TickPermanentPortal(world.State, world.Object) (CowPortalClickResult, error) {
	m.portalTicks++
	return m.portalResult, nil
}
func (m *cowRecipeInputMock) ResetPermanentPortal() { m.portalTicks = 0 }

func TestCowPortalRecipeHappyPathUsesOneTransmuteAndBoundPortal(t *testing.T) {
	executor, in := newCowRecipeTestExecutor(t)
	binding := CowPortalBinding{LegUnitID: 11, TomeUnitID: 12, CubeUnitID: 10}
	base := time.Date(2026, 8, 1, 20, 0, 0, 0, time.UTC)
	state := cowRecipeState(1, base, binding)

	tickRecipe(executor, &state, base, binding) // verify personal items
	state.Generation++
	tickRecipe(executor, &state, base.Add(100*time.Millisecond), binding) // send inventory key
	state.Generation++
	state.UI.InventoryOpen = true
	tickRecipe(executor, &state, base.Add(200*time.Millisecond), binding) // inventory confirmed
	state.Generation++
	tickRecipe(executor, &state, base.Add(300*time.Millisecond), binding) // right-click Cube
	state.Generation++
	state.UI.CubeOpenKnown, state.UI.CubeOpen = true, true
	tickRecipe(executor, &state, base.Add(400*time.Millisecond), binding) // Cube confirmed
	state.Generation++
	tickRecipe(executor, &state, base.Add(500*time.Millisecond), binding) // transfer leg
	state.Generation++
	setItemLocation(&state, binding.LegUnitID, world.ItemLocationCube)
	tickRecipe(executor, &state, base.Add(600*time.Millisecond), binding) // leg confirmed
	state.Generation++
	tickRecipe(executor, &state, base.Add(700*time.Millisecond), binding) // transfer tome
	state.Generation++
	setItemLocation(&state, binding.TomeUnitID, world.ItemLocationCube)
	tickRecipe(executor, &state, base.Add(800*time.Millisecond), binding) // tome confirmed
	state.Generation++
	tickRecipe(executor, &state, base.Add(900*time.Millisecond), binding) // exact contents
	state.Generation++
	tickRecipe(executor, &state, base.Add(time.Second), binding) // one Transmute

	state.Items = state.Items[:1]
	state.Objects = []world.Object{{Kind: world.ObjectKindPermanentPortal, ID: world.PermanentPortalID, UnitID: 90, Position: world.Position{X: 100, Y: 100}, Name: "Permanent Portal"}}
	for i := 0; i < 4; i++ {
		state.Generation++
		tickRecipe(executor, &state, base.Add(time.Duration(1100+i*100)*time.Millisecond), binding)
	}
	state.Generation++
	state.UI = world.UIState{CubeOpenKnown: true}
	tickRecipe(executor, &state, base.Add(1600*time.Millisecond), binding)
	state.Generation++
	in.portalResult = CowPortalClickResult{Clicked: true, Done: true}
	tickRecipe(executor, &state, base.Add(1700*time.Millisecond), binding)
	state.Generation++
	state.Area = world.LookupArea(world.MooMooFarm)
	result := executor.Tick(state, base.Add(1800*time.Millisecond), binding)

	if !result.Done || result.Reason != "" || result.PortalUnitID != 90 {
		t.Fatalf("result=%+v", result)
	}
	if len(in.modified) != 2 || in.modified[0] != "ctrl:left" || in.modified[1] != "ctrl:left" {
		t.Fatalf("modified clicks=%v", in.modified)
	}
	leftClicks := 0
	for _, button := range in.clicks {
		if button == input.MouseLeft {
			leftClicks++
		}
	}
	if leftClicks != 1 || in.focuses != 1 {
		t.Fatalf("transmute clicks=%d focuses=%d all clicks=%v", leftClicks, in.focuses, in.clicks)
	}
	if got := in.moves[len(in.moves)-1]; got != [2]int{270, 411} {
		t.Fatalf("last fixed move=%v, want transmute coordinate", got)
	}
	if in.portalTicks != 1 {
		t.Fatalf("portal ticks=%d", in.portalTicks)
	}
}

func TestCowPortalRecipeRejectsUnavailableCubeAndInvalidContents(t *testing.T) {
	binding := CowPortalBinding{LegUnitID: 11, TomeUnitID: 12, CubeUnitID: 10}
	base := time.Now()

	t.Run("Cube state unavailable", func(t *testing.T) {
		executor, _ := newCowRecipeTestExecutor(t)
		executor.binding, executor.stage, executor.stageStartedAt = binding, cowRecipeWaitCube, base
		state := cowRecipeState(2, base, binding)
		state.UI.InventoryOpen = true
		result := executor.Tick(state, base, binding)
		if !result.Done || result.Reason != "cow_cube_ui_unavailable" {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("third Cube item", func(t *testing.T) {
		executor, _ := newCowRecipeTestExecutor(t)
		executor.binding, executor.stage, executor.stageStartedAt, executor.stageGeneration = binding, cowRecipeVerifyContent, base, 1
		state := cowRecipeState(2, base, binding)
		state.UI = world.UIState{InventoryOpen: true, CubeOpen: true, CubeOpenKnown: true}
		setItemLocation(&state, binding.LegUnitID, world.ItemLocationCube)
		setItemLocation(&state, binding.TomeUnitID, world.ItemLocationCube)
		state.Items = append(state.Items, world.Item{UnitID: 13, Code: "r01", Location: world.ItemLocationCube})
		result := executor.Tick(state, base, binding)
		if !result.Done || result.Reason != "cow_recipe_contents_invalid" {
			t.Fatalf("result=%+v", result)
		}
	})
}

func TestCowPortalRecipePostTransmuteFailuresNeverClickAgain(t *testing.T) {
	binding := CowPortalBinding{LegUnitID: 11, TomeUnitID: 12, CubeUnitID: 10}
	base := time.Now()
	tests := []struct {
		name  string
		items []world.Item
		stage cowRecipeStage
		want  string
	}{
		{name: "partial consumption", stage: cowRecipeWaitResult, items: []world.Item{{UnitID: 12, Code: "tbk", Location: world.ItemLocationCube}}, want: "cow_recipe_partial_consumption"},
		{name: "ingredients gone portal missing", stage: cowRecipeWaitPortal, want: "cow_portal_missing_after_consumption"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			executor, in := newCowRecipeTestExecutor(t)
			executor.binding, executor.stage, executor.stageStartedAt = binding, test.stage, base
			executor.transmuteSent, executor.transmuteAt = true, base
			state := cowRecipeState(3, base.Add(20*time.Second), binding)
			state.Items = test.items
			result := executor.Tick(state, base.Add(20*time.Second), binding)
			if !result.Done || result.Reason != test.want {
				t.Fatalf("result=%+v", result)
			}
			_ = executor.Tick(state, base.Add(21*time.Second), binding)
			if len(in.clicks) != 0 || len(in.modified) != 0 {
				t.Fatalf("post-Transmute input repeated: clicks=%v modified=%v", in.clicks, in.modified)
			}
		})
	}
}

func TestCowPortalRecipeResetClearsIrreversibleBindings(t *testing.T) {
	executor, _ := newCowRecipeTestExecutor(t)
	executor.binding = CowPortalBinding{LegUnitID: 1, TomeUnitID: 2, CubeUnitID: 3}
	executor.transmuteSent = true
	executor.portalUnitID = 90
	executor.stage = cowRecipeVerifyArea
	executor.Reset()
	if executor.binding != (CowPortalBinding{}) || executor.transmuteSent || executor.portalUnitID != 0 || executor.stage != cowRecipeVerifyItems {
		t.Fatalf("reset state=%+v", executor)
	}
}

func newCowRecipeTestExecutor(t *testing.T) (*CowPortalRecipe, *cowRecipeInputMock) {
	t.Helper()
	in := &cowRecipeInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	executor, err := NewCowPortalRecipe(slog.Default(), in, CowPortalRecipeConfig{
		CubeOpenTimeout: 3 * time.Second, TransferTimeout: 2 * time.Second, ResultTimeout: 5 * time.Second,
		PortalTimeout: 8 * time.Second, CloseTimeout: 3 * time.Second, EntryTimeout: 15 * time.Second,
		InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33, TransmuteX: 270, TransmuteY: 411,
	})
	if err != nil {
		t.Fatal(err)
	}
	return executor, in
}

func cowRecipeState(generation uint64, at time.Time, binding CowPortalBinding) world.State {
	return world.State{
		At: at, Generation: generation, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Items: []world.Item{
			{UnitID: binding.CubeUnitID, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 0, GridY: 0, Width: 2, Height: 2},
			{UnitID: binding.LegUnitID, Code: "leg", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 4, GridY: 0, Width: 1, Height: 3},
			{UnitID: binding.TomeUnitID, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 5, GridY: 0, Width: 1, Height: 2},
		},
	}
}

func setItemLocation(state *world.State, unitID uint32, location world.ItemLocation) {
	for index := range state.Items {
		if state.Items[index].UnitID == unitID {
			state.Items[index].Location = location
			return
		}
	}
}

func tickRecipe(executor *CowPortalRecipe, state *world.State, now time.Time, binding CowPortalBinding) CowPortalRecipeResult {
	state.At = now
	return executor.Tick(*state, now, binding)
}
