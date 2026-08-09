// Command generate-skill-catalog regenerates the static D2R skill catalog from skills.txt.
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
	"unicode"
)

const supportedSourceVersion = "3.2.92777"

// skillRow is one readable skills.txt data row used by the catalog.
type skillRow struct {
	SourceName string
	Key        string
	ID         uint16
	CharClass  string
	LeftSkill  bool
	RightSkill bool
	InTown     bool
	Scroll     bool
	Passive    bool
}

// productAlias keeps the temporary named constants used by existing Go callers.
type productAlias struct {
	ConstName string
	Key       string
}

var productAliases = []productAlias{
	{ConstName: "SkillAttack", Key: "attack"},
	{ConstName: "SkillThrow", Key: "throw"},
	{ConstName: "SkillTeleport", Key: "teleport"},
	{ConstName: "SkillAmplifyDamage", Key: "amplify_damage"},
	{ConstName: "SkillBoneArmor", Key: "bone_armor"},
	{ConstName: "SkillCorpseExplosion", Key: "corpse_explosion"},
	{ConstName: "SkillBoneWall", Key: "bone_wall"},
	{ConstName: "SkillBoneSpear", Key: "bone_spear"},
	{ConstName: "SkillBonePrison", Key: "bone_prison"},
	{ConstName: "SkillTownPortal", Key: "town_portal"},
}

