package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestCharacterSavePrefixReadsExactlyBoundedV105Data(t *testing.T) {
	data := append(characterSaveTestPrefix("MrBones", 105, 2), bytes.Repeat([]byte{0xCC}, 128)...)
	reader := &countingReader{reader: bytes.NewReader(data)}
	header, err := readCharacterSavePrefix(reader, "mrbones")
	if err != nil {
		t.Fatal(err)
	}
	if reader.read != Phase16D2SPrefixLength {
		t.Fatalf("read=%d want=%d", reader.read, Phase16D2SPrefixLength)
	}
	if header.Name != "MrBones" || header.SaveVersion != 105 || header.Class != world.CharacterClassNecromancer {
		t.Fatalf("header=%+v", header)
	}
}

func TestCharacterSavePrefixMapsAllEightClasses(t *testing.T) {
	for id := byte(0); id <= 7; id++ {
		header, err := readCharacterSavePrefix(bytes.NewReader(characterSaveTestPrefix("Hero", 105, id)), "Hero")
		if err != nil {
			t.Fatalf("class %d: %v", id, err)
		}
		if header.Class != world.CharacterClass(id) {
			t.Fatalf("class %d mapped to %v", id, header.Class)
		}
	}
}

func TestCharacterSavePrefixRejectsEveryShortLength(t *testing.T) {
	valid := characterSaveTestPrefix("Hero", 105, 2)
	for length := 0; length < Phase16D2SPrefixLength; length++ {
		_, err := readCharacterSavePrefix(bytes.NewReader(valid[:length]), "Hero")
		requireCharacterSaveReason(t, err, Phase16ReasonCharacterSaveHeaderInvalid)
	}
}

func TestCharacterSavePrefixReasonPrecedenceAndNameRules(t *testing.T) {
	tests := []struct {
		name     string
		prefix   []byte
		filename string
		reason   Phase16CharacterReasonCode
	}{
		{name: "magic", prefix: func() []byte {
			value := characterSaveTestPrefix("Hero", 105, 2)
			value[0] = 0
			return value
		}(), filename: "Hero", reason: Phase16ReasonCharacterSaveHeaderInvalid},
		{name: "version before name", prefix: func() []byte {
			value := characterSaveTestPrefix("Hero", 106, 2)
			value[Phase16D2SNameOffset] = 0
			return value
		}(), filename: "Hero", reason: Phase16ReasonCharacterSaveVersionUnsupported},
		{name: "empty name", prefix: func() []byte {
			value := characterSaveTestPrefix("Hero", 105, 2)
			clear(value[Phase16D2SNameOffset : Phase16D2SNameOffset+Phase16D2SNameLength])
			return value
		}(), filename: "Hero", reason: Phase16ReasonCharacterSaveHeaderInvalid},
		{name: "missing terminator", prefix: func() []byte {
			value := characterSaveTestPrefix("Hero", 105, 2)
			copy(value[Phase16D2SNameOffset:], bytes.Repeat([]byte{'A'}, Phase16D2SNameLength))
			return value
		}(), filename: "Hero", reason: Phase16ReasonCharacterSaveHeaderInvalid},
		{name: "invalid ascii", prefix: func() []byte {
			value := characterSaveTestPrefix("Hero", 105, 2)
			value[Phase16D2SNameOffset] = 0xFF
			return value
		}(), filename: "Hero", reason: Phase16ReasonCharacterSaveHeaderInvalid},
		{name: "name mismatch before class", prefix: characterSaveTestPrefix("Other", 105, 8), filename: "Hero", reason: Phase16ReasonCharacterSaveNameMismatch},
		{name: "unknown class", prefix: characterSaveTestPrefix("Hero", 105, 8), filename: "Hero", reason: Phase16ReasonCharacterClassUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := readCharacterSavePrefix(bytes.NewReader(test.prefix), test.filename)
			requireCharacterSaveReason(t, err, test.reason)
		})
	}
}

func TestCharacterSavePrefixIgnoresBytesAfterFirstNameTerminator(t *testing.T) {
	prefix := characterSaveTestPrefix("MrBook", 105, 7)
	prefix[Phase16D2SNameOffset+len("MrBook")+1] = 0xFF
	header, err := parseCharacterSavePrefix(prefix, "mrbook")
	if err != nil {
		t.Fatal(err)
	}
	if header.Name != "MrBook" || header.Class != world.CharacterClassWarlock {
		t.Fatalf("header=%+v", header)
	}
}

func TestCharacterSavePrefixClassifiesNonEOFReadFailureAsUnreadable(t *testing.T) {
	_, err := readCharacterSavePrefix(errorReader{err: errors.New("read failed")}, "Hero")
	requireCharacterSaveReason(t, err, Phase16ReasonCharacterSaveUnreadable)
}

func TestReadCharacterSaveHeaderUsesRegularReadOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "MrHammer.d2s")
	writeCharacterSaveTestFile(t, path, "MrHammer", 105, 3)
	header, err := readCharacterSaveHeader(path, "MrHammer")
	if err != nil {
		t.Fatal(err)
	}
	if header.Class != world.CharacterClassPaladin {
		t.Fatalf("header=%+v", header)
	}
}

type countingReader struct {
	reader io.Reader
	read   int
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	count, err := r.reader.Read(buffer)
	r.read += count
	return count, err
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func characterSaveTestPrefix(name string, version uint32, classID byte) []byte {
	prefix := make([]byte, Phase16D2SPrefixLength)
	binary.LittleEndian.PutUint32(prefix[0:4], Phase16D2SMagic)
	binary.LittleEndian.PutUint32(prefix[4:8], version)
	prefix[Phase16D2SClassOffset] = classID
	copy(prefix[Phase16D2SNameOffset:Phase16D2SNameOffset+Phase16D2SNameLength], name)
	return prefix
}

func writeCharacterSaveTestFile(t *testing.T, path, name string, version uint32, classID byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, characterSaveTestPrefix(name, version, classID), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireCharacterSaveReason(t *testing.T, err error, want Phase16CharacterReasonCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %q", want)
	}
	if got := characterSaveErrorReason(err); got != want {
		t.Fatalf("reason=%q want=%q err=%v", got, want, err)
	}
}
