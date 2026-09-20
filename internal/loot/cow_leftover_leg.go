package loot

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	leftoverLegClientWidth  = 1280
	leftoverLegClientHeight = 720
	leftoverLegMaxDrops     = 2
	leftoverLegStageTimeout = 4 * time.Second
)

type leftoverLegStage string

const (
	leftoverLegOpen  leftoverLegStage = "open_inventory"
	leftoverLegAim   leftoverLegStage = "aim_inventory_leg"
	leftoverLegDrop  leftoverLegStage = "ctrl_drop_inventory_leg"
	leftoverLegWait  leftoverLegStage = "wait_dropped"
	leftoverLegClose leftoverLegStage = "close_inventory"
)

// CowLeftoverLegInput is the same inventory-cell surface as the Akara sell path.
type CowLeftoverLegInput interface {
	Window() (input.WindowInfo, bool)
	MoveTo(clientX, clientY int) error
	ClickWithModifier(modifier string, button input.MouseButton) error
	PressKey(key string) error
}

// CowLeftoverLegConfig reuses the fixed 1280x720 personal-inventory geometry.
type CowLeftoverLegConfig struct {
	InventoryLeft  int
	InventoryTop   int
	InventoryCellW int
	InventoryCellH int
}

// Validate rejects inventory geometry that would click outside the known grid.
func (c CowLeftoverLegConfig) Validate() error {
	if c.InventoryLeft < 0 || c.InventoryTop < 0 || c.InventoryCellW <= 0 || c.InventoryCellH <= 0 {
		return fmt.Errorf("cow leftover-leg inventory geometry is invalid")
	}
	return nil
}

// CowLeftoverLegResult reports completion or a terminal leftover-drop reason.
type CowLeftoverLegResult struct {
	Done   bool
	Reason string
}

// CowLeftoverLegDrop wirft ein altes persönliches Wirt-Bein per Ctrl+Linksklick
// aus dem Inventar. Das Anvisieren verwendet dieselbe Inventarzelle wie der
// Akara-Verkauf.
type CowLeftoverLegDrop struct {
	log            *slog.Logger
	input          CowLeftoverLegInput
	cfg            CowLeftoverLegConfig
	stage          leftoverLegStage
	stageStartedAt time.Time
	generation     uint64
	boundUnitID    uint32
	drops          int
	openSent       bool
	closeSent      bool
}

