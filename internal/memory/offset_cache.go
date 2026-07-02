package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultScannedCacheFile is the default filename for persisted runtime scan results
// under the config directory (gitignored).
const DefaultScannedCacheFile = "offsets.scanned.yaml"

// LoadScannedOffsetCache loads patch-sensitive module offsets from a prior successful scan.
func LoadScannedOffsetCache(path string) (OffsetSet, error) {
	if path == "" {
		return OffsetSet{}, fmt.Errorf("scan cache path is empty")
	}
	off, err := LoadOffsetSetFile(path)
	if err != nil {
		return OffsetSet{}, err
	}
	if off.UnitTable == 0 || off.UI == 0 {
		return OffsetSet{}, fmt.Errorf("scan cache %q missing unit_table or ui", path)
	}
	return off, nil
}

// ResolveCachedOffsets applies module-base-dependent fields from a cached offset set.
func ResolveCachedOffsets(moduleBase uintptr, cached OffsetSet) OffsetSet {
	return cached
}

// SaveScannedOffsetCache persists patch-sensitive offsets from a successful runtime scan.
func SaveScannedOffsetCache(path string, moduleBase uintptr, base, scanned OffsetSet) error {
	if path == "" {
		return fmt.Errorf("scan cache path is empty")
	}
	if scanned.UnitTable == 0 || scanned.UI == 0 {
		return fmt.Errorf("refusing to save scan cache without unit_table and ui")
	}

	out := base
	if scanned.GameData != 0 {
		out.GameData = scanned.GameData
	}
	if scanned.UnitTable != 0 {
		out.UnitTable = scanned.UnitTable
	}
	if scanned.UI != 0 {
		out.UI = scanned.UI
	}
	out.Expansion = scanned.Expansion
	out.Hover = scanned.Hover
	out.Source = "runtime-scan"
	out.VerifiedAt = time.Now().Format("2006-01-02")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create scan cache dir: %w", err)
	}
	if err := SaveOffsetSetFile(path, out); err != nil {
		return err
	}
	return nil
}
