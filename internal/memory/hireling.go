package memory

import (
	"fmt"
)

// Regular hireling class IDs from the local D2R excel extract `.tmp/d2r-excel/hireling.txt`
// (column `Class`). These are compile-near and not taken from Koolo/d2go catalogs.
const (
	// HirelingClassRogueScout is the Act-1 Rogue Scout class ID.
	HirelingClassRogueScout uint32 = 271
	// HirelingClassDesertMercenary is the Act-2 Desert Mercenary class ID.
	HirelingClassDesertMercenary uint32 = 338
	// HirelingClassEasternSorceror is the Act-3 Eastern Sorceror class ID.
	HirelingClassEasternSorceror uint32 = 359
	// HirelingClassBarbarianA is one Act-5 Barbarian class ID.
	HirelingClassBarbarianA uint32 = 560
	// HirelingClassBarbarianB is the second Act-5 Barbarian class ID.
	HirelingClassBarbarianB uint32 = 561
)

// unitOffsetMode is the live-validated UnitAny Mode field. Gate 18.0 observed
// living hirelings in modes `1`/`2` and a corpse in mode `12`.
const unitOffsetMode = 0x0C

// HirelingLifeScale32768 is the live-validated fractional life denominator.
// Hireling MaxLife uses the normal `>> 8` scale while Life is a fraction of it.
const HirelingLifeScale32768 = 32768

const notHiredConfirmationSnapshots = 3

// IsHirelingClassID reports whether npcID is a regular hireling class from the
// local hireling.txt extract. Summons, pets and town sellers are never included.
func IsHirelingClassID(npcID uint32) bool {
	switch npcID {
	case HirelingClassRogueScout,
		HirelingClassDesertMercenary,
		HirelingClassEasternSorceror,
		HirelingClassBarbarianA,
		HirelingClassBarbarianB:
		return true
	default:
		return false
	}
}

// HirelingClassName returns a stable diagnostic label for a hireling class ID.
func HirelingClassName(npcID uint32) string {
	switch npcID {
	case HirelingClassRogueScout:
		return "rogue_scout"
	case HirelingClassDesertMercenary:
		return "desert_mercenary"
	case HirelingClassEasternSorceror:
		return "eastern_sorceror"
	case HirelingClassBarbarianA, HirelingClassBarbarianB:
		return "barbarian"
	default:
		return ""
	}
}

// HirelingRawEvidence is one raw hireling unit observation for Gate 18.0.
// It intentionally exposes corpse/mode and unscaled life stats so the live
// contract can distinguish Alive, Dead, NotHired and Unknown without guessing.
type HirelingRawEvidence struct {
	UnitAddr            string   `json:"unit_addr"`
	UnitID              uint32   `json:"unit_id"`
	NPCID               uint32   `json:"npc_id"`
	ClassName           string   `json:"class_name"`
	Corpse              uint8    `json:"corpse"`
	Mode                uint32   `json:"mode"`
	ModeKnown           bool     `json:"mode_known"`
	PosX                uint32   `json:"pos_x"`
	PosY                uint32   `json:"pos_y"`
	PositionKnown       bool     `json:"position_known"`
	MonsterTypeFlag     uint8    `json:"monster_type_flag"`
	FlagKnown           bool     `json:"flag_known"`
	StatsListEx         string   `json:"stats_list_ex,omitempty"`
	BaseLifeRaw         *int32   `json:"base_life_raw,omitempty"`
	BaseMaxLifeRaw      *int32   `json:"base_max_life_raw,omitempty"`
	ActiveLifeRaw       *int32   `json:"active_life_raw,omitempty"`
	ActiveMaxLifeRaw    *int32   `json:"active_max_life_raw,omitempty"`
	BaseLifeShift8      *uint32  `json:"base_life_shift8,omitempty"`
	BaseMaxLifeShift8   *uint32  `json:"base_max_life_shift8,omitempty"`
	ActiveLifeShift8    *uint32  `json:"active_life_shift8,omitempty"`
	ActiveMaxLifeShift8 *uint32  `json:"active_max_life_shift8,omitempty"`
	BaseLifeFrac32768   *float64 `json:"base_life_frac_32768,omitempty"`
	ActiveLifeFrac32768 *float64 `json:"active_life_frac_32768,omitempty"`
	StatsError          string   `json:"stats_error,omitempty"`
}

// MercenarySnapshot is the fail-closed hireling state produced by one memory
// snapshot. Unknown is the zero value; consumers must not infer death or
// NotHired from missing unit data.
type MercenarySnapshot struct {
	HiredKnown  bool
	Hired       bool
	Alive       bool
	Dead        bool
	VitalsKnown bool
	UnitID      uint32
	NPCID       uint32
	HP          uint32
	MaxHP       uint32
}

