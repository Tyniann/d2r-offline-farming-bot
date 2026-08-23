package loot

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestLoadPickitEmptyFileMatchesNothing(t *testing.T) {
	p := loadPickitFromTestFile(t, "")
	if p == nil {
		t.Fatal("Pickit should be loaded for empty file")
	}
	if got := p.Evaluate(world.Item{Code: "r01", Type: "rune"}); got.Matched {
		t.Fatalf("empty pickit matched = %+v", got)
	}
}

func TestPickitEvaluatesBareQuotedAndIntegerLiterals(t *testing.T) {
	p := loadPickitFromTestFile(t, `
[type] == rune
[name] == "pk1"
[stat:42] >= 10
`)
	tests := []struct {
		name string
		item world.Item
		line int
	}{
		{name: "bare type", item: world.Item{Type: "rune"}, line: 2},
		{name: "quoted name", item: world.Item{Code: "pk1"}, line: 3},
		{name: "integer stat", item: world.Item{Identified: true, Stats: []world.ItemStat{{ID: 42, Value: 10}}}, line: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.Evaluate(tc.item)
			if !got.Matched || got.Line != tc.line {
				t.Fatalf("Evaluate() = %+v, want line %d", got, tc.line)
			}
		})
	}
}

func TestPickitPrecedenceParenthesesCommentsAndHash(t *testing.T) {
	p := loadPickitFromTestFile(t, `
; full-line comment
// full-line comment
([name] == r01 || [name] == r02) && [type] == rune // inline comment
[name] == pk1 # [flag] == identified
`)
	if got := p.Evaluate(world.Item{Code: "r01", Type: "misc"}); got.Matched {
		t.Fatalf("wrong type should not match, got %+v", got)
	}
	if got := p.Evaluate(world.Item{Code: "r02", Type: "rune"}); !got.Matched || got.Line != 4 {
		t.Fatalf("parenthesized expression = %+v, want line 4", got)
	}
	if got := p.Evaluate(world.Item{Code: "pk1", Identified: false}); got.Matched {
		t.Fatalf("hash AND should require identified, got %+v", got)
	}
	if got := p.Evaluate(world.Item{Code: "pk1", Identified: true}); !got.Matched || got.Line != 5 {
		t.Fatalf("hash AND expression = %+v, want line 5", got)
	}
}

func TestPickitMatchesCountessMVPItems(t *testing.T) {
	p := loadPickitFromTestFile(t, `
[type] == rune
[name] == pk1
[name] == gzv || [name] == gpv
`)
	for _, item := range []world.Item{
		{Code: "r33", Type: "rune"},
		{Code: "pk1", Type: "ques"},
		{Code: "gzv", Type: "gema"},
		{Code: "gpv", Type: "gema"},
	} {
		if got := p.Evaluate(item); !got.Matched {
			t.Fatalf("expected match for %+v", item)
		}
	}
	if got := p.Evaluate(world.Item{Code: "gcv", Type: "gema"}); got.Matched {
		t.Fatalf("chipped gem should not match explicit flawless/perfect list: %+v", got)
	}
}

func TestPickitStringMatchingIsCaseInsensitive(t *testing.T) {
	p := loadPickitFromTestFile(t, `
[type] == Rune
[name] == PK1
[quality] == Unique
[flag] == Ethereal
`)
	tests := []world.Item{
		{Type: "rune"},
		{Code: "pk1"},
		{Quality: world.ItemQualityUnique},
		{Ethereal: true},
	}
	for _, item := range tests {
		if got := p.Evaluate(item); !got.Matched {
			t.Fatalf("expected case-insensitive match for %+v", item)
		}
	}
}

func TestPickitQualityFlagsAndStats(t *testing.T) {
	p := loadPickitFromTestFile(t, `
[quality] == unique
[flag] != identified
[flag] == ethereal
[stat:17] > 5
[stat:17] <= -2
`)
	tests := []struct {
		item world.Item
		line int
	}{
		{item: world.Item{Quality: world.ItemQualityUnique}, line: 2},
		{item: world.Item{Identified: false}, line: 3},
		{item: world.Item{Identified: true, Ethereal: true}, line: 4},
		{item: world.Item{Identified: true, Stats: []world.ItemStat{{ID: 17, Value: 6}}}, line: 5},
		{item: world.Item{Identified: true, Stats: []world.ItemStat{{ID: 17, Value: -2}}}, line: 6},
	}
	for _, tc := range tests {
		got := p.Evaluate(tc.item)
		if !got.Matched || got.Line != tc.line {
			t.Fatalf("Evaluate(%+v) = %+v, want line %d", tc.item, got, tc.line)
		}
	}
}

