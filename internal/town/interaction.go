package town

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// InteractionStatus is one tick outcome of an NPC, dialog, shop, or buy gate.
type InteractionStatus string

const (
	InteractionPending  InteractionStatus = "pending"
	InteractionAction   InteractionStatus = "action"
	InteractionComplete InteractionStatus = "complete"
	InteractionFailed   InteractionStatus = "failed"
)

// InteractionResult reports an ordered action or terminal fail-closed outcome.
type InteractionResult struct {
	Status             InteractionStatus
	Reason             string
	Action             string
	UnitID             uint32
	Code               string
	Name               string
	Quality            world.ItemQuality
	IdentityKind       world.ItemIdentityKind
	IdentityKey        string
	IdentityValid      bool
	Current            int
	Threshold          int
	BeltSlots          []int
	Mode               BuyMode
	Vendor             Anchor
	Cost               int
	VerifiedFinal      int
	Done               bool
	ProfileID          string
	RuleID             string
	PickitAction       string
	ProfileRevision    uint64
	AssignmentRevision uint64
}

// NPCClickTarget is the pinned semantic target passed to the app-level click adapter.
type NPCClickTarget struct {
	UnitID   uint32
	Position world.Position
	Name     string
}

// NPCClickResult reports a hover-confirmed click attempt without pathing internals.
type NPCClickResult struct {
	Clicked bool
	Done    bool
	Reason  string
}

// EntityClicker is the hover-confirmed click subset used for NPC interaction.
type EntityClicker interface {
	TickNPC(world.State, NPCClickTarget, float64) (NPCClickResult, error)
	Reset()
}

// NPCInteractor pins one NPC UnitID and permits exactly one hover-confirmed click.
// The pin prevents regional enumeration changes from silently switching to a
// different NPC instance after interaction has begun.
type NPCInteractor struct {
	clicker     EntityClicker
	npcID       uint32
	maxDistance float64
	timeout     time.Duration
	pinned      uint32
	clicked     bool
	started     time.Time
}

// NewNPCInteractor creates a resettable fail-closed NPC interaction gate.
func NewNPCInteractor(clicker EntityClicker, npcID uint32, maxDistance float64, timeout time.Duration) *NPCInteractor {
	return &NPCInteractor{clicker: clicker, npcID: npcID, maxDistance: maxDistance, timeout: timeout}
}

// Reset discards NPC pinning and click state.
func (i *NPCInteractor) Reset() {
	if i == nil {
		return
	}
	if i.clicker != nil {
		i.clicker.Reset()
	}
	i.pinned = 0
	i.clicked = false
	i.started = time.Time{}
}

// Tick advances NPC pinning, hover confirmation, one click, and dialog verification.
func (i *NPCInteractor) Tick(state world.State) InteractionResult {
	if i == nil || i.clicker == nil || !state.Valid || state.Area.ID != world.RogueEncampment {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_state_invalid", Done: true}
	}
	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if i.started.IsZero() {
		i.started = now
	}
	if i.timeout > 0 && now.Sub(i.started) >= i.timeout {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_interaction_timeout", UnitID: i.pinned, Done: true}
	}
	var npc world.Monster
	found := false
	if i.pinned == 0 {
		npc, found = state.FindNPC(i.npcID)
		if !found {
			return InteractionResult{Status: InteractionFailed, Reason: "npc_not_found", Done: true}
		}
		if world.Distance(state.Player.Position, npc.Position) > i.maxDistance {
			return InteractionResult{Status: InteractionFailed, Reason: "npc_too_far", UnitID: npc.UnitID, Done: true}
		}
		i.pinned = npc.UnitID
	} else {
		npc, found = state.FindMonsterByUnitID(i.pinned)
		if !found || npc.NPCID != i.npcID {
			return InteractionResult{Status: InteractionFailed, Reason: "npc_pin_lost", UnitID: i.pinned, Done: true}
		}
	}
	if i.clicked {
		// A sent click is never repeated. Only the separately observed dialog or
		// shop flag can complete the interaction.
		if state.UI.NPCInteractOpen || state.UI.NPCShopOpen {
			return InteractionResult{Status: InteractionComplete, UnitID: i.pinned, Done: true}
		}
		return InteractionResult{Status: InteractionPending, UnitID: i.pinned}
	}
	if state.UI.NPCInteractOpen || state.UI.NPCShopOpen {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_ui_preopened", UnitID: i.pinned, Done: true}
	}
	result, err := i.clicker.TickNPC(state, NPCClickTarget{UnitID: npc.UnitID, Position: npc.Position, Name: world.LookupNPCName(npc.NPCID)}, i.maxDistance)
	if err != nil {
		return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("npc_input_failed: %v", err), UnitID: i.pinned, Done: true}
	}
	if result.Clicked {
		i.clicked = true
		return InteractionResult{Status: InteractionAction, Action: "npc_click", UnitID: i.pinned}
	}
	if result.Done {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_" + result.Reason, UnitID: i.pinned, Done: true}
	}
	return InteractionResult{Status: InteractionPending, UnitID: i.pinned}
}

