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
	BaseTier   string
}

const supportedSourceVersion = "3.2.92777"

func main() {
	src := flag.String("src", ".tmp/d2r-excel", "directory containing weapons.txt, armor.txt, misc.txt")
	version := flag.String("version", "", "D2R version that produced the TXT files")
	out := flag.String("out", filepath.Join("internal", "world", "item_catalog_data.go"), "generated Go file path")
	flag.Parse()
	if err := generateCatalog(*version, *src, *out); err != nil {
		fmt.Fprintf(os.Stderr, "generate item catalog: %v\n", err)
		os.Exit(1)
	}
}

func generateCatalog(version, src, out string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("-version is required")
	}
	if err := validateSourceVersion(version); err != nil {
		return err
	}
	rows, err := readRows(src)
	if err != nil {
		return err
	}
	identities, _, err := readIdentityRows(src, rows)
	if err != nil {
		return err
	}
	data, err := render(version, rows, identities)
	if err != nil {
		return fmt.Errorf("render item catalog: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

func readRows(src string) ([]row, error) {
	files := []string{"weapons.txt", "armor.txt", "misc.txt"}
	var rows []row
	for _, name := range files {
		fileRows, err := readFile(filepath.Join(src, name))
		if err != nil {
			return nil, err
		}
		if name != "misc.txt" {
			if err := classifyBaseTiers(fileRows); err != nil {
				return nil, fmt.Errorf("%s base tiers: %w", name, err)
			}
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
	for _, required := range []string{"code", "type", "invwidth", "invheight"} {
		if _, ok := header[required]; !ok {
			return nil, fmt.Errorf("%s missing column %q", path, required)
		}
	}
	if _, nameOK := header["name"]; !nameOK {
		if _, nameStrOK := header["namestr"]; !nameStrOK {
			return nil, fmt.Errorf("%s missing column name or namestr", path)
		}
	}
	for _, column := range []string{"normcode", "ubercode", "ultracode"} {
		if strings.HasSuffix(strings.ToLower(path), "weapons.txt") || strings.HasSuffix(strings.ToLower(path), "armor.txt") {
			if _, ok := header[column]; !ok {
				return nil, fmt.Errorf("%s missing column %q", path, column)
			}
		}
	}
	var rows []row
	seen := map[string]bool{}
	for line, rec := range records[1:] {
		code := value(rec, header, "code")
		if code == "" {
			continue
		}
		if seen[code] {
			return nil, fmt.Errorf("%s line %d duplicate code %q", path, line+2, code)
		}
		seen[code] = true
		width, err := requiredIntValue(rec, header, "invwidth")
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line+2, err)
		}
		height, err := requiredIntValue(rec, header, "invheight")
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line+2, err)
		}
		rows = append(rows, row{
			Code:       code,
			Name:       firstNonEmpty(value(rec, header, "name"), value(rec, header, "namestr")),
			Type:       value(rec, header, "type"),
			NormalCode: value(rec, header, "normcode"),
			UberCode:   value(rec, header, "ubercode"),
			UltraCode:  value(rec, header, "ultracode"),
			Width:      width,
			Height:     height,
			BaseTier:   "unknown",
		})
	}
	return rows, nil
}

func classifyBaseTiers(rows []row) error {
	byCode := make(map[string]int, len(rows))
	for i, item := range rows {
		byCode[item.Code] = i
	}
	edges := map[string]string{}
	for _, item := range rows {
		for _, ref := range []string{item.NormalCode, item.UberCode, item.UltraCode} {
			if ref != "" {
				if _, ok := byCode[ref]; !ok {
					return fmt.Errorf("code %q references unknown tier code %q", item.Code, ref)
				}
			}
		}
		if item.NormalCode != "" && item.UberCode != "" {
			edges[item.NormalCode] = item.UberCode
		}
		if item.UberCode != "" && item.UltraCode != "" {
			edges[item.UberCode] = item.UltraCode
		}
	}
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(code string) error {
		if visiting[code] {
			return fmt.Errorf("tier cycle at code %q", code)
		}
		if visited[code] {
			return nil
		}
		visiting[code] = true
		if next := edges[code]; next != "" {
			if err := visit(next); err != nil {
				return err
			}
		}
		delete(visiting, code)
		visited[code] = true
		return nil
	}
	for code := range edges {
		if err := visit(code); err != nil {
			return err
		}
	}
	for i := range rows {
		switch rows[i].Code {
		case rows[i].NormalCode:
			rows[i].BaseTier = "normal"
		case rows[i].UberCode:
			rows[i].BaseTier = "exceptional"
		case rows[i].UltraCode:
			rows[i].BaseTier = "elite"
		}
	}
	return nil
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

func requiredIntValue(rec []string, header map[string]int, name string) (int, error) {
	v := value(rec, header, name)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", name, v)
	}
	return n, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func render(version string, rows []row, identities []identityRow) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local item Excel and item-names.json data; DO NOT EDIT.\n", version)
	b.WriteString("package world\n\n")
	fmt.Fprintf(&b, "const generatedItemIdentityCatalogVersion = %q\n\n", version)
	b.WriteString("var itemCatalog = map[uint32]itemCatalogEntry{\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "\t%d: {Code: %q, Name: %q, Type: %q, NormalCode: %q, UberCode: %q, UltraCode: %q, Width: %d, Height: %d, BaseTier: %s},\n",
			i, r.Code, r.Name, r.Type, r.NormalCode, r.UberCode, r.UltraCode, r.Width, r.Height, baseTierIdentifier(r.BaseTier))
	}
	b.WriteString("}\n\n")
	b.WriteString("var generatedItemIdentityCatalog = []ItemIdentityCatalogEntry{\n")
	for _, identity := range identities {
		fmt.Fprintf(&b, "\t{Kind: %s, RawID: %d, Key: %q, DisplayName: %q, BaseCode: %q, SetKey: %q, SetName: %q, Spawnable: %t},\n",
			identityKindIdentifier(identity.Kind), identity.RawID, identity.Key, identity.DisplayName, identity.BaseCode, identity.SetKey, identity.SetName, identity.Spawnable)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

func validateSourceVersion(version string) error {
	if strings.TrimSpace(version) != supportedSourceVersion {
		return fmt.Errorf("source version %q is unsupported; want %s", version, supportedSourceVersion)
	}
	return nil
}

func identityKindIdentifier(kind string) string {
	if kind == "set" {
		return "ItemIdentitySet"
	}
	return "ItemIdentityUnique"
}

func baseTierIdentifier(tier string) string {
	switch tier {
	case "normal":
		return "BaseTierNormal"
	case "exceptional":
		return "BaseTierExceptional"
	case "elite":
		return "BaseTierElite"
	default:
		return "BaseTierUnknown"
	}
}
