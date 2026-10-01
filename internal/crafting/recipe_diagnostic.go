package crafting

import (
	"fmt"
	"log/slog"
	"maps"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// NewRecipeTestExecutor creates the isolated one-recipe acceptance executor.
// Its explicit CLI authorization replaces only the full inventory-trigger gate;
// it uses real counts, unchanged inventory and the same action confirmations.
// Request.UnitID must be zero and Code must remain the constructor's code.
func NewRecipeTestExecutor(log *slog.Logger, actions Actions, code string) (*Executor, error) {
	if _, ok := LookupRecipe(code); !ok {
		return nil, fmt.Errorf("storage recipe test: unsupported ingredient %q", code)
	}
	e, err := NewExecutor(log, actions)
	if err != nil {
		return nil, err
	}
	e.testCode = code
	return e, nil
}

// ReadyToTransmute identifies the confirmed ingredient boundary for an isolated
// diagnostic pause. It observes state and sends no input or confirmation.
func (e *Executor) ReadyToTransmute() bool {
	return e != nil && e.started && !e.terminal.Done && e.stage == transmute
}

func buildRecipeTestPlan(state world.State, code string) (Plan, error) {
	recipe, _ := LookupRecipe(code)
	tab := world.StorageTabGems
	if recipe.Version == 100 {
		tab = world.StorageTabRunes
	}
	p := Plan{TriggerCode: code, Tab: tab, Initial: make(map[string]int)}
	for _, material := range []string{code, recipe.OutputCode} {
		count, known := state.CollectionCount(material)
		if !known {
			return Plan{}, &Failure{Reason: ReasonUnavailable, TriggerCode: code, MaterialCode: material}
		}
		p.Initial[material] = count
	}
	if p.Initial[code] < 3 || p.Initial[recipe.OutputCode] >= 99 {
		reason, _ := StorageFullReason(code)
		return Plan{}, &Failure{Reason: reason, TriggerCode: code, MaterialCode: recipe.OutputCode}
	}
	p.Final = maps.Clone(p.Initial)
	p.Final[code] -= 3
	p.Final[recipe.OutputCode]++
	p.Steps = []PlannedRecipe{{Recipe: recipe, Before: maps.Clone(p.Initial), After: maps.Clone(p.Final)}}
	return p, nil
}
