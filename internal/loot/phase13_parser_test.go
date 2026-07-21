package loot

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPhase13ExactSetItemMatchesOnlyResolvedIdentity(t *testing.T) {
	pickit := loadPickitFromTestFile(t, `[setitem] == "Tal Rasha's Adjudication"`)

	talAmulet := world.Item{
		Code: "amu", Quality: world.ItemQualitySet,
		IdentityKind: world.ItemIdentitySet, IdentityKey: "Tal Rasha's Adjudication", IdentityValid: true,
	}
	if got := pickit.Evaluate(talAmulet); !got.Matched {
		t.Fatalf("Tal Rasha's Adjudication did not match: %+v", got)
	}
	otherSetAmulet := talAmulet
	otherSetAmulet.IdentityKey = "Civerb's Icon"
	if got := pickit.Evaluate(otherSetAmulet); got.Matched {
		t.Fatalf("different set amulet matched: %+v", got)
	}

	// Ungültige Identität darf auch eine Ungleichheitsregel nicht wahr machen.
	drifted := talAmulet
	drifted.IdentityValid = false
	neq := loadPickitFromTestFile(t, `[setitem] != "Civerb's Icon"`)
	if got := neq.Evaluate(drifted); got.Matched {
		t.Fatalf("identity drift matched fail-open: %+v", got)
	}
}

func TestPhase13ExactIdentitySurvivesIdentificationReevaluation(t *testing.T) {
	pickit := loadPickitFromTestFile(t, `[uniqueitem] == "Harlequin Crest"`)
	item := world.Item{
		Quality: world.ItemQualityUnique, Identified: false,
		IdentityKind: world.ItemIdentityUnique, IdentityKey: "Harlequin Crest", IdentityValid: true,
	}
	before := pickit.Evaluate(item)
	item.Identified = true
	after := pickit.Evaluate(item)
	if !before.Matched || !after.Matched || before.RuleIndex != after.RuleIndex {
		t.Fatalf("identity re-evaluation before=%+v after=%+v", before, after)
	}
}

func TestPhase13EtherealBaseCombination(t *testing.T) {
	pickit := loadPickitFromTestFile(t, `[name] == "7s8" # [flag] == ethereal`)
	if !pickit.Evaluate(world.Item{Code: "7s8", Ethereal: true}).Matched {
		t.Fatal("ethereal Thresher did not match")
	}
	if pickit.Evaluate(world.Item{Code: "7s8", Ethereal: false}).Matched {
		t.Fatal("non-ethereal Thresher matched")
	}
	if pickit.Evaluate(world.Item{Code: "7pa", Ethereal: true}).Matched {
		t.Fatal("different ethereal base matched")
	}
}

func TestPhase13RuleMetadataTraceAndFirstMatch(t *testing.T) {
	pickit, err := CompilePickitRules("profiles", []PickitRuleSpec{
		{ProfileID: "gems", RuleID: "perfect-amethyst", Action: ActionKeep, Expression: `[name] == gpv`},
		{ProfileID: "tal-rasha", RuleID: "adjudication", Action: ActionSell, Expression: `[setitem] == "Tal Rasha's Adjudication"`},
		{ProfileID: "fallback", RuleID: "set", Action: ActionKeep, Expression: `[quality] == set`},
	})
	if err != nil {
		t.Fatal(err)
	}
	item := world.Item{
		Quality:      world.ItemQualitySet,
		IdentityKind: world.ItemIdentitySet, IdentityKey: "Tal Rasha's Adjudication", IdentityValid: true,
	}
	got := pickit.Evaluate(item)
	if !got.Matched || got.ProfileID != "tal-rasha" || got.RuleID != "adjudication" || got.Action != ActionSell || got.RuleIndex != 1 {
		t.Fatalf("result metadata = %+v", got)
	}
	if len(got.Trace) != 2 || got.Trace[0].Matched || !got.Trace[1].Matched {
		t.Fatalf("evaluation trace = %+v", got.Trace)
	}
}

func TestPhase13CanonicalExpressionRoundTripsEscapes(t *testing.T) {
	expression := `[name] == "O'Brien \"quoted\" C:\\loot"`
	first, err := CanonicalPickitExpression(expression)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalPickitExpression(first)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.Contains(first, `o'brien`) || !strings.Contains(first, `\"quoted\"`) || !strings.Contains(first, `c:\\loot`) {
		t.Fatalf("canonical roundtrip first=%q second=%q", first, second)
	}
}

func TestPhase13RejectsUnknownIdentityReferencesAndRuleMetadata(t *testing.T) {
	for _, expression := range []string{
		`[setitem] == "not-a-set-item"`,
		`[uniqueitem] == "not-a-unique-item"`,
	} {
		if _, err := parsePickit("test.nip", expression); err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Fatalf("expression %q error = %v", expression, err)
		}
	}
	if _, err := CompilePickitRules("profiles", []PickitRuleSpec{{ProfileID: "p", RuleID: "r", Action: "drop", Expression: `[name] == r01`}}); err == nil {
		t.Fatal("unsupported action was accepted")
	}
	if _, err := CompilePickitRules("profiles", []PickitRuleSpec{
		{ProfileID: "p", RuleID: "r", Action: ActionKeep, Expression: `[name] == r01`},
		{ProfileID: "p", RuleID: "r", Action: ActionSell, Expression: `[name] == r02`},
	}); err == nil {
		t.Fatal("duplicate profile/rule id was accepted")
	}
}
