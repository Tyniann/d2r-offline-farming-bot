package loot

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	cowRecipeClientWidth     = 1280
	cowRecipeClientHeight    = 720
	cowPortalStableSnapshots = 3
)

type cowRecipeStage string

const (
	cowRecipeVerifyItems   cowRecipeStage = "verify_personal_items"
	cowRecipeOpenInventory cowRecipeStage = "open_inventory"
	cowRecipeOpenCube      cowRecipeStage = "right_click_bound_cube"
	cowRecipeWaitCube      cowRecipeStage = "wait_cube_open_memory_gate"
	cowRecipeTransferLeg   cowRecipeStage = "transfer_bound_leg"
	cowRecipeVerifyLeg     cowRecipeStage = "verify_leg_in_cube"
	cowRecipeTransferTome  cowRecipeStage = "transfer_bound_new_tome"
	cowRecipeVerifyTome    cowRecipeStage = "verify_new_tome_in_cube"
	cowRecipeVerifyContent cowRecipeStage = "verify_exact_cube_contents"
	cowRecipeTransmute     cowRecipeStage = "click_transmute_1280x720"
	cowRecipeWaitResult    cowRecipeStage = "wait_fresh_recipe_result"
	cowRecipeWaitPortal    cowRecipeStage = "verify_new_stable_permanent_portal"
	cowRecipeCloseUI       cowRecipeStage = "close_cube_inventory"
	cowRecipeEnterPortal   cowRecipeStage = "enter_bound_portal"
	cowRecipeVerifyArea    cowRecipeStage = "verify_area_39"
)

// CowPortalRecipeConfig configures finite verification windows and the existing
// fixed 1280x720 inventory layout. It deliberately has no retry count.
type CowPortalRecipeConfig struct {
	CubeOpenTimeout time.Duration
	TransferTimeout time.Duration
	ResultTimeout   time.Duration
	PortalTimeout   time.Duration
	CloseTimeout    time.Duration
	EntryTimeout    time.Duration
	InventoryLeft   int
	InventoryTop    int
	InventoryCellW  int
	InventoryCellH  int
	TransmuteX      int
	TransmuteY      int
}

// Validate rejects settings that could make the fixed-coordinate recipe unsafe.
func (c CowPortalRecipeConfig) Validate() error {
	if c.CubeOpenTimeout <= 0 || c.TransferTimeout <= 0 || c.ResultTimeout <= 0 || c.PortalTimeout <= 0 || c.CloseTimeout <= 0 || c.EntryTimeout <= 0 {
		return fmt.Errorf("cow portal recipe timeouts must be > 0")
	}
	if c.InventoryLeft < 0 || c.InventoryTop < 0 || c.InventoryCellW <= 0 || c.InventoryCellH <= 0 {
		return fmt.Errorf("cow portal recipe inventory geometry is invalid")
	}
	if c.TransmuteX < 0 || c.TransmuteX >= cowRecipeClientWidth || c.TransmuteY < 0 || c.TransmuteY >= cowRecipeClientHeight {
		return fmt.Errorf("cow portal recipe transmute coordinate is outside 1280x720")
	}
	return nil
}

// CowPortalBinding contains the three item identities frozen by the Cow setup
// pipeline. A binding may never change while the executor is active.
type CowPortalBinding struct {
	LegUnitID  uint32
	TomeUnitID uint32
	CubeUnitID uint32
}

// CowPortalClickResult reports one bounded hover-confirmed permanent-portal tick.
type CowPortalClickResult struct {
	Clicked bool
	Done    bool
	Reason  string
}

// CowPortalRecipeInput is the atomic UI and hover-confirmed portal surface used
// by [CowPortalRecipe].
type CowPortalRecipeInput interface {
	Window() (input.WindowInfo, bool)
	Focus() error
	MoveTo(clientX, clientY int) error
	Click(button input.MouseButton) error
	ClickWithModifier(modifier string, button input.MouseButton) error
	PressKey(key string) error
	TickPermanentPortal(world.State, world.Object) (CowPortalClickResult, error)
	ResetPermanentPortal()
}

