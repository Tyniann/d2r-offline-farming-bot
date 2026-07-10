package memory

// UIState is the read-only subset of D2R menu flags needed for safe recovery actions.
type UIState struct {
	InventoryOpen bool
	StashOpen     bool
}

const (
	// uiBuffer starts at UI-0x13. These indices were calibrated read-only
	// against D2R 3.2.92777 for closed, inventory-only, and personal-stash UI.
	uiInventoryIndex = 0x00
	uiGateIndex      = 0x09 // UI-0x0A
	uiStashIndex     = 0x17 // UI+0x04
	uiLoadingIndex   = 0x171
	uiBufferBefore   = 0x13
	uiBufferSize     = 0x172
)
