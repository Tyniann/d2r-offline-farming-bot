package memory

// GamePhase describes high-level client state read from UI gate and loading flags.
// The zero value is GamePhaseUnknown. Conversion to world.GamePhase uses world.mapPhase only.
type GamePhase int

// GamePhase values parallel world.GamePhase ordering for stable mapPhase tests.
const (
	GamePhaseUnknown GamePhase = iota
	GamePhaseMenu
	GamePhaseLoading
	GamePhaseInGame
)

// String returns a stable label for structured logging.
func (p GamePhase) String() string {
	switch p {
	case GamePhaseMenu:
		return "menu"
	case GamePhaseLoading:
		return "loading"
	case GamePhaseInGame:
		return "in_game"
	default:
		return "unknown"
	}
}

// readPhaseInputs reads the in-game gate byte and loading-screen flag from the UI buffer.
// Loading is read whenever off.UI != 0, independent of the gate offset.
func (p *ProbeReader) readPhaseInputs(moduleBase uintptr, off OffsetSet) (gateValue uint8, gateDisabled, loading bool, ui UIState) {
	gate := off.InGameGateOffset()
	if gate == 0 {
		gateDisabled = true
	}

	if off.UI == 0 {
		return gateValue, gateDisabled, false, UIState{}
	}

	uiBase := moduleBase + off.UI - uiBufferBefore
	buf, err := p.reader.ReadBytes(uiBase, uiBufferSize)
	if err != nil || len(buf) <= uiLoadingIndex {
		return gateValue, gateDisabled, false, UIState{}
	}
	gateValue = buf[uiGateIndex]
	loading = buf[uiLoadingIndex] != 0
	ui.InventoryOpen = buf[uiInventoryIndex] != 0
	ui.StashOpen = buf[uiStashIndex] != 0
	return gateValue, gateDisabled, loading, ui
}

// finalizePhase resolves GamePhase after player discovery.
// Valid and Phase are orthogonal: loading overrides in_game even when the player is readable.
func finalizePhase(gateValue uint8, gateDisabled, loading, playerFound bool) GamePhase {
	if loading {
		return GamePhaseLoading
	}
	if gateDisabled {
		if playerFound {
			return GamePhaseInGame
		}
		return GamePhaseUnknown
	}
	if gateValue != 1 {
		if playerFound {
			return GamePhaseInGame
		}
		return GamePhaseMenu
	}
	if playerFound {
		return GamePhaseInGame
	}
	return GamePhaseMenu
}
