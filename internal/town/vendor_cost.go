package town

// AkaraVendorCost returns the conservative undiscounted purchase cost for one
// vendor item. Values are bound to D2R `3.2.92777` `misc.txt`; Akara's
// `npc.txt` sell multiplier is `1024`, so no additional scaling is required.
func AkaraVendorCost(code string) (int, bool) {
	cost, ok := akaraVendorCosts[code]
	return cost, ok
}

// MaximumAkaraUnitCost returns the highest supported undiscounted unit cost
// used to prove gold sufficiency before navigation or shop input begins.
func MaximumAkaraUnitCost(resource RestockResource) (int, bool) {
	switch resource {
	case RestockHealing:
		return 500, true
	case RestockMana:
		return 1000, true
	case RestockTownPortalScroll:
		return 100, true
	case RestockIdentifyScroll:
		return 80, true
	default:
		return 0, false
	}
}

var akaraVendorCosts = map[string]int{
	"hp1": 30,
	"hp2": 75,
	"hp3": 125,
	"hp4": 250,
	"hp5": 500,
	"mp1": 60,
	"mp2": 150,
	"mp3": 300,
	"mp4": 500,
	"mp5": 1000,
	"tsc": 100,
	"isc": 80,
}
