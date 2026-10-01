package loot

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type collectionInputMock struct {
	modified, plain []CollectionPoint
	buttons         []input.MouseButton
	width           int
	err             error
}

func (m *collectionInputMock) ClickAt(x, y int, _ input.MouseButton) error {
	m.plain = append(m.plain, CollectionPoint{x, y})
	return m.err
}
func (m *collectionInputMock) ClickAtWithCtrlShift(x, y int, button input.MouseButton) error {
	m.modified = append(m.modified, CollectionPoint{x, y})
	m.buttons = append(m.buttons, button)
	return m.err
}
func (m *collectionInputMock) Window() (input.WindowInfo, bool) {
	w := m.width
	if w == 0 {
		w = 1280
	}
	return input.WindowInfo{ClientWidth: w, ClientHeight: 720}, true
}
func (m *collectionInputMock) Status() input.Status { return input.Status{Enabled: true} }

func collectionActionFixture() (*CollectionActions, *collectionInputMock, world.State) {
	m := &collectionInputMock{}
	// Arbitrary synthetic targets, explicitly not calibrated live coordinates.
	layout := CollectionLayout{Gems: CollectionTabLayout{Known: true, Select: CollectionPoint{100, 100}, Cells: map[string]CollectionPoint{"gsr": {200, 200}, "gpr": {200, 300}}}, Runes: CollectionTabLayout{Known: true, Select: CollectionPoint{400, 100}, Cells: map[string]CollectionPoint{"r01": {400, 200}}}, Cube: CollectionCubeLayout{Known: true, Left: 30, Top: 200, CellWidth: 20, CellHeight: 20, Columns: 3, Rows: 4, Transmute: CollectionPoint{100, 300}}, Settle: 100 * time.Millisecond}
	s := world.State{At: time.Unix(100, 0), Generation: 1, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), UI: world.UIState{StashOpen: true, InventoryOpen: true}, Items: []world.Item{{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true}}, Collection: world.CollectionState{ScopeID: "fixture", ScopeKnown: true, Generation: 1, ObservedAt: time.Unix(100, 0), Tab: world.StorageTabGems, TabKnown: true, Counts: []world.MaterialCount{{Code: "gsr", Count: 22, Known: true}, {Code: "gpr", Count: 5, Known: true}, {Code: "r01", Count: 5, Known: true}}}}
	return NewCollectionActions(slog.New(slog.NewTextHandler(io.Discard, nil)), m, layout), m, s
}
func nextCollectionFrame(s world.State) world.State {
	s.Generation++
	s.At = s.At.Add(100 * time.Millisecond)
	s.Collection.Generation = s.Generation
	s.Collection.ObservedAt = s.At
	return s
}

func TestCollectionActionsSelectSettleAndSingleTransfer(t *testing.T) {
	a, m, s := collectionActionFixture()
	// A matching fresh tab alone cannot authorize a material before the proven
	// delay; the calibrated default permits the transfer once both are present.
	layout := DefaultCollectionLayout()
	if layout.Settle != 300*time.Millisecond {
		t.Fatalf("live tab delay=%s", layout.Settle)
	}
	calibration := NewCollectionActions(a.log, m, layout)
	if err := calibration.SelectTab(s, world.StorageTabRunes); err != nil {
		t.Fatal(err)
	}
	calibrationState := nextCollectionFrame(s)
	calibrationState.Collection.Tab = world.StorageTabRunes
	if err := calibration.WithdrawOne(calibrationState, "r01"); err == nil || len(m.modified) != 0 {
		t.Fatal("transfer before calibrated delay")
	}
	calibrationState = nextCollectionFrame(nextCollectionFrame(calibrationState))
	if err := calibration.WithdrawOne(calibrationState, "r01"); err != nil {
		t.Fatal(err)
	}
	if len(m.modified) != 1 || m.modified[0] != layout.Runes.Cells["r01"] {
		t.Fatal("wrong calibrated source click")
	}
	m.modified = nil
	m.plain = nil
	if err := a.SelectTab(s, world.StorageTabRunes); err != nil {
		t.Fatal(err)
	}
	if len(m.plain) != 1 || m.plain[0] != (CollectionPoint{400, 100}) {
		t.Fatal("wrong tab click")
	}
	if err := a.WithdrawOne(s, "r01"); err == nil {
		t.Fatal("same-frame transfer accepted")
	}
	s = nextCollectionFrame(s)
	if err := a.WithdrawOne(s, "r01"); err == nil {
		t.Fatal("wrong active tab accepted")
	}
	s.Collection.Tab = world.StorageTabRunes
	if err := a.WithdrawOne(s, "r01"); err != nil {
		t.Fatal(err)
	}
	if len(m.modified) != 1 || m.modified[0] != (CollectionPoint{400, 200}) {
		t.Fatal("wrong source click")
	}
	if err := a.WithdrawOne(s, "r01"); err == nil || len(m.modified) != 1 {
		t.Fatal("repeated transfer")
	}
}