func TestPickitStatRulesRequireIdentification(t *testing.T) {
	p := loadPickitFromTestFile(t, "[stat:39] >= 30")
	item := world.Item{Identified: false, Stats: []world.ItemStat{{ID: 39, Value: 40}}}
	if p.Evaluate(item).Matched {
		t.Fatal("unidentified item matched a stat rule")
	}
	item.Identified = true
	if !p.Evaluate(item).Matched {
		t.Fatal("identified item with matching stat did not match")
	}
}

func TestPickitSocketsAndSocketedFailClosed(t *testing.T) {
	socketed4 := world.Item{
		Type: "pole", BaseTier: world.BaseTierElite, Identified: false,
		Sockets: 4, SocketsAvailable: true, Socketed: true,
	}
	first := loadPickitFromTestFile(t, `
[sockets] == 4
[sockets] >= 3
[flag] == socketed
[type] == "pole" && [tier] == "elite" && [sockets] == 4
`)
	if got := first.Evaluate(socketed4); !got.Matched || got.Line != 2 {
		t.Fatalf("4os elite pole first-match = %+v, want line 2", got)
	}

	for _, expr := range []string{
		"[sockets] > 3", "[sockets] >= 4", "[sockets] < 5",
		"[sockets] <= 4", "[sockets] == 4", "[sockets] != 3",
		`[type] == "pole" && [tier] == "elite" && [sockets] == 4`,
		"[flag] == socketed",
	} {
		if !loadPickitFromTestFile(t, expr).Evaluate(socketed4).Matched {
			t.Fatalf("%s did not match 4os item", expr)
		}
	}

	unavailable := world.Item{Sockets: 4, SocketsAvailable: false, Socketed: true}
	for _, expr := range []string{
		"[sockets] == 4", "[sockets] != 0", "[sockets] > 0",
		"[flag] == socketed", "[flag] != socketed",
	} {
		if loadPickitFromTestFile(t, expr).Evaluate(unavailable).Matched {
			t.Fatalf("unavailable matched %s", expr)
		}
	}

	unsocketedKnown := world.Item{Sockets: 0, SocketsAvailable: true, Socketed: false}
	if !loadPickitFromTestFile(t, `[flag] != socketed`).Evaluate(unsocketedKnown).Matched {
		t.Fatal("known unsocketed should match != socketed")
	}
	if loadPickitFromTestFile(t, `[flag] == socketed`).Evaluate(unsocketedKnown).Matched {
		t.Fatal("known unsocketed matched == socketed")
	}

	canonical, err := CanonicalPickitExpression(`[type] == "pole" && [sockets] == 4 && [flag] == socketed`)
	if err != nil {
		t.Fatal(err)
	}
	if canonical != `[type] == "pole" && [sockets] == 4 && [flag] == "socketed"` {
		t.Fatalf("canonical = %q", canonical)
	}
}

func TestPickitQualityCanSelectUnidentifiedPickup(t *testing.T) {
	p := loadPickitFromTestFile(t, "[quality] == unique")
	item := world.Item{Quality: world.ItemQualityUnique, Identified: false}
	if !p.Evaluate(item).Matched {
		t.Fatal("quality rule should be able to select an unidentified unique for pickup")
	}
	if !RequiresIdentificationForKeep(item) {
		t.Fatal("unidentified unique should require identification before keep/stash")
	}
}

func TestRequiresIdentificationForKeepByQuality(t *testing.T) {
	for _, quality := range []world.ItemQuality{world.ItemQualityMagic, world.ItemQualitySet, world.ItemQualityRare, world.ItemQualityUnique, world.ItemQualityCrafted} {
		if !RequiresIdentificationForKeep(world.Item{Quality: quality}) {
			t.Fatalf("quality %s should require identification", quality)
		}
	}
	if RequiresIdentificationForKeep(world.Item{Quality: world.ItemQualityNormal}) {
		t.Fatal("normal item should not require identification")
	}
	if RequiresIdentificationForKeep(world.Item{Quality: world.ItemQualityUnique, Identified: true}) {
		t.Fatal("identified unique should not remain gated")
	}
}

func TestPickitResultUsesRuleIndexAndLine(t *testing.T) {
	p := loadPickitFromTestFile(t, `

[name] == r01
[name] == r02
`)
	got := p.Evaluate(world.Item{Code: "r02"})
	if !got.Matched || got.RuleIndex != 1 || got.Line != 4 || got.Rule != "[name] == r02" {
		t.Fatalf("PickitResult = %+v, want zero-based rule index and one-based line", got)
	}
}

