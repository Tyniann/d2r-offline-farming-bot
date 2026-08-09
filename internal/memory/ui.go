package memory

import (
	"fmt"
	"time"
)

// UIState is the read-only subset of D2R menu flags needed for safe recovery actions.
type UIState struct {
	InventoryOpen   bool
	NPCInteractOpen bool
	NPCShopOpen     bool
	WaypointOpen    bool
	StashOpen       bool
	QuitMenuOpen    bool
	CubeOpen        bool
	CubeOpenKnown   bool
}

// UIBufferCapture is one read-only copy of the D2R UI buffer around the
// signature-resolved UI anchor. It is intended for controlled state research,
// not as a gameplay decision source.
type UIBufferCapture struct {
	At     time.Time
	Bytes  []byte
	Anchor int
}

// OffsetFromAnchor converts a capture index into its signed offset relative to
// the signature-resolved UI anchor.
func (c UIBufferCapture) OffsetFromAnchor(index int) int {
	return index - c.Anchor
}

const (
	// uiBuffer starts at UI-0x13. These indices were calibrated read-only
	// against D2R 3.2.92777 for closed, inventory-only, and personal-stash UI.
	uiInventoryIndex   = 0x00
	uiNPCInteractIndex = 0x07 // UI-0x0C; d2go OpenMenus.NPCInteract.
	uiNPCShopIndex     = 0x0A // UI-0x09; d2go OpenMenus.NPCShop.
	uiQuitMenuIndex    = 0x08 // UI-0x0B, live-validated in Phase 7.1.
	uiGateIndex        = 0x09 // UI-0x0A
	uiStashIndex       = 0x17 // UI+0x04
	uiCubeIndex        = 0x18 // UI+0x05; live-validated in Phase 20.0.
	uiWaypointIndex    = 0x1C // UI+0x09; d2go OpenMenus.Waypoint.
	uiLoadingIndex     = 0x171
	uiBufferBefore     = 0x13
	uiBufferSize       = 0x172

	// The Phase-20.0 research window deliberately remains diagnostic. It is
	// wider than the semantically validated UI buffer so live closed/open
	// captures can locate a relocated Cube flag without guessing an offset.
	uiResearchBufferBefore = 0x8000
	uiResearchBufferSize   = 0x10000
)

// CaptureUIBuffer reads the complete diagnostic UI window without producing
// input. Callers must treat individual bytes as unknown until live validation.
func (p *ProbeReader) CaptureUIBuffer() (UIBufferCapture, error) {
	return p.captureUIBuffer(uiBufferBefore, uiBufferSize)
}

// CaptureUIResearchBuffer reads a wider read-only window around the resolved
// UI anchor. Its bytes are research evidence only and never authorize input.
func (p *ProbeReader) CaptureUIResearchBuffer() (UIBufferCapture, error) {
	return p.captureUIBuffer(uiResearchBufferBefore, uiResearchBufferSize)
}

func (p *ProbeReader) captureUIBuffer(before, size int) (UIBufferCapture, error) {
	if p == nil || p.reader == nil || p.reader.access == nil {
		return UIBufferCapture{}, fmt.Errorf("capture UI buffer: reader not attached")
	}
	moduleBase := p.reader.access.ModuleBase()
	if moduleBase == 0 {
		return UIBufferCapture{}, fmt.Errorf("capture UI buffer: module base unavailable")
	}
	off := p.ensureOffsets(moduleBase)
	if off.UI < uintptr(before) {
		return UIBufferCapture{}, fmt.Errorf("capture UI buffer: UI offset unavailable")
	}
	buf, err := p.reader.ReadBytes(moduleBase+off.UI-uintptr(before), size)
	if err != nil {
		return UIBufferCapture{}, fmt.Errorf("capture UI buffer: %w", err)
	}
	if len(buf) != size {
		return UIBufferCapture{}, fmt.Errorf("capture UI buffer: got %d bytes, want %d", len(buf), size)
	}
	return UIBufferCapture{At: time.Now(), Bytes: append([]byte(nil), buf...), Anchor: before}, nil
}
