package loot

import "log/slog"

// Filter applies pickit rules and inventory/stash logic.
type Filter struct {
	log           *slog.Logger
	inventoryLock InventoryLock
	pickit        *Pickit
}

// NewFilter creates a read-only loot filter with an inventory protection grid.
func NewFilter(log *slog.Logger, inventoryLock InventoryLock, pickit *Pickit) *Filter {
	return &Filter{log: log.With("component", "loot"), inventoryLock: inventoryLock, pickit: pickit}
}

func (f *Filter) Ready() bool {
	return f.pickit != nil
}

// InventoryLock returns the configured inventory protection grid.
func (f *Filter) InventoryLock() InventoryLock {
	return f.inventoryLock
}

// Pickit returns the loaded Pickit evaluator.
func (f *Filter) Pickit() *Pickit {
	return f.pickit
}

// SetPickit aktiviert eine vollständig kompilierte Policy ausschließlich an einer sicheren Run-Grenze.
func (f *Filter) SetPickit(pickit *Pickit) {
	f.pickit = pickit
}
