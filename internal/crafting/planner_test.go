package crafting

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func stockState(counts map[string]int) world.State {
	s := world.State{Valid: true, Phase: world.GamePhaseInGame, At: time.Unix(100, 0), Generation: 1, Area: world.LookupArea(world.RogueEncampment), UI: world.UIState{InventoryOpen: true, StashOpen: true}, Collection: world.CollectionState{ScopeKnown: true, ScopeID: "stock", TabKnown: true, Tab: world.StorageTabGems}}
	for code, n := range counts {
		s.Collection.Counts = append(s.Collection.Counts, world.MaterialCount{Code: code, Count: n, Known: true})
	}
	s.Collection.Generation, s.Collection.ObservedAt = s.Generation, s.At
	return s
}

func TestBuildPlanMaterialMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, code   string
		stock, final map[string]int
		steps        int
		full         string
	}{
		{"flawless", "glr", map[string]int{"glr": 99, "gpr": 0}, map[string]int{"glr": 0, "gpr": 33}, 33, ""},
		{"limited_perfect", "glr", map[string]int{"glr": 99, "gpr": 90}, map[string]int{"glr": 72, "gpr": 99}, 9, ""},
		{"normal", "gsg", map[string]int{"gsg": 99, "glg": 0, "gpg": 0}, map[string]int{"gsg": 0, "glg": 0, "gpg": 11}, 44, ""},
		{"full_intermediate", "gsg", map[string]int{"gsg": 99, "glg": 99, "gpg": 0}, map[string]int{"gsg": 0, "glg": 0, "gpg": 44}, 77, ""},
		{"one_perfect_space", "gsg", map[string]int{"gsg": 99, "glg": 99, "gpg": 98}, map[string]int{"gsg": 90, "glg": 99, "gpg": 99}, 4, ""},
		{"perfect_full", "gsg", map[string]int{"gsg": 99, "glg": 0, "gpg": 99}, nil, 0, "gpg"},
		{"flawless_full_target", "glr", map[string]int{"glr": 99, "gpr": 99}, nil, 0, "gpr"},
		{"el", "r01", map[string]int{"r01": 99, "r02": 90, "r03": 0}, map[string]int{"r01": 72, "r02": 99}, 9, ""},
		{"ort", "r09", map[string]int{"r09": 99, "r10": 0}, map[string]int{"r09": 0, "r10": 33}, 33, ""},
		{"thul_full", "r09", map[string]int{"r09": 99, "r10": 99}, nil, 0, "r10"},
		{"skull", "sku", map[string]int{"sku": 99, "skl": 0, "skz": 0, "glr": 99}, map[string]int{"sku": 0, "skl": 0, "skz": 11}, 44, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := BuildPlan(stockState(tc.stock), tc.code)
			if tc.full != "" {
				var f *Failure
				reason, _ := StorageFullReason(tc.code)
				if !errors.As(err, &f) || f.Reason != reason || f.MaterialCode != tc.full || len(p.Steps) != 0 {
					t.Fatalf("plan=%+v err=%v", p, err)
				}
				return
			}
			if err != nil || len(p.Steps) != tc.steps || !maps.Equal(p.Final, tc.final) {
				t.Fatalf("plan=%+v err=%v", p, err)
			}
			if tc.code == "gsg" && tc.stock["glg"] == 99 && p.Steps[0].Recipe.InputCode != "glg" {
				t.Fatal("full intermediate was not cleared first")
			}
			for _, step := range p.Steps {
				for _, n := range step.After {
					if n < 0 || n > 99 {
						t.Fatal("capacity overflow")
					}
				}
			}
		})
	}
	for _, code := range []string{"gcr", "gfr", "gpr", "r10", "r33", "key"} {
		if _, err := BuildPlan(stockState(map[string]int{code: 99}), code); err == nil {
			t.Fatalf("excluded %s accepted", code)
		}
	}
	if _, err := BuildPlan(stockState(map[string]int{"glr": 99}), "glr"); err == nil {
		t.Fatal("missing target treated as zero")
	}
}
