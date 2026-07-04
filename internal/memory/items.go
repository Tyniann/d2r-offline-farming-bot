package memory

import "fmt"

const (
	itemUnitType = 4

	itemOffsetRawLocation = 0x0C

	itemDataOffsetQuality = 0x00
	itemDataOffsetFlags   = 0x18

	itemFlagIdentified = 0x10
	itemFlagEthereal   = 0x400000

	itemRawLocationGround   = 3
	itemRawLocationDropping = 5
)

func (p *ProbeReader) enumerateItems(moduleBase uintptr, off OffsetSet, snap *Snapshot) error {
	visited := 0
	snap.Items = make([]ItemUnit, 0)

	return p.walkUnitSegment(moduleBase, off, unitSegmentItem, &visited, maxItemUnitVisits, func(unitAddr uintptr) (unitWalkAction, error) {
		if len(snap.Items) >= maxItemsPerSnapshot {
			return unitWalkStop, nil
		}

		itemUnit, ok := p.readGroundItemUnit(unitAddr, off)
		if !ok {
			return unitWalkContinue, nil
		}

		snap.Items = append(snap.Items, itemUnit)
		return unitWalkContinue, nil
	})
}

func (p *ProbeReader) readGroundItemUnit(unitAddr uintptr, off OffsetSet) (ItemUnit, bool) {
	unitType, err := p.reader.ReadUint32(unitAddr + unitOffsetUnitType)
	if err != nil || unitType != itemUnitType {
		return ItemUnit{}, false
	}

	rawLocation, err := p.reader.ReadUint32(unitAddr + itemOffsetRawLocation)
	if err != nil || !isGroundItemLocation(rawLocation) {
		return ItemUnit{}, false
	}

	txtFileNo, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
	if err != nil {
		return ItemUnit{}, false
	}
	unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
	if err != nil {
		return ItemUnit{}, false
	}

	unitData, err := p.reader.ReadUint64(unitAddr + unitOffsetUnitData)
	if err != nil || unitData == 0 {
		return ItemUnit{}, false
	}
	quality, err := p.reader.ReadUint32(uintptr(unitData) + itemDataOffsetQuality)
	if err != nil {
		return ItemUnit{}, false
	}
	flags, err := p.reader.ReadUint32(uintptr(unitData) + itemDataOffsetFlags)
	if err != nil {
		return ItemUnit{}, false
	}
	pathPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Path)
	if err != nil || pathPtr == 0 {
		return ItemUnit{}, false
	}
	posX, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectX)
	if err != nil {
		return ItemUnit{}, false
	}
	posY, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectY)
	if err != nil {
		return ItemUnit{}, false
	}

	stats := p.readItemStats(unitAddr, off)

	return ItemUnit{
		TxtFileNo:   txtFileNo,
		UnitID:      unitID,
		Quality:     quality,
		RawLocation: rawLocation,
		PosX:        uint32(posX),
		PosY:        uint32(posY),
		Flags:       flags,
		Identified:  flags&itemFlagIdentified != 0,
		Ethereal:    flags&itemFlagEthereal != 0,
		Stats:       stats,
	}, true
}

func (p *ProbeReader) readItemStats(unitAddr uintptr, off OffsetSet) []RawStat {
	statsListEx, err := p.reader.ReadUint64(unitAddr + off.Unit.StatsListEx)
	if err != nil || statsListEx == 0 {
		return nil
	}

	activeHeader := uintptr(statsListEx) + off.Unit.StatsListActive
	stats, err := parseRawStats(p.reader, activeHeader, off.Stats)
	if err == nil {
		return stats
	}

	baseHeader := uintptr(statsListEx) + off.Unit.StatsListBase
	stats, baseErr := parseRawStats(p.reader, baseHeader, off.Stats)
	if baseErr != nil {
		p.reader.log.Debug("item stats read failed",
			"unit_addr", fmt.Sprintf("0x%X", unitAddr),
			"active_error", err,
			"base_error", baseErr,
		)
		return nil
	}
	return stats
}

func isGroundItemLocation(raw uint32) bool {
	return raw == itemRawLocationGround || raw == itemRawLocationDropping
}
