package memory

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadOffsetSetFileExample(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "offsets.example.yaml")
	got, err := LoadOffsetSetFile(path)
	if err != nil {
		t.Fatalf("LoadOffsetSetFile() error = %v", err)
	}

	def := DefaultOffsetSet()
	if got.Name != def.Name {
		t.Errorf("Name = %q, want %q", got.Name, def.Name)
	}
	if got.UnitTable != def.UnitTable {
		t.Errorf("UnitTable = 0x%X, want 0x%X", got.UnitTable, def.UnitTable)
	}
	if got.Stats.EntryStride != 8 {
		t.Fatalf("Stats.EntryStride = %d, want 8", got.Stats.EntryStride)
	}
}

func TestLoadOffsetSetFilePartialOverlay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.yaml")
	content := `name: custom-set
unit_table: "0x1234567"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadOffsetSetFile(path)
	if err != nil {
		t.Fatalf("LoadOffsetSetFile() error = %v", err)
	}
	if got.Name != "custom-set" {
		t.Errorf("Name = %q, want custom-set", got.Name)
	}
	if got.UnitTable != 0x1234567 {
		t.Errorf("UnitTable = 0x%X, want 0x1234567", got.UnitTable)
	}
	if got.UI != DefaultOffsetSet().UI {
		t.Errorf("UI should keep default, got 0x%X", got.UI)
	}
}

func TestLoadOffsetSetFileInvalidHex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("unit_table: 0xZZ\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOffsetSetFile(path); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestLoadOffsetSetFileValidationFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(path, []byte("name: broken\nunit_table: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOffsetSetFile(path); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestResolveOffsetSetDefault(t *testing.T) {
	got, err := ResolveOffsetSet("")
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultOffsetSet()
	if got.Name != def.Name {
		t.Errorf("Name = %q, want %q", got.Name, def.Name)
	}
}

func TestHexUintptrUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want uintptr
	}{
		{"hex", "0x1A", 0x1A},
		{"decimal", "42", 42},
		{"zero", "0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h hexUintptr
			node := &yaml.Node{Kind: yaml.ScalarNode, Value: tt.raw}
			if err := h.UnmarshalYAML(node); err != nil {
				t.Fatal(err)
			}
			if !h.Set || h.Value != tt.want {
				t.Fatalf("got (%d, set=%v), want (%d, set=true)", h.Value, h.Set, tt.want)
			}
		})
	}
}
