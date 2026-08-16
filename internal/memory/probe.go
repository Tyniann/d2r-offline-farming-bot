package memory

import (
	"errors"
	"fmt"
	"time"
)

// Probe invalid-reason constants for logging and tests.
const (
	ReasonNotAttached              = "not_attached"
	ReasonNotInGame                = "not_in_game"
	ReasonUnitTableUnavailable     = "unit_table_unavailable"
	ReasonPlayerPointerUnavailable = "player_pointer_unavailable"
	ReasonStatsUnavailable         = "stats_unavailable"
	ReasonReadFailed               = "read_failed"
)

// Snapshot is a read-only player and entity state sample for probing.
type Snapshot struct {
	At                    time.Time
	Generation            uint64
	Valid                 bool
	Reason                string
	Phase                 GamePhase
	PlayerPtr             uintptr
	PlayerUnitID          uint32
	StatsSource           string // `base` or `active`, identifying the stat list used for vitals.
	HP                    uint32
	MaxHP                 uint32
	Mana                  uint32
	MaxMana               uint32
	Gold                  uint32
	PrivateStashGold      uint32
	GoldKnown             bool
	PrivateStashGoldKnown bool
	AreaID                uint32
	PosX                  uint32
	PosY                  uint32
	Objects               []ObjectUnit
	Entrances             []EntranceUnit
	Monsters              []MonsterUnit
	CowEvidence           []CowRawEvidence
	CowEvidenceComplete   bool
	CowCorpses            []CowCorpseUnit
	CowCorpsesComplete    bool
	MonsterCoverage       MonsterCoverage
	Mercenary             MercenarySnapshot
	Items                 []ItemUnit
	PlayerSkills          PlayerSkills
	ActiveWeaponSet       WeaponSetSnapshot
	Hover                 HoverState
	UI                    UIState
	Identity              IdentityProbe

	runtimeNonPriorityMonsterCount int
}

// ProbeReader resolves the main player via the unit table and reads vital stats.
type ProbeReader struct {
	reader                   *Reader
	offsets                  OffsetSet
	activeOffsets            OffsetSet
	offsetsResolved          bool
	lastModuleBase           uintptr
	scannedCachePath         string
	lastScanAttempt          time.Time
	scanFailCount            int
	lastPlayerPtr            uintptr
	observedMaxHP            uint32
	observedMaxMana          uint32
	lastGateValue            uint8
	lastGateLog              time.Time
	hasGateValue             bool
	lastIdentityProbe        IdentityProbe
	identityCandidate        IdentityProbe
	identityStableTicks      uint8
	noHirelingStableTicks    uint8
	snapshotGeneration       uint64
	weaponSetSecondarySkills [2]uint16
}

const (
	scanRetryInterval    = 2 * time.Second
	scanAttemptsPerRound = 3
	scanAttemptBackoff   = 75 * time.Millisecond
)

// NewProbeReader wires a probe reader to an existing memory reader and offset set.
func NewProbeReader(reader *Reader, offsets OffsetSet) *ProbeReader {
	return &ProbeReader{
		reader:  reader,
		offsets: offsets,
	}
}

// SetScannedCachePath configures where successful runtime scans are persisted and loaded from.
func (p *ProbeReader) SetScannedCachePath(path string) {
	p.scannedCachePath = path
}

func (p *ProbeReader) ensureOffsets(moduleBase uintptr) OffsetSet {
	if p.lastModuleBase != 0 && p.lastModuleBase != moduleBase {
		p.offsetsResolved = false
		p.resetMercenaryStability()
	}
	p.lastModuleBase = moduleBase

	if p.offsetsResolved {
		return p.activeOffsets
	}

	now := time.Now()
	if p.scanFailCount > 0 && now.Sub(p.lastScanAttempt) < scanRetryInterval {
		return p.pendingOffsets(moduleBase)
	}
	p.lastScanAttempt = now

	if scanned, ok := p.tryScanOffsets(); ok {
		p.applyScannedOffsets(scanned, "live scan")
		return p.activeOffsets
	}

	if cached, ok := p.tryCachedOffsets(moduleBase); ok {
		p.applyScannedOffsets(cached, "scan cache")
		return p.activeOffsets
	}

	p.scanFailCount++
	p.reader.log.Warn("probe offset scan unavailable, retrying",
		"retry_in", scanRetryInterval,
		"unit_table_fallback", fmt.Sprintf("0x%X", p.offsets.UnitTable),
		"ui_fallback", fmt.Sprintf("0x%X", p.offsets.UI),
	)
	return p.pendingOffsets(moduleBase)
}

