package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadIdentityRowsFromCurrentExport(t *testing.T) {
	src := filepath.Join("..", "..", ".tmp", "d2r-excel")
	bases, err := readRows(src)
	if err != nil {
		t.Fatal(err)
	}
	identities, stats, err := readIdentityRows(src, bases)
	if err != nil {
		t.Fatal(err)
	}
	if stats.SetRows != 140 || stats.SetMarkers != 1 || stats.UniqueRows != 433 || stats.UniqueMarkers != 6 {
		t.Fatalf("identity stats = %+v", stats)
	}
	var talRasha []identityRow
	for _, identity := range identities {
		if identity.SetKey == "Tal Rasha's Wrappings" {
			talRasha = append(talRasha, identity)
		}
	}
	if len(talRasha) != 5 {
		t.Fatalf("Tal Rasha entries = %d, want 5", len(talRasha))
	}
	want := map[string]string{
		"Tal Rasha's Fire-Spun Cloth": "zmb", "Tal Rasha's Adjudication": "amu", "Tal Rasha's Lidless Eye": "oba",
		"Tal Rasha's Howling Wind": "uth", "Tal Rasha's Horadric Crest": "xsk",
	}
	for _, identity := range talRasha {
		if identity.BaseCode != want[identity.SourceKey] || identity.DisplayName == "" || identity.SetName != "Tal Rasha's Wrappings" {
			t.Fatalf("Tal Rasha identity = %+v", identity)
		}
	}

	first, err := render(supportedSourceVersion, bases, identities)
	if err != nil {
		t.Fatal(err)
	}
	second, err := render(supportedSourceVersion, bases, identities)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two catalog renders are not byte-identical")
	}
}

func TestReadIdentityFileRejectsCorruptHeaderIDSpawnabilityAndBase(t *testing.T) {
	baseCodes := map[string]struct{}{"amu": {}}
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "header", content: "index\t*ID\tset\titem\nA\t1\tS\tamu\n", want: "spawnable"},
		{name: "duplicate id", content: "index\t*ID\tset\titem\tspawnable\nA\t1\tS\tamu\t1\nB\t1\tS\tamu\t1\n", want: "duplicate relevant ID"},
		{name: "spawnability", content: "index\t*ID\tset\titem\tspawnable\nA\t1\tS\tamu\tyes\n", want: "invalid spawnable"},
		{name: "base", content: "index\t*ID\tset\titem\tspawnable\nA\t1\tS\tmissing\t1\n", want: "unknown base code"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "setitems.txt")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readIdentityFile(path, "set", baseCodes); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestIdentityMetadataRowsAndRelevantKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "setitems.txt")
	content := "index\t*ID\tset\titem\tspawnable\nExpansion\t\t\t\t\nA\t1\tS\tamu\t1\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, markers, err := readIdentityFile(path, "set", map[string]struct{}{"amu": {}})
	if err != nil || len(rows) != 1 || markers != 1 {
		t.Fatalf("rows=%+v markers=%d error=%v", rows, markers, err)
	}

	duplicates := []identityRow{
		{Kind: "unique", SourceKey: "Same", disambiguator: "amu;a;b;c"},
		{Kind: "unique", SourceKey: "Same", disambiguator: "amu;a;b;c"},
	}
	if err := assignStableIdentityKeys(duplicates); err == nil || !strings.Contains(err.Error(), "duplicate relevant") {
		t.Fatalf("duplicate key error = %v", err)
	}
}

func TestResolveIdentityNamesRejectsMissingEmptyAndDuplicateRelevantNames(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "missing", json: `[{"Key":"Other","enUS":"Other"}]`, want: "missing relevant key"},
		{name: "empty", json: `[{"Key":"Item","enUS":""}]`, want: "no enUS"},
		{name: "duplicate", json: `[{"Key":"Item","enUS":"One"},{"Key":"Item","enUS":"Two"}]`, want: "duplicate relevant translation"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "item-names.json")
			if err := os.WriteFile(path, []byte(test.json), 0o644); err != nil {
				t.Fatal(err)
			}
			rows := []identityRow{{SourceKey: "Item"}}
			if err := resolveIdentityNames(path, rows); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	path := filepath.Join(t.TempDir(), "item-names.json")
	data := `[{"Key":"Item","enUS":"Item"},{"Key":"Unrelated","enUS":"One"},{"Key":"Unrelated","enUS":"Two"}]`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := []identityRow{{SourceKey: "Item"}}
	if err := resolveIdentityNames(path, rows); err != nil || rows[0].DisplayName != "Item" {
		t.Fatalf("unrelated duplicate affected relevant resolution: rows=%+v error=%v", rows, err)
	}
}
