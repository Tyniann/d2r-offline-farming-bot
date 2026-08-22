package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAreaNamePairsKeepsDistinctGermanAndEnglishNames(t *testing.T) {
	pairs, err := readAreaNamePairs(filepath.Join("testdata", "game_i18n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := pairs["1"]; got.DE != "Testgebiet" || got.EN != "Fixture Area" {
		t.Fatalf("area 1 = %#v", got)
	}
}

func TestReadAreaNamePairsRejectsMissingRequiredLocale(t *testing.T) {
	dir := t.TempDir()
	levels, err := os.ReadFile(filepath.Join("testdata", "game_i18n", "levels.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(filepath.Join(dir, "levels.txt"), levels, 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	if writeErr := os.WriteFile(filepath.Join(dir, "levels.json"), []byte(`[{"Key":"FixtureArea","enUS":"Fixture Area","deDE":""}]`), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = readAreaNamePairs(dir)
	if err == nil || !strings.Contains(err.Error(), "missing levels.json key") {
		t.Fatalf("error = %v", err)
	}
}

func TestCleanD2RNameRemovesGenderMarker(t *testing.T) {
	if got := cleanD2RName("[fs]Io-Rune"); got != "Io-Rune" {
		t.Fatalf("cleanD2RName() = %q", got)
	}
}
