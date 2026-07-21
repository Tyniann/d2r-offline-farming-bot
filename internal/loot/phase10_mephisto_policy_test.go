package loot

import (
	"fmt"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestMephistoPoliciesCoverQualityTierAndIdentifiedMatrix(t *testing.T) {
	pickup := mustLoadPhase10Policy(t, "mephisto.nip")
	sell := mustLoadPhase10Policy(t, "mephisto-sell.nip")
	qualities := []world.ItemQuality{
		world.ItemQualityUnknown, world.ItemQualityLowQuality, world.ItemQualityNormal,
		world.ItemQualitySuperior, world.ItemQualityMagic, world.ItemQualitySet,
		world.ItemQualityRare, world.ItemQualityUnique, world.ItemQualityCrafted,
	}
	tiers := []world.BaseTier{world.BaseTierUnknown, world.BaseTierNormal, world.BaseTierExceptional, world.BaseTierElite}
	for _, quality := range qualities {
		for _, tier := range tiers {
			for _, identified := range []bool{false, true} {
				item := world.Item{Code: "matrix", Quality: quality, BaseTier: tier, Identified: identified}
				want := (quality == world.ItemQualitySet || quality == world.ItemQualityUnique) && (tier == world.BaseTierExceptional || tier == world.BaseTierElite)
				if got := pickup.Evaluate(item).Matched; got != want {
					t.Errorf("pickup quality=%s tier=%s identified=%t = %t, want %t", quality.String(), tier, identified, got, want)
				}
				if got := sell.Evaluate(item).Matched; got != want {
					t.Errorf("sell quality=%s tier=%s identified=%t = %t, want %t", quality.String(), tier, identified, got, want)
				}
			}
		}
	}
}

func TestMephistoPoliciesProtectFlawlessAndPerfectGems(t *testing.T) {
	pickup := mustLoadPhase10Policy(t, "mephisto.nip")
	sell := mustLoadPhase10Policy(t, "mephisto-sell.nip")
	for _, code := range []string{"gzv", "gpv", "gly", "gpy", "glb", "gpb", "glg", "gpg", "glr", "gpr", "glw", "gpw", "skl", "skz"} {
		item := world.Item{Code: code, Quality: world.ItemQualityNormal, BaseTier: world.BaseTierUnknown, Identified: true}
		if !pickup.Evaluate(item).Matched {
			t.Errorf("gem %s is not kept", code)
		}
		if sell.Evaluate(item).Matched {
			t.Errorf("gem %s is a sell candidate", code)
		}
	}
}

func TestMephistoSellCandidateIsExcludedFromStashButGemRemains(t *testing.T) {
	pickup, err := CompilePickitRules("phase13 effective", []PickitRuleSpec{
		{ProfileID: "gems", RuleID: "perfect", Action: ActionKeep, Expression: `[name] == "gpv"`},
		{ProfileID: "mephisto", RuleID: "sell", Action: ActionSell, Expression: `([quality] == "unique") && ([tier] == "exceptional")`},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := NewInventoryLock(unlockedInventory())
	if err != nil {
		t.Fatal(err)
	}
	executor := &StashExecutor{filter: NewFilter(testLogger(), lock, pickup)}
	gem := world.Item{UnitID: 1, Code: "gpv", Quality: world.ItemQualityNormal, BaseTier: world.BaseTierUnknown, Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Width: 1, Height: 1}
	candidate := world.Item{UnitID: 2, Code: "xap", Quality: world.ItemQualityUnique, BaseTier: world.BaseTierExceptional, Identified: true, Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 2, Width: 2, Height: 2}
	candidates, safe := executor.candidates(stashState(candidate, gem))
	if !safe || len(candidates) != 1 || candidates[0].UnitID != gem.UnitID {
		t.Fatalf("stash candidates = %+v safe=%t, want protected gem only", candidates, safe)
	}
}

func mustLoadPhase10Policy(t *testing.T, name string) *Pickit {
	t.Helper()
	var expressions []string
	switch name {
	case "countess.nip":
		expressions = []string{
			`[type] == rune`, `[name] == pk1`, `[type] == rpot`,
			`[name] == gzv || [name] == gpv`, `[name] == gly || [name] == gpy`,
			`[name] == glb || [name] == gpb`, `[name] == glg || [name] == gpg`,
			`[name] == glr || [name] == gpr`, `[name] == glw || [name] == gpw`,
			`[name] == skl || [name] == skz`,
		}
	case "mephisto.nip":
		expressions = []string{
			`[name] == gzv || [name] == gpv`, `[name] == gly || [name] == gpy`,
			`[name] == glb || [name] == gpb`, `[name] == glg || [name] == gpg`,
			`[name] == glr || [name] == gpr`, `[name] == glw || [name] == gpw`,
			`[name] == skl || [name] == skz`,
			`([quality] == set || [quality] == unique) && ([tier] == exceptional || [tier] == elite)`,
		}
	case "mephisto-sell.nip":
		expressions = []string{`([quality] == set || [quality] == unique) && ([tier] == exceptional || [tier] == elite)`}
	default:
		t.Fatalf("unknown characterized policy %q", name)
	}
	specs := make([]PickitRuleSpec, 0, len(expressions))
	for index, expression := range expressions {
		specs = append(specs, PickitRuleSpec{ProfileID: "legacy-characterization", RuleID: fmt.Sprintf("rule-%d", index+1), Action: ActionKeep, Expression: expression})
	}
	policy, err := CompilePickitRules(name, specs)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}
