package memory

import (
	"errors"
	"fmt"
	"time"
)

// Probe invalid-reason constants for logging and tests.
const (
	ReasonNotAttached             = "not_attached"
	ReasonNotInGame               = "not_in_game"
	ReasonUnitTableUnavailable    = "unit_table_unavailable"
	ReasonPlayerPointerUnavailable = "player_pointer_unavailable"
	ReasonStatsUnavailable        = "stats_unavailable"
	ReasonReadFailed              = "read_failed"
)

const (
	unitTableBuckets     = 128
	maxUnitsPerBucket    = 256
	maxTotalUnitVisits   = 4096
)

// Snapshot is a minimal read-only player state sample for Phase-1 probing.
type Snapshot struct {
	At        time.Time
	Valid     bool
	Reason    string
	PlayerPtr uintptr
	StatsSource string // `base` or `active`, identifying the stat list used for vitals.
	HP        uint32
	MaxHP     uint32
	Mana      uint32
	MaxMana   uint32
	AreaID    uint32
	PosX      uint32
	PosY      uint32
}

// ProbeReader resolves the main player via the unit table and reads vital stats.
type ProbeReader struct {
	reader          *Reader
	offsets         OffsetSet
	activeOffsets   OffsetSet
	offsetsResolved bool
	lastModuleBase  uintptr
	lastPlayerPtr   uintptr
	observedMaxHP   uint32
	observedMaxMana uint32
	lastGateValue   uint8
	lastGateLog     time.Time
	hasGateValue    bool
}

// NewProbeReader wires a probe reader to an existing memory reader and offset set.
func NewProbeReader(reader *Reader, offsets OffsetSet) *ProbeReader {
	return &ProbeReader{
		reader:  reader,
		offsets: offsets,
	}
}

func (p *ProbeReader) ensureOffsets(moduleBase uintptr) OffsetSet {
	if p.lastModuleBase != 0 && p.lastModuleBase != moduleBase {
		p.offsetsResolved = false
	}
	p.lastModuleBase = moduleBase

	if p.offsetsResolved {
		return p.activeOffsets
	}

	scanned, err := ScanProbeOffsets(p.reader, p.offsets)
	if err != nil {
		p.reader.log.Info("probe offset scan failed, using static offsets",
			"error", err,
			"unit_table", fmt.Sprintf("0x%X", p.offsets.UnitTable),
			"ui", fmt.Sprintf("0x%X", p.offsets.UI),
		)
		p.activeOffsets = p.offsets
	} else {
		p.reader.log.Info("probe module offsets scanned",
			"unit_table", fmt.Sprintf("0x%X", scanned.UnitTable),
			"ui", fmt.Sprintf("0x%X", scanned.UI),
			"expansion", fmt.Sprintf("0x%X", scanned.Expansion),
		)
		p.activeOffsets = scanned
	}
	p.offsetsResolved = true
	return p.activeOffsets
}

// Snapshot reads the current main-player probe state. Invalid snapshots carry a short Reason.
func (p *ProbeReader) Snapshot() Snapshot {
	now := time.Now()
	invalid := func(reason string) Snapshot {
		return Snapshot{At: now, Valid: false, Reason: reason}
	}

	if p.reader == nil || p.reader.access == nil {
		return invalid(ReasonNotAttached)
	}

	moduleBase := p.reader.access.ModuleBase()
	if moduleBase == 0 {
		return invalid(ReasonReadFailed)
	}

	off := p.ensureOffsets(moduleBase)

	inGameGate := p.isInGame(moduleBase, off)

	playerPtr, err := p.findMainPlayer(moduleBase, off)
	if err != nil {
		if !inGameGate {
			return invalid(ReasonNotInGame)
		}
		switch {
		case errors.Is(err, errUnitTableUnavailable):
			return invalid(ReasonUnitTableUnavailable)
		case errors.Is(err, errPlayerNotFound):
			return invalid(ReasonPlayerPointerUnavailable)
		default:
			return invalid(ReasonReadFailed)
		}
	}

	areaID, posX, posY, err := p.readAreaAndPosition(playerPtr, off)
	if err != nil {
		return invalid(ReasonReadFailed)
	}

	statsListEx, err := p.reader.ReadUint64(playerPtr + off.Unit.StatsListEx)
	if err != nil || statsListEx == 0 {
		return invalid(ReasonStatsUnavailable)
	}

	vitals, statsSource, err := p.parseProbeVitalStats(uintptr(statsListEx), off)
	if err != nil {
		p.reader.log.Debug("probe stats unavailable",
			"player_ptr", fmt.Sprintf("0x%X", playerPtr),
			"stats_list_ex", fmt.Sprintf("0x%X", statsListEx),
			"error", err,
		)
		return invalid(ReasonStatsUnavailable)
	}
	vitals = p.normalizeVitalStats(playerPtr, vitals)

	return Snapshot{
		At:        now,
		Valid:     true,
		PlayerPtr: playerPtr,
		StatsSource: statsSource,
		HP:        vitals.HP,
		MaxHP:     vitals.MaxHP,
		Mana:      vitals.Mana,
		MaxMana:   vitals.MaxMana,
		AreaID:    areaID,
		PosX:      posX,
		PosY:      posY,
	}
}

