package crafting

import "testing"

func TestStorageRecipeCatalogScopeAndOwnership(t *testing.T) {
	all := Recipes()
	if len(all) != 23 {
		t.Fatalf("recipes=%d", len(all))
	}
	for _, recipe := range all {
		found, ok := LookupRecipe(recipe.InputCode)
		if !ok || found != recipe || recipe.InputQuantity != 3 || recipe.SourceFile != "cubemain.txt" || recipe.SourceKey == "" {
			t.Fatalf("invalid recipe: %+v", recipe)
		}
	}
	for input, output := range map[string]string{"gsg": "glg", "glg": "gpg", "sku": "skl", "skl": "skz", "r01": "r02", "r09": "r10"} {
		got, ok := LookupRecipe(input)
		if !ok || got.OutputCode != output {
			t.Fatalf("%s -> %+v", input, got)
		}
	}
	for _, excluded := range []string{"gcv", "gfv", "gpg", "skz", "r10", "r33", "key", ""} {
		if _, ok := LookupRecipe(excluded); ok {
			t.Fatalf("out-of-scope ingredient %q", excluded)
		}
	}
	first := all[0]
	all[0].InputCode = "changed"
	if got, _ := LookupRecipe(first.InputCode); got != first {
		t.Fatal("caller mutated catalog")
	}
}