func main() {
	src := flag.String("src", ".tmp/d2r-excel", "directory containing skills.txt")
	version := flag.String("version", "", "D2R version that produced skills.txt")
	out := flag.String("out", filepath.Join("internal", "memory", "skill_catalog_data.go"), "generated Go file path")
	flag.Parse()
	if err := generateCatalog(*version, *src, *out); err != nil {
		fmt.Fprintf(os.Stderr, "generate skill catalog: %v\n", err)
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
	rows, err := readSkillRows(filepath.Join(src, "skills.txt"))
	if err != nil {
		return err
	}
	if aliasErr := validateProductAliases(rows); aliasErr != nil {
		return aliasErr
	}
	data, err := render(version, rows)
	if err != nil {
		return fmt.Errorf("render skill catalog: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

func validateSourceVersion(version string) error {
	if strings.TrimSpace(version) != supportedSourceVersion {
		return fmt.Errorf("source version %q is unsupported; want %s", version, supportedSourceVersion)
	}
	return nil
}

func readSkillRows(path string) ([]skillRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	header := headerIndex(records[0])
	for _, required := range []string{"skill", "*id", "charclass", "leftskill", "rightskill", "intown", "scroll", "passive"} {
		if _, ok := header[required]; !ok {
			return nil, fmt.Errorf("%s missing column %q", path, required)
		}
	}

	rows := make([]skillRow, 0, len(records)-1)
	seenIDs := map[uint16]string{}
	seenKeys := map[string]string{}
	for line, rec := range records[1:] {
		sourceName := value(rec, header, "skill")
		if sourceName == "" {
			return nil, fmt.Errorf("%s line %d: empty skill name", path, line+2)
		}
		idRaw := value(rec, header, "*id")
		if idRaw == "" {
			return nil, fmt.Errorf("%s line %d: empty *Id for skill %q", path, line+2, sourceName)
		}
		id64, err := strconv.ParseUint(idRaw, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: invalid *Id %q for skill %q: %w", path, line+2, idRaw, sourceName, err)
		}
		id := uint16(id64)
		key, err := canonicalizeSkillKey(sourceName)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line+2, err)
		}
		if previous, ok := seenIDs[id]; ok {
			return nil, fmt.Errorf("%s line %d: duplicate skill id %d (%q and %q)", path, line+2, id, previous, sourceName)
		}
		if previous, ok := seenKeys[key]; ok {
			return nil, fmt.Errorf("%s line %d: duplicate skill key %q (%q and %q)", path, line+2, key, previous, sourceName)
		}
		seenIDs[id] = sourceName
		seenKeys[key] = sourceName
		rows = append(rows, skillRow{
			SourceName: sourceName,
			Key:        key,
			ID:         id,
			CharClass:  value(rec, header, "charclass"),
			LeftSkill:  truthyFlag(value(rec, header, "leftskill")),
			RightSkill: truthyFlag(value(rec, header, "rightskill")),
			InTown:     truthyFlag(value(rec, header, "intown")),
			Scroll:     truthyFlag(value(rec, header, "scroll")),
			Passive:    truthyFlag(value(rec, header, "passive")),
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s contains no skill rows", path)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID == rows[j].ID {
			return rows[i].Key < rows[j].Key
		}
		return rows[i].ID < rows[j].ID
	})
	return rows, nil
}

func validateProductAliases(rows []skillRow) error {
	byKey := make(map[string]skillRow, len(rows))
	for _, row := range rows {
		byKey[row.Key] = row
	}
	for _, alias := range productAliases {
		row, ok := byKey[alias.Key]
		if !ok {
			return fmt.Errorf("product alias %s requires catalog key %q", alias.ConstName, alias.Key)
		}
		switch alias.Key {
		case "teleport":
			if row.ID != 54 {
				return fmt.Errorf("teleport id = %d, want 54", row.ID)
			}
		case "amplify_damage":
			if row.ID != 66 {
				return fmt.Errorf("amplify_damage id = %d, want 66", row.ID)
			}
		case "bone_armor":
			if row.ID != 68 {
				return fmt.Errorf("bone_armor id = %d, want 68", row.ID)
			}
		case "corpse_explosion":
			if row.ID != 74 {
				return fmt.Errorf("corpse_explosion id = %d, want 74", row.ID)
			}
		case "bone_wall":
			if row.ID != 78 {
				return fmt.Errorf("bone_wall id = %d, want 78", row.ID)
			}
		case "bone_spear":
			if row.ID != 84 {
				return fmt.Errorf("bone_spear id = %d, want 84", row.ID)
			}
		case "bone_prison":
			if row.ID != 88 {
				return fmt.Errorf("bone_prison id = %d, want 88", row.ID)
			}
		case "town_portal":
			if row.ID != 359 {
				return fmt.Errorf("town_portal id = %d, want 359", row.ID)
			}
		}
	}
	return nil
}

// canonicalizeSkillKey turns a skills.txt skill name into a stable snake_case key.
// CamelCase names such as TownPortal become town_portal; spaced names keep
// underscore separation. Empty or punctuation-only names are rejected.
func canonicalizeSkillKey(sourceName string) (string, error) {
	trimmed := strings.TrimSpace(sourceName)
	if trimmed == "" {
		return "", fmt.Errorf("empty skill name")
	}
	runes := []rune(trimmed)
	var b strings.Builder
	b.Grow(len(runes) + 8)
	prevUnderscore := true
	for i, r := range runes {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if unicode.IsUpper(r) && i > 0 && !prevUnderscore {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			prevUnderscore = false
		default:
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	key := strings.Trim(b.String(), "_")
	if key == "" {
		return "", fmt.Errorf("skill name %q normalizes to an empty key", sourceName)
	}
	return key, nil
}

func truthyFlag(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
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

func render(version string, rows []skillRow) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("// Code generated from D2R ")
	buf.WriteString(version)
	buf.WriteString(" local data/global/excel/skills.txt; DO NOT EDIT.\n")
	buf.WriteString("package memory\n\n")
	buf.WriteString("// generatedSkillCatalogVersion is the D2R build that produced the embedded skills.txt.\n")
	buf.WriteString("const generatedSkillCatalogVersion = \"")
	buf.WriteString(version)
	buf.WriteString("\"\n\n")
	buf.WriteString("// Skill IDs used by existing bot paths. Values are asserted against the generated catalog.\n")
	buf.WriteString("const (\n")
	byKey := make(map[string]skillRow, len(rows))
	for _, row := range rows {
		byKey[row.Key] = row
	}
	for _, alias := range productAliases {
		row := byKey[alias.Key]
		fmt.Fprintf(&buf, "\t%s uint16 = %d // %s\n", alias.ConstName, row.ID, alias.Key)
	}
	buf.WriteString(")\n\n")
	buf.WriteString("var skillCatalogByKey = map[string]SkillCatalogEntry{\n")
	for _, row := range rows {
		fmt.Fprintf(
			&buf,
			"\t%q: {Key: %q, ID: %d, CharClass: %q, SourceName: %q, LeftSkill: %t, RightSkill: %t, InTown: %t, Scroll: %t, Passive: %t},\n",
			row.Key, row.Key, row.ID, row.CharClass, row.SourceName, row.LeftSkill, row.RightSkill, row.InTown, row.Scroll, row.Passive,
		)
	}
	buf.WriteString("}\n\n")
	buf.WriteString("var skillCatalogByID = map[uint16]SkillCatalogEntry{\n")
	for _, row := range rows {
		fmt.Fprintf(
			&buf,
			"\t%d: {Key: %q, ID: %d, CharClass: %q, SourceName: %q, LeftSkill: %t, RightSkill: %t, InTown: %t, Scroll: %t, Passive: %t},\n",
			row.ID, row.Key, row.ID, row.CharClass, row.SourceName, row.LeftSkill, row.RightSkill, row.InTown, row.Scroll, row.Passive,
		)
	}
	buf.WriteString("}\n")
	return format.Source(buf.Bytes())
}
