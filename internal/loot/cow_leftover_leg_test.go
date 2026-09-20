package loot

import (
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type leftoverInputMock struct {
	window   input.WindowInfo
	keys     []string
	moves    [][2]int
	modified []string
}

func (m *leftoverInputMock) Window() (input.WindowInfo, bool) { return m.window, true }
func (m *leftoverInputMock) MoveTo(x, y int) error {
	m.moves = append(m.moves, [2]int{x, y})
	return nil
}
func (m *leftoverInputMock) ClickWithModifier(modifier string, button input.MouseButton) error {
	m.modified = append(m.modified, modifier+":"+string(button))
	return nil
}
func (m *leftoverInputMock) PressKey(key string) error { m.keys = append(m.keys, key); return nil }

func TestCowLeftoverLegDropAimsThenCtrlClicksAndClosesInventory(t *testing.T) {
	in := &leftoverInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	drop, err := NewCowLeftoverLegDrop(slog.Default(), in, CowLeftoverLegConfig{
		InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	state := leftoverTownState(1, now, world.Item{
		UnitID: 9, Code: "leg", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 5, GridY: 0, Width: 1, Height: 3,
	})

	if result := drop.Tick(state, now); result.Done {
		t.Fatalf("open tick completed: %+v", result)
	}
	if len(in.keys) != 1 || in.keys[0] != "i" {
		t.Fatalf("inventory open keys=%v", in.keys)
	}

	state.Generation = 2
	state.UI.InventoryOpen = true
	state.At = now.Add(100 * time.Millisecond)
	if result := drop.Tick(state, state.At); result.Done {
		t.Fatalf("inventory-open tick completed: %+v", result)
	}

	state.Generation = 3
	state.At = now.Add(200 * time.Millisecond)
	if result := drop.Tick(state, state.At); result.Done || len(in.moves) != 1 || len(in.modified) != 0 {
		t.Fatalf("aim tick=%+v moves=%d modified=%v", result, len(in.moves), in.modified)
	}
	if in.moves[0] != [2]int{847 + 5*33 + 16, 369 + 16} {
		t.Fatalf("aim cell=%v", in.moves[0])
	}

	state.Generation = 4
	state.At = now.Add(300 * time.Millisecond)
	if result := drop.Tick(state, state.At); result.Done || len(in.modified) != 1 || in.modified[0] != "ctrl:left" {
		t.Fatalf("drop tick=%+v modified=%v", result, in.modified)
	}

	state.Generation = 5
	state.Items[0].Location = world.ItemLocationGround
	state.At = now.Add(400 * time.Millisecond)
	if result := drop.Tick(state, state.At); result.Done {
		t.Fatalf("dropped tick completed before close: %+v", result)
	}

	state.Generation = 6
	state.At = now.Add(500 * time.Millisecond)
	if result := drop.Tick(state, state.At); result.Done || len(in.keys) != 2 {
		t.Fatalf("close tick=%+v keys=%v", result, in.keys)
	}

	state.Generation = 7
	state.UI.InventoryOpen = false
	state.At = now.Add(600 * time.Millisecond)
	if result := drop.Tick(state, state.At); !result.Done || result.Reason != "" {
		t.Fatalf("final tick=%+v", result)
	}
}

func TestCowLeftoverLegDropRejectsStashLeg(t *testing.T) {
	in := &leftoverInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	drop, err := NewCowLeftoverLegDrop(slog.Default(), in, CowLeftoverLegConfig{
		InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	state := leftoverTownState(1, now, world.Item{
		UnitID: 9, Code: "leg", Location: world.ItemLocationStash, PlayerOwned: true, Page: 0, GridX: 0, GridY: 0, Width: 1, Height: 3,
	})
	if result := drop.Tick(state, now); !result.Done || result.Reason != "cow_existing_leg" {
		t.Fatalf("stash leftover=%+v", result)
	}
	if len(in.keys) != 0 || len(in.modified) != 0 {
		t.Fatalf("stash leftover sent input keys=%v modified=%v", in.keys, in.modified)
	}
}

func leftoverTownState(generation uint64, at time.Time, item world.Item) world.State {
	return world.State{
		At: at, Generation: generation, Valid: true, Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.RogueEncampment),
		Items: []world.Item{item},
	}
}