func (p *ProbeReader) readMercenarySnapshot(unitAddr uintptr, npcID uint32, off OffsetSet) (MercenarySnapshot, error) {
	unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
	if err != nil {
		return MercenarySnapshot{}, fmt.Errorf("read hireling unit id: %w", err)
	}
	corpse, err := p.reader.ReadUint8(unitAddr + unitOffsetCorpse)
	if err != nil {
		return MercenarySnapshot{}, fmt.Errorf("read hireling corpse: %w", err)
	}
	mode, err := p.reader.ReadUint32(unitAddr + unitOffsetMode)
	if err != nil {
		return MercenarySnapshot{}, fmt.Errorf("read hireling mode: %w", err)
	}

	result := MercenarySnapshot{
		HiredKnown: true,
		Hired:      true,
		UnitID:     unitID,
		NPCID:      npcID,
	}
	if corpse != 0 || mode == 12 {
		result.Dead = true
		return result, nil
	}
	result.Alive = true

	statsListEx, err := p.reader.ReadUint64(unitAddr + off.Unit.StatsListEx)
	if err != nil || statsListEx == 0 {
		return result, nil
	}
	rawLife, rawMaxLife, err := readRawLifePair(
		p.reader,
		uintptr(statsListEx)+off.Unit.StatsListBase,
		off.Stats,
	)
	if err != nil {
		return result, nil
	}
	hp, maxHP, ok := decodeMercenaryVitals(rawLife, rawMaxLife)
	if !ok {
		return result, nil
	}
	result.VitalsKnown = true
	result.HP = hp
	result.MaxHP = maxHP
	return result, nil
}

func decodeMercenaryVitals(rawLife, rawMaxLife int32) (hp, maxHP uint32, ok bool) {
	if rawLife < 0 || rawMaxLife <= 0 {
		return 0, 0, false
	}
	maxHP = uint32(rawMaxLife) >> 8
	if maxHP == 0 {
		return 0, 0, false
	}
	lifeFraction := uint32(rawLife)
	if lifeFraction > HirelingLifeScale32768 {
		lifeFraction = HirelingLifeScale32768
	}
	hp = uint32(uint64(maxHP) * uint64(lifeFraction) / HirelingLifeScale32768)
	return hp, maxHP, true
}

func (p *ProbeReader) observeNoHireling(snap *Snapshot) {
	snap.Mercenary = MercenarySnapshot{}
	if !snap.Identity.Valid || !snap.Identity.Confirmed {
		p.noHirelingStableTicks = 0
		return
	}
	if p.noHirelingStableTicks < notHiredConfirmationSnapshots {
		p.noHirelingStableTicks++
	}
	if p.noHirelingStableTicks >= notHiredConfirmationSnapshots {
		snap.Mercenary.HiredKnown = true
	}
}

func (p *ProbeReader) resetMercenaryStability() {
	p.noHirelingStableTicks = 0
}

