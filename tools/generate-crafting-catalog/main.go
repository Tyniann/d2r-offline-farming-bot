// Command generate-crafting-catalog selects the Phase-25 recipes from local CASC extracts.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type recipeSpec struct {
	key, inputName, outputName string
	version                    int
}

type recipeRow struct {
	key, input, output string
	version            int
}

func main() {
	src := flag.String("src", ".tmp/d2r-excel", "directory containing cubemain.txt and misc.txt")
	version := flag.String("version", "", "D2R version of the local extracts")
	out := flag.String("out", "internal/crafting/catalog_data.go", "generated Go catalog")
	flag.Parse()
	if *version != "3.2.92777" {
		fatal(fmt.Errorf("-version must match the validated source version 3.2.92777"))
	}
	rows, err := generate(*src)
	if err != nil {
		fatal(err)
	}
	data, err := render(*version, rows)
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "generate crafting catalog:", err)
	os.Exit(1)
}

func specs() []recipeSpec {
	var out []recipeSpec
	// These stable description keys select the agreed subset, not numeric IDs.
	for _, family := range []struct{ singular, plural string }{
		{"Amethyst", "Amethysts"}, {"Ruby", "Rubies"}, {"Sapphire", "Sapphires"},
		{"Topaz", "Topazes"}, {"Emerald", "Emeralds"}, {"Diamond", "Diamonds"}, {"Skull", "Skulls"},
	} {
		out = append(out,
			recipeSpec{"3 Standard " + family.plural + " -> Flawless " + family.singular, family.singular, "Flawless " + family.singular, 0},
			recipeSpec{"3 Flawless " + family.plural + " -> Perfect " + family.singular, "Flawless " + family.singular, "Perfect " + family.singular, 0})
	}
	runes := []string{"El", "Eld", "Tir", "Nef", "Eth", "Ith", "Tal", "Ral", "Ort", "Thul"}
	for i := 0; i < len(runes)-1; i++ {
		out = append(out, recipeSpec{"3 " + runes[i] + " Runes -> " + runes[i+1] + " Rune", runes[i] + " Rune", runes[i+1] + " Rune", 100})
	}
	return out
}

func readTable(path string, required []string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%s: no rows", path)
	}
	header := make(map[string]bool)
	for i, key := range records[0] {
		key = strings.TrimPrefix(key, "\ufeff")
		if header[key] {
			return nil, fmt.Errorf("%s: duplicate column %q", path, key)
		}
		header[key] = true
		records[0][i] = key
	}
	for _, key := range required {
		if !header[key] {
			return nil, fmt.Errorf("%s: missing column %q", path, key)
		}
	}
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(header))
		for i, key := range records[0] {
			row[key] = strings.TrimSpace(record[i])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func generate(src string) ([]recipeRow, error) {
	cube, err := readTable(filepath.Join(src, "cubemain.txt"), []string{"description", "enabled", "version", "numinputs", "input 1", "input 2", "input 3", "input 4", "input 5", "input 6", "input 7", "output", "output b", "output c", "class", "op", "min diff", "firstLadderSeason", "lastLadderSeason"})
	if err != nil {
		return nil, err
	}
	misc, err := readTable(filepath.Join(src, "misc.txt"), []string{"name", "code", "type", "invwidth", "invheight"})
	if err != nil {
		return nil, err
	}
	byName := make(map[string][]map[string]string)
	byCode := make(map[string]int)
	for _, row := range misc {
		byName[row["name"]] = append(byName[row["name"]], row)
		if row["code"] != "" {
			byCode[row["code"]]++
		}
	}
	var out []recipeRow
	for _, spec := range specs() {
		var selected []map[string]string
		for _, row := range cube {
			if row["description"] == spec.key {
				selected = append(selected, row)
			}
		}
		if len(selected) != 1 {
			return nil, fmt.Errorf("cubemain.txt description=%q: want one row, got %d", spec.key, len(selected))
		}
		row := selected[0]
		inputs, outputs := byName[spec.inputName], byName[spec.outputName]
		if len(inputs) != 1 || len(outputs) != 1 {
			return nil, fmt.Errorf("misc.txt names for %q are missing or ambiguous", spec.key)
		}
		input, output := inputs[0], outputs[0]
		for _, item := range []map[string]string{input, output} {
			if item["code"] == "" || byCode[item["code"]] != 1 || item["invwidth"] != "1" || item["invheight"] != "1" {
				return nil, fmt.Errorf("misc.txt name=%q: invalid code or dimensions", item["name"])
			}
		}
		if input["type"] != output["type"] || (spec.version == 100 && input["type"] != "rune") || (spec.version == 0 && !strings.HasPrefix(input["type"], "gem")) {
			return nil, fmt.Errorf("misc.txt types for %q disagree", spec.key)
		}
		expected := map[string]string{"description": spec.key, "enabled": "1", "version": strconv.Itoa(spec.version), "numinputs": "3", "input 1": input["code"] + ",qty=3", "output": output["code"]}
		// Every other field is forbidden, including modifiers and extra outputs.
		// Only the source row terminator is metadata rather than recipe behavior.
		for field, value := range row {
			if field == "*eol" {
				continue
			}
			if value != expected[field] {
				return nil, fmt.Errorf("cubemain.txt description=%q column=%q: got %q, want %q", spec.key, field, value, expected[field])
			}
		}
		out = append(out, recipeRow{spec.key, input["code"], output["code"], spec.version})
	}
	return out, nil
}

func render(version string, rows []recipeRow) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, "// Code generated by tools/generate-crafting-catalog; DO NOT EDIT.")
	fmt.Fprintf(&b, "// Source: local D2R %s cubemain.txt (description) and misc.txt (name/code).\npackage crafting\n\n", version)
	fmt.Fprintln(&b, "var recipes = [...]Recipe{")
	for _, row := range rows {
		fmt.Fprintf(&b, "{SourceFile: %q, SourceKey: %q, InputCode: %q, InputQuantity: 3, OutputCode: %q, Version: %d},\n", "cubemain.txt", row.key, row.input, row.output, row.version)
	}
	fmt.Fprintln(&b, "}")
	return format.Source(b.Bytes())
}
