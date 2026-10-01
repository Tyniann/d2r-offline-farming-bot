package crafting

import (
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type recipeCall struct {
	kind, code string
	id         uint32
	ids        []uint32
}
type recipeActions struct {
	calls  []recipeCall
	resets int
	err    error
}

func (a *recipeActions) SelectTab(_ world.State, tab world.StorageTab) error {
	a.calls = append(a.calls, recipeCall{kind: "select", code: string(tab)})
	return a.err
}
func (a *recipeActions) TabReady(s world.State, tab world.StorageTab) bool {
	return s.Collection.Tab == tab
}
func (a *recipeActions) WithdrawIngredients(_ world.State, code string) error {
	a.calls = append(a.calls, recipeCall{kind: "withdraw", code: code})
	return a.err
}
func (a *recipeActions) Transmute(_ world.State, ids []uint32) error {
	a.calls = append(a.calls, recipeCall{kind: "transmute", ids: slices.Clone(ids)})
	return a.err
}
func (a *recipeActions) StoreOutput(_ world.State, id uint32, code string) error {
	a.calls = append(a.calls, recipeCall{kind: "store", code: code, id: id})
	return a.err
}
func (a *recipeActions) Reset() { a.resets++ }

func executorFixture(t *testing.T, code string, stock map[string]int) (*Executor, *recipeActions, world.State, Request) {
	t.Helper()
	a := &recipeActions{}
	e, err := NewExecutor(slog.New(slog.NewTextHandler(io.Discard, nil)), a)
	if err != nil {
		t.Fatal(err)
	}
	s := stockState(stock)
	s.Items = []world.Item{{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true, GridX: 0, GridY: 0, Width: 2, Height: 2}, {UnitID: 2, Code: code, Location: world.ItemLocationInventory, PlayerOwned: true, GridX: 9, GridY: 2, Width: 1, Height: 1}}
	return e, a, s, Request{UnitID: 2, Code: code, GridX: 9, GridY: 2, RunGeneration: 1}
}
func freshFrame(s *world.State) {
	s.At = s.At.Add(100 * time.Millisecond)
	s.Generation++
	s.Collection.ObservedAt = s.At
	s.Collection.Generation = s.Generation
}
func changeCount(s *world.State, code string, delta int) {
	for i := range s.Collection.Counts {
		if s.Collection.Counts[i].Code == code {
			s.Collection.Counts[i].Count += delta
			return
		}
	}
}
func cubeItem(id uint32, code string, x, y int) world.Item {
	return world.Item{UnitID: id, Code: code, Location: world.ItemLocationCube, PlayerOwned: true, Width: 1, Height: 1, GridX: x, GridY: y}
}
func applyRecipeCall(t *testing.T, s *world.State, call recipeCall, nextID *uint32) {
	t.Helper()
	switch call.kind {
	case "select":
		s.Collection.Tab = world.StorageTab(call.code)
	case "withdraw":
		changeCount(s, call.code, -3)
		for x := 2; x >= 0; x-- {
			*nextID++
			s.Items = append(s.Items, cubeItem(*nextID, call.code, x, 3))
		}
	case "transmute":
		items := s.ItemsByLocation(world.ItemLocationCube)
		if len(items) != 3 || len(call.ids) != 3 {
			t.Fatal("wrong Transmute binding")
		}
		for _, item := range items {
			if !slices.Contains(call.ids, item.UnitID) {
				t.Fatal("unbound ingredient")
			}
		}
		recipe, _ := LookupRecipe(items[0].Code)
		s.Items = slices.DeleteFunc(s.Items, func(i world.Item) bool { return i.Location == world.ItemLocationCube })
		*nextID++
		s.Items = append(s.Items, cubeItem(*nextID, recipe.OutputCode, 2, 3))
	case "store":
		if items := s.ItemsByLocation(world.ItemLocationCube); len(items) != 1 || items[0].UnitID != call.id {
			t.Fatal("unbound output")
		}
		changeCount(s, call.code, 1)
		s.Items = slices.DeleteFunc(s.Items, func(i world.Item) bool { return i.UnitID == call.id })
	}
}
func driveTo(t *testing.T, e *Executor, a *recipeActions, s *world.State, r Request, target executionStage) {
	t.Helper()
	nextID := uint32(100)
	for tick := 0; tick < 1000; tick++ {
		before := len(a.calls)
		result := e.Tick(*s, s.At, r, false, false)
		if result.Done {
			t.Fatalf("early terminal %+v", result)
		}
		if len(a.calls) > before {
			applyRecipeCall(t, s, a.calls[before], &nextID)
		}
		freshFrame(s)
		if e.started && e.stage == target {
			return
		}
	}
	t.Fatal("stage not reached")
}

func TestExecutorVerifiedBatchAndTerminalLatch(t *testing.T) {
	for _, tc := range []struct {
		code    string
		stock   map[string]int
		recipes int
	}{
		{"gsg", map[string]int{"gsg": 99, "glg": 99, "gpg": 0}, 77},
		{"glr", map[string]int{"glr": 99, "gpr": 98}, 1},
		{"r01", map[string]int{"r01": 99, "r02": 90}, 9},
		{"sku", map[string]int{"sku": 99, "skl": 0, "skz": 0}, 44},
	} {
		t.Run(tc.code, func(t *testing.T) {
			e, a, s, r := executorFixture(t, tc.code, tc.stock)
			nextID := uint32(100)
			progress := 0
			done := false
			for tick := 0; tick < 1000; tick++ {
				before := len(a.calls)
				result := e.Tick(s, s.At, r, false, false)
				if len(a.calls) > before+1 {
					t.Fatal("multiple actions in tick")
				}
				if result.Failure != nil {
					t.Fatal(result.Failure)
				}
				if result.Progress != nil {
					progress++
					if result.Progress.InputBefore-result.Progress.InputAfter != 3 || result.Progress.OutputAfter-result.Progress.OutputBefore != 1 {
						t.Fatal("wrong verified deltas")
					}
				}
				if result.Done {
					if !result.Success {
						t.Fatal("terminal without success")
					}
					done = true
					break
				}
				if len(a.calls) > before {
					applyRecipeCall(t, &s, a.calls[before], &nextID)
				}
				freshFrame(&s)
			}
			if !done || progress != tc.recipes || len(a.calls) != 1+tc.recipes*3 || len(s.ItemsByLocation(world.ItemLocationCube)) != 0 {
				t.Fatalf("done=%v progress=%d calls=%d", done, progress, len(a.calls))
			}
			if count, _ := s.CollectionCount(r.Code); count >= 99 {
				t.Fatal("trigger still blocked")
			}
			before := len(a.calls)
			freshFrame(&s)
			if result := e.Tick(s, s.At, r, false, false); !result.Success || result.Progress != nil || len(a.calls) != before {
				t.Fatal("terminal replayed action/progress")
			}
		})
	}
}

func TestExecutorConfirmationAndRevokeMatrix(t *testing.T) {
	for _, kind := range []string{"stale", "delayed_output", "partial_load", "partial_consumption", "foreign", "drift", "scope", "tab", "inventory", "binding", "pause_resume", "reset", "stop", "job_timeout", "input_error"} {
		t.Run(kind, func(t *testing.T) {
			e, a, s, r := executorFixture(t, "glr", map[string]int{"glr": 99, "gpr": 98})
			target := verifyWithdrawal
			if kind == "delayed_output" || kind == "partial_consumption" || kind == "pause_resume" {
				target = transmute
			}
			driveTo(t, e, a, &s, r, target)
			if target == transmute {
				result := e.Tick(s, s.At, r, false, false)
				if result.Done {
					t.Fatal(result.Failure)
				}
				freshFrame(&s)
				// Keep the old ingredients until a later frame supplies a result.
			}
			before := len(a.calls)
			switch kind {
			case "stale":
				s.Generation = e.generation
				s.Collection.Generation = s.Generation
			case "partial_load":
				s.Items = s.Items[:len(s.Items)-1]
				changeCount(&s, "glr", 1)
			case "partial_consumption":
				s.Items = s.Items[:len(s.Items)-1]
			case "foreign":
				s.Items[len(s.Items)-1].Code = "gpr"
			case "drift":
				changeCount(&s, "gpr", -1)
			case "scope":
				s.Collection.ScopeID = "other"
			case "tab":
				s.Collection.Tab = world.StorageTabRunes
			case "inventory":
				s.Items[1].GridX = 8
			case "binding":
				r.RunGeneration++
			case "reset":
				e.Reset()
				if a.resets != 1 || len(a.calls) != before {
					t.Fatal("Reset sent input")
				}
				return
			case "job_timeout":
				s.At = s.At.Add(JobTimeout)
			case "input_error":
				e.stage = withdrawIngredients
				s.Items = s.Items[:2]
				changeCount(&s, "glr", 3)
				a.err = errors.New("partial sender")
			}
			if kind == "pause_resume" {
				active := e.active
				s.At = s.At.Add(time.Minute)
				if res := e.Tick(s, s.At, r, true, false); res.Done || len(a.calls) != before {
					t.Fatal("pause consumed action")
				}
				s.At = s.At.Add(time.Minute)
				freshFrame(&s)
				if res := e.Tick(s, s.At, r, false, false); res.Done || e.active != active || len(a.calls) != before {
					t.Fatal("resume repeated action or charged pause")
				}
				id := uint32(110)
				applyRecipeCall(t, &s, a.calls[before-1], &id)
				freshFrame(&s)
				if res := e.Tick(s, s.At, r, false, false); res.Done || e.stage != storeOutput {
					t.Fatal("resume did not recheck output")
				}
				return
			}
			res := e.Tick(s, s.At, r, false, kind == "stop")
			if kind == "stale" || kind == "delayed_output" || kind == "partial_load" {
				if res.Done || len(a.calls) != before {
					t.Fatal("waiting frame generated action")
				}
				if kind == "delayed_output" {
					id := uint32(110)
					applyRecipeCall(t, &s, a.calls[before-1], &id)
					freshFrame(&s)
					if res = e.Tick(s, s.At, r, false, false); res.Done || e.stage != storeOutput || len(a.calls) != before {
						t.Fatal("late output retransmuted")
					}
					return
				}
				freshFrame(&s)
				s.At = s.At.Add(ActionTimeout)
				res = e.Tick(s, s.At, r, false, false)
			}
			if !res.Done || res.Success || res.Failure == nil {
				t.Fatalf("unsafe frame accepted: %+v", res)
			}
			if kind == "input_error" {
				before++
			}
			freshFrame(&s)
			e.Tick(s, s.At, r, false, false)
			if len(a.calls) != before {
				t.Fatal("failed action repeated")
			}
		})
	}
}

func TestExecutorPreflightRequiresEmptyCubeAndFullKnownSlot(t *testing.T) {
	for _, kind := range []string{"not_full", "missing_count", "full_target", "cursor", "cube", "overlap", "missing_trigger"} {
		t.Run(kind, func(t *testing.T) {
			e, a, s, r := executorFixture(t, "glr", map[string]int{"glr": 99, "gpr": 98})
			switch kind {
			case "not_full":
				changeCount(&s, "glr", -1)
			case "missing_count":
				s.Collection.Counts = nil
			case "full_target":
				changeCount(&s, "gpr", 1)
			case "cursor":
				s.Items = append(s.Items, world.Item{UnitID: 7, Location: world.ItemLocationCursor})
			case "cube":
				s.Items = append(s.Items, cubeItem(7, "gsr", 0, 0))
			case "overlap":
				s.Items[1].GridX = 0
				s.Items[1].GridY = 0
				r.GridX = 0
				r.GridY = 0
			case "missing_trigger":
				s.Items = s.Items[:1]
			}
			res := e.Tick(s, s.At, r, false, false)
			if !res.Done || res.Failure == nil || len(a.calls) != 0 {
				t.Fatal("unsafe preflight sent input")
			}
		})
	}
}
