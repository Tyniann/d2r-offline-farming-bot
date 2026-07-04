package loot

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func testLock(t *testing.T, cells [][]int) InventoryLock {
	t.Helper()
	lock, err := NewInventoryLock(cells)
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func TestInventoryLockCountsLockedSlots(t *testing.T) {
	lock := testLock(t, [][]int{
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
	})

	if !lock.Locked(0, 0) || lock.Locked(0, 2) {
		t.Fatalf("Locked() did not respect grid")
	}
	if !lock.Locked(-1, 0) || !lock.Locked(0, 10) {
		t.Fatalf("out-of-bounds slots should be treated as locked")
	}
	if got := lock.LockedSlots(); got != 8 {
		t.Fatalf("LockedSlots = %d, want 8", got)
	}
}

func TestInventoryCapacityRespectsLockedAndOccupiedSlots(t *testing.T) {
	lock := testLock(t, [][]int{
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 0, 0, 0, 0, 0, 0, 0, 0},
	})
	grid := NewInventoryGrid(lock, []world.Item{
		{GridX: 2, GridY: 0, Width: 1, Height: 1},
		{GridX: 3, GridY: 0, Width: 2, Height: 2},
	})

	capacity := grid.Capacity()
	if capacity.Unsafe {
		t.Fatalf("Capacity unsafe = %+v", capacity)
	}
	if capacity.LockedSlots != 8 || capacity.FreeSlots != 27 {
		t.Fatalf("Capacity = %+v, want locked=8 free=27", capacity)
	}
	if !grid.CanFit(2, 2) {
		t.Fatal("CanFit(2,2) = false, want true")
	}
	if grid.CanFit(0, 1) || grid.CanFit(11, 1) {
		t.Fatal("invalid dimensions should not fit")
	}
}

func TestInventoryCapacityUnknownSizeUnsafe(t *testing.T) {
	lock := testLock(t, allFreeLock())
	grid := NewInventoryGrid(lock, []world.Item{{GridX: 0, GridY: 0, Width: 0, Height: 1}})

	capacity := grid.Capacity()
	if !capacity.Unsafe || capacity.Reason != CapacityReasonUnknownSize || capacity.FreeSlots != 0 {
		t.Fatalf("Capacity = %+v, want unknown_size unsafe with zero free slots", capacity)
	}
	if grid.CanFit(1, 1) {
		t.Fatal("CanFit should fail when capacity is unsafe")
	}
}

func TestInventoryCapacityOutOfBoundsUnsafe(t *testing.T) {
	lock := testLock(t, allFreeLock())
	grid := NewInventoryGrid(lock, []world.Item{{GridX: 9, GridY: 3, Width: 2, Height: 1}})

	capacity := grid.Capacity()
	if !capacity.Unsafe || capacity.Reason != CapacityReasonOutOfBounds {
		t.Fatalf("Capacity = %+v, want out_of_bounds", capacity)
	}
}

func TestInventoryCapacityOverlapUnsafe(t *testing.T) {
	lock := testLock(t, allFreeLock())
	grid := NewInventoryGrid(lock, []world.Item{
		{GridX: 0, GridY: 0, Width: 2, Height: 2},
		{GridX: 1, GridY: 1, Width: 1, Height: 1},
	})

	capacity := grid.Capacity()
	if !capacity.Unsafe || capacity.Reason != CapacityReasonOverlap {
		t.Fatalf("Capacity = %+v, want overlap", capacity)
	}
}

func allFreeLock() [][]int {
	return [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
}