// NewCowLeftoverLegDrop creates the narrow leftover-inventory-leg dropper.
func NewCowLeftoverLegDrop(log *slog.Logger, in CowLeftoverLegInput, cfg CowLeftoverLegConfig) (*CowLeftoverLegDrop, error) {
	if log == nil || in == nil {
		return nil, fmt.Errorf("cow leftover-leg logger and input are required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	drop := &CowLeftoverLegDrop{log: log.With("component", "loot.cow_leftover_leg"), input: in, cfg: cfg}
	drop.Reset()
	return drop, nil
}

// Reset clears drop state without sending input.
func (e *CowLeftoverLegDrop) Reset() {
	if e == nil {
		return
	}
	e.stage = leftoverLegOpen
	e.stageStartedAt = time.Time{}
	e.generation = 0
	e.boundUnitID = 0
	e.drops = 0
	e.openSent = false
	e.closeSent = false
}

// Tick wirft höchstens zwei persönliche Inventar-Beine per Ctrl+Linksklick
// und schließt das Inventar danach. Stash- oder Cube-Beine bleiben fail-closed.
func (e *CowLeftoverLegDrop) Tick(state world.State, now time.Time) CowLeftoverLegResult {
	if e == nil || e.input == nil || !state.Valid || state.Phase != world.GamePhaseInGame {
		return CowLeftoverLegResult{}
	}
	if now.IsZero() {
		now = state.At
	}
	if now.IsZero() {
		now = time.Now()
	}
	if win, ok := e.input.Window(); !ok || win.ClientWidth != leftoverLegClientWidth || win.ClientHeight != leftoverLegClientHeight {
		return e.fail("cow_leg_drop_failed")
	}
	if leftoverBlockingLocation(state) {
		return e.fail("cow_existing_leg")
	}
	if e.stageStartedAt.IsZero() {
		e.stageStartedAt, e.generation = now, state.Generation
	}

	switch e.stage {
	case leftoverLegOpen:
		return e.tickOpen(state, now)
	case leftoverLegAim:
		return e.tickAim(state, now)
	case leftoverLegDrop:
		return e.tickDrop(state, now)
	case leftoverLegWait:
		return e.tickWaitDropped(state, now)
	case leftoverLegClose:
		return e.tickClose(state, now)
	default:
		return e.fail("cow_leg_drop_failed")
	}
}

func (e *CowLeftoverLegDrop) tickOpen(state world.State, now time.Time) CowLeftoverLegResult {
	if leftoverInventoryEmpty(state) {
		e.advance(leftoverLegClose, now, state.Generation)
		return CowLeftoverLegResult{}
	}
	if state.UI.InventoryOpen {
		e.advance(leftoverLegAim, now, state.Generation)
		return CowLeftoverLegResult{}
	}
	if e.expired(now) {
		return e.fail("cow_leg_drop_failed")
	}
	if !e.openSent {
		if err := e.input.PressKey("i"); err != nil {
			return e.fail("cow_leg_drop_failed")
		}
		e.openSent = true
	}
	return CowLeftoverLegResult{}
}

func (e *CowLeftoverLegDrop) tickAim(state world.State, now time.Time) CowLeftoverLegResult {
	if !state.UI.InventoryOpen {
		return e.fail("cow_leg_drop_failed")
	}
	if leftoverInventoryEmpty(state) {
		e.advance(leftoverLegClose, now, state.Generation)
		return CowLeftoverLegResult{}
	}
	if e.drops >= leftoverLegMaxDrops {
		return e.fail("cow_leg_drop_failed")
	}
	item, ok := leftoverFirstInventoryLeg(state)
	if !ok {
		return e.fail("cow_leg_drop_failed")
	}
	if err := e.moveInventoryItem(item); err != nil {
		return e.fail("cow_leg_drop_failed")
	}
	e.boundUnitID = item.UnitID
	e.log.Info("Altes Wirt-Bein anvisiert", "unit_id", item.UnitID)
	e.advance(leftoverLegDrop, now, state.Generation)
	return CowLeftoverLegResult{}
}

func (e *CowLeftoverLegDrop) tickDrop(state world.State, now time.Time) CowLeftoverLegResult {
	if !state.UI.InventoryOpen {
		return e.fail("cow_leg_drop_failed")
	}
	item, ok := leftoverBoundInventoryLeg(state, e.boundUnitID)
	if !ok {
		return e.fail("cow_leg_drop_failed")
	}
	if err := e.moveInventoryItem(item); err != nil {
		return e.fail("cow_leg_drop_failed")
	}
	if err := e.input.ClickWithModifier("ctrl", input.MouseLeft); err != nil {
		return e.fail("cow_leg_drop_failed")
	}
	e.drops++
	e.log.Info("Altes Wirt-Bein per Ctrl+Linksklick abgelegt", "unit_id", e.boundUnitID, "drop", e.drops)
	e.advance(leftoverLegWait, now, state.Generation)
	return CowLeftoverLegResult{}
}

func (e *CowLeftoverLegDrop) tickWaitDropped(state world.State, now time.Time) CowLeftoverLegResult {
	if state.Generation <= e.generation {
		return CowLeftoverLegResult{}
	}
	if leftoverBoundInInventory(state, e.boundUnitID) {
		if e.expired(now) {
			return e.fail("cow_leg_drop_failed")
		}
		return CowLeftoverLegResult{}
	}
	e.boundUnitID = 0
	if leftoverInventoryEmpty(state) {
		e.advance(leftoverLegClose, now, state.Generation)
		return CowLeftoverLegResult{}
	}
	e.advance(leftoverLegAim, now, state.Generation)
	return CowLeftoverLegResult{}
}

func (e *CowLeftoverLegDrop) tickClose(state world.State, now time.Time) CowLeftoverLegResult {
	if leftoverBlockingLocation(state) {
		return e.fail("cow_existing_leg")
	}
	if !leftoverInventoryEmpty(state) {
		return e.fail("cow_leg_drop_failed")
	}
	if !state.UI.InventoryOpen {
		return CowLeftoverLegResult{Done: true}
	}
	if e.expired(now) {
		return e.fail("cow_leg_drop_failed")
	}
	if !e.closeSent {
		if err := e.input.PressKey("i"); err != nil {
			return e.fail("cow_leg_drop_failed")
		}
		e.closeSent = true
	}
	return CowLeftoverLegResult{}
}

func (e *CowLeftoverLegDrop) moveInventoryItem(item world.Item) error {
	x := e.cfg.InventoryLeft + item.GridX*e.cfg.InventoryCellW + e.cfg.InventoryCellW/2
	y := e.cfg.InventoryTop + item.GridY*e.cfg.InventoryCellH + e.cfg.InventoryCellH/2
	return e.input.MoveTo(x, y)
}

func (e *CowLeftoverLegDrop) advance(stage leftoverLegStage, now time.Time, generation uint64) {
	e.stage, e.stageStartedAt, e.generation = stage, now, generation
	if stage == leftoverLegClose {
		e.closeSent = false
	}
}

func (e *CowLeftoverLegDrop) expired(now time.Time) bool {
	return !e.stageStartedAt.IsZero() && now.Sub(e.stageStartedAt) >= leftoverLegStageTimeout
}

func (e *CowLeftoverLegDrop) fail(reason string) CowLeftoverLegResult {
	e.log.Warn("Altes Wirt-Bein konnte nicht abgelegt werden", "stage", e.stage, "reason", reason)
	return CowLeftoverLegResult{Done: true, Reason: reason}
}

func leftoverFirstInventoryLeg(state world.State) (world.Item, bool) {
	for _, item := range state.Items {
		if leftoverPersonalInventoryLeg(item) {
			return item, true
		}
	}
	return world.Item{}, false
}

func leftoverInventoryEmpty(state world.State) bool {
	_, ok := leftoverFirstInventoryLeg(state)
	return !ok
}

func leftoverBoundInventoryLeg(state world.State, unitID uint32) (world.Item, bool) {
	if unitID == 0 {
		return world.Item{}, false
	}
	item, ok := state.FindItemByUnitID(unitID)
	if !ok || !leftoverPersonalInventoryLeg(item) {
		return world.Item{}, false
	}
	return item, true
}

func leftoverBoundInInventory(state world.State, unitID uint32) bool {
	_, ok := leftoverBoundInventoryLeg(state, unitID)
	return ok
}

func leftoverPersonalInventoryLeg(item world.Item) bool {
	return item.Code == "leg" && item.Location == world.ItemLocationInventory && item.PlayerOwned && item.Page == 0 &&
		item.GridX >= 0 && item.GridY >= 0 && item.Width > 0 && item.Height > 0
}

func leftoverBlockingLocation(state world.State) bool {
	for _, item := range state.Items {
		if item.Code != "leg" {
			continue
		}
		switch item.Location {
		case world.ItemLocationCube, world.ItemLocationStash,
			world.ItemLocationSharedStash1, world.ItemLocationSharedStash2, world.ItemLocationSharedStash3:
			return true
		}
	}
	return false
}
