package memory

import (
	"encoding/binary"
	"testing"
)

func TestParseCharacterName(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want string
		ok   bool
	}{
		{name: "valid", raw: append([]byte("MyNecro"), make([]byte, 9)...), want: "MyNecro", ok: true},
		{name: "hyphen", raw: []byte("Bone-Necro\x00"), want: "Bone-Necro", ok: true},
		{name: "too short", raw: []byte("A\x00"), ok: false},
		{name: "unterminated too long", raw: []byte("1234567890123456"), ok: false},
		{name: "control", raw: []byte{'A', '\n', 0}, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCharacterName(tt.raw)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("parseCharacterName() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestReadIdentityProbeUsesKnownPlayerSources(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	off := testOffsetSet()
	const player = uintptr(0x20000)
	const playerData = uintptr(0x2A000)
	writeU64(access, player+off.Unit.UnitData, uint64(playerData))
	name := make([]byte, identityPlayerNameMaxBytes)
	copy(name, "MyNecro")
	access.setBytes(playerData, name)
	writeU32(access, player+identityPlayerClassOffset, 2)

	got := probe.readIdentityProbe(player, off)
	if !got.Valid || got.CharacterName != "MyNecro" || got.ClassID != 2 {
		t.Fatalf("readIdentityProbe() = %+v", got)
	}
	if got.DifficultyOK || got.Reason != "difficulty_source_unresolved" {
		t.Fatalf("difficulty must remain unresolved in 6.1a, got %+v", got)
	}
}

func TestReadIdentityProbeAcceptsWarlockAndRejectsUnknownClass(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	off := testOffsetSet()
	const player = uintptr(0x20000)
	const playerData = uintptr(0x2A000)
	writeU64(access, player+off.Unit.UnitData, uint64(playerData))
	name := make([]byte, identityPlayerNameMaxBytes)
	copy(name, "MyNecro")
	access.setBytes(playerData, name)
	writeU32(access, player+identityPlayerClassOffset, 7)

	got := probe.readIdentityProbe(player, off)
	if !got.Valid || got.ClassID != 7 {
		t.Fatalf("warlock readIdentityProbe() = %+v", got)
	}
	writeU32(access, player+identityPlayerClassOffset, 8)
	got = probe.readIdentityProbe(player, off)
	if got.Valid || got.Reason != "character_class_invalid" {
		t.Fatalf("readIdentityProbe() = %+v", got)
	}
}

func TestReverseMapSeed(t *testing.T) {
	const seed = uint32(0x12345678)
	endHash := uint32((uint64(seed)*0x6AC690C5 + 666) & 0xffffffff)
	initHash := seed ^ 0x00ABCDEF
	got, ok := reverseMapSeed(initHash, endHash)
	if !ok || got != seed {
		t.Fatalf("reverseMapSeed() = %#x, %v; want %#x, true", got, ok, seed)
	}
}

func TestReadMapSeedUsesKooloActChain(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	const player = uintptr(0x20000)
	const act = uintptr(0x2B000)
	const actMisc = uintptr(0x2C000)
	const seed = uint32(0x10203040)
	endHash := uint32((uint64(seed)*0x6AC690C5 + 666) & 0xffffffff)
	writeU64(access, player+identityPlayerActOffset, uint64(act))
	writeU64(access, act+identityActMiscOffset, uint64(actMisc))
	writeU32(access, actMisc+identityInitSeedOffset, seed^0x00112233)
	writeU32(access, actMisc+identityEndSeedOffset, endHash)

	got, err := probe.readMapSeed(player)
	if err != nil || got != seed {
		t.Fatalf("readMapSeed() = %#x, %v; want %#x", got, err, seed)
	}
}

func TestSnapshotCarriesIdentityProbe(t *testing.T) {
	access, probe, _ := setupProbeMock(t)
	off := testOffsetSet()
	const player = uintptr(0x20000)
	const playerData = uintptr(0x2A000)
	writeU64(access, player+off.Unit.UnitData, uint64(playerData))
	name := make([]byte, identityPlayerNameMaxBytes)
	copy(name, "MyNecro")
	access.setBytes(playerData, name)
	class := make([]byte, 4)
	binary.LittleEndian.PutUint32(class, 2)
	access.setBytes(player+identityPlayerClassOffset, class)

	probe.Snapshot()
	probe.Snapshot()
	snap := probe.Snapshot()
	if !snap.Identity.Valid || !snap.Identity.Confirmed || snap.Identity.StableTicks != 3 || snap.Identity.CharacterName != "MyNecro" {
		t.Fatalf("Snapshot().Identity = %+v", snap.Identity)
	}
}

func TestStabilizeIdentityResetsOnChangeAndInvalid(t *testing.T) {
	_, probe, _ := setupProbeMock(t)
	a := IdentityProbe{Valid: true, CharacterName: "Alpha", ClassID: 2}
	b := IdentityProbe{Valid: true, CharacterName: "Beta", ClassID: 3}
	for i := 0; i < identityConfirmTicks; i++ {
		a = probe.stabilizeIdentity(a)
	}
	if !a.Confirmed {
		t.Fatalf("Alpha not confirmed: %+v", a)
	}
	b = probe.stabilizeIdentity(b)
	if b.Confirmed || b.StableTicks != 1 {
		t.Fatalf("changed identity should restart confirmation: %+v", b)
	}
	probe.stabilizeIdentity(IdentityProbe{Reason: "not_in_game"})
	a = probe.stabilizeIdentity(IdentityProbe{Valid: true, CharacterName: "Alpha", ClassID: 2})
	if a.Confirmed || a.StableTicks != 1 {
		t.Fatalf("identity after invalid state should restart: %+v", a)
	}
}
