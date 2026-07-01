package memory

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadScannedOffsetCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultScannedCacheFile)

	base := DefaultOffsetSet()
	moduleBase := uintptr(0x7FF7C75B0000)
	scanned := base
	scanned.UnitTable = 0x1EB9430
	scanned.UI = 0x1EC9134
	scanned.Expansion = 0x1EA0000

	if err := SaveScannedOffsetCache(path, moduleBase, base, scanned); err != nil {
		t.Fatalf("SaveScannedOffsetCache() error = %v", err)
	}

	got, err := LoadScannedOffsetCache(path)
	if err != nil {
		t.Fatalf("LoadScannedOffsetCache() error = %v", err)
	}
	if got.UnitTable != scanned.UnitTable || got.UI != scanned.UI {
		t.Fatalf("loaded offsets = unit_table %#x ui %#x, want %#x %#x",
			got.UnitTable, got.UI, scanned.UnitTable, scanned.UI)
	}
	resolved := ResolveCachedOffsets(moduleBase, got)
	if resolved.UnitTable != got.UnitTable || resolved.UI != got.UI {
		t.Fatalf("resolved offsets = unit_table %#x ui %#x, want %#x %#x",
			resolved.UnitTable, resolved.UI, got.UnitTable, got.UI)
	}
	if got.Source != "runtime-scan" {
		t.Fatalf("Source = %q, want runtime-scan", got.Source)
	}
}

func TestProbeEnsureOffsetsUsesReadableScanCache(t *testing.T) {
	_, probe, moduleBase := setupProbeMock(t)
	off := testOffsetSet()

	cacheDir := t.TempDir()
	cachePath := filepath.Join(cacheDir, DefaultScannedCacheFile)
	cached := DefaultOffsetSet()
	cached.UnitTable = off.UnitTable
	cached.UI = off.UI
	if err := SaveScannedOffsetCache(cachePath, moduleBase, cached, cached); err != nil {
		t.Fatal(err)
	}

	probe.scannedCachePath = cachePath
	probe.offsetsResolved = false
	probe.offsets = DefaultOffsetSet()
	probe.offsets.UnitTable = 0xDEAD
	probe.offsets.UI = 0xBEEF

	active := probe.ensureOffsets(moduleBase)
	if !probe.offsetsResolved {
		t.Fatal("expected offsets to resolve from scan cache")
	}
	if active.UnitTable != off.UnitTable || active.UI != off.UI {
		t.Fatalf("active offsets = unit_table %#x ui %#x, want cache %#x %#x",
			active.UnitTable, active.UI, off.UnitTable, off.UI)
	}
}