func TestPickitRejectsUnsupportedSyntax(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unsupported keyword", content: "[maxquantity] == 1", want: "unsupported keyword"},
		{name: "prefix", content: "[prefix] == fools", want: "unsupported keyword"},
		{name: "suffix", content: "[suffix] == whale", want: "unsupported keyword"},
		{name: "unknown NIP section", content: "[name] == r01 # [unknown] == value", want: "unsupported keyword"},
		{name: "invalid string operator", content: "[name] > r01", want: "supports only == and !="},
		{name: "invalid flag", content: "[flag] == enchanted", want: "unsupported flag"},
		{name: "invalid stat literal", content: "[stat:12] >= rune", want: "requires an integer literal"},
		{name: "invalid sockets literal", content: "[sockets] == four", want: "requires an integer literal"},
		{name: "invalid syntax", content: "[type] == rune &&", want: "expected field"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writePickitTestFile(t, tc.content)
			_, err := LoadPickit(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), ":1:") {
				t.Fatalf("error = %v, want %q with line context", err, tc.want)
			}
		})
	}
}

func TestSummarizePickitExpressionProjectsTypedLanguageNeutralParams(t *testing.T) {
	trueValue := true
	four := 4
	tests := []struct {
		name       string
		expression string
		want       PickitRuleSummary
	}{
		{name: "runes", expression: `[type] == rune`, want: PickitRuleSummary{Kind: "runes"}},
		{name: "rejuvenation", expression: `[type] == rpot`, want: PickitRuleSummary{Kind: "rejuvenation"}},
		{name: "item codes", expression: `[name] == pk1 || [name] == pk2`, want: PickitRuleSummary{Kind: "item_codes", Params: PickitRuleSummaryParams{Codes: []string{"pk1", "pk2"}}}},
		{name: "item types", expression: `[type] == pole || [type] == spea`, want: PickitRuleSummary{Kind: "item_types", Params: PickitRuleSummaryParams{Types: []string{"pole", "spea"}}}},
		{name: "quality", expression: `[quality] == unique || [quality] == set`, want: PickitRuleSummary{Kind: "quality", Params: PickitRuleSummaryParams{Qualities: []string{"unique", "set"}}}},
		{name: "tier", expression: `[tier] == elite`, want: PickitRuleSummary{Kind: "tier", Params: PickitRuleSummaryParams{Tiers: []string{"elite"}}}},
		{name: "quality and tier", expression: `([quality] == unique || [quality] == set) && [tier] == elite`, want: PickitRuleSummary{Kind: "quality_tier", Params: PickitRuleSummaryParams{Qualities: []string{"unique", "set"}, Tiers: []string{"elite"}}}},
		{name: "set item", expression: `[setitem] == "Tal Rasha's Adjudication"`, want: PickitRuleSummary{Kind: "set_item", Params: PickitRuleSummaryParams{SetKey: "Tal Rasha's Adjudication"}}},
		{name: "unique item", expression: `[uniqueitem] == "Harlequin Crest"`, want: PickitRuleSummary{Kind: "unique_item", Params: PickitRuleSummaryParams{UniqueKey: "Harlequin Crest"}}},
		{name: "socket filter", expression: `[type] == pole && [tier] == elite && [sockets] == 4 && [flag] == ethereal`, want: PickitRuleSummary{Kind: "socket_filter", Params: PickitRuleSummaryParams{Types: []string{"pole"}, Tiers: []string{"elite"}, SocketOperator: "==", SocketCount: &four, Ethereal: &trueValue}}},
		{name: "manual stat", expression: `[stat:39] >= 30`, want: PickitRuleSummary{Kind: "custom"}},
		{name: "unrepresentable negation", expression: `[type] != rune`, want: PickitRuleSummary{Kind: "custom"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := SummarizePickitExpression(test.expression)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("summary = %#v, want %#v", got, test.want)
			}
		})
	}
	if _, err := SummarizePickitExpression(`[type] ==`); err == nil {
		t.Fatal("invalid expression returned a summary")
	}
}

func TestFilterReadyReflectsLoadedPickit(t *testing.T) {
	lock, err := NewInventoryLock(allLockedInventory())
	if err != nil {
		t.Fatal(err)
	}
	if NewFilter(testLogger(), lock, nil).Ready() {
		t.Fatal("Ready() should be false without Pickit")
	}
	if !NewFilter(testLogger(), lock, &Pickit{}).Ready() {
		t.Fatal("Ready() should be true for an empty loaded Pickit")
	}
}

func loadPickitFromTestFile(t *testing.T, content string) *Pickit {
	t.Helper()
	p, err := LoadPickit(writePickitTestFile(t, content))
	if err != nil {
		t.Fatalf("LoadPickit() error = %v", err)
	}
	return p
}

func writePickitTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.nip")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func allLockedInventory() [][]int {
	return [][]int{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