func (p *ProbeReader) normalizeVitalStats(playerPtr uintptr, vitals VitalStats) VitalStats {
	if p.lastPlayerPtr != playerPtr {
		p.lastPlayerPtr = playerPtr
		p.observedMaxHP = 0
		p.observedMaxMana = 0
	}

	// D2R exposes current Life/Mana and base MaxLife/MaxMana separately. Equipment can
	// make the current value exceed the base max, so keep a peak observed max for logs.
	p.observedMaxHP = maxUint32(p.observedMaxHP, vitals.MaxHP, vitals.HP)
	p.observedMaxMana = maxUint32(p.observedMaxMana, vitals.MaxMana, vitals.Mana)
	vitals.MaxHP = p.observedMaxHP
	vitals.MaxMana = p.observedMaxMana
	return vitals
}

func (p *ProbeReader) parseProbeVitalStats(statsListEx uintptr, off OffsetSet) (VitalStats, string, error) {
	baseHeader := statsListEx + off.Unit.StatsListBase
	vitals, baseErr := parseVitalStats(p.reader, baseHeader, off.Stats)
	if baseErr == nil {
		return vitals, "base", nil
	}

	activeHeader := statsListEx + off.Unit.StatsListActive
	vitals, activeErr := parseVitalStats(p.reader, activeHeader, off.Stats)
	if activeErr == nil {
		return vitals, "active", nil
	}

	return VitalStats{}, "", fmt.Errorf("base stats at %#x: %v; active stats at %#x: %w",
		baseHeader, baseErr, activeHeader, activeErr)
}

var (
	errUnitTableUnavailable = errors.New("unit table unavailable")
	errPlayerNotFound       = errors.New("main player not found")
)

func (p *ProbeReader) isInGame(moduleBase uintptr, off OffsetSet) bool {
	gate := off.InGameGateOffset()
	if gate == 0 {
		return true
	}
	b, err := p.reader.ReadUint8(moduleBase + gate)
	if err != nil {
		if time.Since(p.lastGateLog) >= 5*time.Second {
			p.reader.log.Debug("in-game gate read failed", "gate", fmt.Sprintf("0x%X", moduleBase+gate), "error", err)
			p.lastGateLog = time.Now()
		}
		return false
	}
	if b != 1 {
		if !p.hasGateValue || p.lastGateValue != b || time.Since(p.lastGateLog) >= 5*time.Second {
			p.reader.log.Debug("not in game", "gate", fmt.Sprintf("0x%X", moduleBase+gate), "value", b)
			p.lastGateLog = time.Now()
		}
	}
	p.lastGateValue = b
	p.hasGateValue = true
	return b == 1
}

