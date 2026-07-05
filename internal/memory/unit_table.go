package memory

import (
	"encoding/binary"
	"fmt"
)

const (
	unitTableSegmentBytes = 1024 // 128 list heads × 8 bytes per segment (d2go layout).
	unitTableListHeads    = 128
	unitTableBuckets      = unitTableListHeads // alias for legacy tests
	unitTableHeadStride   = 8

	unitSegmentPlayer   = 0
	unitSegmentMonster  = 1
	unitSegmentObject   = 2
	unitSegmentItem     = 4
	unitSegmentEntrance = 5

	maxUnitsPerBucket      = 256
	maxTotalUnitVisits     = 4096
	maxEntitiesPerCategory = 32
	maxItemUnitVisits      = 4096
	maxItemsPerSnapshot    = 512

	// Per-segment visit budgets for entity enumeration (entrances/monsters before objects).
	maxVisitsEntranceSegment = 1024
)

// unitSegmentBase returns the module address of a unit-type segment (d2go: UnitTable + unitType*1024).
func unitSegmentBase(moduleBase, unitTable uintptr, unitType int) uintptr {
	return moduleBase + unitTable + uintptr(unitType)*unitTableSegmentBytes
}

// readUnitTableSegment reads all 128 list-head pointers for a unit-type segment in one buffer read.
// Unreadable segments return a zero-filled buffer so entity walks degrade gracefully instead of
// invalidating entities from other segments (live game: full table is normally readable).
func (p *ProbeReader) readUnitTableSegment(moduleBase uintptr, off OffsetSet, unitType int) ([]byte, error) {
	if off.UnitTable == 0 {
		return nil, errUnitTableUnavailable
	}
	segmentBase := unitSegmentBase(moduleBase, off.UnitTable, unitType)
	buf, err := p.reader.ReadBytes(segmentBase, unitTableListHeads*unitTableHeadStride)
	if err != nil {
		p.reader.log.Debug("unit table segment unreadable, treating as empty",
			"segment", unitType,
			"base", fmt.Sprintf("0x%X", segmentBase),
			"error", err,
		)
		return make([]byte, unitTableListHeads*unitTableHeadStride), nil
	}
	return buf, nil
}

type unitWalkAction int

const (
	unitWalkContinue unitWalkAction = iota
	unitWalkStop
)

// unitVisitor is called for each unit in a segment walk. Returning unitWalkStop ends the bucket
// and segment walk; an error aborts the entire walk.
type unitVisitor func(unitAddr uintptr) (unitWalkAction, error)

// walkUnitSegment walks the linked lists of segment unitType, invoking visit for each unit node.
// visited is incremented globally and capped by maxTotalUnitVisits across all walks in one Snapshot.
// segmentLimit caps visits within this walk only (0 = no per-segment cap).
func (p *ProbeReader) walkUnitSegment(moduleBase uintptr, off OffsetSet, unitType int, visited *int, segmentLimit int, visit unitVisitor) error {
	if off.UnitTable == 0 {
		return errUnitTableUnavailable
	}

	heads, err := p.readUnitTableSegment(moduleBase, off, unitType)
	if err != nil {
		return err
	}

	segmentStart := *visited
	for i := 0; i < unitTableListHeads; i++ {
		offset := i * unitTableHeadStride
		if offset+8 > len(heads) {
			break
		}
		unitAddr := uintptr(binary.LittleEndian.Uint64(heads[offset:]))

		perBucket := 0
		for unitAddr != 0 {
			if segmentLimit > 0 && *visited-segmentStart >= segmentLimit {
				return nil
			}
			if perBucket >= maxUnitsPerBucket || *visited >= maxTotalUnitVisits {
				break
			}
			perBucket++
			*visited++

			action, err := visit(unitAddr)
			if err != nil {
				return err
			}
			if action == unitWalkStop {
				return nil
			}

			next, err := p.reader.ReadUint64(unitAddr + off.Unit.NextUnit)
			if err != nil {
				return fmt.Errorf("next unit at %#x: %w", unitAddr, err)
			}
			unitAddr = uintptr(next)
		}
	}
	return nil
}
