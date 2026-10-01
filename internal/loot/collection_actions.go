package loot

import (
	"fmt"
	"log/slog"
	"maps"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// CollectionInput supplies atomic positioned input and its bound client state.
type CollectionInput interface {
	ClickAt(int, int, input.MouseButton) error
	ClickAtWithCtrlShift(int, int, input.MouseButton) error
	Window() (input.WindowInfo, bool)
	Status() input.Status
}

// CollectionPoint is a calibrated client-relative click target.
type CollectionPoint struct{ X, Y int }

// CollectionTabLayout contains proven targets for one material tab. Cells bind
// CASC item codes to targets; no recipe policy belongs here.
type CollectionTabLayout struct {
	Known  bool
	Select CollectionPoint
	Cells  map[string]CollectionPoint
}

// CollectionCubeLayout describes the Cube grid and transmute button embedded
// below the stash items whenever the personal inventory contains a Cube. This
// panel has no separate open/close action. The zero value blocks Cube actions.
type CollectionCubeLayout struct {
	Known                                           bool
	Left, Top, CellWidth, CellHeight, Columns, Rows int
	Transmute                                       CollectionPoint
}

// CollectionLayout is a code-owned, calibrated 1280x720 layout. Settle is the
// measured tab delay; zero blocks material input. It is not character config.
type CollectionLayout struct {
	Gems, Runes CollectionTabLayout
	Cube        CollectionCubeLayout
	Settle      time.Duration
}

// DefaultCollectionLayout returns the 1280x720 targets and tab delay proven in
// Gate 25.4. A fresh matching tab snapshot is still required after the delay.
func DefaultCollectionLayout() CollectionLayout {
	// Unmodified operator captures: docs/plans/assets/phase-25-{gems,runes}-1280x720.png.
	// Only the grades and runes used by Phase 25 have click targets.
	gemCells := make(map[string]CollectionPoint, 21)
	for _, family := range []struct {
		x     int
		codes [3]string
	}{
		{151, [3]string{"gsw", "glw", "gpw"}},
		{191, [3]string{"gsg", "glg", "gpg"}},
		{232, [3]string{"gsr", "glr", "gpr"}},
		{273, [3]string{"gsy", "gly", "gpy"}},
		{313, [3]string{"gsv", "gzv", "gpv"}},
		{354, [3]string{"gsb", "glb", "gpb"}},
		{395, [3]string{"sku", "skl", "skz"}},
	} {
		for grade, code := range family.codes {
			gemCells[code] = CollectionPoint{family.x, [3]int{229, 262, 294}[grade]}
		}
	}
	runeCells := make(map[string]CollectionPoint, 10)
	for i, x := range []int{133, 167, 203, 237, 273, 307, 342, 377, 411} {
		runeCells[fmt.Sprintf("r%02d", i+1)] = CollectionPoint{x, 175}
	}
	runeCells["r10"] = CollectionPoint{133, 210} // Ort's output, never an ingredient.
	return CollectionLayout{
		Gems:  CollectionTabLayout{Known: true, Select: CollectionPoint{269, 128}, Cells: gemCells},
		Runes: CollectionTabLayout{Known: true, Select: CollectionPoint{397, 128}, Cells: runeCells},
		Cube:  CollectionCubeLayout{Known: true, Left: 223, Top: 331, CellWidth: 32, CellHeight: 32, Columns: 3, Rows: 4, Transmute: CollectionPoint{272, 500}},
		// Operator logs d2rbot-20261001-{031742,032213}.log measured 299/298 ms.
		// Round the larger observation up to 300 ms; timing never replaces tab evidence.
		Settle: 300 * time.Millisecond,
	}
}

// CollectionActions translates independently authorized World actions into
// primitive clicks. It owns tab settle and one-action-per-generation guards,
// not recipes, retries, ingredient selection or batch progress.
type CollectionActions struct {
	log                *slog.Logger
	input              CollectionInput
	layout             CollectionLayout
	scope              string
	lastAction         uint64
	selected           world.StorageTab
	selectedAt         time.Time
	selectedGeneration uint64
}

// NewCollectionActions copies a code-owned layout; callers cannot mutate its
// code-to-cell mapping while a transfer is in progress.
func NewCollectionActions(log *slog.Logger, in CollectionInput, layout CollectionLayout) *CollectionActions {
	layout.Gems.Cells = maps.Clone(layout.Gems.Cells)
	layout.Runes.Cells = maps.Clone(layout.Runes.Cells)
	return &CollectionActions{log: log.With("component", "loot.collection"), input: in, layout: layout}
}

// Reset revokes scope, pending tab settle and generation bindings without input.
func (a *CollectionActions) Reset() {
	a.scope = ""
	a.lastAction = 0
	a.selected = ""
	a.selectedAt = time.Time{}
	a.selectedGeneration = 0
}

func (a *CollectionActions) guard(state world.State) error {
	if a == nil || a.input == nil || state.At.IsZero() || !state.CollectionAvailable() || state.Area.ID != world.RogueEncampment || !state.UI.InventoryOpen || state.UI.NPCShopOpen || state.UI.NPCInteractOpen || state.UI.QuitMenuOpen || !state.Collection.TabKnown {
		return fmt.Errorf("collection state unavailable")
	}
	// The embedded stash grid appears because the player carries a Cube. The
	// standalone UI.CubeOpen flag belongs to the Cow flow and is irrelevant here.
	cubes := 0
	for _, item := range state.InventoryItems() {
		if item.Code == "box" && item.UnitID != 0 {
			cubes++
		}
	}
	if cubes != 1 {
		return fmt.Errorf("personal inventory Cube unavailable")
	}
	if state.Collection.Tab != world.StorageTabBasic && state.Collection.Tab != world.StorageTabMaterials && state.Collection.Tab != world.StorageTabGems && state.Collection.Tab != world.StorageTabRunes {
		return fmt.Errorf("collection tab unavailable")
	}
	status := a.input.Status()
	if !status.Enabled || status.Paused || status.Stopped {
		return fmt.Errorf("collection input unavailable")
	}
	win, bound := a.input.Window()
	if !bound || win.ClientWidth != 1280 || win.ClientHeight != 720 {
		return fmt.Errorf("collection requires 1280x720")
	}
	if len(state.ItemsByLocation(world.ItemLocationCursor)) != 0 {
		return fmt.Errorf("collection cursor occupied")
	}
	if a.scope != "" && a.scope != state.Collection.ScopeID {
		return fmt.Errorf("collection scope changed")
	}
	if state.Generation <= a.lastAction {
		return fmt.Errorf("collection snapshot not newer than last input")
	}
	a.scope = state.Collection.ScopeID
	return nil
}

func (a *CollectionActions) tabLayout(tab world.StorageTab) (CollectionTabLayout, error) {
	var layout CollectionTabLayout
	switch tab {
	case world.StorageTabGems:
		layout = a.layout.Gems
	case world.StorageTabRunes:
		layout = a.layout.Runes
	default:
		return layout, fmt.Errorf("collection tab unsupported")
	}
	if !layout.Known {
		return layout, fmt.Errorf("collection tab layout uncalibrated")
	}
	return layout, nil
}

func collectionPointValid(point CollectionPoint) bool {
	return point.X >= 10 && point.X < 1270 && point.Y >= 10 && point.Y < 672
}

func (a *CollectionActions) click(state world.State, point CollectionPoint, modified bool, button input.MouseButton, action, code string) error {
	if !collectionPointValid(point) {
		return fmt.Errorf("collection target outside safe client area")
	}
	// Even a sender error may mean partial delivery. Never retry this generation.
	a.lastAction = state.Generation
	var err error
	if modified {
		err = a.input.ClickAtWithCtrlShift(point.X, point.Y, button)
	} else {
		err = a.input.ClickAt(point.X, point.Y, button)
	}
	a.log.Info("collection input action", "action", action, "item_code", code, "button", button, "scope", a.scope, "generation", state.Generation, "client_x", point.X, "client_y", point.Y, "success", err == nil, "error", err)
	if err != nil {
		return fmt.Errorf("collection %s: %w", action, err)
	}
	return nil
}

// SelectTab sends at most one calibrated tab click. A material click requires a
// later generation, matching active-tab evidence and the calibrated settle.
// Known Basic/Materials sections permit navigation only; unknown UI stays blocked.
func (a *CollectionActions) SelectTab(state world.State, tab world.StorageTab) error {
	if err := a.guard(state); err != nil {
		return err
	}
	layout, err := a.tabLayout(tab)
	if err != nil {
		return err
	}
	// A proven tab target may be selected to measure its live readiness. Unknown
	// Settle still blocks every material and transmute action in materialReady.
	if a.selected != "" {
		if a.selected != tab {
			return fmt.Errorf("collection selection changed before reset")
		}
		return nil
	}
	a.selected = tab
	a.selectedGeneration = state.Generation
	a.selectedAt = state.At
	if state.Collection.Tab == tab {
		return nil
	}
	return a.click(state, layout.Select, false, input.MouseLeft, "select_tab", string(tab))
}

func (a *CollectionActions) materialReady(state world.State, tab world.StorageTab) error {
	if err := a.guard(state); err != nil {
		return err
	}
	// Non-material tabs authorize only selection, never a Cube or item click.
	if tab != world.StorageTabGems && tab != world.StorageTabRunes {
		return fmt.Errorf("collection material tab unavailable")
	}
	if a.selected != tab || state.Collection.Tab != tab || state.Generation <= a.selectedGeneration || a.layout.Settle <= 0 || state.At.Sub(a.selectedAt) < a.layout.Settle {
		return fmt.Errorf("collection tab not settled and confirmed")
	}
	return nil
}

// TabReady reports whether fresh tab evidence and the calibrated delay permit
// material input. It checks current safety without sending an input action.
func (a *CollectionActions) TabReady(state world.State, tab world.StorageTab) bool {
	return a.materialReady(state, tab) == nil
}

// WithdrawIngredients sends one Ctrl+Shift right click into an empty Cube.
// The game chooses the quantity; the Crafting caller must verify the full CASC
// ingredient set and count delta before Transmute. There is no click fallback.
func (a *CollectionActions) WithdrawIngredients(state world.State, code string) error {
	if len(state.ItemsByLocation(world.ItemLocationCube)) != 0 {
		return fmt.Errorf("ingredient transfer requires an empty Cube")
	}
	return a.withdraw(state, code, input.MouseRight, "withdraw_ingredients")
}

// WithdrawOne transfers one unit of a bound material code directly to the Cube.
// Success means input was sent; the Crafting caller must verify the count and
// new Cube unit before another action. This method never repeats a transfer.
func (a *CollectionActions) WithdrawOne(state world.State, code string) error {
	return a.withdraw(state, code, input.MouseLeft, "withdraw_one")
}

func (a *CollectionActions) withdraw(state world.State, code string, button input.MouseButton, action string) error {
	if err := a.materialReady(state, state.Collection.Tab); err != nil {
		return err
	}
	layout, err := a.tabLayout(state.Collection.Tab)
	if err != nil {
		return err
	}
	count, known := state.CollectionCount(code)
	point, exists := layout.Cells[code]
	if !known || count < 1 || !exists || !a.layout.Cube.Known {
		return fmt.Errorf("collection material or layout unavailable")
	}
	return a.click(state, point, true, button, action, code)
}

// StoreOutput transfers a unique bound personal Cube item back to the confirmed
// material tab. The caller verifies target count +1 and an empty Cube afterward.
func (a *CollectionActions) StoreOutput(state world.State, unitID uint32, code string) error {
	if err := a.materialReady(state, state.Collection.Tab); err != nil {
		return err
	}
	if !a.layout.Cube.Known {
		return fmt.Errorf("embedded Cube layout uncalibrated")
	}
	if _, err := a.tabLayout(state.Collection.Tab); err != nil {
		return err
	}
	count, known := state.CollectionCount(code)
	if !known || count >= 99 {
		return fmt.Errorf("collection output capacity unavailable")
	}
	var item world.Item
	matches := 0
	for _, candidate := range state.ItemsByLocation(world.ItemLocationCube) {
		if candidate.UnitID == unitID && candidate.Code == code && candidate.PlayerOwned {
			item = candidate
			matches++
		}
	}
	g := a.layout.Cube
	if unitID == 0 || matches != 1 || item.Width != 1 || item.Height != 1 || g.CellWidth <= 0 || g.CellHeight <= 0 || g.Columns <= 0 || g.Rows <= 0 || item.GridX < 0 || item.GridY < 0 || item.GridX >= g.Columns || item.GridY >= g.Rows {
		return fmt.Errorf("bound Cube output unavailable")
	}
	point := CollectionPoint{X: g.Left + int(item.GridX)*g.CellWidth + g.CellWidth/2, Y: g.Top + int(item.GridY)*g.CellHeight + g.CellHeight/2}
	return a.click(state, point, true, input.MouseLeft, "store_output", code)
}

// Transmute clicks the calibrated embedded-Cube button once for an exact set
// of bound personal Cube units. Recipe eligibility and result verification are
// the Crafting caller's responsibility; this action never selects ingredients.
func (a *CollectionActions) Transmute(state world.State, units []uint32) error {
	if err := a.materialReady(state, state.Collection.Tab); err != nil {
		return err
	}
	if !a.layout.Cube.Known || len(units) == 0 {
		return fmt.Errorf("embedded Cube unavailable")
	}
	items := state.ItemsByLocation(world.ItemLocationCube)
	if len(items) != len(units) {
		return fmt.Errorf("bound Cube contents changed")
	}
	bound := make(map[uint32]bool, len(units))
	for _, id := range units {
		if id == 0 || bound[id] {
			return fmt.Errorf("cube binding invalid")
		}
		bound[id] = true
	}
	for _, item := range items {
		if !item.PlayerOwned || !bound[item.UnitID] {
			return fmt.Errorf("cube binding changed")
		}
		delete(bound, item.UnitID)
	}
	return a.click(state, a.layout.Cube.Transmute, false, input.MouseLeft, "transmute", "")
}
