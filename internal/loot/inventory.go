package loot

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	inventoryRows = 4
	inventoryCols = 10

	// CapacityReasonUnknownSize marks a personal inventory item without a known footprint.
	CapacityReasonUnknownSize = "unknown_size"
	// CapacityReasonOutOfBounds marks a personal inventory item outside the 4x10 grid.
	CapacityReasonOutOfBounds = "out_of_bounds"
	// CapacityReasonOverlap marks two personal inventory items occupying the same slot.
	CapacityReasonOverlap = "overlap"
)

// InventoryCapacity describes the conservative pickup capacity of personal inventory.
type InventoryCapacity struct {
	FreeSlots   int
	LockedSlots int
	Unsafe      bool
	Reason      string
}

// InventoryLock marks protected personal inventory slots.
type InventoryLock struct {
	locked [inventoryRows][inventoryCols]bool
}

// NewInventoryLock builds an inventory lock from a validated 4x10 config grid.
func NewInventoryLock(cells [][]int) (InventoryLock, error) {
	if len(cells) != inventoryRows {
		return InventoryLock{}, fmt.Errorf("inventory lock must have %d rows", inventoryRows)
	}
	var lock InventoryLock
	for row, values := range cells {
		if len(values) != inventoryCols {
			return InventoryLock{}, fmt.Errorf("inventory lock row %d must have %d columns", row, inventoryCols)
		}
		for col, value := range values {
			switch value {
			case 0:
			case 1:
				lock.locked[row][col] = true
			default:
				return InventoryLock{}, fmt.Errorf("inventory lock row %d column %d must be 0 or 1", row, col)
			}
		}
	}
	return lock, nil
}

// Locked reports whether a zero-based row/column is protected.
func (l InventoryLock) Locked(row, col int) bool {
	if row < 0 || row >= inventoryRows || col < 0 || col >= inventoryCols {
		return true
	}
	return l.locked[row][col]
}

// LockedSlots returns the number of protected slots in the 4x10 grid.
func (l InventoryLock) LockedSlots() int {
	count := 0
	for row := 0; row < inventoryRows; row++ {
		for col := 0; col < inventoryCols; col++ {
			if l.locked[row][col] {
				count++
			}
		}
	}
	return count
}

// Grid returns a defensive 4×10 int copy of the lock for adapters that still need cell values.
func (l InventoryLock) Grid() [][]int {
	grid := make([][]int, inventoryRows)
	for row := 0; row < inventoryRows; row++ {
		grid[row] = make([]int, inventoryCols)
		for col := 0; col < inventoryCols; col++ {
			if l.locked[row][col] {
				grid[row][col] = 1
			}
		}
	}
	return grid
}

// InventoryGrid projects personal inventory items onto the lock grid.
type InventoryGrid struct {
	lock     InventoryLock
	occupied [inventoryRows][inventoryCols]bool
	capacity InventoryCapacity
}

// NewInventoryGrid computes conservative capacity for personal inventory items.
func NewInventoryGrid(lock InventoryLock, items []world.Item) InventoryGrid {
	grid := InventoryGrid{
		lock: lock,
		capacity: InventoryCapacity{
			LockedSlots: lock.LockedSlots(),
		},
	}

	for _, item := range items {
		if item.Width <= 0 || item.Height <= 0 {
			grid.markUnsafe(CapacityReasonUnknownSize)
			return grid
		}
		if item.GridX < 0 || item.GridY < 0 ||
			item.GridX+item.Width > inventoryCols ||
			item.GridY+item.Height > inventoryRows {
			grid.markUnsafe(CapacityReasonOutOfBounds)
			return grid
		}
		for row := item.GridY; row < item.GridY+item.Height; row++ {
			for col := item.GridX; col < item.GridX+item.Width; col++ {
				if grid.occupied[row][col] {
					grid.markUnsafe(CapacityReasonOverlap)
					return grid
				}
				grid.occupied[row][col] = true
			}
		}
	}

	free := 0
	for row := 0; row < inventoryRows; row++ {
		for col := 0; col < inventoryCols; col++ {
			if !lock.Locked(row, col) && !grid.occupied[row][col] {
				free++
			}
		}
	}
	grid.capacity.FreeSlots = free
	return grid
}

// Capacity returns the computed conservative pickup capacity.
func (g InventoryGrid) Capacity() InventoryCapacity {
	return g.capacity
}

// CanFit reports whether a width x height item can fit in unlocked, unoccupied personal inventory.
func (g InventoryGrid) CanFit(width, height int) bool {
	if g.capacity.Unsafe || width <= 0 || height <= 0 || width > inventoryCols || height > inventoryRows {
		return false
	}
	for row := 0; row <= inventoryRows-height; row++ {
		for col := 0; col <= inventoryCols-width; col++ {
			if g.canFitAt(col, row, width, height) {
				return true
			}
		}
	}
	return false
}

func (g InventoryGrid) canFitAt(startCol, startRow, width, height int) bool {
	for row := startRow; row < startRow+height; row++ {
		for col := startCol; col < startCol+width; col++ {
			if g.lock.Locked(row, col) || g.occupied[row][col] {
				return false
			}
		}
	}
	return true
}

func (g *InventoryGrid) markUnsafe(reason string) {
	g.capacity.Unsafe = true
	g.capacity.Reason = reason
	g.capacity.FreeSlots = 0
}
