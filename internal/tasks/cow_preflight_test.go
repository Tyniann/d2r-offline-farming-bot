package tasks

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestCowPreflightMatrix(t *testing.T) {
	baseConfig, baseState := validCowPreflightFixture()
	tests := []struct {
		name   string
		mutate func(*CowConfig, *world.State)
		want   string
	}{
		{name: "valid"},
		{name: "scope", mutate: func(_ *CowConfig, state *world.State) { state.Identity.CharacterName = "Other" }, want: CowReasonScopeUnsupported},
		{name: "cube missing", mutate: func(_ *CowConfig, state *world.State) { state.Items = state.Items[1:] }, want: CowReasonCubeMissing},
		{name: "cube ambiguous", mutate: func(_ *CowConfig, state *world.State) { state.Items = append(state.Items, state.Items[0]) }, want: CowReasonCubeAmbiguous},
		{name: "cube unprotected", mutate: func(cfg *CowConfig, _ *world.State) { cfg.InventoryLocked[1][1] = false }, want: CowReasonCubeUnprotected},
		{name: "cube dirty", mutate: func(_ *CowConfig, state *world.State) {
			state.Items = append(state.Items, world.Item{UnitID: 8, Code: "r01", Location: world.ItemLocationCube})
		}, want: CowReasonCubeNotEmpty},
		{name: "inventory leg", mutate: addVisibleLeg(world.ItemLocationInventory), want: CowReasonExistingLeg},
		{name: "cube leg", mutate: addVisibleLeg(world.ItemLocationCube), want: CowReasonCubeNotEmpty},
		{name: "stash leg", mutate: addVisibleLeg(world.ItemLocationStash), want: CowReasonExistingLeg},
		{name: "space", mutate: func(cfg *CowConfig, _ *world.State) {
			for row := range cfg.InventoryLocked {
				for col := range cfg.InventoryLocked[row] {
					cfg.InventoryLocked[row][col] = true
				}
			}
		}, want: CowReasonInventorySpaceMissing},
		{name: "portal tome", mutate: func(_ *CowConfig, state *world.State) { state.Items = state.Items[:1] }, want: CowReasonReturnPortalUnavailable},
		{name: "combat skill", mutate: func(cfg *CowConfig, _ *world.State) { cfg.HasCorpseExplosion = false }, want: CowReasonCombatSkillMissing},
		{name: "route capability", mutate: func(cfg *CowConfig, _ *world.State) { cfg.HasTownServices = false }, want: CowReasonCapabilityMissing},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, state := baseConfig, cloneCowState(baseState)
			if test.mutate != nil {
				test.mutate(&cfg, &state)
			}
			got, _ := evaluateCowPreflight(cfg, state, "setup", "sweep", 1280, 720)
			if got != test.want {
				t.Fatalf("reason=%q, want %q", got, test.want)
			}
		})
	}
}

func TestCowInventoryRequiresDisjointLegAndTomeRectangles(t *testing.T) {
	locked := [4][10]bool{}
	for row := range locked {
		for col := range locked[row] {
			locked[row][col] = true
		}
	}
	for row := 0; row < 3; row++ {
		locked[row][2] = false
	}
	if cowInventoryCanFitBoth(locked, nil) {
		t.Fatal("one overlapping 1x3/1x2 corridor was accepted")
	}
	for row := 0; row < 2; row++ {
		locked[row][3] = false
	}
	if !cowInventoryCanFitBoth(locked, nil) {
		t.Fatal("disjoint 1x3 and 1x2 rectangles were rejected")
	}
}

func TestCowPreflightUsesThreeFreshSnapshots(t *testing.T) {
	cfg, state := validCowPreflightFixture()
	preflight := cowPreflight{config: cfg}
	for generation := uint64(1); generation <= 2; generation++ {
		state.Generation = generation
		if done, _ := preflight.tick(state, "setup", "sweep", 1280, 720); done {
			t.Fatalf("completed on snapshot %d", generation)
		}
	}
	if done, _ := preflight.tick(state, "setup", "sweep", 1280, 720); done {
		t.Fatal("duplicate generation advanced stability")
	}
	state.Generation = 3
	if done, reason := preflight.tick(state, "setup", "sweep", 1280, 720); !done || reason != "" {
		t.Fatalf("third fresh snapshot done=%t reason=%q", done, reason)
	}
}

