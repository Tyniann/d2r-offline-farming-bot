package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestReadFileRejectsMissingRequiredColumnAndInvalidDimension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weapons.txt")
	missing := "code\tname\ttype\tinvwidth\tinvheight\tnormcode\tubercode\ncap\tCap\thelm\t2\t2\tcap\txap\n"
	if err := os.WriteFile(path, []byte(missing), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readFile(path); err == nil || !strings.Contains(err.Error(), "ultracode") {
		t.Fatalf("missing-column error = %v", err)
	}
	broken := "code\tname\ttype\tinvwidth\tinvheight\tnormcode\tubercode\tultracode\ncap\tCap\thelm\twide\t2\tcap\txap\tuap\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readFile(path); err == nil || !strings.Contains(err.Error(), "invalid invwidth") {
		t.Fatalf("invalid-dimension error = %v", err)
	}
}

func TestClassifyBaseTiersFromEquipmentChain(t *testing.T) {
	rows := []row{
		{Code: "cap", NormalCode: "cap", UberCode: "xap", UltraCode: "uap", BaseTier: "unknown"},
		{Code: "xap", NormalCode: "cap", UberCode: "xap", UltraCode: "uap", BaseTier: "unknown"},
		{Code: "uap", NormalCode: "cap", UberCode: "xap", UltraCode: "uap", BaseTier: "unknown"},
	}
	if err := classifyBaseTiers(rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].BaseTier != "normal" || rows[1].BaseTier != "exceptional" || rows[2].BaseTier != "elite" {
		t.Fatalf("tiers = %q/%q/%q", rows[0].BaseTier, rows[1].BaseTier, rows[2].BaseTier)
	}
}

func TestClassifyBaseTiersRejectsUnknownReferenceAndCycle(t *testing.T) {
	tests := []struct {
		name string
		rows []row
		want string
	}{
		{"unknown", []row{{Code: "cap", NormalCode: "cap", UberCode: "missing"}}, "unknown tier code"},
		{"cycle", []row{{Code: "a", NormalCode: "a", UberCode: "b"}, {Code: "b", NormalCode: "b", UberCode: "a"}}, "tier cycle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyBaseTiers(tt.rows)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRenderIncludesVersionAndTier(t *testing.T) {
	data, err := render(supportedSourceVersion, []row{{Code: "uap", BaseTier: "elite"}}, []identityRow{{Kind: "set", RawID: 7, Key: "Set Key", DisplayName: "Set Name", BaseCode: "uap"}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "D2R "+supportedSourceVersion) || !strings.Contains(text, "BaseTier: BaseTierElite") || !strings.Contains(text, "Kind: ItemIdentitySet") {
		t.Fatalf("generated source = %s", text)
	}
}

func TestValidateSourceVersionRejectsWrongVersion(t *testing.T) {
	if err := validateSourceVersion("3.2.wrong"); err == nil {
		t.Fatal("wrong source version was accepted")
	}
	out := filepath.Join(t.TempDir(), "catalog.go")
	want := []byte("existing output")
	if err := os.WriteFile(out, want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generateCatalog("3.2.wrong", t.TempDir(), out); err == nil {
		t.Fatal("generation with wrong source version succeeded")
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("wrong source version changed output to %q", got)
	}
}

func TestValidateGemDimensionsRejectsUnsafeExport(t *testing.T) {
	rows := []row{{Name: "Flawed Ruby", Code: "gfr", Type: "gemr", Width: 0, Height: 0}}
	if err := validateGemDimensions(rows); err == nil {
		t.Fatal("validateGemDimensions() error = nil, want invalid gem dimensions")
	}
}
