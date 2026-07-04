package loot

import "log/slog"

// Filter applies pickit rules and inventory/stash logic.
type Filter struct {
	log           *slog.Logger
	inventoryLock InventoryLock
}

// NewFilter creates a read-only loot filter with an inventory protection grid.
func NewFilter(log *slog.Logger, inventoryLock InventoryLock) *Filter {
	return &Filter{log: log.With("component", "loot"), inventoryLock: inventoryLock}
}

func (f *Filter) Ready() bool {
	f.log.Debug("loot filter placeholder ready")
	return true
}

// InventoryLock returns the configured inventory protection grid.
func (f *Filter) InventoryLock() InventoryLock {
	return f.inventoryLock
}