func TestCowPreflightFailureSendsNoAutomaticInput(t *testing.T) {
	inputs := &cowInputCounter{}
	profileActions := &cowProfileCounter{}
	cowActions := &cowActionCounter{}
	runner := NewRunner(slog.Default(), RunSelection{Run: string(RunIDCows)}, RunConfig{
		StepTimeout: time.Second, SetupRouteID: "setup", RouteID: "sweep",
		Cow: CowConfig{Character: "MrBones", Difficulty: "hell", ClientWidth: 1280, ClientHeight: 720},
	}, Deps{Input: inputs, Profile: profileActions, Actions: inputs, Cow: cowActions})
	state := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), Identity: world.GameIdentity{Valid: true, CharacterName: "Wrong", Class: world.CharacterClassNecromancer}}
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for generation := uint64(1); generation <= 3; generation++ {
		state.Generation = generation
		result := runner.Tick(context.Background(), state, base.Add(time.Duration(generation)*time.Millisecond))
		if generation == 3 && (result.Outcome != RunOutcomeFailed || result.Reason != CowReasonScopeUnsupported) {
			t.Fatalf("terminal result=%+v", result)
		}
	}
	if inputs.calls != 0 || profileActions.calls != 0 || cowActions.calls != 0 {
		t.Fatalf("preflight input calls: input=%d profile=%d cow=%d", inputs.calls, profileActions.calls, cowActions.calls)
	}
}

func validCowPreflightFixture() (CowConfig, world.State) {
	cfg := CowConfig{
		Character: "MrBones", Difficulty: "hell", ClientWidth: 1280, ClientHeight: 720,
		HasTownPortal: true, HasTeleport: true, HasAmplifyDamage: true, HasCorpseExplosion: true, HasBoneSpear: true, HasTownServices: true,
	}
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			cfg.InventoryLocked[row][col] = true
		}
	}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
		Items: []world.Item{
			{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 0, GridY: 0, Width: 2, Height: 2},
			{UnitID: 2, Code: "tbk", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 9, GridY: 0, Width: 1, Height: 2},
		},
	}
	return cfg, state
}

func addVisibleLeg(location world.ItemLocation) func(*CowConfig, *world.State) {
	return func(_ *CowConfig, state *world.State) {
		state.Items = append(state.Items, world.Item{UnitID: 9, Code: "leg", Location: location, PlayerOwned: true, Page: 0, GridX: 5, GridY: 0, Width: 1, Height: 3})
	}
}

func cloneCowState(state world.State) world.State {
	state.Items = append([]world.Item(nil), state.Items...)
	return state
}

type cowInputCounter struct{ calls int }

func (c *cowInputCounter) Status() input.Status { return input.Status{Enabled: true} }
func (c *cowInputCounter) Bound() bool          { return true }
func (c *cowInputCounter) Window() (input.WindowInfo, bool) {
	return input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}, true
}
func (c *cowInputCounter) CastBelt(int) error    { c.calls++; return nil }
func (c *cowInputCounter) CastTownPortal(time.Time, world.Player) error { c.calls++; return nil }

type cowProfileCounter struct {
	calls  int
	result profile.Result
}

func (c *cowProfileCounter) TickHook(context.Context, profile.Hook, world.State, profile.EncounterTarget, time.Time) profile.Result {
	c.calls++
	return c.result
}
func (c *cowProfileCounter) TickResources(world.State, profile.ResourceContext, time.Time) profile.Result {
	c.calls++
	return profile.Result{}
}
func (c *cowProfileCounter) TickRouteMaintenance(world.State, time.Time) profile.Result {
	return profile.Result{Status: profile.StatusComplete}
}
func (c *cowProfileCounter) Reset() {}

type cowActionCounter struct{ calls int }

func (c *cowActionCounter) TickWirt(context.Context, world.State) CowSetupActionResult {
	c.calls++
	return CowSetupActionResult{}
}
func (c *cowActionCounter) TickTome(context.Context, world.State) CowSetupActionResult {
	c.calls++
	return CowSetupActionResult{}
}
func (c *cowActionCounter) Reset() {}
