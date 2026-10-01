package crafting

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// ActionTimeout bounds one outstanding confirmation in active time.
	ActionTimeout = 3 * time.Second
	// JobTimeout bounds the complete compaction job in active time.
	JobTimeout = 240 * time.Second
)

// Request freezes a previously failed, unlocked Keep-item for one run generation.
// The caller owns Pickit/lock authorization. The executor preserves its inventory
// identity and position; it never consumes this item as a recipe ingredient.
type Request struct {
	UnitID        uint32
	Code          string
	GridX, GridY  int
	RunGeneration uint64
}

// Actions is the primitive collection surface. A successful method means input
// was sent, never that its effect was confirmed. TabReady sends no input; Reset
// clears software bindings. WithdrawIngredients is one Ctrl+Shift right click.
type Actions interface {
	SelectTab(world.State, world.StorageTab) error
	TabReady(world.State, world.StorageTab) bool
	WithdrawIngredients(world.State, string) error
	Transmute(world.State, []uint32) error
	StoreOutput(world.State, uint32, string) error
	Reset()
}

// Progress records one fully verified recipe after output storage, never a drop.
type Progress struct {
	RecipeIndex                                        int
	InputCode, OutputCode                              string
	InputBefore, InputAfter, OutputBefore, OutputAfter int
}

// Result reports pending work, one verified progress event, or terminal success
// or failure. Terminal results remain latched until Reset and repeat no progress.
type Result struct {
	Done, Success bool
	Failure       *Failure
	Progress      *Progress
}

type executionStage uint8

const (
	selectTab executionStage = iota
	waitTab
	withdrawIngredients
	verifyWithdrawal
	transmute
	verifyResult
	storeOutput
	verifyStore
	finish
)

type inventoryBinding struct {
	id         uint32
	code       string
	x, y, w, h int
}

// Executor owns one finite, non-retrying collection job. Its stages are tab
// selection/confirmation, ingredient load/confirmation, Transmute/result,
// output storage/confirmation and finish. Bindings and one-send stages survive
// pause; each transition requires a newer frame. Reset never moves game items.
type Executor struct {
	log                 *slog.Logger
	actions             Actions
	started             bool
	request             Request
	scope               string
	plan                Plan
	index               int
	stage               executionStage
	inventory           []inventoryBinding
	ingredients         []uint32
	output              uint32
	beforeUnits         map[uint32]bool
	generation          uint64
	observedAt          time.Time
	lastTime            time.Time
	wasPaused           bool
	active, stageActive time.Duration
	terminal            Result
	testCode            string
}

// NewExecutor creates the storage-only executor without sending input.
func NewExecutor(log *slog.Logger, actions Actions) (*Executor, error) {
	if log == nil || actions == nil {
		return nil, fmt.Errorf("crafting logger and actions are required")
	}
	return &Executor{log: log.With("component", "crafting"), actions: actions}, nil
}

// Reset revokes the job and action bindings without clearing any Cube item.
func (e *Executor) Reset() {
	if e == nil {
		return
	}
	log, actions, testCode := e.log, e.actions, e.testCode
	*e = Executor{log: log, actions: actions, testCode: testCode}
	if actions != nil {
		actions.Reset()
	}
}