func (p *ProbeReader) tryScanOffsets() (OffsetSet, bool) {
	var lastErr error
	for attempt := 0; attempt < scanAttemptsPerRound; attempt++ {
		if attempt > 0 {
			time.Sleep(scanAttemptBackoff)
		}
		scanned, err := ScanProbeOffsets(p.reader, p.offsets)
		if err == nil {
			return scanned, true
		}
		lastErr = err
	}
	if lastErr != nil {
		p.reader.log.Info("probe offset scan failed",
			"error", lastErr,
			"attempts", scanAttemptsPerRound,
		)
	}
	return OffsetSet{}, false
}

func (p *ProbeReader) tryCachedOffsets(moduleBase uintptr) (OffsetSet, bool) {
	if p.scannedCachePath == "" {
		return OffsetSet{}, false
	}
	cached, err := LoadScannedOffsetCache(p.scannedCachePath)
	if err != nil {
		p.reader.log.Debug("scan cache not loaded", "path", p.scannedCachePath, "error", err)
		return OffsetSet{}, false
	}
	cached = ResolveCachedOffsets(moduleBase, cached)
	if !p.offsetsReadable(moduleBase, cached) {
		p.reader.log.Warn("scan cache offsets not readable in attached process",
			"path", p.scannedCachePath,
			"unit_table", fmt.Sprintf("0x%X", cached.UnitTable),
			"ui", fmt.Sprintf("0x%X", cached.UI),
		)
		return OffsetSet{}, false
	}
	return cached, true
}

func (p *ProbeReader) applyScannedOffsets(scanned OffsetSet, source string) {
	p.activeOffsets = scanned
	p.offsetsResolved = true
	p.scanFailCount = 0
	p.reader.log.Info("probe module offsets active",
		"source", source,
		"unit_table", fmt.Sprintf("0x%X", scanned.UnitTable),
		"ui", fmt.Sprintf("0x%X", scanned.UI),
		"expansion", fmt.Sprintf("0x%X", scanned.Expansion),
	)
	if p.scannedCachePath != "" && source == "live scan" {
		if err := SaveScannedOffsetCache(p.scannedCachePath, p.lastModuleBase, p.offsets, scanned); err != nil {
			p.reader.log.Warn("failed to persist scanned offsets", "path", p.scannedCachePath, "error", err)
		} else {
			p.reader.log.Info("probe offsets saved to scan cache", "path", p.scannedCachePath)
		}
	}
}

func (p *ProbeReader) pendingOffsets(moduleBase uintptr) OffsetSet {
	if p.activeOffsets.UnitTable != 0 && p.activeOffsets.UI != 0 && p.offsetsReadable(moduleBase, p.activeOffsets) {
		return p.activeOffsets
	}
	return p.offsets
}

func (p *ProbeReader) offsetsReadable(moduleBase uintptr, off OffsetSet) bool {
	if off.UnitTable == 0 || off.UI < 0xA {
		return false
	}
	if _, err := p.reader.ReadBytes(moduleBase+off.UnitTable, 8); err != nil {
		return false
	}
	if _, err := p.reader.ReadUint8(moduleBase + off.InGameGateOffset()); err != nil {
		return false
	}
	return true
}

