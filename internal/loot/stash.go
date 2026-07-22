package loot

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	stashClientWidth  = 1280
	stashClientHeight = 720
)

// StashConfig controls personal-inventory coordinates and Memory verification timeouts.
type StashConfig struct {
	MaxRetries     int
	VerifyTimeout  time.Duration
	CloseTimeout   time.Duration
	InventoryLeft  int
	InventoryTop   int
	InventoryCellW int
	InventoryCellH int
}

// Validate reports settings that would make fixed-coordinate stash input unsafe.
func (c StashConfig) Validate() error {
	if c.MaxRetries <= 0 || c.VerifyTimeout <= 0 || c.CloseTimeout <= 0 {
		return fmt.Errorf("stash retries and timeouts must be > 0")
	}
	if c.InventoryLeft < 0 || c.InventoryTop < 0 || c.InventoryCellW <= 0 || c.InventoryCellH <= 0 {
		return fmt.Errorf("stash inventory geometry is invalid")
	}
	return nil
}

// StashInput is the atomic input surface required by [StashExecutor].
type StashInput interface {
	Window() (input.WindowInfo, bool)
	MoveTo(clientX, clientY int) error
	ClickWithModifier(modifier string, button input.MouseButton) error
	PressKey(key string) error
}

// StashStatus describes one personal-stash executor outcome.
type StashStatus string

// Personal stash executor statuses.
const (
	StashPending               StashStatus = "pending"
	StashSuccess               StashStatus = "success"
	StashFailed                StashStatus = "stash_failed"
	StashFull                  StashStatus = "stash_full"
	StashCloseFailed           StashStatus = "stash_close_failed"
	StashClosed                StashStatus = "closed"
	StashUnsupportedResolution StashStatus = "unsupported_resolution"
)

// StashResult reports one verified transfer or terminal stash outcome.
type StashResult struct {
	Status        StashStatus
	Done          bool
	Attempted     bool
	Transferred   bool
	UnitID        uint32
	Code          string
	Name          string
	Quality       world.ItemQuality
	IdentityKind  world.ItemIdentityKind
	IdentityKey   string
	IdentityValid bool
	Attempt       int
	GridX         int
	GridY         int
	Pickit        PickitResult
}

type stashTarget struct {
	item   world.Item
	pickit PickitResult
}

// StashExecutor transfers Pickit-matching unlocked inventory items via Ctrl+LMB and verifies Memory state.
type StashExecutor struct {
	log    *slog.Logger
	filter *Filter
	input  StashInput
	cfg    StashConfig

	active    *stashTarget
	attempt   int
	attemptAt time.Time
	closing   bool
	closeAt   time.Time
}

// NewStashExecutor creates a fail-closed personal-stash executor.
func NewStashExecutor(log *slog.Logger, filter *Filter, in StashInput, cfg StashConfig) (*StashExecutor, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &StashExecutor{log: log.With("component", "loot.stash"), filter: filter, input: in, cfg: cfg}, nil
}

// Reset clears transfer and close state without sending input.
func (e *StashExecutor) Reset() {
	if e == nil {
		return
	}
	e.active = nil
	e.attempt = 0
	e.attemptAt = time.Time{}
	e.closing = false
	e.closeAt = time.Time{}
}

// Tick transfers at most one item at a time and advances only after Memory verification.
func (e *StashExecutor) Tick(state world.State, now time.Time) StashResult {
	if e == nil || e.filter == nil || e.input == nil {
		return StashResult{Status: StashFailed, Done: true}
	}
	if !e.supportedResolution() {
		e.Reset()
		return StashResult{Status: StashUnsupportedResolution, Done: true}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return StashResult{Status: StashPending}
	}
	if !state.UI.StashOpen || !state.UI.InventoryOpen {
		e.Reset()
		return StashResult{Status: StashFailed, Done: true}
	}
	if now.IsZero() {
		now = time.Now()
	}

	if e.active != nil {
		item, found := state.FindItemByUnitID(e.active.item.UnitID)
		if !found || item.Location != world.ItemLocationInventory {
			result := StashResult{Status: StashPending, Transferred: true, UnitID: e.active.item.UnitID, Code: e.active.item.Code, Name: e.active.item.Name, Quality: e.active.item.Quality, IdentityKind: e.active.item.IdentityKind, IdentityKey: e.active.item.IdentityKey, IdentityValid: e.active.item.IdentityValid, Attempt: e.attempt, GridX: e.active.item.GridX, GridY: e.active.item.GridY, Pickit: e.active.pickit}
			e.log.Info("stash_success", "unit_id", result.UnitID, "code", result.Code, "name", result.Name, "attempt", result.Attempt, "profile_id", result.Pickit.ProfileID, "rule_id", result.Pickit.RuleID, "action", result.Pickit.Action, "profile_revision", result.Pickit.ProfileRevision, "assignment_revision", result.Pickit.AssignmentRevision)
			e.active = nil
			e.attempt = 0
			e.attemptAt = time.Time{}
			return result
		}
		current := e.filter.evaluate(item)
		if item.GridX != e.active.item.GridX || item.GridY != e.active.item.GridY || !stashEligible(e.filter.inventoryLock, item) || !current.Matched || current.Action != ActionKeep || current.ProfileID != e.active.pickit.ProfileID || current.RuleID != e.active.pickit.RuleID {
			return e.failActive("target_changed")
		}
		if now.Sub(e.attemptAt) < e.cfg.VerifyTimeout {
			return StashResult{Status: StashPending, UnitID: item.UnitID, Code: item.Code, Name: item.Name, Quality: item.Quality, IdentityKind: item.IdentityKind, IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid, Attempt: e.attempt, Pickit: e.active.pickit}
		}
		if e.attempt >= e.cfg.MaxRetries {
			return e.failActive("verify_timeout")
		}
		return e.transfer(item, now)
	}

	candidates, safe := e.candidates(state)
	if !safe {
		return StashResult{Status: StashFailed, Done: true}
	}
	if len(candidates) == 0 {
		return StashResult{Status: StashSuccess, Done: true}
	}
	match := e.filter.evaluate(candidates[0])
	e.active = &stashTarget{item: candidates[0], pickit: match}
	e.attempt = 0
	return e.transfer(candidates[0], now)
}

