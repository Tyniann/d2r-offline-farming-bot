package memory

import "time"

const (
	unitOffsetTxtFileNo = 0x04
	unitOffsetUnitType  = 0x00
	unitOffsetCorpse    = 0x1AE
	unitOffsetUnitData  = 0x10
	unitDataMonsterFlag = 0x1A

	pathOffsetMonsterX = 0x02
	pathOffsetMonsterY = 0x06
	pathOffsetObjectX  = 0x10
	pathOffsetObjectY  = 0x14

	unitTypeObject   = 2
	unitTypeEntrance = 5
)

func (p *ProbeReader) enumerateEntities(moduleBase uintptr, off OffsetSet, snap *Snapshot) error {
	visited := 0
	snap.Objects = make([]ObjectUnit, 0)
	snap.Entrances = make([]EntranceUnit, 0)
	snap.Monsters = make([]MonsterUnit, 0)

	// Entrances and monsters before objects: the object segment is large and would
	// exhaust maxTotalUnitVisits before smaller segments are walked (d2go walks each
	// segment independently without a shared cap).
	if err := p.enumerateEntrances(moduleBase, off, &visited, snap); err != nil {
		return err
	}
	if err := p.enumerateMonsters(moduleBase, off, &visited, snap); err != nil {
		return err
	}
	if err := p.enumerateObjects(moduleBase, off, &visited, snap); err != nil {
		return err
	}
	return nil
}

func (p *ProbeReader) enumerateObjects(moduleBase uintptr, off OffsetSet, visited *int, snap *Snapshot) error {
	return p.walkUnitSegment(moduleBase, off, unitSegmentObject, visited, 0, func(unitAddr uintptr) (unitWalkAction, error) {
		if len(snap.Objects) >= maxEntitiesPerCategory {
			return unitWalkContinue, nil
		}

		rawTxt, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
		if err != nil {
			return unitWalkContinue, nil
		}
		txtFileNo := rawTxt & 0xFFFF
		if !IsRuntimeObjectID(txtFileNo) {
			return unitWalkContinue, nil
		}

		unitType, err := p.reader.ReadUint32(unitAddr + unitOffsetUnitType)
		if err != nil || unitType != unitTypeObject {
			return unitWalkContinue, nil
		}

		unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
		if err != nil {
			return unitWalkContinue, nil
		}

		unitData, err := p.reader.ReadUint64(unitAddr + unitOffsetUnitData)
		if err != nil || unitData == 0 {
			return unitWalkContinue, nil
		}

		pathPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Path)
		if err != nil || pathPtr == 0 {
			return unitWalkContinue, nil
		}

		posX, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectX)
		if err != nil {
			return unitWalkContinue, nil
		}
		posY, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectY)
		if err != nil {
			return unitWalkContinue, nil
		}

		snap.Objects = append(snap.Objects, ObjectUnit{
			TxtFileNo: txtFileNo,
			UnitID:    unitID,
			PosX:      uint32(posX),
			PosY:      uint32(posY),
		})
		return unitWalkContinue, nil
	})
}

func (p *ProbeReader) enumerateEntrances(moduleBase uintptr, off OffsetSet, visited *int, snap *Snapshot) error {
	return p.walkUnitSegment(moduleBase, off, unitSegmentEntrance, visited, maxVisitsEntranceSegment, func(unitAddr uintptr) (unitWalkAction, error) {
		if len(snap.Entrances) >= maxEntitiesPerCategory {
			return unitWalkContinue, nil
		}

		unitType, err := p.reader.ReadUint32(unitAddr + unitOffsetUnitType)
		if err != nil || unitType != unitTypeEntrance {
			return unitWalkContinue, nil
		}

		txtFileNo, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
		if err != nil || !IsCountessEntranceID(txtFileNo) {
			return unitWalkContinue, nil
		}

		unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
		if err != nil {
			return unitWalkContinue, nil
		}

		pathPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Path)
		if err != nil || pathPtr == 0 {
			return unitWalkContinue, nil
		}

		posX, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectX)
		if err != nil {
			return unitWalkContinue, nil
		}
		posY, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetObjectY)
		if err != nil {
			return unitWalkContinue, nil
		}

		snap.Entrances = append(snap.Entrances, EntranceUnit{
			TxtFileNo: txtFileNo,
			UnitID:    unitID,
			PosX:      uint32(posX),
			PosY:      uint32(posY),
		})
		return unitWalkContinue, nil
	})
}

func (p *ProbeReader) enumerateMonsters(moduleBase uintptr, off OffsetSet, visited *int, snap *Snapshot) error {
	segmentLimit := maxTotalUnitVisits - *visited
	if segmentLimit < 1 {
		segmentLimit = 1
	}
	return p.walkUnitSegment(moduleBase, off, unitSegmentMonster, visited, segmentLimit, func(unitAddr uintptr) (unitWalkAction, error) {
		if len(snap.Monsters) >= maxEntitiesPerCategory {
			return unitWalkContinue, nil
		}

		corpse, err := p.reader.ReadUint8(unitAddr + unitOffsetCorpse)
		if err != nil || corpse != 0 {
			return unitWalkContinue, nil
		}

		txtFileNo, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
		if err != nil {
			return unitWalkContinue, nil
		}

		unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
		if err != nil {
			return unitWalkContinue, nil
		}

		unitData, err := p.reader.ReadUint64(unitAddr + unitOffsetUnitData)
		if err != nil || unitData == 0 {
			return unitWalkContinue, nil
		}

		flag, err := p.reader.ReadUint8(uintptr(unitData) + unitDataMonsterFlag)
		if err != nil {
			return unitWalkContinue, nil
		}
		if !IsRuntimeMonsterCandidate(txtFileNo, flag) {
			return unitWalkContinue, nil
		}

		pathPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Path)
		if err != nil || pathPtr == 0 {
			return unitWalkContinue, nil
		}

		posX, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetMonsterX)
		if err != nil {
			return unitWalkContinue, nil
		}
		posY, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetMonsterY)
		if err != nil {
			return unitWalkContinue, nil
		}

		snap.Monsters = append(snap.Monsters, MonsterUnit{
			NPCID:           txtFileNo,
			UnitID:          unitID,
			PosX:            uint32(posX),
			PosY:            uint32(posY),
			MonsterTypeFlag: flag,
		})
		return unitWalkContinue, nil
	})
}

// emptyEntitySlices returns a snapshot with non-nil empty entity slices for stable fingerprints.
func emptyEntitySlices(snap Snapshot) Snapshot {
	snap.Objects = make([]ObjectUnit, 0)
	snap.Entrances = make([]EntranceUnit, 0)
	snap.Monsters = make([]MonsterUnit, 0)
	snap.Items = make([]ItemUnit, 0)
	return snap
}

func invalidSnapshot(now time.Time, phase GamePhase, reason string) Snapshot {
	snap := Snapshot{At: now, Valid: false, Reason: reason, Phase: phase}
	return emptyEntitySlices(snap)
}

func invalidSnapshotWithUI(now time.Time, phase GamePhase, reason string, ui UIState) Snapshot {
	snap := invalidSnapshot(now, phase, reason)
	snap.UI = ui
	return snap
}
