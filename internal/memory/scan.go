package memory

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrPatternNotFound indicates a d2go signature was not found in the module image.
var ErrPatternNotFound = errors.New("pattern not found in module image")

type patternSpec struct {
	bytes []byte
	mask  string
}

// ScanProbeOffsets resolves patch-sensitive module offsets via d2go signatures (commit 16d248a53591).
func ScanProbeOffsets(reader *Reader, base OffsetSet) (OffsetSet, error) {
	if reader == nil || reader.access == nil {
		return base, ErrNotBound
	}

	moduleBase := reader.access.ModuleBase()
	moduleSize := reader.access.ModuleSize()
	if moduleBase == 0 || moduleSize == 0 {
		return base, fmt.Errorf("module image unavailable")
	}
	if moduleSize > maxModuleImageSize {
		moduleSize = maxModuleImageSize
	}

	image, err := reader.readModuleImage(moduleBase, moduleSize)
	if err != nil {
		return base, fmt.Errorf("read module image: %w", err)
	}

	out := base

	gameData, err := scanGameDataOffset(image)
	if err != nil {
		reader.log.Debug("game data pattern not found, keeping static offset", "error", err)
	} else {
		out.GameData = gameData
	}

	unitTable, err := scanUnitTableOffset(image)
	if err != nil {
		return base, fmt.Errorf("unit table: %w", err)
	}
	out.UnitTable = unitTable

	ui, err := scanUIOffset(image)
	if err != nil {
		return base, fmt.Errorf("ui: %w", err)
	}
	out.UI = ui

	expansion, err := scanExpansionOffset(image)
	if err != nil {
		reader.log.Debug("expansion pattern not found, probing both main-player flags", "error", err)
		out.Expansion = 0
	} else {
		out.Expansion = expansion
	}

	reader.log.Debug("probe module offsets scanned",
		"game_data", fmt.Sprintf("0x%X", out.GameData),
		"unit_table", fmt.Sprintf("0x%X", out.UnitTable),
		"ui", fmt.Sprintf("0x%X", out.UI),
		"expansion", fmt.Sprintf("0x%X", out.Expansion),
	)

	return out, nil
}

func (r *Reader) readModuleImage(moduleBase uintptr, moduleSize uint32) ([]byte, error) {
	if r.access == nil {
		return nil, ErrNotBound
	}
	size := int(moduleSize)
	buf := make([]byte, size)

	const pageSize = 4096
	successfulBytes := 0
	for offset := 0; offset < size; offset += pageSize {
		chunkSize := pageSize
		if remaining := size - offset; remaining < chunkSize {
			chunkSize = remaining
		}

		chunk := buf[offset : offset+chunkSize]
		if err := r.readWithRetry(moduleBase+uintptr(offset), chunk); err != nil {
			// D2R.exe can contain unreadable/guarded pages. Keep their bytes as zeroes
			// and continue scanning the readable parts, matching d2go's best-effort style.
			continue
		}
		successfulBytes += chunkSize
	}
	if successfulBytes == 0 {
		return nil, fmt.Errorf("no readable pages in module image")
	}
	return buf, nil
}

func scanGameDataOffset(image []byte) (uintptr, error) {
	idx, err := findPattern(image, patternSpec{
		bytes: []byte{0x44, 0x88, 0x25, 0x00, 0x00, 0x00, 0x00, 0x66, 0x44, 0x89, 0x25, 0x00, 0x00, 0x00, 0x00},
		mask:  "xxx????xxxx????",
	})
	if err != nil {
		return 0, err
	}
	if idx+7 > len(image) {
		return 0, fmt.Errorf("game data pattern operand out of range")
	}
	offsetInt := uintptr(binary.LittleEndian.Uint32(image[idx+3 : idx+7]))
	return uintptr(idx) - 0x121 + offsetInt, nil
}

func scanUnitTableOffset(image []byte) (uintptr, error) {
	idx, err := findPattern(image, patternSpec{
		bytes: []byte{0x48, 0x03, 0xC7, 0x49, 0x8B, 0x8C, 0xC6},
		mask:  "xxxxxxx",
	})
	if err != nil {
		return 0, err
	}
	if idx+11 > len(image) {
		return 0, fmt.Errorf("unit table pattern operand out of range")
	}
	return uintptr(binary.LittleEndian.Uint32(image[idx+7 : idx+11])), nil
}

func scanUIOffset(image []byte) (uintptr, error) {
	idx, err := findPattern(image, patternSpec{
		bytes: []byte{0x40, 0x84, 0xed, 0x0f, 0x94, 0x05},
		mask:  "xxxxxx",
	})
	if err != nil {
		return 0, err
	}
	if idx+10 > len(image) {
		return 0, fmt.Errorf("ui pattern operand out of range")
	}
	uiRel := uintptr(binary.LittleEndian.Uint32(image[idx+6 : idx+10]))
	return uintptr(idx) + 10 + uiRel, nil
}

func scanExpansionOffset(image []byte) (uintptr, error) {
	idx, err := findPattern(image, patternSpec{
		bytes: []byte{0x48, 0x8B, 0x05, 0x00, 0x00, 0x00, 0x00, 0x48, 0x8B, 0xD9, 0xF3, 0x0F, 0x10, 0x50, 0x00},
		mask:  "xxx????xxxxxxx?",
	})
	if err != nil {
		return 0, err
	}
	if idx+7 > len(image) {
		return 0, fmt.Errorf("expansion pattern operand out of range")
	}
	offsetPtr := uintptr(binary.LittleEndian.Uint32(image[idx+3 : idx+7]))
	return uintptr(idx) + 7 + offsetPtr, nil
}

func findPattern(image []byte, spec patternSpec) (int, error) {
	pattern := spec.bytes
	mask := spec.mask
	if len(pattern) != len(mask) {
		return 0, fmt.Errorf("pattern/mask length mismatch")
	}
	if len(pattern) == 0 || len(image) < len(pattern) {
		return 0, ErrPatternNotFound
	}

	limit := len(image) - len(pattern)
	for i := 0; i <= limit; i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if mask[j] == '?' {
				continue
			}
			if image[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return i, nil
		}
	}
	return 0, ErrPatternNotFound
}