// TickClose presses Esc once and confirms that both stash and inventory UI flags close.
func (e *StashExecutor) TickClose(state world.State, now time.Time) StashResult {
	if e == nil || e.input == nil {
		return StashResult{Status: StashCloseFailed, Done: true}
	}
	if !e.supportedResolution() {
		return StashResult{Status: StashUnsupportedResolution, Done: true}
	}
	if !state.UI.StashOpen && !state.UI.InventoryOpen {
		e.closing = false
		e.closeAt = time.Time{}
		return StashResult{Status: StashClosed, Done: true}
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !e.closing {
		if err := e.input.PressKey("esc"); err != nil {
			return StashResult{Status: StashCloseFailed, Done: true}
		}
		e.closing = true
		e.closeAt = now
		e.log.Info("personal stash close requested")
		return StashResult{Status: StashPending}
	}
	if now.Sub(e.closeAt) >= e.cfg.CloseTimeout {
		e.closing = false
		return StashResult{Status: StashCloseFailed, Done: true}
	}
	return StashResult{Status: StashPending}
}

func (e *StashExecutor) candidates(state world.State) ([]world.Item, bool) {
	items := state.InventoryItems()
	if NewInventoryGrid(e.filter.inventoryLock, items).Capacity().Unsafe {
		return nil, false
	}
	out := make([]world.Item, 0)
	for _, item := range items {
		result := e.filter.evaluate(item)
		if result.Matched && result.Action == ActionKeep && !RequiresIdentificationForKeep(item) && stashEligible(e.filter.inventoryLock, item) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GridY != out[j].GridY {
			return out[i].GridY < out[j].GridY
		}
		if out[i].GridX != out[j].GridX {
			return out[i].GridX < out[j].GridX
		}
		return out[i].UnitID < out[j].UnitID
	})
	return out, true
}

func (e *StashExecutor) transfer(item world.Item, now time.Time) StashResult {
	result := e.filter.evaluate(item)
	if !result.Matched || result.Action != ActionKeep || (e.active != nil && (result.ProfileID != e.active.pickit.ProfileID || result.RuleID != e.active.pickit.RuleID)) {
		return e.failActive("policy_changed")
	}
	x := e.cfg.InventoryLeft + item.GridX*e.cfg.InventoryCellW + e.cfg.InventoryCellW/2
	y := e.cfg.InventoryTop + item.GridY*e.cfg.InventoryCellH + e.cfg.InventoryCellH/2
	if err := e.input.MoveTo(x, y); err != nil {
		return e.failActive("move_failed")
	}
	if err := e.input.ClickWithModifier("ctrl", input.MouseLeft); err != nil {
		return e.failActive("ctrl_click_failed")
	}
	e.attempt++
	e.attemptAt = now
	e.log.Info("stash_attempt", "unit_id", item.UnitID, "code", item.Code, "name", item.Name, "grid_x", item.GridX, "grid_y", item.GridY, "client_x", x, "client_y", y, "attempt", e.attempt, "profile_id", result.ProfileID, "rule_id", result.RuleID, "action", result.Action, "profile_revision", result.ProfileRevision, "assignment_revision", result.AssignmentRevision)
	return StashResult{Status: StashPending, Attempted: true, UnitID: item.UnitID, Code: item.Code, Name: item.Name, Quality: item.Quality, IdentityKind: item.IdentityKind, IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid, Attempt: e.attempt, GridX: item.GridX, GridY: item.GridY, Pickit: result}
}

func (e *StashExecutor) failActive(reason string) StashResult {
	result := StashResult{Status: StashFailed, Done: true}
	if e.active != nil {
		result.UnitID = e.active.item.UnitID
		result.Code = e.active.item.Code
		result.Name = e.active.item.Name
		result.Attempt = e.attempt
		result.Pickit = e.active.pickit
	}
	e.log.Warn("stash_failed", "reason", reason, "unit_id", result.UnitID, "code", result.Code, "attempt", result.Attempt)
	e.Reset()
	return result
}

func (e *StashExecutor) supportedResolution() bool {
	win, ok := e.input.Window()
	return ok && win.ClientWidth == stashClientWidth && win.ClientHeight == stashClientHeight
}
