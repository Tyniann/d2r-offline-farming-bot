package main

import "testing"

func TestValidateGemDimensions(t *testing.T) {
	rows := []row{
		{Name: "Flawless Emerald", Code: "glg", Type: "geme", Width: 1, Height: 1},
		{Name: "Perfect Skull", Code: "skz", Type: "gemz", Width: 1, Height: 1},
		{Name: "Grand Charm", Code: "cm3", Type: "lcha", Width: 1, Height: 3},
	}
	if err := validateGemDimensions(rows); err != nil {
		t.Fatalf("validateGemDimensions() error = %v", err)
	}
}

func TestValidateGemDimensionsRejectsUnsafeExport(t *testing.T) {
	rows := []row{{Name: "Flawed Ruby", Code: "gfr", Type: "gemr", Width: 0, Height: 0}}
	if err := validateGemDimensions(rows); err == nil {
		t.Fatal("validateGemDimensions() error = nil, want invalid gem dimensions")
	}
}