// Snapshot reads the current probe state. Invalid snapshots carry a short Reason and Phase.
// Order: (1) gate + loading UI, (2) player + vitals + area, (3) finalizePhase, (4) entities when Valid && in_game.
func (p *ProbeReader) Snapshot() Snapshot {
	now := time.Now()
	p.snapshotGeneration++
	generation := p.snapshotGeneration

	if p.reader == nil || p.reader.access == nil {
		p.resetMercenaryStability()
		return invalidSnapshot(now, generation, GamePhaseUnknown, ReasonNotAttached)
	}

	moduleBase := p.reader.access.ModuleBase()
	if moduleBase == 0 {
		p.resetMercenaryStability()
		return invalidSnapshot(now, generation, GamePhaseUnknown, ReasonReadFailed)
	}

	off := p.ensureOffsets(moduleBase)

	// Step 1: gate byte + UI buffer (loading flag).
	gateValue, gateDisabled, loading, ui := p.readPhaseInputs(moduleBase, off)
	p.logGateChange(moduleBase, off, gateValue, gateDisabled)

	// Step 2: player + vitals + area/position.
	playerPtr, playerErr := p.findMainPlayer(moduleBase, off)
	playerFound := playerErr == nil

	phase := finalizePhase(gateValue, gateDisabled, loading, playerFound)

	if !playerFound {
		p.resetIdentityStability()
		p.resetMercenaryStability()
		reason := p.playerNotFoundReason(playerErr, gateValue, gateDisabled, loading)
		return invalidSnapshotWithUI(now, generation, phase, reason, ui)
	}

	areaID, posX, posY, err := p.readAreaAndPosition(playerPtr, off)
	if err != nil {
		p.resetMercenaryStability()
		return invalidSnapshotWithUI(now, generation, phase, ReasonReadFailed, ui)
	}
	playerUnitID, err := p.reader.ReadUint32(playerPtr + off.Unit.UnitID)
	if err != nil {
		p.resetMercenaryStability()
		return invalidSnapshotWithUI(now, generation, phase, ReasonReadFailed, ui)
	}

	statsListEx, err := p.reader.ReadUint64(playerPtr + off.Unit.StatsListEx)
	if err != nil || statsListEx == 0 {
		p.resetMercenaryStability()
		return invalidSnapshotWithUI(now, generation, phase, ReasonStatsUnavailable, ui)
	}

	vitals, statsSource, err := p.parseProbeVitalStats(uintptr(statsListEx), off)
	if err != nil {
		p.reader.log.Debug("probe stats unavailable",
			"player_ptr", fmt.Sprintf("0x%X", playerPtr),
			"stats_list_ex", fmt.Sprintf("0x%X", statsListEx),
			"error", err,
		)
		p.resetMercenaryStability()
		return invalidSnapshotWithUI(now, generation, phase, ReasonStatsUnavailable, ui)
	}
	vitals = p.normalizeVitalStats(playerPtr, vitals)
	statsHeader := uintptr(statsListEx) + off.Unit.StatsListBase
	if statsSource == "active" {
		statsHeader = uintptr(statsListEx) + off.Unit.StatsListActive
	}
	gold, goldErr := parseGoldStats(p.reader, statsHeader, off.Stats)
	if goldErr != nil {
		p.reader.log.Debug("player gold stats unavailable", "stats_source", statsSource, "error", goldErr)
	}

	snap := Snapshot{
		At:                    now,
		Generation:            generation,
		Valid:                 true,
		Phase:                 phase,
		PlayerPtr:             playerPtr,
		PlayerUnitID:          playerUnitID,
		StatsSource:           statsSource,
		HP:                    vitals.HP,
		MaxHP:                 vitals.MaxHP,
		Mana:                  vitals.Mana,
		MaxMana:               vitals.MaxMana,
		Gold:                  gold.Carried,
		PrivateStashGold:      gold.PrivateStash,
		GoldKnown:             gold.CarriedKnown,
		PrivateStashGoldKnown: gold.StashKnown,
		AreaID:                areaID,
		PosX:                  posX,
		PosY:                  posY,
		UI:                    ui,
	}

	// Step 4: entities and hover only when Valid && Phase == in_game.
	if snap.Valid && snap.Phase == GamePhaseInGame {
		snap.Identity = p.stabilizeIdentity(p.readIdentityProbe(playerPtr, off))
		p.logIdentityProbe(snap.Identity)
		p.enrichPlayerSkills(playerPtr, off, &snap)
		snap.ActiveWeaponSet = p.readActiveWeaponSetFromSkills(snap.PlayerSkills)
		snap.Hover = p.readHover(moduleBase, off)
		if err := p.enumerateEntities(moduleBase, off, &snap); err != nil {
			p.reader.log.Debug("entity enumeration failed", "error", err)
			p.resetMercenaryStability()
			snap.Mercenary = MercenarySnapshot{}
			snap.Objects = make([]ObjectUnit, 0)
			snap.Entrances = make([]EntranceUnit, 0)
			snap.Monsters = make([]MonsterUnit, 0)
		}
		if err := p.enumerateItems(moduleBase, off, &snap); err != nil {
			p.reader.log.Debug("item enumeration failed", "error", err)
			snap.Items = make([]ItemUnit, 0)
		}
		if snap.Items == nil {
			snap.Items = make([]ItemUnit, 0)
		}
		return snap
	}
	p.resetIdentityStability()
	p.resetMercenaryStability()

	return emptyEntitySlices(snap)
}

