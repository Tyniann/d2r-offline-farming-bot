package memory

import (
	"encoding/hex"
	"math"
	"time"
)

const (
	unitOffsetTxtFileNo = 0x04
	unitOffsetUnitType  = 0x00
	unitOffsetCorpse    = 0x1AE
	unitOffsetUnitData  = 0x10
	unitDataMonsterFlag = 0x1A

	// This window contains both live-validated CE-consumption bytes. Keeping the
	// full window preserves diagnostic evidence alongside the productive bits.
	cowStateWindowOffset = 0xA80
	cowStateWindowSize   = 0x100
	cowConsumedOffsetA   = 0xB3D
	cowConsumedOffsetB   = 0xB5D
	cowConsumedMask      = 0x01

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
	snap.CowEvidence = make([]CowRawEvidence, 0)
	snap.CowCorpses = make([]CowCorpseUnit, 0)

	// Entrances and monsters before objects: the object segment is large and would
	// exhaust maxTotalUnitVisits before smaller segments are walked (d2go walks each
	// segment independently without a shared cap).
	if err := p.enumerateEntrances(moduleBase, off, &visited, snap); err != nil {
		return err
	}
	if err := p.enumerateMonsters(moduleBase, off, &visited, snap); err != nil {
		return err
	}
	finalizeMonsterCoverage(snap)
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

		object := ObjectUnit{
			TxtFileNo: txtFileNo,
			UnitID:    unitID,
			PosX:      uint32(posX),
			PosY:      uint32(posY),
		}
		if mode, modeErr := p.reader.ReadUint32(unitAddr + unitOffsetMode); modeErr == nil {
			object.Mode = mode
			object.ModeKnown = true
		}
		snap.Objects = append(snap.Objects, object)
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
	hirelingCount := 0
	hirelingInvalid := false
	snap.CowEvidenceComplete = true
	snap.CowCorpsesComplete = true
	err := p.walkUnitSegment(moduleBase, off, unitSegmentMonster, visited, segmentLimit, func(unitAddr uintptr) (unitWalkAction, error) {
		txtFileNo, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
		if err != nil {
			return unitWalkContinue, nil
		}
		if IsHirelingClassID(txtFileNo) {
			hirelingCount++
			if hirelingCount > 1 {
				hirelingInvalid = true
				return unitWalkContinue, nil
			}
			mercenary, readErr := p.readMercenarySnapshot(unitAddr, txtFileNo, off)
			if readErr != nil {
				hirelingInvalid = true
				return unitWalkContinue, nil
			}
			snap.Mercenary = mercenary
			return unitWalkContinue, nil
		}
		if isPhase20CowNPCID(txtFileNo) {
			evidence, complete := p.readCowRawEvidence(unitAddr, txtFileNo, off)
			if !complete {
				snap.CowEvidenceComplete = false
				snap.CowCorpsesComplete = false
				return unitWalkContinue, nil
			}
			snap.CowEvidence = append(snap.CowEvidence, evidence)
			if evidence.Corpse != 0 {
				if !evidence.ConsumptionKnown {
					snap.CowCorpsesComplete = false
				}
				snap.CowCorpses = append(snap.CowCorpses, CowCorpseUnit{
					NPCID: evidence.NPCID, UnitID: evidence.UnitID,
					PosX: evidence.PosX, PosY: evidence.PosY,
					MonsterTypeFlag: evidence.MonsterTypeFlag,
					Consumed:        evidence.Consumed, ConsumptionKnown: evidence.ConsumptionKnown,
				})
				return unitWalkContinue, nil
			}
		}

		corpse, err := p.reader.ReadUint8(unitAddr + unitOffsetCorpse)
		if err != nil || corpse != 0 {
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

		appendRuntimeMonster(snap, MonsterUnit{
			NPCID:           txtFileNo,
			UnitID:          unitID,
			PosX:            uint32(posX),
			PosY:            uint32(posY),
			MonsterTypeFlag: flag,
		})
		return unitWalkContinue, nil
	})
	if err != nil {
		snap.CowEvidenceComplete = false
		snap.CowCorpsesComplete = false
		p.resetMercenaryStability()
		snap.Mercenary = MercenarySnapshot{}
		return err
	}
	if hirelingInvalid {
		p.resetMercenaryStability()
		snap.Mercenary = MercenarySnapshot{}
		return nil
	}
	if hirelingCount == 0 {
		p.observeNoHireling(snap)
		return nil
	}
	p.resetMercenaryStability()
	return nil
}

func isPhase20CowNPCID(id uint32) bool {
	return id == phase20NPCIDHellBovine || id == phase20NPCIDCowKing
}

func (p *ProbeReader) readCowRawEvidence(unitAddr uintptr, npcID uint32, off OffsetSet) (CowRawEvidence, bool) {
	unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
	if err != nil {
		return CowRawEvidence{}, false
	}
	corpse, err := p.reader.ReadUint8(unitAddr + unitOffsetCorpse)
	if err != nil {
		return CowRawEvidence{}, false
	}
	mode, err := p.reader.ReadUint32(unitAddr + unitOffsetMode)
	if err != nil {
		return CowRawEvidence{}, false
	}
	unitData, err := p.reader.ReadUint64(unitAddr + unitOffsetUnitData)
	if err != nil || unitData == 0 {
		return CowRawEvidence{}, false
	}
	flag, err := p.reader.ReadUint8(uintptr(unitData) + unitDataMonsterFlag)
	if err != nil {
		return CowRawEvidence{}, false
	}
	pathPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Path)
	if err != nil || pathPtr == 0 {
		return CowRawEvidence{}, false
	}
	posX, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetMonsterX)
	if err != nil {
		return CowRawEvidence{}, false
	}
	posY, err := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetMonsterY)
	if err != nil {
		return CowRawEvidence{}, false
	}
	evidence := CowRawEvidence{
		NPCID: npcID, UnitID: unitID, Corpse: corpse, Mode: mode,
		PosX: uint32(posX), PosY: uint32(posY), MonsterTypeFlag: flag,
		StateWindowOffset: cowStateWindowOffset,
	}
	if statsListEx, statsErr := p.reader.ReadUint64(unitAddr + off.Unit.StatsListEx); statsErr == nil && statsListEx != 0 {
		if stateWindow, stateErr := p.reader.ReadBytes(uintptr(statsListEx)+cowStateWindowOffset, cowStateWindowSize); stateErr == nil {
			evidence.StateWindowHex = hex.EncodeToString(stateWindow)
			evidence.StateWindowComplete = true
			bitA := stateWindow[cowConsumedOffsetA-cowStateWindowOffset]&cowConsumedMask != 0
			bitB := stateWindow[cowConsumedOffsetB-cowStateWindowOffset]&cowConsumedMask != 0
			evidence.ConsumptionKnown = bitA == bitB
			evidence.Consumed = evidence.ConsumptionKnown && bitA
		}
	}
	return evidence, true
}