// CollectHirelingEvidence walks the monster unit-table segment once and returns
// raw hireling candidates, including corpses. Candidates never enter
// Snapshot.Monsters and are not hostile/threat material.
func (p *ProbeReader) CollectHirelingEvidence() ([]HirelingRawEvidence, error) {
	if p == nil || p.reader == nil || p.reader.access == nil {
		return nil, fmt.Errorf("collect hireling evidence: reader not attached")
	}
	moduleBase := p.reader.access.ModuleBase()
	if moduleBase == 0 {
		return nil, fmt.Errorf("collect hireling evidence: module base unavailable")
	}
	off := p.ensureOffsets(moduleBase)
	if off.UnitTable == 0 {
		return nil, errUnitTableUnavailable
	}

	out := make([]HirelingRawEvidence, 0)
	visited := 0
	err := p.walkUnitSegment(moduleBase, off, unitSegmentMonster, &visited, 0, func(unitAddr uintptr) (unitWalkAction, error) {
		txtFileNo, err := p.reader.ReadUint32(unitAddr + unitOffsetTxtFileNo)
		if err != nil {
			return unitWalkContinue, nil
		}
		if !IsHirelingClassID(txtFileNo) {
			return unitWalkContinue, nil
		}
		evidence, collectErr := p.readHirelingEvidence(unitAddr, txtFileNo, off)
		if collectErr != nil {
			evidence.StatsError = collectErr.Error()
		}
		out = append(out, evidence)
		return unitWalkContinue, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect hireling evidence: %w", err)
	}
	return out, nil
}

func (p *ProbeReader) readHirelingEvidence(unitAddr uintptr, npcID uint32, off OffsetSet) (HirelingRawEvidence, error) {
	ev := HirelingRawEvidence{
		UnitAddr:  fmt.Sprintf("0x%X", unitAddr),
		NPCID:     npcID,
		ClassName: HirelingClassName(npcID),
	}

	unitID, err := p.reader.ReadUint32(unitAddr + off.Unit.UnitID)
	if err != nil {
		return ev, fmt.Errorf("read unit id: %w", err)
	}
	ev.UnitID = unitID

	if corpse, corpseErr := p.reader.ReadUint8(unitAddr + unitOffsetCorpse); corpseErr == nil {
		ev.Corpse = corpse
	} else {
		return ev, fmt.Errorf("read corpse: %w", corpseErr)
	}

	if mode, modeErr := p.reader.ReadUint32(unitAddr + unitOffsetMode); modeErr == nil {
		ev.Mode = mode
		ev.ModeKnown = true
	}

	if unitData, dataErr := p.reader.ReadUint64(unitAddr + unitOffsetUnitData); dataErr == nil && unitData != 0 {
		if flag, flagErr := p.reader.ReadUint8(uintptr(unitData) + unitDataMonsterFlag); flagErr == nil {
			ev.MonsterTypeFlag = flag
			ev.FlagKnown = true
		}
	}

	if pathPtr, pathErr := p.reader.ReadUint64(unitAddr + off.Unit.Path); pathErr == nil && pathPtr != 0 {
		posX, xErr := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetMonsterX)
		posY, yErr := p.reader.ReadUint16(uintptr(pathPtr) + pathOffsetMonsterY)
		if xErr == nil && yErr == nil {
			ev.PosX = uint32(posX)
			ev.PosY = uint32(posY)
			ev.PositionKnown = true
		}
	}

	statsListEx, err := p.reader.ReadUint64(unitAddr + off.Unit.StatsListEx)
	if err != nil {
		return ev, fmt.Errorf("read stats list ex: %w", err)
	}
	if statsListEx == 0 {
		return ev, fmt.Errorf("stats list ex is null")
	}
	ev.StatsListEx = fmt.Sprintf("0x%X", statsListEx)

	baseLife, baseMax, baseErr := readRawLifePair(p.reader, uintptr(statsListEx)+off.Unit.StatsListBase, off.Stats)
	if baseErr == nil {
		ev.BaseLifeRaw = int32Ptr(baseLife)
		ev.BaseMaxLifeRaw = int32Ptr(baseMax)
		ev.BaseLifeShift8 = uint32Ptr(scaleVitalStat(StatLife, baseLife))
		ev.BaseMaxLifeShift8 = uint32Ptr(scaleVitalStat(StatMaxLife, baseMax))
		ev.BaseLifeFrac32768 = frac32768Ptr(baseLife)
	} else if ev.StatsError == "" {
		ev.StatsError = "base: " + baseErr.Error()
	}

	activeLife, activeMax, activeErr := readRawLifePair(p.reader, uintptr(statsListEx)+off.Unit.StatsListActive, off.Stats)
	if activeErr == nil {
		ev.ActiveLifeRaw = int32Ptr(activeLife)
		ev.ActiveMaxLifeRaw = int32Ptr(activeMax)
		ev.ActiveLifeShift8 = uint32Ptr(scaleVitalStat(StatLife, activeLife))
		ev.ActiveMaxLifeShift8 = uint32Ptr(scaleVitalStat(StatMaxLife, activeMax))
		ev.ActiveLifeFrac32768 = frac32768Ptr(activeLife)
	} else if ev.StatsError == "" {
		ev.StatsError = "active: " + activeErr.Error()
	} else {
		ev.StatsError += "; active: " + activeErr.Error()
	}

	if baseErr != nil && activeErr != nil {
		return ev, fmt.Errorf("hireling life unavailable: base=%v active=%v", baseErr, activeErr)
	}
	return ev, nil
}

func readRawLifePair(r *Reader, listHeader uintptr, off StatOffsets) (life, maxLife int32, err error) {
	stats, err := parseRawStats(r, listHeader, off)
	if err != nil {
		return 0, 0, err
	}
	var foundLife, foundMax bool
	for _, stat := range stats {
		if stat.Layer != 0 {
			continue
		}
		switch stat.ID {
		case StatLife:
			life = stat.Value
			foundLife = true
		case StatMaxLife:
			maxLife = stat.Value
			foundMax = true
		}
	}
	if !foundLife || !foundMax {
		return 0, 0, fmt.Errorf("missing life stats life=%v max=%v", foundLife, foundMax)
	}
	return life, maxLife, nil
}

func int32Ptr(v int32) *int32    { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }
func frac32768Ptr(raw int32) *float64 {
	if raw < 0 {
		zero := 0.0
		return &zero
	}
	v := float64(raw) / float64(HirelingLifeScale32768)
	return &v
}
