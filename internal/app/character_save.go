package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type characterSaveHeader struct {
	Name        string
	SaveVersion uint32
	Class       world.CharacterClass
}

type characterSaveError struct {
	reason Phase16CharacterReasonCode
	err    error
}

func (e *characterSaveError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.reason, e.err)
}

func (e *characterSaveError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newCharacterSaveError(reason Phase16CharacterReasonCode, format string, values ...any) error {
	return &characterSaveError{reason: reason, err: fmt.Errorf(format, values...)}
}

func characterSaveErrorReason(err error) Phase16CharacterReasonCode {
	var saveErr *characterSaveError
	if errors.As(err, &saveErr) {
		return saveErr.reason
	}
	return Phase16ReasonCharacterSaveUnreadable
}

func readCharacterSaveHeader(path, filenameName string) (characterSaveHeader, error) {
	file, err := os.Open(path)
	if err != nil {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveUnreadable, "open save: %w", err)
	}
	defer file.Close()

	openedInfo, err := file.Stat()
	if err != nil {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveUnreadable, "stat opened save: %w", err)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveUnreadable, "recheck save path: %w", err)
	}
	// Der erneute Pfadcheck bindet den geöffneten Handle an denselben weiterhin
	// regulären, reparse-freien Eintrag. Ein Austausch während des Scans wird
	// fail-closed statt über einen neuen Pfad weiterverfolgt.
	if !openedInfo.Mode().IsRegular() || !pathInfo.Mode().IsRegular() ||
		pathInfo.Mode()&os.ModeSymlink != 0 || fileInfoIsReparsePoint(pathInfo) ||
		!os.SameFile(openedInfo, pathInfo) {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveUnreadable, "save path changed or is not a regular file")
	}

	return readCharacterSavePrefix(file, filenameName)
}

func readCharacterSavePrefix(reader io.Reader, filenameName string) (characterSaveHeader, error) {
	prefix := make([]byte, Phase16D2SPrefixLength)
	if _, err := io.ReadFull(reader, prefix); err != nil {
		reason := Phase16ReasonCharacterSaveUnreadable
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			reason = Phase16ReasonCharacterSaveHeaderInvalid
		}
		return characterSaveHeader{}, newCharacterSaveError(reason, "read %d-byte save prefix: %w", Phase16D2SPrefixLength, err)
	}
	return parseCharacterSavePrefix(prefix, filenameName)
}

func parseCharacterSavePrefix(prefix []byte, filenameName string) (characterSaveHeader, error) {
	if len(prefix) != Phase16D2SPrefixLength {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveHeaderInvalid, "save prefix has %d bytes, want %d", len(prefix), Phase16D2SPrefixLength)
	}
	if magic := binary.LittleEndian.Uint32(prefix[0:4]); magic != Phase16D2SMagic {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveHeaderInvalid, "unexpected save magic 0x%08X", magic)
	}
	version := binary.LittleEndian.Uint32(prefix[4:8])
	if !phase16D2SVersionAllowed(version) {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveVersionUnsupported, "save version %d is not allowed", version)
	}

	nameSlot := prefix[Phase16D2SNameOffset : Phase16D2SNameOffset+Phase16D2SNameLength]
	terminator := bytes.IndexByte(nameSlot, 0)
	if terminator <= 0 {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveHeaderInvalid, "save name is empty or not terminated in the first %d bytes", Phase16D2SNameLength)
	}
	headerName, err := validateOfflineCharacter(string(nameSlot[:terminator]))
	if err != nil {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveHeaderInvalid, "validate save name: %w", err)
	}
	validatedFilename, err := validateOfflineCharacter(filenameName)
	if err != nil {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveNameMismatch, "validate save filename: %w", err)
	}
	if !strings.EqualFold(headerName, validatedFilename) {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterSaveNameMismatch, "save filename and header name differ")
	}

	classID := prefix[Phase16D2SClassOffset]
	class, ok := phase16D2SClass(classID)
	if !ok {
		return characterSaveHeader{}, newCharacterSaveError(Phase16ReasonCharacterClassUnknown, "unknown character class id %d", classID)
	}
	return characterSaveHeader{Name: headerName, SaveVersion: version, Class: class}, nil
}

func phase16D2SVersionAllowed(version uint32) bool {
	for _, allowed := range Phase16D2SAllowedVersions() {
		if version == allowed {
			return true
		}
	}
	return false
}

func phase16D2SClass(id byte) (world.CharacterClass, bool) {
	for _, mapping := range Phase16D2SClasses() {
		if mapping.ID == id {
			return mapping.Class, true
		}
	}
	return 0, false
}
