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
	"sort"
	"strconv"
	"strings"
)

type monsterRow struct {
	ID      string
	HCIdx   uint32
	Name    string
	Enabled bool
}

// bossTarget is one run boss selected by monstats *hcIdx from .tmp/d2r-excel.
type bossTarget struct {
	HCIdx     uint32
	ID        string
	ConstName string
}

var defaultBossTargets = []bossTarget{
	{HCIdx: 242, ID: "mephisto", ConstName: "Mephisto"},
	{HCIdx: 250, ID: "summoner", ConstName: "Summoner"},
	{HCIdx: 526, ID: "nihlathakboss", ConstName: "Nihlathak"},
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
	selected, err := selectBosses(rows, defaultBossTargets)
	if err != nil {
		fatalf("select bosses: %v", err)
	}
	for _, output := range []struct {
		path string
		data []byte
	}{{*memoryOut, renderMemory(*version, selected)}, {*worldOut, renderWorld(*version, selected)}} {
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

type selectedBoss struct {
	Target bossTarget
	Row    monsterRow
}

func selectBosses(rows []monsterRow, targets []bossTarget) ([]selectedBoss, error) {
	byHCIdx := make(map[uint32]monsterRow, len(rows))
	for _, row := range rows {
		byHCIdx[row.HCIdx] = row
	}
	out := make([]selectedBoss, 0, len(targets))
	for _, target := range targets {
		row, ok := byHCIdx[target.HCIdx]
		if !ok {
			return nil, fmt.Errorf("*hcIdx %d missing", target.HCIdx)
		}
		if !strings.EqualFold(row.ID, target.ID) || !row.Enabled {
			return nil, fmt.Errorf("*hcIdx %d has unexpected row %+v (want id %q enabled)", target.HCIdx, row, target.ID)
		}
		out = append(out, selectedBoss{Target: target, Row: row})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Row.HCIdx < out[j].Row.HCIdx })
	return out, nil
}

func renderMemory(version string, bosses []selectedBoss) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local data/global/excel/monstats.txt; DO NOT EDIT.\n", version)
	b.WriteString("package memory\n\n")
	b.WriteString("var runtimeBossNPCIDs = map[uint32]struct{}{\n")
	for _, boss := range bosses {
		fmt.Fprintf(&b, "\t%d: {}, // %s\n", boss.Row.HCIdx, boss.Row.Name)
	}
	b.WriteString("}\n")
	return b.Bytes()
}

func renderWorld(version string, bosses []selectedBoss) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local data/global/excel/monstats.txt; DO NOT EDIT.\n", version)
	b.WriteString("package world\n\n")
	b.WriteString("const (\n")
	for _, boss := range bosses {
		fmt.Fprintf(&b, "\t// %s is the generated monstats *hcIdx for this D2R version.\n\t%s uint32 = %d\n", boss.Target.ConstName, boss.Target.ConstName, boss.Row.HCIdx)
	}
	b.WriteString(")\n\n")
	b.WriteString("var generatedNPCNames = map[uint32]string{\n")
	for _, boss := range bosses {
		fmt.Fprintf(&b, "\t%s: %q,\n", boss.Target.ConstName, boss.Row.Name)
	}
	b.WriteString("}\n")
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