func appendRuntimeMonster(snap *Snapshot, candidate MonsterUnit) {
	snap.MonsterCoverage.EligibleMonsterCount++
	if isRuntimePriorityMonsterCandidate(candidate.NPCID, candidate.MonsterTypeFlag) {
		snap.Monsters = append(snap.Monsters, candidate)
		return
	}
	snap.runtimeNonPriorityMonsterCount++
	if snap.runtimeNonPriorityMonsterCount <= maxRuntimeMonsters {
		snap.Monsters = append(snap.Monsters, candidate)
		return
	}
	snap.MonsterCoverage.MonstersTruncated = true

	// Runtime candidates use a bounded nearest-to-player reservoir. Priority
	// entities (bosses, super-uniques and Town NPCs) are never replaced, so a
	// crowded area cannot starve boss acquisition while nearby trash remains
	// available for post-kill cleanup.
	farthestIndex := -1
	var farthestDistance uint64
	for i, monster := range snap.Monsters {
		if isRuntimePriorityMonsterCandidate(monster.NPCID, monster.MonsterTypeFlag) {
			continue
		}
		distance := squaredTileDistance(snap.PosX, snap.PosY, monster.PosX, monster.PosY)
		if farthestIndex < 0 || distance > farthestDistance {
			farthestIndex = i
			farthestDistance = distance
		}
	}
	if farthestIndex >= 0 && squaredTileDistance(snap.PosX, snap.PosY, candidate.PosX, candidate.PosY) < farthestDistance {
		snap.Monsters[farthestIndex] = candidate
	}
}

func finalizeMonsterCoverage(snap *Snapshot) {
	if !snap.MonsterCoverage.MonstersTruncated {
		snap.MonsterCoverage.MonsterCoverageRadiusTiles = 0
		return
	}
	var farthest uint64
	for _, monster := range snap.Monsters {
		if isRuntimePriorityMonsterCandidate(monster.NPCID, monster.MonsterTypeFlag) {
			continue
		}
		distance := squaredTileDistance(snap.PosX, snap.PosY, monster.PosX, monster.PosY)
		if distance > farthest {
			farthest = distance
		}
	}
	snap.MonsterCoverage.MonsterCoverageRadiusTiles = math.Sqrt(float64(farthest))
}

func squaredTileDistance(ax, ay, bx, by uint32) uint64 {
	dx := int64(ax) - int64(bx)
	dy := int64(ay) - int64(by)
	return uint64(dx*dx + dy*dy)
}

// emptyEntitySlices returns a snapshot with non-nil empty entity slices for stable fingerprints.
func emptyEntitySlices(snap Snapshot) Snapshot {
	snap.Objects = make([]ObjectUnit, 0)
	snap.Entrances = make([]EntranceUnit, 0)
	snap.Monsters = make([]MonsterUnit, 0)
	snap.CowEvidence = make([]CowRawEvidence, 0)
	snap.CowEvidenceComplete = false
	snap.CowCorpses = make([]CowCorpseUnit, 0)
	snap.CowCorpsesComplete = false
	snap.Mercenary = MercenarySnapshot{}
	snap.Items = make([]ItemUnit, 0)
	return snap
}

func invalidSnapshot(now time.Time, generation uint64, phase GamePhase, reason string) Snapshot {
	snap := Snapshot{At: now, Generation: generation, Valid: false, Reason: reason, Phase: phase}
	return emptyEntitySlices(snap)
}

func invalidSnapshotWithUI(now time.Time, generation uint64, phase GamePhase, reason string, ui UIState) Snapshot {
	snap := invalidSnapshot(now, generation, phase, reason)
	snap.UI = ui
	return snap
}