func (p *ProbeReader) expansionActive(moduleBase uintptr, off OffsetSet) (bool, error) {
	if off.Expansion == 0 {
		return false, nil
	}
	expPtr, err := p.reader.ReadUint64(moduleBase + off.Expansion)
	if err != nil {
		p.reader.log.Debug("expansion flag read failed, probing both main-player flags",
			"addr", fmt.Sprintf("0x%X", moduleBase+off.Expansion),
			"error", err,
		)
		return false, nil
	}
	if expPtr == 0 {
		return false, nil
	}
	expChar, err := p.reader.ReadUint16(uintptr(expPtr) + off.Unit.ExpansionCharFlag)
	if err != nil {
		p.reader.log.Debug("expansion char flag read failed, probing both main-player flags",
			"addr", fmt.Sprintf("0x%X", uintptr(expPtr)+off.Unit.ExpansionCharFlag),
			"error", err,
		)
		return false, nil
	}
	return expChar > 0, nil
}

func (p *ProbeReader) isMainPlayer(unitAddr uintptr, expansion bool, off OffsetSet) (bool, error) {
	invPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Inventory)
	if err != nil {
		return false, err
	}
	if invPtr == 0 {
		return false, nil
	}

	markerOffsets := []uintptr{off.Unit.MainPlayerNormal, off.Unit.MainPlayerExpansion}
	if expansion {
		markerOffsets[0], markerOffsets[1] = markerOffsets[1], markerOffsets[0]
	}

	for _, markerOff := range markerOffsets {
		flag, err := p.reader.ReadUint16(uintptr(invPtr) + markerOff)
		if err != nil {
			return false, err
		}
		if flag > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (p *ProbeReader) findMainPlayer(moduleBase uintptr, off OffsetSet) (uintptr, error) {
	if off.UnitTable == 0 {
		return 0, errUnitTableUnavailable
	}

	expansion, err := p.expansionActive(moduleBase, off)
	if err != nil {
		return 0, fmt.Errorf("expansion flag: %w", err)
	}

	visited := 0
	for bucket := 0; bucket < unitTableBuckets; bucket++ {
		bucketAddr := moduleBase + off.UnitTable + uintptr(bucket*8)
		unitAddr, err := p.reader.ReadUint64(bucketAddr)
		if err != nil {
			return 0, err
		}

		perBucket := 0
		for unitAddr != 0 {
			if perBucket >= maxUnitsPerBucket || visited >= maxTotalUnitVisits {
				break
			}
			perBucket++
			visited++

			isMain, err := p.isMainPlayer(uintptr(unitAddr), expansion, off)
			if err != nil {
				return 0, err
			}
			if isMain {
				return uintptr(unitAddr), nil
			}

			unitAddr, err = p.reader.ReadUint64(uintptr(unitAddr) + off.Unit.NextUnit)
			if err != nil {
				return 0, err
			}
		}
	}

	return 0, errPlayerNotFound
}

func (p *ProbeReader) readAreaAndPosition(unitAddr uintptr, off OffsetSet) (areaID, posX, posY uint32, err error) {
	pathPtr, err := p.reader.ReadUint64(unitAddr + off.Unit.Path)
	if err != nil {
		return 0, 0, 0, err
	}
	if pathPtr == 0 {
		return 0, 0, 0, fmt.Errorf("path pointer is null")
	}

	x, err := p.reader.ReadUint16(uintptr(pathPtr) + off.Unit.PositionX)
	if err != nil {
		return 0, 0, 0, err
	}
	y, err := p.reader.ReadUint16(uintptr(pathPtr) + off.Unit.PositionY)
	if err != nil {
		return 0, 0, 0, err
	}

	room1, err := p.reader.ReadUint64(uintptr(pathPtr) + off.Unit.PathRoom1)
	if err != nil || room1 == 0 {
		return 0, uint32(x), uint32(y), err
	}
	room2, err := p.reader.ReadUint64(uintptr(room1) + off.Unit.Room2)
	if err != nil || room2 == 0 {
		return 0, uint32(x), uint32(y), err
	}
	level, err := p.reader.ReadUint64(uintptr(room2) + off.Unit.Level)
	if err != nil || level == 0 {
		return 0, uint32(x), uint32(y), err
	}
	area, err := p.reader.ReadUint32(uintptr(level) + off.Unit.Area)
	if err != nil {
		return 0, uint32(x), uint32(y), err
	}

	return area, uint32(x), uint32(y), nil
}

func maxUint32(values ...uint32) uint32 {
	var maxValue uint32
	for _, v := range values {
		if v > maxValue {
			maxValue = v
		}
	}
	return maxValue
}
