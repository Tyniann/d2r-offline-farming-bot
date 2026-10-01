// Package crafting plans and verifies CASC-backed storage compaction recipes.
// It consumes World evidence and injected actions without reading raw memory.
package crafting

import "github.com/Tyniann/d2r-offline-farming-bot/internal/world"

//go:generate go run ../../tools/generate-crafting-catalog -src ../../.tmp/d2r-excel -version 3.2.92777 -out catalog_data.go

// Recipe is one enabled, unconditional three-to-one recipe from cubemain.txt.
// SourceKey is its stable description; item codes are validated against misc.txt.
type Recipe struct {
	SourceFile    string
	SourceKey     string
	InputCode     string
	InputQuantity int
	OutputCode    string
	Version       int
}

// Recipes returns a copy of the 23 supported storage recipes.
// It excludes chipped/flawed gems and every rune ingredient above Ort.
func Recipes() []Recipe {
	return append([]Recipe(nil), recipes[:]...)
}

// LookupRecipe finds the sole supported recipe for an ingredient code.
func LookupRecipe(code string) (Recipe, bool) {
	for _, recipe := range recipes {
		if recipe.InputCode == code {
			return recipe, true
		}
	}
	return Recipe{}, false
}

// StorageFullReason classifies supported gem grades, perfect outputs and CASC
// rune codes for operator errors. It grants no recipe or input authorization.
func StorageFullReason(code string) (string, bool) {
	for _, recipe := range recipes {
		if recipe.Version != 100 && (recipe.InputCode == code || recipe.OutputCode == code) {
			return ReasonGemFull, true
		}
	}
	for _, item := range world.ItemCatalogEntries() {
		if item.Code == code && item.Type == "rune" {
			return ReasonRuneFull, true
		}
	}
	return "", false
}