func TestCollectionActionsRejectRevokeAndUncalibratedLayout(t *testing.T) {
	for _, kind := range []string{"unknown_tab", "missing_cube", "stale", "scope", "cursor", "resolution", "zero", "missing_cell", "unsettled", "unknown_settle"} {
		t.Run(kind, func(t *testing.T) {
			a, m, s := collectionActionFixture()
			if err := a.SelectTab(s, world.StorageTabGems); err != nil {
				t.Fatal(err)
			}
			s = nextCollectionFrame(s)
			switch kind {
			case "unknown_tab":
				s.Collection.TabKnown = false
			case "missing_cube":
				s.Items = nil
			case "stale":
				s.Collection.Generation--
			case "scope":
				s.Collection.ScopeID = "other"
			case "cursor":
				s.Items = []world.Item{{Location: world.ItemLocationCursor}}
			case "resolution":
				m.width = 800
			case "zero":
				s.Collection.Counts[0].Count = 0
			case "missing_cell":
				delete(a.layout.Gems.Cells, "gsr")
			case "unsettled":
				s.At = s.At.Add(-time.Millisecond)
				s.Collection.ObservedAt = s.At
			case "unknown_settle":
				a.layout.Settle = 0
			}
			if err := a.WithdrawOne(s, "gsr"); err == nil {
				t.Fatal("unsafe source click accepted")
			}
			if len(m.modified)+len(m.plain) != 0 {
				t.Fatal("input after revoke")
			}
		})
	}
}

func TestCollectionActionsBoundOutputAndEmbeddedTransmute(t *testing.T) {
	a, m, s := collectionActionFixture()
	if err := a.SelectTab(s, world.StorageTabGems); err != nil {
		t.Fatal(err)
	}
	s = nextCollectionFrame(s)
	// The standalone Cube remains closed throughout the stash workflow.
	s.Items = append(s.Items, world.Item{UnitID: 40, Code: "gpr", Location: world.ItemLocationCube, PlayerOwned: true, Width: 1, Height: 1, GridX: 1, GridY: 2})
	if err := a.StoreOutput(s, 41, "gpr"); err == nil {
		t.Fatal("replacement output accepted")
	}
	if err := a.StoreOutput(s, 40, "gpr"); err != nil {
		t.Fatal(err)
	}
	if m.modified[0] != (CollectionPoint{60, 250}) {
		t.Fatalf("Cube target=%+v", m.modified[0])
	}
	s = nextCollectionFrame(s)
	if err := a.Transmute(s, []uint32{41}); err == nil {
		t.Fatal("foreign binding transmuted")
	}
	if err := a.Transmute(s, []uint32{40}); err != nil {
		t.Fatal(err)
	}
	if len(m.plain) != 1 {
		t.Fatal("expected only the embedded transmute click")
	}
}

func TestCollectionActionsOneRightClickIngredientSet(t *testing.T) {
	a, m, s := collectionActionFixture()
	if err := a.SelectTab(s, world.StorageTabGems); err != nil {
		t.Fatal(err)
	}
	s = nextCollectionFrame(s)
	if !a.TabReady(s, world.StorageTabGems) {
		t.Fatal("settled tab unavailable")
	}
	if err := a.WithdrawIngredients(s, "gsr"); err != nil {
		t.Fatal(err)
	}
	if len(m.modified) != 1 || m.buttons[0] != input.MouseRight {
		t.Fatal("ingredient set did not use one right click")
	}
	if err := a.WithdrawIngredients(s, "gsr"); err == nil || len(m.modified) != 1 {
		t.Fatal("duplicate ingredient click")
	}
	s = nextCollectionFrame(s)
	s.Items = append(s.Items, world.Item{UnitID: 40, Code: "gsr", Location: world.ItemLocationCube})
	if err := a.WithdrawIngredients(s, "gsr"); err == nil || len(m.modified) != 1 {
		t.Fatal("loaded into occupied Cube")
	}
}

func TestCollectionActionsSenderFailureConsumesFrameAndReset(t *testing.T) {
	a, m, s := collectionActionFixture()
	if err := a.SelectTab(s, world.StorageTabGems); err != nil {
		t.Fatal(err)
	}
	s = nextCollectionFrame(s)
	m.err = errors.New("partial send")
	if err := a.WithdrawOne(s, "gsr"); err == nil {
		t.Fatal("sender error lost")
	}
	if err := a.WithdrawOne(s, "gsr"); err == nil || len(m.modified) != 1 {
		t.Fatal("partial send repeated")
	}
	a.Reset()
	if len(m.modified) != 1 || a.scope != "" || a.lastAction != 0 {
		t.Fatal("reset sent input or retained binding")
	}
}
