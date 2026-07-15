package world

// UIState is the semantic read-only menu state used to gate fixed-coordinate actions.
type UIState struct {
	InventoryOpen   bool
	NPCInteractOpen bool
	NPCShopOpen     bool
	WaypointOpen    bool
	StashOpen       bool
	QuitMenuOpen    bool
}
