package loot

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPickitTierPredicateIsReadOnlyAndExplicit(t *testing.T) {
	pickit, err := parsePickit("tier.nip", "[tier] == exceptional || [tier] == elite")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		tier world.BaseTier
		want bool
	}{{world.BaseTierNormal, false}, {world.BaseTierExceptional, true}, {world.BaseTierElite, true}, {world.BaseTierUnknown, false}, {"", false}} {
		if got := pickit.Evaluate(world.Item{BaseTier: tt.tier}).Matched; got != tt.want {
			t.Fatalf("tier %q matched=%t, want %t", tt.tier, got, tt.want)
		}
	}
}

func TestPickitRejectsUnknownTierLiteral(t *testing.T) {
	if _, err := parsePickit("tier.nip", "[tier] == ancient"); err == nil {
		t.Fatal("unknown tier literal accepted")
	}
}