// CowPortalRecipeResult reports completion or a stable terminal recipe reason.
type CowPortalRecipeResult struct {
	Done         bool
	Reason       string
	PortalUnitID uint32
	ProgressKind string
}

// CowPortalRecipe executes only Wirt's Leg plus a newly bought Town Portal Tome.
// Every irreversible transition is Memory-gated and Transmute can be clicked at
// most once for one executor generation.
type CowPortalRecipe struct {
	log   *slog.Logger
	input CowPortalRecipeInput
	cfg   CowPortalRecipeConfig

	stage           cowRecipeStage
	stageStartedAt  time.Time
	stageGeneration uint64
	binding         CowPortalBinding
	initialPortals  map[uint32]bool
	transmuteSent   bool
	transmuteAt     time.Time
	portalUnitID    uint32
	portalPosition  world.Position
	portalStable    int
	closeInputs     int
	inventoryInput  bool
}

// NewCowPortalRecipe creates the narrow fail-closed Cow recipe executor.
func NewCowPortalRecipe(log *slog.Logger, in CowPortalRecipeInput, cfg CowPortalRecipeConfig) (*CowPortalRecipe, error) {
	if log == nil || in == nil {
		return nil, fmt.Errorf("cow portal recipe logger and input are required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	executor := &CowPortalRecipe{log: log.With("component", "loot.cow_portal_recipe"), input: in, cfg: cfg}
	executor.Reset()
	return executor, nil
}

// Reset clears all item and portal bindings without sending input.
func (e *CowPortalRecipe) Reset() {
	if e == nil {
		return
	}
	e.stage = cowRecipeVerifyItems
	e.stageStartedAt = time.Time{}
	e.stageGeneration = 0
	e.binding = CowPortalBinding{}
	e.initialPortals = nil
	e.transmuteSent = false
	e.transmuteAt = time.Time{}
	e.portalUnitID = 0
	e.portalPosition = world.Position{}
	e.portalStable = 0
	e.closeInputs = 0
	e.inventoryInput = false
	if e.input != nil {
		e.input.ResetPermanentPortal()
	}
}

// Tick advances at most one recipe action. It never retries a transfer or the
// Transmute click; later snapshots alone decide success or terminal failure.
func (e *CowPortalRecipe) Tick(state world.State, now time.Time, binding CowPortalBinding) CowPortalRecipeResult {
	if e == nil || e.input == nil || !state.Valid || state.Phase != world.GamePhaseInGame {
		return CowPortalRecipeResult{}
	}
	if now.IsZero() {
		now = state.At
	}
	if now.IsZero() {
		now = time.Now()
	}
	if e.binding == (CowPortalBinding{}) {
		if binding.LegUnitID == 0 || binding.TomeUnitID == 0 || binding.CubeUnitID == 0 {
			return e.fail("cow_recipe_binding_invalid")
		}
		e.binding = binding
	} else if e.binding != binding {
		return e.fail("cow_recipe_binding_changed")
	}
	if e.stageStartedAt.IsZero() {
		e.stageStartedAt, e.stageGeneration = now, state.Generation
	}

	switch e.stage {
	case cowRecipeVerifyItems:
		return e.tickVerifyItems(state, now)
	case cowRecipeOpenInventory:
		if state.UI.InventoryOpen {
			e.advance(cowRecipeOpenCube, now, state.Generation)
			return CowPortalRecipeResult{}
		}
		if e.expired(now, e.cfg.CubeOpenTimeout) {
			return e.fail("cow_inventory_open_failed")
		}
		if !e.inventoryInput {
			if err := e.input.PressKey("i"); err != nil {
				return e.fail("cow_inventory_open_input_failed")
			}
			e.inventoryInput = true
		}
		return CowPortalRecipeResult{}
	case cowRecipeOpenCube:
		if !state.UI.InventoryOpen {
			return e.fail("cow_inventory_closed_before_cube")
		}
		cube, reason := e.boundInventoryItem(state, e.binding.CubeUnitID, "box")
		if reason != "" {
			return e.fail(reason)
		}
		if err := e.moveInventoryItem(cube); err != nil {
			return e.fail("cow_cube_open_input_failed")
		}
		if err := e.input.Click(input.MouseRight); err != nil {
			return e.fail("cow_cube_open_input_failed")
		}
		e.logStage(cowRecipeOpenCube, "cube_unit_id", cube.UnitID)
		e.advance(cowRecipeWaitCube, now, state.Generation)
		return CowPortalRecipeResult{}
	case cowRecipeWaitCube:
		if !state.UI.CubeOpenKnown {
			return e.fail("cow_cube_ui_unavailable")
		}
		if state.UI.CubeOpen {
			e.advance(cowRecipeTransferLeg, now, state.Generation)
			return CowPortalRecipeResult{}
		}
		if e.expired(now, e.cfg.CubeOpenTimeout) {
			return e.fail("cow_cube_open_unconfirmed")
		}
		return CowPortalRecipeResult{}
	case cowRecipeTransferLeg:
		return e.transferBoundItem(state, now, e.binding.LegUnitID, "leg", cowRecipeVerifyLeg)
	case cowRecipeVerifyLeg:
		return e.verifyTransfer(state, now, e.binding.LegUnitID, "leg", cowRecipeTransferTome)
	case cowRecipeTransferTome:
		return e.transferBoundItem(state, now, e.binding.TomeUnitID, "tbk", cowRecipeVerifyTome)
	case cowRecipeVerifyTome:
		return e.verifyTransfer(state, now, e.binding.TomeUnitID, "tbk", cowRecipeVerifyContent)
	case cowRecipeVerifyContent:
		if state.Generation <= e.stageGeneration {
			return CowPortalRecipeResult{}
		}
		if reason := e.verifyExactContents(state); reason != "" {
			return e.fail(reason)
		}
		e.logStage(cowRecipeVerifyContent, "leg_unit_id", e.binding.LegUnitID, "tome_unit_id", e.binding.TomeUnitID)
		e.advance(cowRecipeTransmute, now, state.Generation)
		return CowPortalRecipeResult{ProgressKind: "ingredients_confirmed"}
	case cowRecipeTransmute:
		return e.tickTransmute(state, now)
	case cowRecipeWaitResult:
		return e.tickResult(state, now)
	case cowRecipeWaitPortal:
		return e.tickPortal(state, now)
	case cowRecipeCloseUI:
		return e.tickCloseUI(state, now)
	case cowRecipeEnterPortal:
		return e.tickEnterPortal(state, now)
	case cowRecipeVerifyArea:
		if state.Area.ID == world.MooMooFarm {
			e.logStage(cowRecipeVerifyArea, "portal_unit_id", e.portalUnitID)
			return CowPortalRecipeResult{Done: true, PortalUnitID: e.portalUnitID, ProgressKind: "area_39_confirmed"}
		}
		if state.Area.ID != world.RogueEncampment || e.expired(now, e.cfg.EntryTimeout) {
			return e.fail("cow_portal_entry_unconfirmed")
		}
		return CowPortalRecipeResult{}
	default:
		return e.fail("cow_recipe_state_invalid")
	}
}

func (e *CowPortalRecipe) tickVerifyItems(state world.State, now time.Time) CowPortalRecipeResult {
	if state.Area.ID != world.RogueEncampment || len(state.ItemsByLocation(world.ItemLocationCube)) != 0 {
		return e.fail("cow_recipe_initial_state_invalid")
	}
	for unitID, code := range map[uint32]string{e.binding.CubeUnitID: "box", e.binding.LegUnitID: "leg", e.binding.TomeUnitID: "tbk"} {
		if _, reason := e.boundInventoryItem(state, unitID, code); reason != "" {
			return e.fail(reason)
		}
	}
	e.initialPortals = make(map[uint32]bool)
	for _, object := range state.Objects {
		if object.Kind == world.ObjectKindPermanentPortal {
			e.initialPortals[object.UnitID] = true
		}
	}
	e.logStage(cowRecipeVerifyItems, "leg_unit_id", e.binding.LegUnitID, "tome_unit_id", e.binding.TomeUnitID, "cube_unit_id", e.binding.CubeUnitID)
	e.advance(cowRecipeOpenInventory, now, state.Generation)
	return CowPortalRecipeResult{}
}

func (e *CowPortalRecipe) transferBoundItem(state world.State, now time.Time, unitID uint32, code string, next cowRecipeStage) CowPortalRecipeResult {
	if !e.cubeOpen(state) {
		return e.fail("cow_cube_ui_revoked")
	}
	item, reason := e.boundInventoryItem(state, unitID, code)
	if reason != "" {
		return e.fail(reason)
	}
	if err := e.moveInventoryItem(item); err != nil {
		return e.fail("cow_recipe_transfer_input_failed")
	}
	if err := e.input.ClickWithModifier("ctrl", input.MouseLeft); err != nil {
		return e.fail("cow_recipe_transfer_input_failed")
	}
	e.logStage(e.stage, "unit_id", unitID, "code", code)
	e.advance(next, now, state.Generation)
	return CowPortalRecipeResult{}
}

func (e *CowPortalRecipe) verifyTransfer(state world.State, now time.Time, unitID uint32, code string, next cowRecipeStage) CowPortalRecipeResult {
	if !e.cubeOpen(state) {
		return e.fail("cow_cube_ui_revoked")
	}
	item, ok := state.FindItemByUnitID(unitID)
	if ok && item.Code == code && item.Location == world.ItemLocationCube {
		e.advance(next, now, state.Generation)
		return CowPortalRecipeResult{}
	}
	if !ok || item.Code != code || item.Location != world.ItemLocationInventory {
		return e.fail("cow_recipe_transfer_state_invalid")
	}
	if e.expired(now, e.cfg.TransferTimeout) {
		return e.fail("cow_recipe_transfer_unconfirmed")
	}
	return CowPortalRecipeResult{}
}

func (e *CowPortalRecipe) verifyExactContents(state world.State) string {
	if !e.cubeOpen(state) {
		return "cow_cube_ui_revoked"
	}
	items := state.ItemsByLocation(world.ItemLocationCube)
	if len(items) != 2 {
		return "cow_recipe_contents_invalid"
	}
	want := map[uint32]string{e.binding.LegUnitID: "leg", e.binding.TomeUnitID: "tbk"}
	for _, item := range items {
		if want[item.UnitID] != item.Code {
			return "cow_recipe_contents_invalid"
		}
		delete(want, item.UnitID)
	}
	if len(want) != 0 {
		return "cow_recipe_contents_invalid"
	}
	return ""
}

func (e *CowPortalRecipe) tickTransmute(state world.State, now time.Time) CowPortalRecipeResult {
	if e.transmuteSent {
		return e.fail("cow_transmute_repeat_blocked")
	}
	if state.Generation <= e.stageGeneration {
		return CowPortalRecipeResult{}
	}
	if reason := e.verifyExactContents(state); reason != "" {
		return e.fail(reason)
	}
	win, ok := e.input.Window()
	if !ok || win.ClientWidth != cowRecipeClientWidth || win.ClientHeight != cowRecipeClientHeight {
		return e.fail("cow_recipe_resolution_invalid")
	}
	if err := e.input.Focus(); err != nil {
		return e.fail("cow_transmute_focus_failed")
	}
	if err := e.input.MoveTo(e.cfg.TransmuteX, e.cfg.TransmuteY); err != nil {
		return e.fail("cow_transmute_input_failed")
	}
	if err := e.input.Click(input.MouseLeft); err != nil {
		return e.fail("cow_transmute_input_failed")
	}
	e.transmuteSent, e.transmuteAt = true, now
	e.logStage(cowRecipeTransmute, "leg_unit_id", e.binding.LegUnitID, "tome_unit_id", e.binding.TomeUnitID)
	e.advance(cowRecipeWaitResult, now, state.Generation)
	return CowPortalRecipeResult{ProgressKind: "transmute_sent"}
}

func (e *CowPortalRecipe) tickResult(state world.State, now time.Time) CowPortalRecipeResult {
	if state.Generation <= e.stageGeneration {
		return CowPortalRecipeResult{}
	}
	legGone, legInvalid := ingredientConsumed(state, e.binding.LegUnitID)
	tomeGone, tomeInvalid := ingredientConsumed(state, e.binding.TomeUnitID)
	if legInvalid || tomeInvalid {
		return e.fail("cow_recipe_result_state_invalid")
	}
	if legGone && tomeGone {
		e.logStage(cowRecipeWaitResult, "leg_unit_id", e.binding.LegUnitID, "tome_unit_id", e.binding.TomeUnitID)
		e.advance(cowRecipeWaitPortal, e.transmuteAt, state.Generation)
		return CowPortalRecipeResult{ProgressKind: "ingredients_consumed"}
	}
	if e.expired(now, e.cfg.ResultTimeout) {
		if legGone != tomeGone {
			return e.fail("cow_recipe_partial_consumption")
		}
		return e.fail("cow_recipe_result_unconfirmed")
	}
	return CowPortalRecipeResult{}
}

func ingredientConsumed(state world.State, unitID uint32) (gone bool, invalid bool) {
	item, ok := state.FindItemByUnitID(unitID)
	if !ok {
		return true, false
	}
	if item.Location == world.ItemLocationInventory || item.Location == world.ItemLocationCube {
		return false, false
	}
	return false, true
}

func (e *CowPortalRecipe) tickPortal(state world.State, now time.Time) CowPortalRecipeResult {
	if state.Area.ID != world.RogueEncampment {
		return e.fail("cow_portal_state_invalid")
	}
	portals := make([]world.Object, 0, 1)
	for _, object := range state.Objects {
		if object.Kind == world.ObjectKindPermanentPortal && !e.initialPortals[object.UnitID] {
			portals = append(portals, object)
		}
	}
	if len(portals) > 1 {
		return e.fail("cow_portal_ambiguous")
	}
	if len(portals) == 1 {
		portal := portals[0]
		if e.portalUnitID != portal.UnitID || e.portalPosition != portal.Position {
			e.portalUnitID, e.portalPosition, e.portalStable = portal.UnitID, portal.Position, 1
		} else if state.Generation > e.stageGeneration {
			e.portalStable++
		}
		e.stageGeneration = state.Generation
		if e.portalStable >= cowPortalStableSnapshots {
			e.logStage(cowRecipeWaitPortal, "portal_unit_id", e.portalUnitID)
			e.advance(cowRecipeCloseUI, now, state.Generation)
			return CowPortalRecipeResult{PortalUnitID: e.portalUnitID, ProgressKind: "portal_confirmed"}
		}
	}
	if now.Sub(e.transmuteAt) >= e.cfg.PortalTimeout {
		return e.fail("cow_portal_missing_after_consumption")
	}
	return CowPortalRecipeResult{}
}

func (e *CowPortalRecipe) tickCloseUI(state world.State, now time.Time) CowPortalRecipeResult {
	if !state.UI.CubeOpenKnown {
		return e.fail("cow_cube_ui_unavailable")
	}
	if !state.UI.CubeOpen && !state.UI.InventoryOpen {
		e.advance(cowRecipeEnterPortal, now, state.Generation)
		return CowPortalRecipeResult{}
	}
	if e.expired(now, e.cfg.CloseTimeout) || e.closeInputs >= 2 {
		return e.fail("cow_recipe_ui_close_failed")
	}
	if err := e.input.PressKey("esc"); err != nil {
		return e.fail("cow_recipe_ui_close_failed")
	}
	e.closeInputs++
	return CowPortalRecipeResult{}
}

func (e *CowPortalRecipe) tickEnterPortal(state world.State, now time.Time) CowPortalRecipeResult {
	if state.Area.ID == world.MooMooFarm {
		e.advance(cowRecipeVerifyArea, now, state.Generation)
		return CowPortalRecipeResult{}
	}
	if state.Area.ID != world.RogueEncampment {
		return e.fail("cow_portal_entry_unconfirmed")
	}
	portal, ok := findPermanentPortal(state, e.portalUnitID)
	if !ok || portal.Position != e.portalPosition {
		return e.fail("cow_portal_binding_lost")
	}
	result, err := e.input.TickPermanentPortal(state, portal)
	if err != nil {
		return e.fail("cow_portal_entry_input_failed")
	}
	if result.Clicked {
		e.logStage(cowRecipeEnterPortal, "portal_unit_id", e.portalUnitID)
		e.advance(cowRecipeVerifyArea, now, state.Generation)
		return CowPortalRecipeResult{}
	}
	if result.Done {
		return e.fail("cow_portal_" + result.Reason)
	}
	if e.expired(now, e.cfg.EntryTimeout) {
		return e.fail("cow_portal_entry_unconfirmed")
	}
	return CowPortalRecipeResult{}
}

func findPermanentPortal(state world.State, unitID uint32) (world.Object, bool) {
	for _, object := range state.Objects {
		if object.UnitID == unitID && object.Kind == world.ObjectKindPermanentPortal {
			return object, true
		}
	}
	return world.Object{}, false
}

func (e *CowPortalRecipe) boundInventoryItem(state world.State, unitID uint32, code string) (world.Item, string) {
	item, ok := state.FindItemByUnitID(unitID)
	if !ok || item.Code != code || item.Location != world.ItemLocationInventory || !item.PlayerOwned || item.Page != 0 || item.GridX < 0 || item.GridY < 0 {
		return world.Item{}, "cow_recipe_bound_item_invalid"
	}
	return item, ""
}

func (e *CowPortalRecipe) cubeOpen(state world.State) bool {
	return state.UI.CubeOpenKnown && state.UI.CubeOpen && state.UI.InventoryOpen
}

func (e *CowPortalRecipe) moveInventoryItem(item world.Item) error {
	x := e.cfg.InventoryLeft + item.GridX*e.cfg.InventoryCellW + e.cfg.InventoryCellW/2
	y := e.cfg.InventoryTop + item.GridY*e.cfg.InventoryCellH + e.cfg.InventoryCellH/2
	return e.input.MoveTo(x, y)
}

func (e *CowPortalRecipe) advance(stage cowRecipeStage, now time.Time, generation uint64) {
	e.stage, e.stageStartedAt, e.stageGeneration = stage, now, generation
}

func (e *CowPortalRecipe) expired(now time.Time, timeout time.Duration) bool {
	return !e.stageStartedAt.IsZero() && now.Sub(e.stageStartedAt) >= timeout
}

func (e *CowPortalRecipe) fail(reason string) CowPortalRecipeResult {
	e.log.Warn("Cow-Portal-Rezept abgebrochen", "stage", e.stage, "reason", reason, "transmute_sent", e.transmuteSent)
	return CowPortalRecipeResult{Done: true, Reason: reason, PortalUnitID: e.portalUnitID}
}

func (e *CowPortalRecipe) logStage(stage cowRecipeStage, args ...any) {
	fields := []any{"stage", stage}
	fields = append(fields, args...)
	e.log.Info("Cow-Portal-Rezept bestätigt", fields...)
}
