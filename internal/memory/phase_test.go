package memory

import "testing"

func TestFinalizePhaseLoadingOverridesInGame(t *testing.T) {
	got := finalizePhase(1, false, true, true)
	if got != GamePhaseLoading {
		t.Fatalf("Phase = %s, want loading", got)
	}
}

func TestFinalizePhaseGateDisabledUsesPlayer(t *testing.T) {
	if got := finalizePhase(0, true, false, true); got != GamePhaseInGame {
		t.Fatalf("player found, gate disabled: Phase = %s, want in_game", got)
	}
	if got := finalizePhase(0, true, false, false); got != GamePhaseUnknown {
		t.Fatalf("no player, gate disabled: Phase = %s, want unknown", got)
	}
}

func TestFinalizePhaseGateZeroPlayerReadableUsesPlayerHeuristic(t *testing.T) {
	got := finalizePhase(0, false, false, true)
	if got != GamePhaseInGame {
		t.Fatalf("gate=0 with player: Phase = %s, want in_game", got)
	}
}

func TestFinalizePhaseMenuWhenGateClosedNoPlayer(t *testing.T) {
	got := finalizePhase(0, false, false, false)
	if got != GamePhaseMenu {
		t.Fatalf("Phase = %s, want menu", got)
	}
}

func TestFinalizePhaseLoadingWithNoPlayer(t *testing.T) {
	got := finalizePhase(1, false, true, false)
	if got != GamePhaseLoading {
		t.Fatalf("Phase = %s, want loading", got)
	}
}

func TestReadPhaseInputsLoadingIndependentOfGate(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	uiBase := moduleBase + off.UI - uiBufferBefore
	buf := make([]byte, uiBufferSize)
	buf[uiLoadingIndex] = 1
	access.setBytes(uiBase, buf)

	_, _, loading, _ := probe.readPhaseInputs(moduleBase, off)
	if !loading {
		t.Fatal("expected loading=true from UI buffer")
	}
}

func TestReadPhaseInputsGateDisabled(t *testing.T) {
	access := newMockAccess()
	access.moduleBase = 0x10000000
	off := testOffsetSet()
	off.UI = 0 // InGameGateOffset() == 0

	reader := newTestReader(access)
	reader.Bind(access)
	probe := NewProbeReader(reader, off)

	_, gateDisabled, _, _ := probe.readPhaseInputs(access.moduleBase, off)
	if !gateDisabled {
		t.Fatal("expected gate disabled when UI offset is zero")
	}
}

func TestReadPhaseInputsInventoryAndStashFlags(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	uiBase := moduleBase + off.UI - uiBufferBefore
	buf := make([]byte, uiBufferSize)
	buf[uiInventoryIndex] = 1
	buf[uiNPCInteractIndex] = 1
	buf[uiNPCShopIndex] = 1
	buf[uiWaypointIndex] = 1
	buf[uiQuitMenuIndex] = 1
	buf[uiStashIndex] = 1
	access.setBytes(uiBase, buf)

	_, _, _, ui := probe.readPhaseInputs(moduleBase, off)
	if !ui.InventoryOpen || !ui.NPCInteractOpen || !ui.NPCShopOpen || !ui.WaypointOpen || !ui.StashOpen || !ui.QuitMenuOpen {
		t.Fatalf("UI = %+v, want inventory, NPC interaction, shop, waypoint, stash, and quit menu open", ui)
	}
}

func TestReadPhaseInputsInventoryDoesNotImplyStash(t *testing.T) {
	access, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()
	uiBase := moduleBase + off.UI - uiBufferBefore
	buf := make([]byte, uiBufferSize)
	buf[uiInventoryIndex] = 1
	access.setBytes(uiBase, buf)

	_, _, _, ui := probe.readPhaseInputs(moduleBase, off)
	if !ui.InventoryOpen || ui.StashOpen {
		t.Fatalf("UI = %+v, want inventory open and stash closed", ui)
	}
}

func TestReadPhaseInputsCubeOpenTruthTable(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value byte
		open  bool
		known bool
	}{
		{name: "closed", value: 0, known: true},
		{name: "open", value: 1, open: true, known: true},
		{name: "unexpected byte", value: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			access, probe, moduleBase := setupProbeMock(t)
			off := testOffsetSet()
			buf := make([]byte, uiBufferSize)
			buf[uiCubeIndex] = tt.value
			access.setBytes(moduleBase+off.UI-uiBufferBefore, buf)
			_, _, _, ui := probe.readPhaseInputs(moduleBase, off)
			if ui.CubeOpen != tt.open || ui.CubeOpenKnown != tt.known {
				t.Fatalf("UI=%+v", ui)
			}
		})
	}
}

func TestReadPhaseInputsCubeOpenUnavailableOnReadFailure(t *testing.T) {
	access := newMockAccess()
	access.moduleBase = 0x10000000
	off := testOffsetSet()
	reader := newTestReader(access)
	reader.Bind(access)
	_, _, _, ui := NewProbeReader(reader, off).readPhaseInputs(access.moduleBase, off)
	if ui.CubeOpen || ui.CubeOpenKnown {
		t.Fatalf("UI=%+v, want unavailable Cube state", ui)
	}
}

// TestProbeSnapshotLoadingBlocksEntities is covered by TestProbeSnapshotEntitiesOnlyWhenInGame in probe_test.go.
