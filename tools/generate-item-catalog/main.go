// Command generate-item-catalog regenerates world item catalog data from local D2R TXT files.
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

type row struct {
	Code       string
	Name       string
	Type       string
	NormalCode string
	UberCode   string
	UltraCode  string
	Width      int
	Height     int
}

func main() {
	src := flag.String("src", ".tmp/d2r-excel", "directory containing weapons.txt, armor.txt, misc.txt")
	out := flag.String("out", filepath.Join("internal", "world", "item_catalog_data.go"), "generated Go file path")
	flag.Parse()

	rows, err := readRows(*src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate item catalog: %v\n", err)
		os.Exit(1)
	}
	data, err := render(rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render item catalog: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
		os.Exit(1)
	}
}

func readRows(src string) ([]row, error) {
	files := []string{"weapons.txt", "armor.txt", "misc.txt"}
	var rows []row
	for _, name := range files {
		fileRows, err := readFile(filepath.Join(src, name))
		if err != nil {
			return nil, err
		}
		rows = append(rows, fileRows...)
	}
	if err := validateGemDimensions(rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func validateGemDimensions(rows []row) error {
	for _, item := range rows {
		if strings.HasPrefix(item.Type, "gem") && (item.Width != 1 || item.Height != 1) {
			return fmt.Errorf("gem %q (%s) has inventory dimensions %dx%d, want 1x1", item.Name, item.Code, item.Width, item.Height)
		}
	}
	return nil
}

func readFile(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	header := headerIndex(records[0])
	var rows []row
	for _, rec := range records[1:] {
		code := value(rec, header, "code")
		if code == "" {
			continue
		}
		rows = append(rows, row{
			Code:       code,
			Name:       firstNonEmpty(value(rec, header, "name"), value(rec, header, "namestr")),
			Type:       value(rec, header, "type"),
			NormalCode: value(rec, header, "normcode"),
			UberCode:   value(rec, header, "ubercode"),
			UltraCode:  value(rec, header, "ultracode"),
			Width:      intValue(rec, header, "invwidth"),
			Height:     intValue(rec, header, "invheight"),
		})
	}
	return rows, nil
}

func headerIndex(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, name := range header {
		out[strings.ToLower(strings.TrimSpace(name))] = i
	}
	return out
}

func value(rec []string, header map[string]int, name string) string {
	i, ok := header[strings.ToLower(name)]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func intValue(rec []string, header map[string]int, name string) int {
	v := value(rec, header, name)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func render(rows []row) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("// Code generated from D2R local data/global/excel weapons.txt, armor.txt, misc.txt; DO NOT EDIT.\n")
	b.WriteString("package world\n\n")
	b.WriteString("var itemCatalog = map[uint32]itemCatalogEntry{\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "\t%d: {Code: %q, Name: %q, Type: %q, NormalCode: %q, UberCode: %q, UltraCode: %q, Width: %d, Height: %d},\n",
			i, r.Code, r.Name, r.Type, r.NormalCode, r.UberCode, r.UltraCode, r.Width, r.Height)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
