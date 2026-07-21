package loot

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPhase13BaselineCountessAndMephistoPolicyMatrix(t *testing.T) {
	countess := mustLoadPhase10Policy(t, "countess.nip")
	mephisto := mustLoadPhase10Policy(t, "mephisto.nip")
	mephistoSell := mustLoadPhase10Policy(t, "mephisto-sell.nip")
	tests := []struct {
		name                     string
		item                     world.Item
		countess, mephisto, sell bool
	}{
		{name: "rune", item: world.Item{Code: "r33", Type: "rune"}, countess: true},
		{name: "key of terror", item: world.Item{Code: "pk1", Type: "ques"}, countess: true},
		{name: "rejuvenation potion", item: world.Item{Code: "rvl", Type: "rpot"}, countess: true},
		{name: "flawless gem", item: world.Item{Code: "gzv", Type: "gema", Identified: true}, countess: true, mephisto: true},
		{name: "perfect skull", item: world.Item{Code: "skz", Type: "gemz", Identified: true}, countess: true, mephisto: true},
		{name: "chipped gem", item: world.Item{Code: "gcv", Type: "gema", Identified: true}},
		{name: "unidentified exceptional set", item: world.Item{Code: "xap", Quality: world.ItemQualitySet, BaseTier: world.BaseTierExceptional}, mephisto: true, sell: true},
		{name: "identified elite unique", item: world.Item{Code: "7wc", Quality: world.ItemQualityUnique, BaseTier: world.BaseTierElite, Identified: true}, mephisto: true, sell: true},
		{name: "normal unique", item: world.Item{Code: "cap", Quality: world.ItemQualityUnique, BaseTier: world.BaseTierNormal, Identified: true}},
		{name: "elite rare", item: world.Item{Code: "7wc", Quality: world.ItemQualityRare, BaseTier: world.BaseTierElite, Identified: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := countess.Evaluate(test.item).Matched; got != test.countess {
				t.Errorf("Countess pickup = %t, want %t", got, test.countess)
			}
			if got := mephisto.Evaluate(test.item).Matched; got != test.mephisto {
				t.Errorf("Mephisto pickup = %t, want %t", got, test.mephisto)
			}
			if got := mephistoSell.Evaluate(test.item).Matched; got != test.sell {
				t.Errorf("Mephisto sell = %t, want %t", got, test.sell)
			}
		})
	}
}

func TestPhase13BaselineFirstMatchRemainsAuthoritative(t *testing.T) {
	pickit, err := parsePickit("first-match.nip", "[type] == rune\n[name] == r01\n")
	if err != nil {
		t.Fatal(err)
	}
	result := pickit.Evaluate(world.Item{Code: "r01", Type: "rune"})
	if !result.Matched || result.RuleIndex != 0 || result.Line != 1 || result.Rule != "[type] == rune" {
		t.Fatalf("first match = %+v", result)
	}
}