// ShopInput is the narrow atomic input contract for dialog and purchase actions.
type ShopInput interface {
	MoveTo(int, int) error
	Click(input.MouseButton) error
	ClickWithModifier(string, input.MouseButton) error
	PressKey(string) error
}

// ShopOpener selects Akara's Trade dialog option and separately verifies NPCShopOpen.
// The fixed key sequence is Akara-specific and reusable only behind a confirmed
// `NPCInteractOpen` gate at the supported client/UI version.
type ShopOpener struct {
	input   ShopInput
	keys    []string
	index   int
	started time.Time
	timeout time.Duration
}

// NewShopOpener creates the bounded Home/Down/Enter dialog selector used for Akara.
func NewShopOpener(in ShopInput, timeout time.Duration) *ShopOpener {
	return &ShopOpener{input: in, keys: []string{"home", "down", "enter"}, timeout: timeout}
}

// Tick sends at most one dialog key and waits for Memory-confirmed shop UI.
func (o *ShopOpener) Tick(state world.State) InteractionResult {
	if o == nil || o.input == nil || !state.Valid {
		return InteractionResult{Status: InteractionFailed, Reason: "shop_state_invalid", Done: true}
	}
	if state.UI.NPCShopOpen {
		// Dialog selection and shop readiness are distinct Memory states; vendor
		// coordinates are forbidden until this stronger state is observed.
		return InteractionResult{Status: InteractionComplete, Done: true}
	}
	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if o.started.IsZero() {
		o.started = now
	}
	if o.timeout > 0 && now.Sub(o.started) >= o.timeout {
		return InteractionResult{Status: InteractionFailed, Reason: "shop_open_timeout", Done: true}
	}
	if !state.UI.NPCInteractOpen && o.index < len(o.keys) {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_dialog_not_open", Done: true}
	}
	if o.index >= len(o.keys) {
		return InteractionResult{Status: InteractionPending}
	}
	key := o.keys[o.index]
	if err := o.input.PressKey(key); err != nil {
		return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("shop_dialog_input_failed: %v", err), Done: true}
	}
	o.index++
	return InteractionResult{Status: InteractionAction, Action: "dialog_key"}
}

// MenuSelector sends a fixed Home/Down/Enter dialog sequence one key per tick.
// Unlike [ShopOpener], completion is the final Enter itself; callers verify
// Memory outcomes separately so revive never waits on NPCShopOpen.
type MenuSelector struct {
	input   ShopInput
	keys    []string
	index   int
	started time.Time
	timeout time.Duration
}

// NewMenuSelector creates a bounded dialog selector for Kashya revive.
func NewMenuSelector(in ShopInput, timeout time.Duration) *MenuSelector {
	return &MenuSelector{input: in, keys: []string{"home", "down", "enter"}, timeout: timeout}
}

// Tick sends at most one dialog key and completes after the final Enter is sent.
func (o *MenuSelector) Tick(state world.State) InteractionResult {
	if o == nil || o.input == nil || !state.Valid {
		return InteractionResult{Status: InteractionFailed, Reason: "menu_state_invalid", Done: true}
	}
	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if o.started.IsZero() {
		o.started = now
	}
	if o.timeout > 0 && now.Sub(o.started) >= o.timeout {
		return InteractionResult{Status: InteractionFailed, Reason: "menu_select_timeout", Done: true}
	}
	if o.index >= len(o.keys) {
		return InteractionResult{Status: InteractionComplete, Done: true}
	}
	if !state.UI.NPCInteractOpen {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_dialog_not_open", Done: true}
	}
	if state.UI.NPCShopOpen {
		return InteractionResult{Status: InteractionFailed, Reason: "npc_shop_unexpected", Done: true}
	}
	key := o.keys[o.index]
	if err := o.input.PressKey(key); err != nil {
		return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("menu_dialog_input_failed: %v", err), Done: true}
	}
	o.index++
	action := "dialog_key"
	if key == "enter" {
		action = "dialog_enter"
	}
	return InteractionResult{Status: InteractionAction, Action: action}
}

