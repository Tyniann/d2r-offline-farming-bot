package loot

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type stashInputMock struct {
	window input.WindowInfo
	moves  [][2]int
	clicks int
	keys   []string
}

func (m *stashInputMock) Window() (input.WindowInfo, bool) { return m.window, true }
func (m *stashInputMock) MoveTo(x, y int) error            { m.moves = append(m.moves, [2]int{x, y}); return nil }
func (m *stashInputMock) ClickWithModifier(mod string, button input.MouseButton) error {
	if mod != "ctrl" || button != input.MouseLeft {
		panic("unexpected modified click")
	}
	m.clicks++
	return nil
}
func (m *stashInputMock) PressKey(key string) error { m.keys = append(m.keys, key); return nil }

func stashTestExecutor(t *testing.T, cells [][]int) (*StashExecutor, *stashInputMock) {
	t.Helper()
	lock, err := NewInventoryLock(cells)
	if err != nil {
		t.Fatal(err)
	}
	pickit, err := parsePickit("test.nip", "[type] == rune")
	if err != nil {
		t.Fatal(err)
	}
	in := &stashInputMock{window: input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}}
	executor, err := NewStashExecutor(testLogger(), NewFilter(testLogger(), lock, pickit), in, StashConfig{
		MaxRetries: 2, VerifyTimeout: time.Second, CloseTimeout: time.Second,
		InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33,
	})
	if err != nil {
		t.Fatal(err)
	}
	return executor, in
}

func stashState(items ...world.Item) world.State {
	return world.State{Valid: true, Phase: world.GamePhaseInGame, UI: world.UIState{InventoryOpen: true, StashOpen: true}, Items: items}
}

func stashRune(unit uint32, col, row int) world.Item {
	return world.Item{UnitID: unit, Code: "r03", Name: "Tir Rune", Type: "rune", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: col, GridY: row, Width: 1, Height: 1}
}

func unlockedInventory() [][]int {
	return [][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
}

func TestStashExecutorCtrlClicksAndVerifiesDisappearance(t *testing.T) {
	e, in := stashTestExecutor(t, unlockedInventory())
	now := time.Now()
	res := e.Tick(stashState(stashRune(7, 6, 2)), now)
	if res.Status != StashPending || !res.Attempted || len(in.moves) != 1 || in.moves[0] != [2]int{1061, 451} || in.clicks != 1 {
		t.Fatalf("attempt=%+v moves=%v clicks=%d", res, in.moves, in.clicks)
	}
	res = e.Tick(stashState(), now.Add(time.Millisecond))
	if !res.Transferred || res.Done || res.UnitID != 7 {
		t.Fatalf("verified result=%+v, want transferred pending", res)
	}
	res = e.Tick(stashState(), now.Add(2*time.Millisecond))
	if res.Status != StashSuccess || !res.Done {
		t.Fatalf("final result=%+v, want success", res)
	}
}

func TestStashExecutorNeverTransfersLockedItem(t *testing.T) {
	cells := unlockedInventory()
	cells[2][6] = 1
	e, in := stashTestExecutor(t, cells)
	res := e.Tick(stashState(stashRune(7, 6, 2)), time.Now())
	if res.Status != StashSuccess || !res.Done || len(in.moves) != 0 || in.clicks != 0 {
		t.Fatalf("result=%+v moves=%v clicks=%d, want untouched locked item", res, in.moves, in.clicks)
	}
}

func TestStashExecutorLeavesUnidentifiedQualityItemForIdentifyRoutine(t *testing.T) {
	e, in := stashTestExecutor(t, unlockedInventory())
	item := stashRune(7, 6, 2)
	item.Quality = world.ItemQualityUnique
	item.Identified = false
	res := e.Tick(stashState(item), time.Now())
	if res.Status != StashSuccess || !res.Done || in.clicks != 0 {
		t.Fatalf("result=%+v clicks=%d, want unidentified item untouched", res, in.clicks)
	}
}

func TestStashExecutorFailsAfterVerifiedRetries(t *testing.T) {
	e, in := stashTestExecutor(t, unlockedInventory())
	now := time.Now()
	st := stashState(stashRune(7, 6, 2))
	_ = e.Tick(st, now)
	_ = e.Tick(st, now.Add(time.Second))
	res := e.Tick(st, now.Add(2*time.Second))
	if res.Status != StashFailed || !res.Done || res.Attempt != 2 || in.clicks != 2 {
		t.Fatalf("result=%+v clicks=%d, want stash_failed after two attempts", res, in.clicks)
	}
}

func TestStashExecutorClosesWithEscAndMemoryConfirmation(t *testing.T) {
	e, in := stashTestExecutor(t, unlockedInventory())
	now := time.Now()
	res := e.TickClose(stashState(), now)
	if res.Status != StashPending || len(in.keys) != 1 || in.keys[0] != "esc" {
		t.Fatalf("close request=%+v keys=%v", res, in.keys)
	}
	closed := stashState()
	closed.UI = world.UIState{}
	res = e.TickClose(closed, now.Add(time.Millisecond))
	if res.Status != StashClosed || !res.Done {
		t.Fatalf("close confirmation=%+v", res)
	}
}
