package memory

import "fmt"

const (
	itemUnitType = 4

	itemOffsetRawLocation = 0x0C

	itemDataOffsetQuality     = 0x00
	itemDataOffsetOwnerID     = 0x0C
	itemDataOffsetFlags       = 0x18
	itemDataOffsetUniqueSetID = 0x34
	itemDataOffsetPage        = 0x55

	itemFlagIdentified = 0x10
	itemFlagEthereal   = 0x400000
	// itemFlagSocketed is the live-confirmed ItemData.Flags bit from Gate 19.0
	// (2026-07-31, D2R 3.2.92777): set on socketed whites/grays and runewords,
	// clear on unsocketed controls such as Skullder's Ire. See docs/features/socket-pickit.md.
	itemFlagSocketed = 0x800

	itemRawLocationInventory = 0
	itemRawLocationEquipped  = 1
	itemRawLocationBelt      = 2
	itemRawLocationGround    = 3
	itemRawLocationCursor    = 4
	itemRawLocationDropping  = 5
	itemRawLocationSocket    = 6

	itemOwnerPlayerSentinel = 1
)

func (p *ProbeReader) enumerateItems(moduleBase uintptr, off OffsetSet, snap *Snapshot) error {
	visited := 0
	snap.Items = make([]ItemUnit, 0)

	return p.walkUnitSegment(moduleBase, off, unitSegmentItem, &visited, maxItemUnitVisits, func(unitAddr uintptr) (unitWalkAction, error) {
		if len(snap.Items) >= maxItemsPerSnapshot {
			return unitWalkStop, nil
		}

		itemUnit, ok := p.readItemUnit(unitAddr, off, snap.PlayerUnitID)
		if !ok {
			return unitWalkContinue, nil
		}

		snap.Items = append(snap.Items, itemUnit)
		return unitWalkContinue, nil
	})
}

