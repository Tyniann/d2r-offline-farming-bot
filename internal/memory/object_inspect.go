package memory

import (
	"fmt"
)

const maxObjectInspectUnits = 512

// ObjectInspectEvidence is one raw object observation for Gate 23.0.
// The walk is not filtered by the productive runtime allowlist, so Supertruhen
// can appear before any product ID is checked in.
type ObjectInspectEvidence struct {
	TxtFileNo     uint32 `json:"txt_file_no"`
	UnitID        uint32 `json:"unit_id"`
	PosX          uint32 `json:"pos_x"`
	PosY          uint32 `json:"pos_y"`
	PositionKnown bool   `json:"position_known"`
	Mode          uint32 `json:"mode"`
	ModeKnown     bool   `json:"mode_known"`
	Hovered       bool   `json:"hovered"`
}

// CollectObjectInspectEvidence walks the object unit-table segment once and
// returns every readable object unit, including IDs outside the productive
// allowlist. Mode uses the same UnitAny offset as hirelings and cows.
func (p *ProbeReader) CollectObjectInspectEvidence() ([]ObjectInspectEvidence, error) {
	if p == nil || p.reader == nil || p.reader.access == nil {
		return nil, fmt.Errorf("collect object inspect evidence: reader not attached")
	}
	moduleBase := p.reader.access.ModuleBase()
	if moduleBase == 0 {
		return nil, fmt.Errorf("collect object inspect evidence: module base unavailable")
	}
	off := p.ensureOffsets(moduleBase)
	if off.UnitTable == 0 {
		return nil, errUnitTableUnavailable
	}

	hover := p.readHover(moduleBase, off)
	out := make([]ObjectInspectEvidence, 0)
	visited := 0
	err := p.walkUnitSegment(moduleBase, off, unitSegmentObject, &visited, 0, func(unitAddr uintptr) (unitWalkAction, error) {
		if len(out) >= maxObjectInspectUnits {
			return unitWalkStop, nil
		}
		evidence, ok := p.readObjectInspectEvidence(unitAddr, off, hover)
		if !ok {
			return unitWalkContinue, nil
		}
		out = append(out, evidence)
		return unitWalkContinue, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect object inspect evidence: %w", err)
	}
	return out, nil
}

func (p *ProbeReader) readObjectInspectEvidence(unitAddr uintptr, off OffsetSet, hover HoverState) (ObjectInspectEvidence, bool) {
	unitType, err := p.reader.ReadUint32(unitAddr + unitOffsetUnitType)
	if err != nil || unitType != unitTypeObject {
		return ObjectInspectEvidence{}, false
	}

	rawTxt, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
	if err != nil {
		return ObjectInspectEvidence{}, false
	}
	txtFileNo := rawTxt & 0xFFFF
	if txtFileNo == 0 {
		return ObjectInspectEvidence{}, false
	}

	unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
	if err != nil {
		return ObjectInspectEvidence{}, false
	}

	ev := ObjectInspectEvidence{
		TxtFileNo: txtFileNo,
		UnitID:    unitID,
		Hovered:   hover.IsHovered && hover.UnitType == HoverUnitTypeObject && hover.UnitID == unitID,
	}

	if mode, modeErr := p.reader.ReadUint32(unitAddr + unitOffsetMode); modeErr == nil {
		ev.Mode = mode
		ev.ModeKnown = true
	}

	pathPtr, pathErr := p.reader.ReadUint64(unitAddr + off.Unit.Path)
	if pathErr != nil || pathPtr == 0 {
		return ev, true
	}
	posX, xErr := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectX)
	posY, yErr := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectY)
	if xErr != nil || yErr != nil {
		return ev, true
	}
	ev.PosX = uint32(posX)
	ev.PosY = uint32(posY)
	ev.PositionKnown = true
	return ev, true
}