// Tick sends at most one primitive action and verifies only fresh World frames.
// Call it while paused as well: paused intervals consume neither budget, and
// resume rechecks bindings. Stopped jobs latch failure without cleanup input.
func (e *Executor) Tick(s world.State, now time.Time, request Request, paused, stopped bool) Result {
	if e.terminal.Done {
		return e.terminal
	}
	if !e.started {
		e.request = request
	}
	if now.IsZero() {
		now = s.At
	}
	if !e.lastTime.IsZero() && !paused && !e.wasPaused && now.After(e.lastTime) {
		e.active += now.Sub(e.lastTime)
	}
	e.lastTime, e.wasPaused = now, paused
	if stopped {
		return e.fail(ReasonUnconfirmed, request.Code, nil)
	}
	if paused {
		return Result{}
	}
	if e.started && e.request != request {
		return e.fail(ReasonUnconfirmed, request.Code, nil)
	}
	if e.active >= JobTimeout {
		return e.fail(ReasonTimeout, request.Code, nil)
	}
	if e.started && e.active-e.stageActive >= ActionTimeout {
		return e.fail(ReasonUnconfirmed, request.Code, nil)
	}
	if !s.Valid || s.Generation == 0 || s.At.IsZero() {
		if !e.started {
			return e.fail(ReasonUnavailable, request.Code, nil)
		}
		return Result{}
	}
	if !e.started {
		if f := e.start(s, request); f != nil {
			e.terminal = Result{Done: true, Failure: f}
			return e.terminal
		}
	} else {
		if s.Generation <= e.generation || !s.At.After(e.observedAt) {
			return Result{}
		}
		if s.Phase != world.GamePhaseInGame || s.Area.ID != world.RogueEncampment || !s.UI.StashOpen || !s.UI.InventoryOpen || s.UI.NPCShopOpen || s.UI.NPCInteractOpen || s.UI.QuitMenuOpen {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
		if s.Collection.ScopeKnown && s.Collection.ScopeID != e.scope {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
		current, ok := bindInventory(s)
		if !ok || !slices.Equal(current, e.inventory) || len(s.ItemsByLocation(world.ItemLocationCursor)) != 0 {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
	}
	if !s.CollectionAvailable() || !s.Collection.TabKnown {
		return Result{}
	}
	if s.Collection.ScopeID != e.scope {
		return e.fail(ReasonUnconfirmed, request.Code, nil)
	}
	// Wrong known tabs are tolerated only while the single selection is pending.
	if e.stage > waitTab && s.Collection.Tab != e.plan.Tab {
		return e.fail(ReasonUnconfirmed, request.Code, nil)
	}
	e.generation, e.observedAt = s.Generation, s.At
	step := PlannedRecipe{}
	if e.index < len(e.plan.Steps) {
		step = e.plan.Steps[e.index]
	}
	counts, known := e.counts(s)
	if !known {
		return Result{}
	}
	items := s.ItemsByLocation(world.ItemLocationCube)
	switch e.stage {
	case selectTab:
		if !maps.Equal(counts, step.Before) || len(items) != 0 {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
		e.advance(waitTab)
		if err := e.actions.SelectTab(s, e.plan.Tab); err != nil {
			return e.fail(ReasonUnconfirmed, request.Code, err)
		}
	case waitTab:
		if !maps.Equal(counts, step.Before) || len(items) != 0 {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
		if e.actions.TabReady(s, e.plan.Tab) {
			e.advance(withdrawIngredients)
		}
	case withdrawIngredients:
		if !maps.Equal(counts, step.Before) || len(items) != 0 || !e.actions.TabReady(s, e.plan.Tab) {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
		e.beforeUnits = unitSet(s.Items)
		// Advance before calling input: even a partial sender failure cannot retry.
		e.advance(verifyWithdrawal)
		if err := e.actions.WithdrawIngredients(s, step.Recipe.InputCode); err != nil {
			return e.fail(ReasonUnconfirmed, step.Recipe.InputCode, err)
		}
	case verifyWithdrawal:
		expected := maps.Clone(step.Before)
		expected[step.Recipe.InputCode] = counts[step.Recipe.InputCode]
		delta := step.Before[step.Recipe.InputCode] - counts[step.Recipe.InputCode]
		if !maps.Equal(counts, expected) || delta < 0 || delta > step.Recipe.InputQuantity || len(items) > step.Recipe.InputQuantity || !validCubeSet(items, step.Recipe.InputCode, e.beforeUnits) {
			return e.fail(ReasonUnconfirmed, step.Recipe.InputCode, nil)
		}
		// A read may observe only part of the one-click transfer. Wait for the
		// complete count AND set without topping it up or sending Transmute.
		if delta == step.Recipe.InputQuantity && len(items) == step.Recipe.InputQuantity {
			e.ingredients = nil
			for _, item := range items {
				e.ingredients = append(e.ingredients, item.UnitID)
			}
			e.advance(transmute)
		}
	case transmute:
		expected := maps.Clone(step.Before)
		expected[step.Recipe.InputCode] -= step.Recipe.InputQuantity
		if !maps.Equal(counts, expected) || !boundIngredients(items, e.ingredients, step.Recipe.InputCode) {
			return e.fail(ReasonUnconfirmed, step.Recipe.InputCode, nil)
		}
		e.beforeUnits = unitSet(s.Items)
		e.advance(verifyResult)
		if err := e.actions.Transmute(s, slices.Clone(e.ingredients)); err != nil {
			return e.fail(ReasonUnconfirmed, step.Recipe.InputCode, err)
		}
	case verifyResult:
		expected := maps.Clone(step.Before)
		expected[step.Recipe.InputCode] -= step.Recipe.InputQuantity
		if !maps.Equal(counts, expected) {
			return e.fail(ReasonUnconfirmed, step.Recipe.InputCode, nil)
		}
		if boundIngredients(items, e.ingredients, step.Recipe.InputCode) || len(items) == 0 {
			return Result{}
		}
		if len(items) != 1 || !validCubeSet(items, step.Recipe.OutputCode, e.beforeUnits) {
			return e.fail(ReasonUnconfirmed, step.Recipe.OutputCode, nil)
		}
		e.output = items[0].UnitID
		e.advance(storeOutput)
	case storeOutput:
		expected := maps.Clone(step.Before)
		expected[step.Recipe.InputCode] -= step.Recipe.InputQuantity
		if !maps.Equal(counts, expected) || len(items) != 1 || items[0].UnitID != e.output || !validCubeSet(items, step.Recipe.OutputCode, e.beforeUnits) {
			return e.fail(ReasonUnconfirmed, step.Recipe.OutputCode, nil)
		}
		e.advance(verifyStore)
		if err := e.actions.StoreOutput(s, e.output, step.Recipe.OutputCode); err != nil {
			return e.fail(ReasonUnconfirmed, step.Recipe.OutputCode, err)
		}
	case verifyStore:
		expected := maps.Clone(step.After)
		expected[step.Recipe.OutputCode] = counts[step.Recipe.OutputCode]
		delta := counts[step.Recipe.OutputCode] - step.Before[step.Recipe.OutputCode]
		if !maps.Equal(counts, expected) || delta < 0 || delta > 1 || len(items) > 1 || (len(items) == 1 && (items[0].UnitID != e.output || !validCubeSet(items, step.Recipe.OutputCode, e.beforeUnits))) {
			return e.fail(ReasonUnconfirmed, step.Recipe.OutputCode, nil)
		}
		if delta == 1 && len(items) == 0 {
			progress := &Progress{RecipeIndex: e.index, InputCode: step.Recipe.InputCode, OutputCode: step.Recipe.OutputCode, InputBefore: step.Before[step.Recipe.InputCode], InputAfter: step.After[step.Recipe.InputCode], OutputBefore: step.Before[step.Recipe.OutputCode], OutputAfter: step.After[step.Recipe.OutputCode]}
			e.log.Info("storage recipe verified", "recipe_index", e.index, "input_code", progress.InputCode, "output_code", progress.OutputCode, "input_before", progress.InputBefore, "input_after", progress.InputAfter, "output_before", progress.OutputBefore, "output_after", progress.OutputAfter)
			e.index++
			e.ingredients = nil
			e.output = 0
			e.beforeUnits = nil
			if e.index == len(e.plan.Steps) {
				e.advance(finish)
			} else {
				e.advance(withdrawIngredients)
			}
			return Result{Progress: progress}
		}
	case finish:
		if len(items) != 0 || !maps.Equal(counts, e.plan.Final) || counts[request.Code] >= 99 {
			return e.fail(ReasonUnconfirmed, request.Code, nil)
		}
		e.terminal = Result{Done: true, Success: true}
		e.log.Info("storage compaction verified", "trigger_code", request.Code, "trigger_unit_id", request.UnitID, "recipes", e.index, "scope", e.scope)
		return e.terminal
	}
	return Result{}
}

func (e *Executor) start(s world.State, r Request) *Failure {
	e.request = r
	diagnostic := e.testCode != ""
	// Only the explicitly constructed one-recipe diagnostic may omit an
	// inventory trigger. Productive executors retain the full-slot binding.
	if (!diagnostic && r.UnitID == 0) || (diagnostic && (r.UnitID != 0 || r.Code != e.testCode)) || r.Code == "" || r.RunGeneration == 0 || !s.CollectionAvailable() || !s.Collection.TabKnown || s.Area.ID != world.RogueEncampment || !s.UI.InventoryOpen || s.UI.NPCShopOpen || s.UI.NPCInteractOpen || s.UI.QuitMenuOpen {
		return &Failure{Reason: ReasonUnavailable, TriggerCode: r.Code, MaterialCode: r.Code}
	}
	if len(s.ItemsByLocation(world.ItemLocationCube, world.ItemLocationCursor)) != 0 {
		return &Failure{Reason: ReasonCubeNotEmpty, TriggerCode: r.Code, MaterialCode: r.Code}
	}
	inv, ok := bindInventory(s)
	trigger, cubes := diagnostic, 0
	for _, item := range inv {
		if item.code == "box" && item.w == 2 && item.h == 2 {
			cubes++
		}
		if item.id == r.UnitID && item.code == r.Code && item.x == r.GridX && item.y == r.GridY && item.w == 1 && item.h == 1 {
			trigger = true
		}
	}
	if !ok || !trigger || cubes != 1 {
		return &Failure{Reason: ReasonUnavailable, TriggerCode: r.Code, MaterialCode: r.Code}
	}
	count, known := s.CollectionCount(r.Code)
	if !known || (!diagnostic && count != 99) || (diagnostic && count < 3) {
		return &Failure{Reason: ReasonUnavailable, TriggerCode: r.Code, MaterialCode: r.Code}
	}
	var plan Plan
	var err error
	if diagnostic {
		plan, err = buildRecipeTestPlan(s, r.Code)
	} else {
		plan, err = BuildPlan(s, r.Code)
	}
	if err != nil {
		var f *Failure
		if errors.As(err, &f) {
			return f
		}
		return &Failure{Reason: ReasonUnavailable, TriggerCode: r.Code, MaterialCode: r.Code, Cause: err}
	}
	e.plan, e.scope, e.inventory, e.started = plan, s.Collection.ScopeID, inv, true
	e.active, e.stageActive = 0, 0
	return nil
}

func (e *Executor) counts(s world.State) (map[string]int, bool) {
	counts := make(map[string]int, len(e.plan.Initial))
	for code := range e.plan.Initial {
		n, ok := s.CollectionCount(code)
		if !ok {
			return nil, false
		}
		counts[code] = n
	}
	return counts, true
}
func (e *Executor) advance(stage executionStage) { e.stage = stage; e.stageActive = e.active }
func (e *Executor) fail(reason, material string, cause error) Result {
	e.terminal = Result{Done: true, Failure: &Failure{Reason: reason, TriggerCode: e.request.Code, MaterialCode: material, Cause: cause}}
	e.log.Error("storage compaction stopped", "reason", reason, "trigger_code", e.request.Code, "material_code", material, "recipe_index", e.index, "error", cause)
	return e.terminal
}
func unitSet(items []world.Item) map[uint32]bool {
	ids := make(map[uint32]bool, len(items))
	for _, item := range items {
		ids[item.UnitID] = true
	}
	return ids
}
func validCubeSet(items []world.Item, code string, old map[uint32]bool) bool {
	ids := make(map[uint32]bool, len(items))
	cells := make(map[[2]int]bool, len(items))
	for _, item := range items {
		cell := [2]int{item.GridX, item.GridY}
		if item.UnitID == 0 || ids[item.UnitID] || old[item.UnitID] || cells[cell] || !item.PlayerOwned || item.Code != code || item.Width != 1 || item.Height != 1 || item.GridX < 0 || item.GridX >= 3 || item.GridY < 0 || item.GridY >= 4 {
			return false
		}
		ids[item.UnitID] = true
		cells[cell] = true
	}
	return true
}
func boundIngredients(items []world.Item, ids []uint32, code string) bool {
	if len(items) != 3 || len(ids) != 3 || !validCubeSet(items, code, nil) {
		return false
	}
	for _, item := range items {
		if !slices.Contains(ids, item.UnitID) {
			return false
		}
	}
	return true
}
func bindInventory(s world.State) ([]inventoryBinding, bool) {
	items := s.InventoryItems()
	result := make([]inventoryBinding, 0, len(items))
	occupied := [4][10]bool{}
	ids := make(map[uint32]bool, len(items))
	for _, item := range items {
		if item.UnitID == 0 || ids[item.UnitID] || !item.PlayerOwned || item.Width <= 0 || item.Height <= 0 || item.GridX < 0 || item.GridY < 0 || item.GridX+item.Width > 10 || item.GridY+item.Height > 4 {
			return nil, false
		}
		ids[item.UnitID] = true
		for y := item.GridY; y < item.GridY+item.Height; y++ {
			for x := item.GridX; x < item.GridX+item.Width; x++ {
				if occupied[y][x] {
					return nil, false
				}
				occupied[y][x] = true
			}
		}
		result = append(result, inventoryBinding{item.UnitID, item.Code, item.GridX, item.GridY, item.Width, item.Height})
	}
	slices.SortFunc(result, func(a, b inventoryBinding) int {
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	return result, true
}
