package town

import (
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type clickerMock struct {
	results []NPCClickResult
	calls   int
}

func (m *clickerMock) TickNPC(world.State, NPCClickTarget, float64) (NPCClickResult, error) {
	m.calls++
	r := m.results[0]
	m.results = m.results[1:]
	return r, nil
}
func (*clickerMock) Reset() {}

func interactionState() world.State {
	return world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), Player: world.Player{Position: world.Position{X: 100, Y: 100}}, Monsters: []world.Monster{{NPCID: world.Akara, UnitID: 77, Position: world.Position{X: 105, Y: 105}, Name: "Akara"}}}
}

func TestNPCInteractorPinsClicksOnceAndWaitsForDialog(t *testing.T) {
	clicker := &clickerMock{results: []NPCClickResult{{}, {Clicked: true, Done: true}}}
	exec := NewNPCInteractor(clicker, world.Akara, 15, time.Second)
	state := interactionState()
	if result := exec.Tick(state); result.Status != InteractionPending {
		t.Fatalf("first = %+v", result)
	}
	if result := exec.Tick(state); result.Status != InteractionAction || result.UnitID != 77 {
		t.Fatalf("click = %+v", result)
	}
	if result := exec.Tick(state); result.Status != InteractionPending {
		t.Fatalf("wait = %+v", result)
	}
	state.UI.NPCInteractOpen = true
	if result := exec.Tick(state); !result.Done || result.Status != InteractionComplete {
		t.Fatalf("complete = %+v", result)
	}
	if clicker.calls != 2 {
		t.Fatalf("clicker calls = %d, want 2", clicker.calls)
	}
}

func TestNPCInteractorReacquiresMovingNPCOnceWhenDialogStaysClosed(t *testing.T) {
	clicker := &clickerMock{results: []NPCClickResult{{Clicked: true, Done: true}, {Clicked: true, Done: true}}}
	exec := NewNPCInteractor(clicker, world.Akara, 15, 3*time.Second)
	state := interactionState()

	if result := exec.Tick(state); result.Action != "npc_click" {
		t.Fatalf("first click = %+v", result)
	}
	state.At = state.At.Add(npcDialogConfirmationWindow)
	state.Monsters[0].Position = world.Position{X: 106, Y: 105}
	if result := exec.Tick(state); result.Action != "npc_click" {
		t.Fatalf("second click = %+v", result)
	}
	state.At = state.At.Add(npcDialogConfirmationWindow)
	if result := exec.Tick(state); result.Status != InteractionPending {
		t.Fatalf("bounded wait = %+v", result)
	}
	if clicker.calls != 2 {
		t.Fatalf("clicker calls = %d, want 2", clicker.calls)
	}
}

func TestNPCInteractorAcceptsDialogOpeningDuringReacquire(t *testing.T) {
	clicker := &clickerMock{results: []NPCClickResult{{Clicked: true, Done: true}, {}}}
	exec := NewNPCInteractor(clicker, world.Akara, 15, 3*time.Second)
	state := interactionState()

	_ = exec.Tick(state)
	state.At = state.At.Add(npcDialogConfirmationWindow)
	_ = exec.Tick(state)
	state.UI.NPCInteractOpen = true
	if result := exec.Tick(state); result.Status != InteractionComplete || !result.Done {
		t.Fatalf("delayed dialog = %+v", result)
	}
	if clicker.calls != 2 {
		t.Fatalf("clicker calls = %d, want 2", clicker.calls)
	}
}

func TestNPCInteractorFailsWhenPinnedNPCDisappears(t *testing.T) {
	exec := NewNPCInteractor(&clickerMock{results: []NPCClickResult{{}}}, world.Akara, 15, time.Second)
	state := interactionState()
	_ = exec.Tick(state)
	state.Monsters = nil
	if result := exec.Tick(state); result.Reason != "npc_pin_lost" || !result.Done {
		t.Fatalf("result = %+v", result)
	}
}

type shopInputMock struct {
	moves    [][2]int
	clicks   []input.MouseButton
	modified []string
	keys     []string
	err      error
}

