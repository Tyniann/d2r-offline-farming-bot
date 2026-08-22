package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type localizedString struct {
	Key  string `json:"Key"`
	EnUS string `json:"enUS"`
	DeDE string `json:"deDE"`
}

type gameNameManifest struct {
	D2RVersion string   `json:"d2r_version"`
	Locale     string   `json:"locale"`
	Sources    []string `json:"sources"`
	KeyFields  []string `json:"stable_key_fields"`
}

type gameNames struct {
	Manifest       gameNameManifest  `json:"manifest"`
	Areas          map[string]string `json:"areas"`
	Skills         map[string]string `json:"skills"`
	Items          map[string]string `json:"items"`
	ItemIdentities map[string]string `json:"item_identities"`
	ItemSets       map[string]string `json:"item_sets"`
}

type localizedPair struct {
	EN string
	DE string
}

var nonLocalizedLegacyItemKeys = map[string]struct{}{
	"hpo": {}, "mpo": {}, "hpf": {}, "mpf": {}, "rps": {}, "rpl": {}, "bps": {}, "bpl": {},
}

var productSkillKeys = map[string]struct{}{
	"amplify_damage": {}, "battle_command": {}, "battle_orders": {}, "blessed_hammer": {},
	"bone_armor": {}, "bone_prison": {}, "bone_spear": {}, "concentration": {},
	"corpse_explosion": {}, "holy_shield": {}, "teleport": {}, "town_portal": {},
}

var gameNameSources = []string{
	"data/global/excel/levels.txt",
	"data/local/lng/strings/levels.json",
	"data/global/excel/skills.txt",
	"data/global/excel/skilldesc.txt",
	"data/local/lng/strings/skills.json",
	"data/global/excel/weapons.txt",
	"data/global/excel/armor.txt",
	"data/global/excel/misc.txt",
	"data/global/excel/setitems.txt",
	"data/global/excel/uniqueitems.txt",
	"data/local/lng/strings/item-names.json",
	"data/local/lng/strings-legacy/item-nameaffixes.json",
	"data/local/lng/strings-legacy/item-runes.json",
	"data/local/lng/strings/ui.json",
	"data/local/lng/strings/item-modifiers.json",
}

func generateGameNames(version, src string, baseRows []row, identities []identityRow, deOut, enOut string) error {
	areas, err := readAreaNamePairs(src)
	if err != nil {
		return err
	}
	skills, err := readSkillNamePairs(src)
	if err != nil {
		return err
	}
	stringsByKey, err := readItemNamePairs(src, baseRows, identities)
	if err != nil {
		return err
	}
	items, itemIdentities, itemSets, err := bindItemNamePairs(baseRows, identities, stringsByKey)
	if err != nil {
		return err
	}
	for locale, output := range map[string]string{"de": deOut, "en": enOut} {
		catalog := gameNames{
			Manifest: gameNameManifest{D2RVersion: version, Locale: locale, Sources: gameNameSources, KeyFields: []string{"area_id", "skill_key", "base_code", "identity_key", "set_key"}},
			Areas:    selectLocale(areas, locale), Skills: selectLocale(skills, locale), Items: selectLocale(items, locale),
			ItemIdentities: selectLocale(itemIdentities, locale), ItemSets: selectLocale(itemSets, locale),
		}
		data, marshalErr := json.MarshalIndent(catalog, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("marshal %s game names: %w", locale, marshalErr)
		}
		data = append(data, '\n')
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return fmt.Errorf("create output directory for %s: %w", output, err)
		}
		if err := os.WriteFile(output, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", output, err)
		}
	}
	return nil
}

