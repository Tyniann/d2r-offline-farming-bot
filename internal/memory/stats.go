package memory

import (
	"fmt"
)

// Stat IDs used by the player probe; gold IDs are live-validated against known values on D2R `3.2.92777`.
const (
	StatLife     uint16 = 6
	StatMaxLife  uint16 = 7
	StatMana     uint16 = 8
	StatMaxMana  uint16 = 9
	StatGold     uint16 = 14
	StatGoldBank uint16 = 15

	maxRawStatEntries = 512
)

// GoldStats holds independently validated carried and private-stash gold values.
// Known flags distinguish a legitimate zero balance from an unavailable stat;
// consumers must not infer one source from the other.
type GoldStats struct {
	Carried      uint32
	PrivateStash uint32
	CarriedKnown bool
	StashKnown   bool
}

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

func parseRawStats(r *Reader, listHeader uintptr, off StatOffsets) ([]RawStat, error) {
	if listHeader == 0 {
		return nil, fmt.Errorf("stat list header is null")
	}

	listPtr, err := r.ReadUint64(listHeader + off.ListPtr)
	if err != nil {
		return nil, fmt.Errorf("read stat array pointer: %w", err)
	}
	if listPtr == 0 {
		return nil, fmt.Errorf("stat array pointer is null")
	}

	count, err := r.ReadUint64(listHeader + off.Count)
	if err != nil {
		return nil, fmt.Errorf("read stat count: %w", err)
	}
	if count == 0 {
		return []RawStat{}, nil
	}

	stride := off.EntryStride
	if stride == 0 {
		stride = 8
	}

	maxEntries := count
	if maxEntries > maxRawStatEntries {
		maxEntries = maxRawStatEntries
	}

	stats := make([]RawStat, 0, maxEntries)
	for i := uint64(0); i < maxEntries; i++ {
		entry := uintptr(listPtr) + uintptr(i*uint64(stride))
		layer, err := r.ReadUint16(entry + off.Layer)
		if err != nil {
			return nil, fmt.Errorf("read stat layer at entry %d: %w", i, err)
		}
		statID, err := r.ReadUint16(entry + off.ID)
		if err != nil {
			return nil, fmt.Errorf("read stat id at entry %d: %w", i, err)
		}
		value, err := r.ReadInt32(entry + off.Value)
		if err != nil {
			return nil, fmt.Errorf("read stat value for id %d: %w", statID, err)
		}
		stats = append(stats, RawStat{ID: statID, Layer: layer, Value: value})
	}
	return stats, nil
}

func parseGoldStats(r *Reader, listHeader uintptr, off StatOffsets) (GoldStats, error) {
	// Gold IDs 14/15 are unscaled table values. Applying the Life/Mana `>> 8`
	// transform would silently understate vendor funds and authorize wrong plans.
	stats, err := parseRawStats(r, listHeader, off)
	if err != nil {
		return GoldStats{}, err
	}
	var result GoldStats
	for _, stat := range stats {
		if stat.Layer != 0 || stat.Value < 0 {
			continue
		}
		switch stat.ID {
		case StatGold:
			result.Carried = uint32(stat.Value)
			result.CarriedKnown = true
		case StatGoldBank:
			result.PrivateStash = uint32(stat.Value)
			result.StashKnown = true
		}
	}
	return result, nil
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