func (m *shopInputMock) MoveTo(x, y int) error { m.moves = append(m.moves, [2]int{x, y}); return m.err }
func (m *shopInputMock) Click(button input.MouseButton) error {
	m.clicks = append(m.clicks, button)
	return m.err
}
func (m *shopInputMock) ClickWithModifier(mod string, button input.MouseButton) error {
	m.modified = append(m.modified, mod+":"+string(button))
	return m.err
}
func (m *shopInputMock) PressKey(key string) error { m.keys = append(m.keys, key); return m.err }

func TestShopOpenerSeparatelyConfirmsDialogAndShop(t *testing.T) {
	in := &shopInputMock{}
	opener := NewShopOpener(in, time.Second)
	state := interactionState()
	state.UI.NPCInteractOpen = true
	for _, key := range []string{"home", "down", "enter"} {
		result := opener.Tick(state)
		if result.Status != InteractionAction || in.keys[len(in.keys)-1] != key {
			t.Fatalf("key %s result=%+v keys=%v", key, result, in.keys)
		}
	}
	if result := opener.Tick(state); result.Status != InteractionPending {
		t.Fatalf("pending = %+v", result)
	}
	state.UI.NPCInteractOpen = false
	state.UI.NPCShopOpen = true
	if result := opener.Tick(state); !result.Done || result.Status != InteractionComplete {
		t.Fatalf("complete = %+v", result)
	}
}

func vendorState() world.State {
	state := interactionState()
	state.UI.NPCShopOpen = true
	state.Items = []world.Item{{TxtFileNo: 606, UnitID: 91, Code: "hp5", Type: "hpot", Location: world.ItemLocationVendor, GridX: 2, GridY: 3}, {TxtFileNo: 611, UnitID: 92, Code: "mp5", Type: "mpot", Location: world.ItemLocationVendor, GridX: 3, GridY: 4}}
	return state
}

func TestVendorBuyerPinsPositionAndBulkBuysExactlyOnce(t *testing.T) {
	in := &shopInputMock{}
	buyer := NewVendorBuyer(in, VendorRequest{Type: "hpot", Mode: BuyModeBulk})
	state := vendorState()
	if result := buyer.Tick(state); result.Status != InteractionAction || result.Action != "vendor_move" || result.UnitID != 91 {
		t.Fatalf("move = %+v", result)
	}
	if len(in.moves) != 1 || in.moves[0] != [2]int{191, 262} {
		t.Fatalf("moves = %v", in.moves)
	}
	if result := buyer.Tick(state); result.Status != InteractionAction || result.Action != "vendor_buy_bulk" {
		t.Fatalf("buy = %+v", result)
	}
	if result := buyer.Tick(state); !result.Done || result.Status != InteractionComplete {
		t.Fatalf("complete = %+v", result)
	}
	if len(in.modified) != 1 || in.modified[0] != "shift:right" {
		t.Fatalf("modified = %v", in.modified)
	}
}

func TestVendorBuyerSingleAndFailures(t *testing.T) {
	in := &shopInputMock{}
	buyer := NewVendorBuyer(in, VendorRequest{Code: "mp5", Mode: BuyModeSingle})
	state := vendorState()
	_ = buyer.Tick(state)
	_ = buyer.Tick(state)
	if len(in.clicks) != 1 || in.clicks[0] != input.MouseRight {
		t.Fatalf("clicks = %v", in.clicks)
	}
	bad := NewVendorBuyer(&shopInputMock{err: errors.New("blocked")}, VendorRequest{Type: "hpot", Mode: BuyModeBulk})
	if result := bad.Tick(state); !result.Done || result.Status != InteractionFailed {
		t.Fatalf("bad = %+v", result)
	}
}

func TestVendorBuyerRejectsMovedPinnedItem(t *testing.T) {
	in := &shopInputMock{}
	buyer := NewVendorBuyer(in, VendorRequest{Type: "hpot", Mode: BuyModeBulk})
	state := vendorState()
	if result := buyer.Tick(state); result.Action != "vendor_move" {
		t.Fatalf("move result = %+v", result)
	}
	state.Items[0].GridX++
	result := buyer.Tick(state)
	if result.Status != InteractionFailed || result.Reason != "vendor_item_position_changed" || len(in.modified) != 0 {
		t.Fatalf("changed-position result = %+v, modified = %v", result, in.modified)
	}
}