func readAreaNamePairs(src string) (map[string]localizedPair, error) {
	records, header, err := readTSV(filepath.Join(src, "levels.txt"))
	if err != nil {
		return nil, err
	}
	if columnErr := requireColumns(filepath.Join(src, "levels.txt"), header, "id", "levelname"); columnErr != nil {
		return nil, columnErr
	}
	relevant := map[string]struct{}{}
	for _, record := range records {
		key := value(record, header, "levelname")
		if key != "" && !strings.EqualFold(key, "Null") {
			relevant[key] = struct{}{}
		}
	}
	translations, err := readLocalizedStrings(filepath.Join(src, "levels.json"), relevant)
	if err != nil {
		return nil, err
	}
	out := map[string]localizedPair{}
	for line, record := range records {
		id, key := value(record, header, "id"), value(record, header, "levelname")
		if id == "" || id == "0" || key == "" || strings.EqualFold(key, "Null") {
			continue
		}
		pair, ok := translations[key]
		if !ok {
			return nil, fmt.Errorf("levels.txt line %d references missing levels.json key %q", line+2, key)
		}
		if _, duplicate := out[id]; duplicate {
			return nil, fmt.Errorf("levels.txt line %d duplicates relevant area ID %s", line+2, id)
		}
		out[id] = pair
	}
	return out, nil
}

func readSkillNamePairs(src string) (map[string]localizedPair, error) {
	descRecords, descHeader, err := readTSV(filepath.Join(src, "skilldesc.txt"))
	if err != nil {
		return nil, err
	}
	if columnErr := requireColumns(filepath.Join(src, "skilldesc.txt"), descHeader, "skilldesc", "str name"); columnErr != nil {
		return nil, columnErr
	}
	descNames := map[string]string{}
	for line, record := range descRecords {
		desc, nameKey := value(record, descHeader, "skilldesc"), value(record, descHeader, "str name")
		if desc == "" || nameKey == "" {
			continue
		}
		if _, duplicate := descNames[desc]; duplicate {
			return nil, fmt.Errorf("skilldesc.txt line %d duplicates relevant skilldesc %q", line+2, desc)
		}
		descNames[desc] = nameKey
	}
	skillRecords, skillHeader, err := readTSV(filepath.Join(src, "skills.txt"))
	if err != nil {
		return nil, err
	}
	if columnErr := requireColumns(filepath.Join(src, "skills.txt"), skillHeader, "skill", "skilldesc"); columnErr != nil {
		return nil, columnErr
	}
	relevantNameKeys := map[string]struct{}{}
	productRows := map[string]string{}
	for _, record := range skillRecords {
		sourceName := value(record, skillHeader, "skill")
		key, keyErr := canonicalGameSkillKey(sourceName)
		if keyErr != nil {
			continue
		}
		if _, required := productSkillKeys[key]; !required {
			continue
		}
		nameKey := descNames[value(record, skillHeader, "skilldesc")]
		if nameKey == "" {
			return nil, fmt.Errorf("skill %q has no localized name through skilldesc.txt", key)
		}
		if _, duplicate := productRows[key]; duplicate {
			return nil, fmt.Errorf("duplicate relevant canonical skill key %q", key)
		}
		productRows[key] = nameKey
		relevantNameKeys[nameKey] = struct{}{}
	}
	translations, err := readLocalizedStrings(filepath.Join(src, "skills.json"), relevantNameKeys)
	if err != nil {
		return nil, err
	}
	out := map[string]localizedPair{}
	for key := range productSkillKeys {
		nameKey, ok := productRows[key]
		if !ok {
			return nil, fmt.Errorf("missing product skill key %q", key)
		}
		pair, ok := translations[nameKey]
		if !ok {
			return nil, fmt.Errorf("skill %q references missing skills.json key %q", key, nameKey)
		}
		out[key] = pair
	}
	return out, nil
}

func readItemNamePairs(src string, baseRows []row, identities []identityRow) (map[string]localizedPair, error) {
	relevant := make(map[string]struct{}, len(baseRows)+len(identities)*2)
	for _, item := range baseRows {
		if _, excluded := nonLocalizedLegacyItemKeys[item.SourceKey]; !excluded {
			relevant[item.SourceKey] = struct{}{}
		}
	}
	for _, identity := range identities {
		relevant[identity.SourceKey] = struct{}{}
		if identity.SetKey != "" {
			relevant[identity.SetKey] = struct{}{}
		}
	}
	paths := []string{
		filepath.Join(src, "item-names.json"), filepath.Join(src, "strings-legacy", "item-nameaffixes.json"),
		filepath.Join(src, "strings-legacy", "item-runes.json"), filepath.Join(src, "ui.json"), filepath.Join(src, "item-modifiers.json"),
	}
	out := map[string]localizedPair{}
	for _, path := range paths {
		catalog, err := readLocalizedStrings(path, relevant)
		if err != nil {
			return nil, err
		}
		for key, pair := range catalog {
			if _, duplicate := out[key]; duplicate {
				return nil, fmt.Errorf("duplicate item translation key %q across local CASC catalogs", key)
			}
			out[key] = pair
		}
	}
	return out, nil
}