func (p *ProbeReader) logIdentityProbe(identity IdentityProbe) {
	if identity == p.lastIdentityProbe {
		return
	}
	p.lastIdentityProbe = identity
	p.reader.log.Info("read-only game identity probe",
		"valid", identity.Valid,
		"confirmed", identity.Confirmed,
		"stable_ticks", identity.StableTicks,
		"character_name", identity.CharacterName,
		"class_id", identity.ClassID,
		"map_seed", identity.MapSeed,
		"map_seed_valid", identity.MapSeedValid,
		"difficulty_valid", identity.DifficultyOK,
		"reason", identity.Reason,
	)
}

func (p *ProbeReader) enrichPlayerSkills(playerPtr uintptr, off OffsetSet, snap *Snapshot) {
	ps, err := p.readPlayerSkills(playerPtr, off)
	if err != nil {
		p.reader.log.Debug("player skills read failed", "error", err)
	} else {
		snap.PlayerSkills = ps
	}
}

func (p *ProbeReader) playerNotFoundReason(playerErr error, gateValue uint8, gateDisabled, loading bool) string {
	if !gateDisabled && gateValue != 1 && !loading {
		return ReasonNotInGame
	}
	if playerErr != nil {
		switch {
		case errors.Is(playerErr, errUnitTableUnavailable):
			return ReasonUnitTableUnavailable
		case errors.Is(playerErr, errPlayerNotFound):
			return ReasonPlayerPointerUnavailable
		default:
			return ReasonReadFailed
		}
	}
	return ReasonPlayerPointerUnavailable
}

func (p *ProbeReader) logGateChange(moduleBase uintptr, off OffsetSet, gateValue uint8, gateDisabled bool) {
	if gateDisabled {
		return
	}
	gate := off.InGameGateOffset()
	if gateValue != 1 {
		if !p.hasGateValue || p.lastGateValue != gateValue || time.Since(p.lastGateLog) >= 5*time.Second {
			p.reader.log.Debug("not in game", "gate", fmt.Sprintf("0x%X", moduleBase+gate), "value", gateValue)
			p.lastGateLog = time.Now()
		}
	}
	p.lastGateValue = gateValue
	p.hasGateValue = true
}

func (p *ProbeReader) normalizeVitalStats(playerPtr uintptr, vitals VitalStats) VitalStats {
	if p.lastPlayerPtr != playerPtr {
		p.lastPlayerPtr = playerPtr
		p.observedMaxHP = 0
		p.observedMaxMana = 0
	}

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

	var found uintptr
	visited := 0
	err = p.walkUnitSegment(moduleBase, off, unitSegmentPlayer, &visited, 0, func(unitAddr uintptr) (unitWalkAction, error) {
		isMain, mainErr := p.isMainPlayer(unitAddr, expansion, off)
		if mainErr != nil {
			return unitWalkContinue, mainErr
		}
		if isMain {
			found = unitAddr
			return unitWalkStop, nil
		}
		return unitWalkContinue, nil
	})
	if err != nil {
		return 0, err
	}
	if found == 0 {
		return 0, errPlayerNotFound
	}
	return found, nil
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
