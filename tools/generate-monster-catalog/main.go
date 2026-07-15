// Command generate-monster-catalog generates version-bound run boss IDs from monstats.txt.
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

type monsterRow struct {
	ID      string
	HCIdx   uint32
	Name    string
	Enabled bool
}

func main() {
	src := flag.String("src", ".tmp/d2r-excel", "directory containing monstats.txt")
	version := flag.String("version", "", "D2R version that produced monstats.txt")
	memoryOut := flag.String("memory-out", filepath.Join("internal", "memory", "monster_ids_data.go"), "generated memory boss allowlist path")
	worldOut := flag.String("world-out", filepath.Join("internal", "world", "monster_ids_data.go"), "generated world boss catalog path")
	flag.Parse()
	if strings.TrimSpace(*version) == "" {
		fatalf("-version is required")
	}
	rows, err := readMonsterRows(filepath.Join(*src, "monstats.txt"))
	if err != nil {
		fatalf("read monstats: %v", err)
	}
	mephisto, err := selectMephisto(rows)
	if err != nil {
		fatalf("select Mephisto: %v", err)
	}
	for _, output := range []struct {
		path string
		data []byte
	}{{*memoryOut, renderMemory(*version, mephisto)}, {*worldOut, renderWorld(*version, mephisto)}} {
		formatted, err := format.Source(output.data)
		if err != nil {
			fatalf("format %s: %v", output.path, err)
		}
		if err := os.WriteFile(output.path, formatted, 0o644); err != nil {
			fatalf("write %s: %v", output.path, err)
		}
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "generate monster catalog: "+format+"\n", args...)
	os.Exit(1)
}

func readMonsterRows(path string) ([]monsterRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.FieldsPerRecord = '\t', -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%q has no data rows", path)
	}
	header := headerIndex(records[0])
	for _, required := range []string{"id", "*hcidx", "namestr", "enabled"} {
		if _, ok := header[required]; !ok {
			return nil, fmt.Errorf("%q missing column %q", path, required)
		}
	}
	rows := make([]monsterRow, 0, len(records)-1)
	seen := map[uint32]bool{}
	for line, record := range records[1:] {
		raw := value(record, header, "*hcidx")
		if raw == "" {
			continue
		}
		idx, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%q line %d invalid *hcIdx %q", path, line+2, raw)
		}
		hcIdx := uint32(idx)
		if seen[hcIdx] {
			return nil, fmt.Errorf("%q line %d duplicate *hcIdx %d", path, line+2, hcIdx)
		}
		seen[hcIdx] = true
		rows = append(rows, monsterRow{ID: value(record, header, "id"), HCIdx: hcIdx, Name: value(record, header, "namestr"), Enabled: value(record, header, "enabled") == "1"})
	}
	return rows, nil
}

func selectMephisto(rows []monsterRow) (monsterRow, error) {
	for _, row := range rows {
		if row.HCIdx != 242 {
			continue
		}
		if !strings.EqualFold(row.ID, "mephisto") || !strings.EqualFold(row.Name, "Mephisto") || !row.Enabled {
			return monsterRow{}, fmt.Errorf("*hcIdx 242 has unexpected row %+v", row)
		}
		return row, nil
	}
	return monsterRow{}, fmt.Errorf("*hcIdx 242 missing")
}

func renderMemory(version string, row monsterRow) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local data/global/excel/monstats.txt; DO NOT EDIT.\n", version)
	b.WriteString("package memory\n\n")
	fmt.Fprintf(&b, "var runtimeBossNPCIDs = map[uint32]struct{}{\n\t%d: {}, // %s\n}\n", row.HCIdx, row.Name)
	return b.Bytes()
}

func renderWorld(version string, row monsterRow) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local data/global/excel/monstats.txt; DO NOT EDIT.\n", version)
	b.WriteString("package world\n\n")
	b.WriteString("const (\n")
	fmt.Fprintf(&b, "\t// Mephisto is the generated monstats *hcIdx for this D2R version.\n\tMephisto uint32 = %d\n", row.HCIdx)
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "var generatedNPCNames = map[uint32]string{\n\tMephisto: %q,\n}\n", row.Name)
	return b.Bytes()
}

func headerIndex(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, name := range header {
		out[strings.ToLower(strings.TrimSpace(name))] = i
	}
	return out
}

func value(record []string, header map[string]int, name string) string {
	i, ok := header[strings.ToLower(name)]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}
