package crafting

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRecipeTestExecutorOneRealRecipeAndProductGate(t *testing.T) {
	for _, tc := range []struct {
		code, out      string
		source, target int
	}{{"gsr", "glr", 22, 97}, {"glr", "gpr", 97, 5}, {"r01", "r02", 7, 6}} {
		t.Run(tc.code, func(t *testing.T) {
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			a := &recipeActions{}
			e, err := NewRecipeTestExecutor(log, a, tc.code)
			if err != nil {
				t.Fatal(err)
			}
			state := stockState(map[string]int{tc.code: tc.source, tc.out: tc.target})
			state.Items = []world.Item{{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true, Width: 2, Height: 2}}
			request := Request{Code: tc.code, RunGeneration: 1}
			nextID := uint32(100)
			progress, paused := 0, false
			for tick := 0; tick < 30; tick++ {
				before := len(a.calls)
				result := e.Tick(state, state.At, request, false, false)
				if result.Progress != nil {
					progress++
				}
				if len(a.calls) > before {
					applyRecipeCall(t, &state, a.calls[before], &nextID)
				}
				if e.ReadyToTransmute() && !paused {
					e.Tick(state, state.At, request, true, false)
					paused = true
					if len(a.calls) != before {
						t.Fatal("pause sent Transmute")
					}
				}
				if result.Done {
					if !result.Success || progress != 1 || len(a.calls) != 4 || len(state.Items) != 1 {
						t.Fatalf("result=%+v progress=%d calls=%v", result, progress, a.calls)
					}
					source, _ := state.CollectionCount(tc.code)
					target, _ := state.CollectionCount(tc.out)
					if source != tc.source-3 || target != tc.target+1 {
						t.Fatalf("counts=%d/%d", source, target)
					}
					normal, err := NewExecutor(log, &recipeActions{})
					if err != nil {
						t.Fatal(err)
					}
					if rejected := normal.Tick(state, state.At, request, false, false); rejected.Failure == nil {
						t.Fatal("product accepted diagnostic request")
					}
					e.Reset()
					wrong := request
					wrong.Code = "unsupported"
					if rejected := e.Tick(state, state.At, wrong, false, false); rejected.Failure == nil || len(a.calls) != 4 {
						t.Fatal("reset authorized another recipe")
					}
					return
				}
				freshFrame(&state)
			}
			t.Fatal("recipe test did not terminate")
		})
	}
}
