package town

// Thresholds contains trigger counts, never fill targets. Equality does not create demand.
type Thresholds struct {
	Healing           int `yaml:"healing"`
	Mana              int `yaml:"mana"`
	TownPortalScrolls int `yaml:"town_portal_scrolls"`
	IdentifyScrolls   int `yaml:"identify_scrolls"`
}

// SupplySnapshot is one coherent, read-only input to demand inspection.
type SupplySnapshot struct {
	Healing            int
	Mana               int
	TownPortalScrolls  int
	IdentifyScrolls    int
	StashRequired      bool
	IdentifyRequired   bool
	VendorCandidates   bool
	RepairRequired     bool
	BeltLayoutComplete bool
}

// DemandSnapshot preserves observed quantities and Boolean plan inputs together.
type DemandSnapshot struct {
	Supply     SupplySnapshot
	Thresholds Thresholds
	Demand     Demand
}

// InspectDemand derives immutable Boolean service needs from a coherent source snapshot.
func InspectDemand(supply SupplySnapshot, thresholds Thresholds) DemandSnapshot {
	return DemandSnapshot{Supply: supply, Thresholds: thresholds, Demand: Demand{
		Stash:    supply.StashRequired,
		Potions:  supply.Healing < thresholds.Healing || supply.Mana < thresholds.Mana,
		Scrolls:  supply.TownPortalScrolls < thresholds.TownPortalScrolls || supply.IdentifyScrolls < thresholds.IdentifyScrolls,
		Identify: supply.IdentifyRequired,
		Sell:     supply.VendorCandidates,
		Repair:   supply.RepairRequired,
	}}
}