func (p *ProbeReader) readItemUnit(unitAddr uintptr, off OffsetSet, mainPlayerUnitID uint32) (ItemUnit, bool) {
	unitType, err := p.reader.ReadUint32(unitAddr + unitOffsetUnitType)
	if err != nil || unitType != itemUnitType {
		return ItemUnit{}, false
	}

	rawLocation, err := p.reader.ReadUint32(unitAddr + itemOffsetRawLocation)
	if err != nil {
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
	ownerID, err := p.reader.ReadUint32(uintptr(unitData) + itemDataOffsetOwnerID)
	if err != nil {
		return ItemUnit{}, false
	}
	page, err := p.reader.ReadUint8(uintptr(unitData) + itemDataOffsetPage)
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
	stats, socketActive, socketBase := p.readItemStats(unitAddr, off)
	sockets, socketsAvailable, socketed := decodeItemSockets(flags, socketActive, socketBase)
	// Die Set-/Unique-Referenz ist eine optionale Diagnosequelle. Ein einzelner
	// fehlgeschlagener Read darf das ansonsten konsistente Item nicht verwerfen.
	uniqueSetIDRaw, uniqueSetIDErr := p.reader.ReadUint32(uintptr(unitData) + itemDataOffsetUniqueSetID)

	return ItemUnit{
		TxtFileNo:            txtFileNo,
		UnitID:               unitID,
		Quality:              quality,
		UniqueSetID:          int32(uniqueSetIDRaw),
		UniqueSetIDAvailable: uniqueSetIDErr == nil,
		RawLocation:          rawLocation,
		OwnerID:              ownerID,
		PlayerOwned:          isPlayerOwnedItem(ownerID, mainPlayerUnitID),
		Page:                 uint32(page),
		GridX:                uint32(posX),
		GridY:                uint32(posY),
		PosX:                 uint32(posX),
		PosY:                 uint32(posY),
		Flags:                flags,
		Identified:           flags&itemFlagIdentified != 0,
		Ethereal:             flags&itemFlagEthereal != 0,
		Stats:                stats,
		SocketStatActive:     socketActive,
		SocketStatBase:       socketBase,
		Sockets:              sockets,
		SocketsAvailable:     socketsAvailable,
		Socketed:             socketed,
	}, true
}

// readItemStats liest Active und Base jeweils höchstens einmal. Die produktive
// Stats-Liste bevorzugt weiterhin Active bei erfolgreichem Parse und fällt nur
// dann auf Base zurück. Socket-Evidenz wird unabhängig aus beiden Listen gewonnen,
// damit Gate 19.0 einen nur in Base vorhandenen Stat 194 nicht verdeckt.
func (p *ProbeReader) readItemStats(unitAddr uintptr, off OffsetSet) ([]RawStat, SocketStatEvidence, SocketStatEvidence) {
	statsListEx, err := p.reader.ReadUint64(unitAddr + off.Unit.StatsListEx)
	if err != nil || statsListEx == 0 {
		return nil, SocketStatEvidence{}, SocketStatEvidence{}
	}

	activeHeader := uintptr(statsListEx) + off.Unit.StatsListActive
	activeStats, activeErr := parseRawStats(p.reader, activeHeader, off.Stats)
	activeEvidence := socketStatEvidenceFrom(activeStats, activeErr)

	baseHeader := uintptr(statsListEx) + off.Unit.StatsListBase
	baseStats, baseErr := parseRawStats(p.reader, baseHeader, off.Stats)
	baseEvidence := socketStatEvidenceFrom(baseStats, baseErr)

	if activeErr == nil {
		return activeStats, activeEvidence, baseEvidence
	}
	if baseErr == nil {
		return baseStats, activeEvidence, baseEvidence
	}
	p.reader.log.Debug("item stats read failed",
		"unit_addr", fmt.Sprintf("0x%X", unitAddr),
		"active_error", activeErr,
		"base_error", baseErr,
	)
	return nil, activeEvidence, baseEvidence
}

func socketStatEvidenceFrom(stats []RawStat, err error) SocketStatEvidence {
	if err != nil {
		return SocketStatEvidence{}
	}
	ev := SocketStatEvidence{ListReadable: true}
	for _, stat := range stats {
		if stat.Layer != 0 || stat.ID != StatNumSockets {
			continue
		}
		ev.Present = true
		ev.Value = stat.Value
		return ev
	}
	return ev
}

// decodeItemSockets applies the Gate-19.0 consistency table.
// Stat 194 is taken from Active if present, otherwise Base; a successful Active
// parse without Stat 194 must not hide a Base-only value. Missing, unreadable,
// out-of-range, or Flag/Stat contradictions stay unavailable (fail-closed).
func decodeItemSockets(flags uint32, active, base SocketStatEvidence) (sockets int, available bool, socketed bool) {
	flagOn := flags&itemFlagSocketed != 0
	value, present := resolveSocketStatValue(active, base)
	if !present {
		return 0, false, false
	}
	if value < 0 || value > 6 {
		return 0, false, false
	}
	if value == 0 {
		if flagOn {
			return 0, false, false
		}
		// Explicit live null case (not observed on 2026-07-31; kept for completeness).
		return 0, true, false
	}
	if !flagOn {
		return 0, false, false
	}
	return int(value), true, true
}

func resolveSocketStatValue(active, base SocketStatEvidence) (value int32, present bool) {
	if active.Present && base.Present && active.Value != base.Value {
		return 0, false
	}
	if active.Present {
		return active.Value, true
	}
	if base.Present {
		return base.Value, true
	}
	return 0, false
}

// FormatSocketStatEvidence returns a stable Gate-19.0 log token for one list.
func FormatSocketStatEvidence(ev SocketStatEvidence) string {
	switch {
	case !ev.ListReadable:
		return "unreadable"
	case !ev.Present:
		return "absent"
	default:
		return fmt.Sprintf("value:%d", ev.Value)
	}
}

func isPlayerOwnedItem(ownerID, mainPlayerUnitID uint32) bool {
	return ownerID == itemOwnerPlayerSentinel || (mainPlayerUnitID != 0 && ownerID == mainPlayerUnitID)
}
