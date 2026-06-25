package memory

import (
	"fmt"
)

// Stat IDs for Life/Mana probing (d2go pkg/data/stat, iota from Strength=0).
const (
	StatLife    uint16 = 6
	StatMaxLife uint16 = 7
	StatMana    uint16 = 8
	StatMaxMana uint16 = 9
)

// VitalStats holds decoded HP/Mana values from a unit stat list.
type VitalStats struct {
	HP      uint32
	MaxHP   uint32
	Mana    uint32
	MaxMana uint32
}

// parseVitalStats reads Life, MaxLife, Mana, and MaxMana from a stat list header address.
// Returns an error if any of the four stats is missing or undecodable (layer 0).
func parseVitalStats(r *Reader, listHeader uintptr, off StatOffsets) (VitalStats, error) {
	if listHeader == 0 {
		return VitalStats{}, fmt.Errorf("stat list header is null")
	}

	listPtr, err := r.ReadUint64(listHeader + off.ListPtr)
	if err != nil {
		return VitalStats{}, fmt.Errorf("read stat array pointer: %w", err)
	}
	if listPtr == 0 {
		return VitalStats{}, fmt.Errorf("stat array pointer is null")
	}

	count, err := r.ReadUint64(listHeader + off.Count)
	if err != nil {
		return VitalStats{}, fmt.Errorf("read stat count: %w", err)
	}
	if count == 0 {
		return VitalStats{}, fmt.Errorf("stat count is zero")
	}

	found := make(map[uint16]uint32, 4)
	stride := off.EntryStride
	if stride == 0 {
		stride = 8
	}

	maxEntries := count
	if maxEntries > 512 {
		maxEntries = 512
	}

	for i := uint64(0); i < maxEntries; i++ {
		entry := uintptr(listPtr) + uintptr(i*uint64(stride))
		layer, err := r.ReadUint16(entry + off.Layer)
		if err != nil {
			return VitalStats{}, fmt.Errorf("read stat layer at entry %d: %w", i, err)
		}
		if layer != 0 {
			continue
		}

		statID, err := r.ReadUint16(entry + off.ID)
		if err != nil {
			return VitalStats{}, fmt.Errorf("read stat id at entry %d: %w", i, err)
		}

		switch statID {
		case StatLife, StatMaxLife, StatMana, StatMaxMana:
			raw, err := r.ReadInt32(entry + off.Value)
			if err != nil {
				return VitalStats{}, fmt.Errorf("read stat value for id %d: %w", statID, err)
			}
			found[statID] = scaleVitalStat(statID, raw)
		}

		if len(found) == 4 {
			break
		}
	}

	var missing []string
	for _, id := range []uint16{StatLife, StatMaxLife, StatMana, StatMaxMana} {
		if _, ok := found[id]; !ok {
			missing = append(missing, statName(id))
		}
	}
	if len(missing) > 0 {
		return VitalStats{}, fmt.Errorf("missing stats: %v", missing)
	}

	return VitalStats{
		HP:      found[StatLife],
		MaxHP:   found[StatMaxLife],
		Mana:    found[StatMana],
		MaxMana: found[StatMaxMana],
	}, nil
}

// scaleVitalStat applies d2go decoding for Life/Mana stats (value >> 8).
func scaleVitalStat(id uint16, raw int32) uint32 {
	switch id {
	case StatLife, StatMaxLife, StatMana, StatMaxMana:
		if raw < 0 {
			return 0
		}
		return uint32(raw) >> 8
	default:
		if raw < 0 {
			return 0
		}
		return uint32(raw)
	}
}

func statName(id uint16) string {
	switch id {
	case StatLife:
		return "life"
	case StatMaxLife:
		return "max_life"
	case StatMana:
		return "mana"
	case StatMaxMana:
		return "max_mana"
	default:
		return fmt.Sprintf("stat_%d", id)
	}
}
