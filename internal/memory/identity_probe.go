package memory

import (
	"bytes"
	"fmt"
	"math"
)

const (
	identityPlayerNameMaxBytes = 16

	// Player/Act offsets follow d2go 8ae0eb55 and Koolo's map-seed reader.
	// They remain experimental until Phase 6.1a live validation is complete.
	// D2UnitStrc.dwClassId is the player class for unit type 0. The older
	// d2go player reader used +0x174, which live validation rejected on D2R 3.2.
	identityPlayerClassOffset = uintptr(0x04)
	identityPlayerActOffset   = uintptr(0x20)
	identityActMiscOffset     = uintptr(0x78)
	identityInitSeedOffset    = uintptr(0x840)
	identityEndSeedOffset     = uintptr(0x868)
)

// IdentityProbe is a read-only Phase 6.1a sample from known player and Act sources.
// Difficulty remains unavailable until a separate source is live-validated.
type IdentityProbe struct {
	Valid         bool
	Confirmed     bool
	StableTicks   uint8
	CharacterName string
	ClassID       uint32
	MapSeed       uint32
	MapSeedValid  bool
	Difficulty    uint8
	DifficultyOK  bool
	Reason        string
}

const identityConfirmTicks = 3

func (p *ProbeReader) stabilizeIdentity(raw IdentityProbe) IdentityProbe {
	if !raw.Valid {
		p.resetIdentityStability()
		return raw
	}
	if !sameIdentityCandidate(p.identityCandidate, raw) {
		p.identityCandidate = raw
		p.identityStableTicks = 1
	} else if p.identityStableTicks < identityConfirmTicks {
		p.identityStableTicks++
	}
	raw.StableTicks = p.identityStableTicks
	raw.Confirmed = p.identityStableTicks >= identityConfirmTicks
	return raw
}

func sameIdentityCandidate(a, b IdentityProbe) bool {
	return a.Valid && b.Valid && a.CharacterName == b.CharacterName && a.ClassID == b.ClassID
}

func (p *ProbeReader) resetIdentityStability() {
	p.identityCandidate = IdentityProbe{}
	p.identityStableTicks = 0
}

func (p *ProbeReader) readIdentityProbe(playerPtr uintptr, off OffsetSet) IdentityProbe {
	unitData, err := p.reader.ReadUint64(playerPtr + off.Unit.UnitData)
	if err != nil || unitData == 0 {
		return IdentityProbe{Reason: "player_data_unavailable"}
	}

	nameBytes, err := p.reader.ReadBytes(uintptr(unitData), identityPlayerNameMaxBytes)
	if err != nil {
		return IdentityProbe{Reason: "character_name_unavailable"}
	}
	name, ok := parseCharacterName(nameBytes)
	if !ok {
		return IdentityProbe{Reason: "character_name_invalid"}
	}

	classID, err := p.reader.ReadUint32(playerPtr + identityPlayerClassOffset)
	if err != nil || classID > 7 {
		return IdentityProbe{CharacterName: name, Reason: "character_class_invalid"}
	}

	probe := IdentityProbe{
		Valid:         true,
		CharacterName: name,
		ClassID:       classID,
		Reason:        "difficulty_source_unresolved",
	}
	if seed, err := p.readMapSeed(playerPtr); err == nil {
		probe.MapSeed = seed
		probe.MapSeedValid = true
	}
	return probe
}

func parseCharacterName(raw []byte) (string, bool) {
	if i := bytes.IndexByte(raw, 0); i >= 0 {
		raw = raw[:i]
	}
	if len(raw) < 2 || len(raw) > 15 {
		return "", false
	}
	for _, b := range raw {
		if b < 0x21 || b > 0x7e {
			return "", false
		}
	}
	return string(raw), true
}

func (p *ProbeReader) readMapSeed(playerPtr uintptr) (uint32, error) {
	act, err := p.reader.ReadUint64(playerPtr + identityPlayerActOffset)
	if err != nil {
		return 0, fmt.Errorf("read player act: %w", err)
	}
	if act == 0 {
		return 0, fmt.Errorf("read player act: null pointer")
	}
	actMisc, err := p.reader.ReadUint64(uintptr(act) + identityActMiscOffset)
	if err != nil {
		return 0, fmt.Errorf("read act misc: %w", err)
	}
	if actMisc == 0 {
		return 0, fmt.Errorf("read act misc: null pointer")
	}
	initHash, err := p.reader.ReadUint32(uintptr(actMisc) + identityInitSeedOffset)
	if err != nil {
		return 0, fmt.Errorf("read initial seed hash: %w", err)
	}
	endHash, err := p.reader.ReadUint32(uintptr(actMisc) + identityEndSeedOffset)
	if err != nil {
		return 0, fmt.Errorf("read final seed hash: %w", err)
	}
	seed, ok := reverseMapSeed(initHash, endHash)
	if !ok {
		return 0, fmt.Errorf("reverse map seed hashes %#x/%#x", initHash, endHash)
	}
	return seed, nil
}

// reverseMapSeed follows the MapAssist algorithm referenced by d2go/Koolo.
func reverseMapSeed(initHash, endHash uint32) (uint32, bool) {
	const divisor = uint64(1 << 16)
	increment := uint64(1)
	for candidate := uint64(0); candidate <= math.MaxUint32; candidate += increment {
		result := (candidate*0x6AC690C5 + 666) & math.MaxUint32
		if uint32(result) == endHash {
			seed := uint32(candidate)
			if initHash^seed == 0 {
				return 0, false
			}
			return seed, true
		}
		if increment == 1 && result%divisor == uint64(endHash)%divisor {
			increment = divisor
		}
	}
	return 0, false
}
