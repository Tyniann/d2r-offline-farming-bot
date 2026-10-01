package crafting

import (
	"fmt"
	"maps"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Failure preserves the original trigger and the material that blocked a job.
// Cause retains input errors for the application log, not for operator text.
type Failure struct {
	Reason, TriggerCode, MaterialCode string
	Cause                             error
}

// Error returns the stable reason and material context.
func (f *Failure) Error() string {
	return fmt.Sprintf("%s: trigger=%s material=%s", f.Reason, f.TriggerCode, f.MaterialCode)
}

// Unwrap retains the original input error for errors.Is and errors.As.
func (f *Failure) Unwrap() error { return f.Cause }

const (
	// ReasonGemFull identifies a plan that cannot create trigger capacity.
	ReasonGemFull = "storage_gem_full"
	// ReasonRuneFull identifies a rune target without storage capacity.
	ReasonRuneFull = "storage_rune_full"
	// ReasonUnavailable identifies missing current collection evidence.
	ReasonUnavailable = "storage_state_unavailable"
	// ReasonCubeNotEmpty blocks a job before the first ingredient transfer.
	ReasonCubeNotEmpty = "storage_cube_not_empty"
	// ReasonUnconfirmed identifies changed bindings or an unconfirmed action.
	ReasonUnconfirmed = "storage_compaction_unconfirmed"
	// ReasonTimeout identifies an exhausted active job budget.
	ReasonTimeout = "storage_compaction_timeout"
)

// PlannedRecipe binds one recipe to the expected whole-family collection counts.
type PlannedRecipe struct {
	Recipe        Recipe
	Before, After map[string]int
}

// Plan is a finite simulation using only supported recipes of the trigger family.
// Maps are owned by this result; the executor builds and retains its own plan.
type Plan struct {
	TriggerCode    string
	Tab            world.StorageTab
	Initial, Final map[string]int
	Steps          []PlannedRecipe
}

// BuildPlan simulates the maximal supported batch from fresh collection counts.
// It has no input effects. Production authorization of a full trigger slot is
// checked separately by the executor; absent counts never mean zero.
func BuildPlan(state world.State, code string) (Plan, error) {
	p := Plan{TriggerCode: code, Tab: world.StorageTabGems, Initial: make(map[string]int)}
	recipe, ok := LookupRecipe(code)
	if !ok {
		return p, &Failure{Reason: ReasonUnavailable, TriggerCode: code, MaterialCode: code}
	}
	if recipe.Version == 100 {
		p.Tab = world.StorageTabRunes
	}
	chain, normal := LookupRecipe(recipe.OutputCode)
	normal = normal && p.Tab == world.StorageTabGems
	codes := []string{code, recipe.OutputCode}
	if normal {
		codes = append(codes, chain.OutputCode)
	}
	for _, material := range codes {
		count, known := state.CollectionCount(material)
		if !known {
			return p, &Failure{Reason: ReasonUnavailable, TriggerCode: code, MaterialCode: material}
		}
		p.Initial[material] = count
	}
	p.Final = maps.Clone(p.Initial)
	appendRecipe := func(r Recipe) {
		before := maps.Clone(p.Final)
		p.Final[r.InputCode] -= r.InputQuantity
		p.Final[r.OutputCode]++
		p.Steps = append(p.Steps, PlannedRecipe{Recipe: r, Before: before, After: maps.Clone(p.Final)})
	}
	full := func(material string) (Plan, error) {
		// No partially useful plan may escape when the trigger still has no room.
		reason, _ := StorageFullReason(code)
		return Plan{}, &Failure{Reason: reason, TriggerCode: code, MaterialCode: material}
	}
	if !normal {
		for p.Final[code] >= recipe.InputQuantity && p.Final[recipe.OutputCode] < 99 {
			appendRecipe(recipe)
		}
		if len(p.Steps) == 0 {
			return full(recipe.OutputCode)
		}
	} else {
		if p.Final[chain.OutputCode] == 99 {
			return full(chain.OutputCode)
		}
		for p.Final[code] >= recipe.InputQuantity {
			if p.Final[recipe.OutputCode] == 99 {
				if p.Final[chain.OutputCode] == 99 {
					break
				}
				appendRecipe(chain)
			}
			appendRecipe(recipe)
		}
		for p.Final[chain.InputCode] >= chain.InputQuantity && p.Final[chain.OutputCode] < 99 {
			appendRecipe(chain)
		}
	}
	if len(p.Steps) == 0 || p.Final[code] >= 99 || p.Final[code] >= p.Initial[code] {
		return full(recipe.OutputCode)
	}
	return p, nil
}
