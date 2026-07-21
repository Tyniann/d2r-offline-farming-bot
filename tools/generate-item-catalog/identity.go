package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type identityRow struct {
	Kind          string
	RawID         uint32
	SourceKey     string
	Key           string
	DisplayName   string
	BaseCode      string
	SetKey        string
	SetName       string
	Spawnable     bool
	disambiguator string
}

type identityReadStats struct {
	SetRows       int
	SetMarkers    int
	UniqueRows    int
	UniqueMarkers int
}

type itemNameTranslation struct {
	Key  string `json:"Key"`
	EnUS string `json:"enUS"`
}

func readIdentityRows(src string, baseRows []row) ([]identityRow, identityReadStats, error) {
	baseCodes := make(map[string]struct{}, len(baseRows))
	for _, item := range baseRows {
		baseCodes[item.Code] = struct{}{}
	}
	sets, setMarkers, err := readIdentityFile(filepath.Join(src, "setitems.txt"), "set", baseCodes)
	if err != nil {
		return nil, identityReadStats{}, err
	}
	uniques, uniqueMarkers, err := readIdentityFile(filepath.Join(src, "uniqueitems.txt"), "unique", baseCodes)
	if err != nil {
		return nil, identityReadStats{}, err
	}
	identities := append(sets, uniques...)
	if err := assignStableIdentityKeys(identities); err != nil {
		return nil, identityReadStats{}, err
	}
	if err := resolveIdentityNames(filepath.Join(src, "item-names.json"), identities); err != nil {
		return nil, identityReadStats{}, err
	}
	sort.Slice(identities, func(i, j int) bool {
		if identities[i].Kind != identities[j].Kind {
			return identities[i].Kind < identities[j].Kind
		}
		return identities[i].RawID < identities[j].RawID
	})
	return identities, identityReadStats{SetRows: len(sets), SetMarkers: setMarkers, UniqueRows: len(uniques), UniqueMarkers: uniqueMarkers}, nil
}

func readIdentityFile(path, kind string, baseCodes map[string]struct{}) ([]identityRow, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, 0, fmt.Errorf("%s is empty", path)
	}
	header := headerIndex(records[0])
	required := []string{"index", "*id", "spawnable"}
	baseColumn := "code"
	if kind == "set" {
		required = append(required, "set", "item")
		baseColumn = "item"
	} else {
		required = append(required, "version", "code")
	}
	for _, column := range required {
		if _, ok := header[column]; !ok {
			return nil, 0, fmt.Errorf("%s missing column %q", path, column)
		}
	}
	var rows []identityRow
	markers := 0
	seenIDs := map[uint32]struct{}{}
	for line, record := range records[1:] {
		rawIDText := value(record, header, "*id")
		rawID64, parseErr := strconv.ParseUint(rawIDText, 10, 32)
		if parseErr != nil {
			markers++
			continue
		}
		rawID := uint32(rawID64)
		if _, duplicate := seenIDs[rawID]; duplicate {
			return nil, 0, fmt.Errorf("%s line %d duplicate relevant ID %d", path, line+2, rawID)
		}
		seenIDs[rawID] = struct{}{}
		sourceKey := value(record, header, "index")
		if sourceKey == "" {
			return nil, 0, fmt.Errorf("%s line %d missing identity key", path, line+2)
		}
		spawnableText := value(record, header, "spawnable")
		if spawnableText != "" && spawnableText != "0" && spawnableText != "1" {
			return nil, 0, fmt.Errorf("%s line %d invalid spawnable %q", path, line+2, spawnableText)
		}
		baseCode := value(record, header, baseColumn)
		spawnable := spawnableText == "1"
		if baseCode == "" {
			if spawnable {
				return nil, 0, fmt.Errorf("%s line %d spawnable identity %q has no base code", path, line+2, sourceKey)
			}
		} else if _, ok := baseCodes[baseCode]; !ok {
			return nil, 0, fmt.Errorf("%s line %d identity %q references unknown base code %q", path, line+2, sourceKey, baseCode)
		}
		setKey := ""
		if kind == "set" {
			setKey = value(record, header, "set")
			if setKey == "" {
				return nil, 0, fmt.Errorf("%s line %d identity %q has no set key", path, line+2, sourceKey)
			}
		}
		disambiguator := strings.Join([]string{baseCode, value(record, header, "prop1"), value(record, header, "prop4"), value(record, header, "par4")}, ";")
		rows = append(rows, identityRow{Kind: kind, RawID: rawID, SourceKey: sourceKey, BaseCode: baseCode, SetKey: setKey, Spawnable: spawnable, disambiguator: disambiguator})
	}
	return rows, markers, nil
}

func assignStableIdentityKeys(rows []identityRow) error {
	groups := make(map[string][]int, len(rows))
	for index, row := range rows {
		group := row.Kind + "\x00" + strings.ToLower(row.SourceKey)
		groups[group] = append(groups[group], index)
	}
	seen := map[string]struct{}{}
	for _, indexes := range groups {
		for _, index := range indexes {
			key := rows[index].SourceKey
			if len(indexes) > 1 {
				key = fmt.Sprintf("%s [%s]", key, rows[index].disambiguator)
			}
			folded := rows[index].Kind + "\x00" + strings.ToLower(key)
			if _, duplicate := seen[folded]; duplicate {
				return fmt.Errorf("duplicate relevant %s identity key %q", rows[index].Kind, key)
			}
			seen[folded] = struct{}{}
			rows[index].Key = key
		}
	}
	return nil
}

func resolveIdentityNames(path string, rows []identityRow) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var translations []itemNameTranslation
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if err := json.Unmarshal(data, &translations); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	relevant := make(map[string]struct{}, len(rows)*2)
	for _, row := range rows {
		relevant[row.SourceKey] = struct{}{}
		if row.SetKey != "" {
			relevant[row.SetKey] = struct{}{}
		}
	}
	resolved := make(map[string]string, len(relevant))
	for _, translation := range translations {
		if _, ok := relevant[translation.Key]; !ok {
			continue
		}
		if _, duplicate := resolved[translation.Key]; duplicate {
			return fmt.Errorf("%s duplicate relevant translation key %q", path, translation.Key)
		}
		if strings.TrimSpace(translation.EnUS) == "" {
			return fmt.Errorf("%s relevant key %q has no enUS display name", path, translation.Key)
		}
		resolved[translation.Key] = translation.EnUS
	}
	for index := range rows {
		displayName, ok := resolved[rows[index].SourceKey]
		if !ok {
			return fmt.Errorf("%s missing relevant key %q", path, rows[index].SourceKey)
		}
		rows[index].DisplayName = displayName
		if rows[index].SetKey != "" {
			setName, setOK := resolved[rows[index].SetKey]
			if !setOK {
				return fmt.Errorf("%s missing relevant set key %q", path, rows[index].SetKey)
			}
			rows[index].SetName = setName
		}
	}
	return nil
}
