package memory

import "encoding/binary"

// Hover unit-type values as reported by the D2R hover buffer (d2go HoveredData).
const (
	HoverUnitTypeMonster  uint32 = 1
	HoverUnitTypeObject   uint32 = 2
	HoverUnitTypeItem     uint32 = 4
	HoverUnitTypeEntrance uint32 = 5
)

// hoverBufferSize is the size of the raw hover buffer at moduleBase+Hover.
const hoverBufferSize = 12

// HoverState mirrors the D2R hover buffer (12 bytes at moduleBase+Hover):
// uint16 hovered flag at +0x00, uint32 unit type at +0x04, uint32 unit ID at +0x08.
// The zero value means nothing is hovered; UnitType and UnitID are only
// meaningful when IsHovered is true.
type HoverState struct {
	IsHovered bool
	UnitType  uint32
	UnitID    uint32
}

// parseHoverBuffer decodes a raw 12-byte hover buffer into a HoverState.
// Buffers that are too short or not hovered yield the zero HoverState.
func parseHoverBuffer(buf []byte) HoverState {
	if len(buf) < hoverBufferSize {
		return HoverState{}
	}
	if binary.LittleEndian.Uint16(buf[0:2]) == 0 {
		return HoverState{}
	}
	return HoverState{
		IsHovered: true,
		UnitType:  binary.LittleEndian.Uint32(buf[4:8]),
		UnitID:    binary.LittleEndian.Uint32(buf[8:12]),
	}
}

// readHover reads the hover buffer once per tick. A zero Hover offset or a
// failed read yields the zero HoverState (fail-open for reading; consumers
// that require hover confirmation must treat it as "not hovered").
func (p *ProbeReader) readHover(moduleBase uintptr, off OffsetSet) HoverState {
	if off.Hover == 0 {
		return HoverState{}
	}
	buf, err := p.reader.ReadBytes(moduleBase+off.Hover, hoverBufferSize)
	if err != nil {
		p.reader.log.Debug("hover read failed", "error", err)
		return HoverState{}
	}
	return parseHoverBuffer(buf)
}
