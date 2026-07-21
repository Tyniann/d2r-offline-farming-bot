package loot

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPhase13RuntimeActionAndFirstMatchMatrix(t *testing.T) {
	policy, err := CompilePickitRules("matrix", []PickitRuleSpec{
		{ProfileID: "priority", RuleID: "keep-rune", Action: ActionKeep, Expression: `[type] == "rune"`, ProfileRevision: 4, AssignmentRevision: 7},
		{ProfileID: "later", RuleID: "sell-rune", Action: ActionSell, Expression: `[type] == "rune"`, ProfileRevision: 2, AssignmentRevision: 7},
		{ProfileID: "later", RuleID: "sell-rare", Action: ActionSell, Expression: `[quality] == "rare"`, ProfileRevision: 2, AssignmentRevision: 7},
	})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := NewInventoryLock(unlockedInventory())
	if err != nil {
		t.Fatal(err)
	}
	filter := NewFilter(testLogger(), lock, policy)
	rune := runtimeInventoryItem(1, "r01", "rune", world.ItemQualityNormal, true)
	result := filter.Evaluate(rune)
	if !result.Matched || result.Action != ActionKeep || result.ProfileID != "priority" || result.ProfileRevision != 4 || result.AssignmentRevision != 7 || len(result.Trace) != 1 {
		t.Fatalf("first match = %+v", result)
	}
	report := filter.Decide(world.State{Items: []world.Item{rune}})
	if !hasRuntimeDecision(report, DecisionKindKeep) || hasRuntimeDecision(report, DecisionKindSell) {
		t.Fatalf("keep report = %+v", report.Decisions)
	}

	rare := runtimeInventoryItem(2, "rar", "armo", world.ItemQualityRare, false)
	report = filter.Decide(world.State{Items: []world.Item{rare}})
	if !hasRuntimeDecision(report, DecisionKindIdentifyRequired) {
		t.Fatalf("unidentified sell report = %+v", report.Decisions)
	}
	rare.Identified = true
	report = filter.Decide(world.State{Items: []world.Item{rare}})
	if !hasRuntimeDecision(report, DecisionKindSell) || hasRuntimeDecision(report, DecisionKindStash) {
		t.Fatalf("identified sell report = %+v", report.Decisions)
	}

	noMatch := runtimeInventoryItem(3, "cap", "helm", world.ItemQualityNormal, true)
	if got := filter.Decide(world.State{Items: []world.Item{noMatch}}); len(got.Decisions) != 0 {
		t.Fatalf("no-match decisions = %+v", got.Decisions)
	}
}

func runtimeInventoryItem(id uint32, code, itemType string, quality world.ItemQuality, identified bool) world.Item {
	return world.Item{UnitID: id, Code: code, Type: itemType, Quality: quality, Identified: identified, Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Width: 1, Height: 1}
}

func hasRuntimeDecision(report DecisionReport, kind DecisionKind) bool {
	for _, decision := range report.Decisions {
		if decision.Kind == kind {
			return true
		}
	}
	return false
}
