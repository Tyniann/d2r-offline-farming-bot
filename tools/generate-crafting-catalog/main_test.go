package main

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCraftingCatalogMatchesAuthenticFixtures(t *testing.T) {
	rows, err := generate("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 23 {
		t.Fatalf("recipes=%d, want 23", len(rows))
	}
	gems, runes := 0, 0
	for _, row := range rows {
		switch row.version {
		case 0:
			gems++
		case 100:
			runes++
		default:
			t.Fatalf("unexpected version: %+v", row)
		}
	}
	if gems != 14 || runes != 9 {
		t.Fatalf("gems=%d runes=%d", gems, runes)
	}
	data, err := render("3.2.92777", rows)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "internal", "crafting", "catalog_data.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")), data) {
		t.Fatal("generated catalog differs from the authentic fixture projection")
	}
}

func TestCraftingCatalogRejectsRecipeDrift(t *testing.T) {
	for _, tc := range []struct{ field, value string }{
		{"input 2", "gsv"}, {"output b", "gpv"}, {"output c", "gpv"},
		{"input 1", "gsv,qty=2"}, {"output", "gpv"}, {"mod 1", "dmg%"},
		{"op", "1"}, {"class", "sor"}, {"firstLadderSeason", "1"},
		{"min diff", "1"}, {"enabled", "0"}, {"version", "100"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			dir := copyFixtures(t)
			mutateFixture(t, filepath.Join(dir, "cubemain.txt"), "description", specs()[0].key, tc.field, tc.value)
			if _, err := generate(dir); err == nil || !strings.Contains(err.Error(), specs()[0].key) {
				t.Fatalf("expected rejection with stable recipe key, got %v", err)
			}
		})
	}
	for _, tc := range []struct{ field, value string }{{"invwidth", "2"}, {"code", ""}, {"type", "rune"}} {
		t.Run("misc-"+tc.field, func(t *testing.T) {
			dir := copyFixtures(t)
			mutateFixture(t, filepath.Join(dir, "misc.txt"), "name", "Amethyst", tc.field, tc.value)
			if _, err := generate(dir); err == nil {
				t.Fatal("invalid misc item accepted")
			}
		})
	}
}

func copyFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"cubemain.txt", "misc.txt"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func mutateFixture(t *testing.T, path, key, value, column, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	keyIndex, columnIndex := -1, -1
	for i, header := range rows[0] {
		if header == key {
			keyIndex = i
		}
		if header == column {
			columnIndex = i
		}
	}
	if keyIndex < 0 || columnIndex < 0 {
		t.Fatal("fixture column missing")
	}
	found := false
	for _, row := range rows[1:] {
		if row[keyIndex] == value {
			row[columnIndex] = replacement
			found = true
		}
	}
	if !found {
		t.Fatal("fixture row missing")
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Comma = '\t'
	if err := w.WriteAll(rows); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
