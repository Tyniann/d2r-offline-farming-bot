package town

import "testing"

func TestAkaraVendorCostsMatchExtractedD2RTables(t *testing.T) {
	for code, want := range map[string]int{"hp1": 30, "hp5": 500, "mp1": 60, "mp5": 1000, "tsc": 100, "isc": 80} {
		got, ok := AkaraVendorCost(code)
		if !ok || got != want {
			t.Fatalf("AkaraVendorCost(%q) = %d/%v, want %d/true", code, got, ok, want)
		}
	}
	if _, ok := AkaraVendorCost("r01"); ok {
		t.Fatal("unsupported item received an Akara cost")
	}
}

func TestMaximumAkaraUnitCostCoversRestockResources(t *testing.T) {
	for resource, want := range map[RestockResource]int{RestockHealing: 500, RestockMana: 1000, RestockTownPortalScroll: 100, RestockIdentifyScroll: 80} {
		got, ok := MaximumAkaraUnitCost(resource)
		if !ok || got != want {
			t.Fatalf("MaximumAkaraUnitCost(%q) = %d/%v, want %d/true", resource, got, ok, want)
		}
	}
	if _, ok := MaximumAkaraUnitCost("rejuvenation"); ok {
		t.Fatal("rejuvenation received a vendor cost")
	}
}
