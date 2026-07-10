package world

// UIState is the semantic read-only menu state used to gate fixed-coordinate actions.
type UIState struct {
	InventoryOpen bool
	StashOpen     bool
}