func bindItemNamePairs(baseRows []row, identities []identityRow, translations map[string]localizedPair) (map[string]localizedPair, map[string]localizedPair, map[string]localizedPair, error) {
	items, identityNames, setNames := map[string]localizedPair{}, map[string]localizedPair{}, map[string]localizedPair{}
	for _, item := range baseRows {
		pair, ok := translations[item.SourceKey]
		if !ok {
			if _, excluded := nonLocalizedLegacyItemKeys[item.SourceKey]; excluded {
				continue
			}
			return nil, nil, nil, fmt.Errorf("item code %q references missing localized key %q", item.Code, item.SourceKey)
		}
		items[item.Code] = pair
	}
	for _, identity := range identities {
		pair, ok := translations[identity.SourceKey]
		if !ok {
			return nil, nil, nil, fmt.Errorf("%s identity %q references missing localized key %q", identity.Kind, identity.Key, identity.SourceKey)
		}
		identityNames[identity.Key] = pair
		if identity.SetKey != "" {
			setPair, setOK := translations[identity.SetKey]
			if !setOK {
				return nil, nil, nil, fmt.Errorf("set identity %q references missing set key %q", identity.Key, identity.SetKey)
			}
			setNames[identity.SetKey] = setPair
		}
	}
	return items, identityNames, setNames, nil
}

func readLocalizedStrings(path string, relevant map[string]struct{}) (map[string]localizedPair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []localizedString
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &rows); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	out := make(map[string]localizedPair, len(rows))
	for _, item := range rows {
		key := strings.TrimSpace(item.Key)
		if _, wanted := relevant[key]; key == "" || !wanted {
			continue
		}
		if _, duplicate := out[key]; duplicate {
			return nil, fmt.Errorf("%s duplicate relevant translation key %q", path, key)
		}
		en, de := cleanD2RName(item.EnUS), cleanD2RName(item.DeDE)
		if en == "" || de == "" {
			continue
		}
		out[key] = localizedPair{EN: en, DE: de}
	}
	return out, nil
}

func cleanD2RName(value string) string {
	value = strings.TrimSpace(value)
	for len(value) >= 4 && value[0] == '[' && value[3] == ']' {
		value = strings.TrimSpace(value[4:])
	}
	return value
}

func readTSV(path string) ([][]string, map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma, reader.FieldsPerRecord = '\t', -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("%s is empty", path)
	}
	return records[1:], headerIndex(records[0]), nil
}

func requireColumns(path string, header map[string]int, columns ...string) error {
	for _, column := range columns {
		if _, ok := header[column]; !ok {
			return fmt.Errorf("%s missing column %q", path, column)
		}
	}
	return nil
}

func canonicalGameSkillKey(sourceName string) (string, error) {
	trimmed := strings.TrimSpace(sourceName)
	if trimmed == "" {
		return "", fmt.Errorf("empty skill name")
	}
	runes := []rune(trimmed)
	var b strings.Builder
	previousUnderscore := true
	for index, current := range runes {
		switch {
		case unicode.IsLetter(current) || unicode.IsDigit(current):
			if unicode.IsUpper(current) && index > 0 && !previousUnderscore && (unicode.IsLower(runes[index-1]) || unicode.IsDigit(runes[index-1])) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(current))
			previousUnderscore = false
		default:
			if !previousUnderscore {
				b.WriteByte('_')
				previousUnderscore = true
			}
		}
	}
	key := strings.Trim(b.String(), "_")
	if key == "" {
		return "", fmt.Errorf("skill name %q normalizes to an empty key", sourceName)
	}
	return key, nil
}

func selectLocale(values map[string]localizedPair, locale string) map[string]string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(values))
	for _, key := range keys {
		if locale == "de" {
			out[key] = values[key].DE
		} else {
			out[key] = values[key].EN
		}
	}
	return out
}