// BuyMode selects one atomic bulk action or a bounded single-buy action.
type BuyMode string

const (
	BuyModeBulk   BuyMode = "bulk"
	BuyModeSingle BuyMode = "single"
)

// VendorRequest selects one concrete vendor item by exact code or semantic type.
type VendorRequest struct {
	Code string
	Type string
	Mode BuyMode
}

// VendorBuyer pins one vendor item UnitID and grid cell before one purchase action.
// Shop items are not reliably represented by the world hover buffer, so the
// second Memory snapshot revalidates UnitID and cell instead of faking a hover gate.
type VendorBuyer struct {
	input                                 ShopInput
	request                               VendorRequest
	inventoryLeft, inventoryTop, cellSize int
	pinned                                uint32
	pinnedGridX, pinnedGridY              int
	moved                                 bool
	acted                                 bool
}

// NewVendorBuyer creates a buyer for the fixed 1280×720 vendor grid after NPCShopOpen.
func NewVendorBuyer(in ShopInput, request VendorRequest) *VendorBuyer {
	return &VendorBuyer{input: in, request: request, inventoryLeft: 109, inventoryTop: 147, cellSize: 33}
}

// Tick sends at most one move or purchase action and never retries a completed purchase.
func (b *VendorBuyer) Tick(state world.State) InteractionResult {
	if b == nil || b.input == nil || !state.Valid || !state.UI.NPCShopOpen {
		return InteractionResult{Status: InteractionFailed, Reason: "vendor_shop_not_confirmed", Done: true}
	}
	if b.acted {
		// Completion reports the previously sent action; it never emits input.
		return InteractionResult{Status: InteractionComplete, UnitID: b.pinned, Code: b.request.Code, Done: true}
	}
	item, found := findVendorItem(state, b.request, b.pinned)
	if !found {
		reason := "vendor_item_not_found"
		if b.pinned != 0 {
			reason = "vendor_item_pin_lost"
		}
		return InteractionResult{Status: InteractionFailed, Reason: reason, UnitID: b.pinned, Code: b.request.Code, Done: true}
	}
	if b.pinned == 0 {
		b.pinned = item.UnitID
		b.pinnedGridX = item.GridX
		b.pinnedGridY = item.GridY
		b.request.Code = item.Code
	}
	if !b.moved {
		x := b.inventoryLeft + item.GridX*b.cellSize + b.cellSize/2
		y := b.inventoryTop + item.GridY*b.cellSize + b.cellSize/2
		if err := b.input.MoveTo(x, y); err != nil {
			return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("vendor_move_failed: %v", err), UnitID: b.pinned, Done: true}
		}
		b.moved = true
		return InteractionResult{Status: InteractionAction, Action: "vendor_move", UnitID: b.pinned, Code: b.request.Code}
	}
	// D2R's world hover buffer does not reliably expose items inside the shop UI.
	// Re-confirm the pinned vendor unit and its grid position after moving.
	if item.GridX != b.pinnedGridX || item.GridY != b.pinnedGridY {
		return InteractionResult{Status: InteractionFailed, Reason: "vendor_item_position_changed", UnitID: b.pinned, Code: b.request.Code, Done: true}
	}
	var err error
	buyAction := ""
	switch b.request.Mode {
	case BuyModeBulk:
		err = b.input.ClickWithModifier("shift", input.MouseRight)
		buyAction = "vendor_buy_bulk"
	case BuyModeSingle:
		err = b.input.Click(input.MouseRight)
		buyAction = "vendor_buy_single"
	default:
		return InteractionResult{Status: InteractionFailed, Reason: "vendor_buy_mode_invalid", UnitID: b.pinned, Done: true}
	}
	if err != nil {
		return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("vendor_buy_failed: %v", err), UnitID: b.pinned, Code: b.request.Code, Done: true}
	}
	b.acted = true
	return InteractionResult{Status: InteractionAction, Action: buyAction, UnitID: b.pinned, Code: b.request.Code}
}

func findVendorItem(state world.State, request VendorRequest, pinned uint32) (world.Item, bool) {
	var best world.Item
	found := false
	for _, item := range state.ItemsByLocation(world.ItemLocationVendor) {
		if pinned != 0 && item.UnitID != pinned {
			continue
		}
		if request.Code != "" && item.Code != request.Code {
			continue
		}
		if request.Code == "" && request.Type != "" && item.Type != request.Type {
			continue
		}
		if !found || item.TxtFileNo > best.TxtFileNo {
			best, found = item, true
		}
	}
	return best, found
}
