package memory

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type offsetSetFile struct {
	Name         string            `yaml:"name"`
	D2RVersion   string            `yaml:"d2r_version"`
	Source       string            `yaml:"source"`
	SourceCommit string            `yaml:"source_commit"`
	VerifiedAt   string            `yaml:"verified_at"`
	ModuleName   string            `yaml:"module_name"`
	GameData     hexUintptr        `yaml:"game_data"`
	UnitTable    hexUintptr        `yaml:"unit_table"`
	UI           hexUintptr        `yaml:"ui"`
	Expansion    hexUintptr        `yaml:"expansion"`
	Unit         unitOffsetsFile `yaml:"unit"`
	Stats        statOffsetsFile `yaml:"stats"`
}

type unitOffsetsFile struct {
	UnitType            hexUintptr `yaml:"unit_type"`
	UnitID              hexUintptr `yaml:"unit_id"`
	UnitData            hexUintptr `yaml:"unit_data"`
	Path                hexUintptr `yaml:"path"`
	StatsListEx         hexUintptr `yaml:"stats_list_ex"`
	Inventory           hexUintptr `yaml:"inventory"`
	NextUnit            hexUintptr `yaml:"next_unit"`
	SkillsList          hexUintptr `yaml:"skills_list"`
	MainPlayerNormal    hexUintptr `yaml:"main_player_normal"`
	MainPlayerExpansion hexUintptr `yaml:"main_player_expansion"`
	ExpansionCharFlag   hexUintptr `yaml:"expansion_char_flag"`
	PositionX           hexUintptr `yaml:"position_x"`
	PositionY           hexUintptr `yaml:"position_y"`
	PathRoom1           hexUintptr `yaml:"path_room1"`
	Room2               hexUintptr `yaml:"room2"`
	Level               hexUintptr `yaml:"level"`
	Area                hexUintptr `yaml:"area"`
	StatsListBase       hexUintptr `yaml:"stats_list_base"`
	StatsListActive     hexUintptr `yaml:"stats_list_active"`
}

type statOffsetsFile struct {
	ListPtr     hexUintptr `yaml:"list_ptr"`
	Count       hexUintptr `yaml:"count"`
	EntryStride hexUintptr `yaml:"entry_stride"`
	Layer       hexUintptr `yaml:"layer"`
	ID          hexUintptr `yaml:"id"`
	Value       hexUintptr `yaml:"value"`
}

// hexUintptr unmarshals YAML integers or hex strings into uintptr.
type hexUintptr struct {
	Value uintptr
	Set   bool
}

func (h *hexUintptr) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		raw := strings.TrimSpace(value.Value)
		if value.Tag == "!!null" || raw == "" || raw == "0" || strings.EqualFold(raw, "null") {
			h.Value = 0
			h.Set = true
			return nil
		}
		if strings.HasPrefix(strings.ToLower(raw), "0x") {
			v, err := strconv.ParseUint(raw[2:], 16, 64)
			if err != nil {
				return fmt.Errorf("parse hex %q: %w", raw, err)
			}
			h.Value = uintptr(v)
			h.Set = true
			return nil
		}
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("parse uintptr %q: %w", raw, err)
		}
		h.Value = uintptr(v)
		h.Set = true
		return nil
	default:
		return fmt.Errorf("expected scalar for uintptr, got %v", value.Kind)
	}
}

func (h hexUintptr) MarshalYAML() (interface{}, error) {
	if !h.Set {
		return nil, nil
	}
	return fmt.Sprintf("0x%X", h.Value), nil
}

// LoadOffsetSetFile loads an optional YAML override and overlays it on [DefaultOffsetSet].
func LoadOffsetSetFile(path string) (OffsetSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OffsetSet{}, fmt.Errorf("read offsets %q: %w", path, err)
	}

	var file offsetSetFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return OffsetSet{}, fmt.Errorf("parse offsets %q: %w", path, err)
	}

	out := DefaultOffsetSet()
	overlayOffsetSet(&out, file)
	if err := validateOffsetSet(out); err != nil {
		return OffsetSet{}, fmt.Errorf("validate offsets %q: %w", path, err)
	}
	return out, nil
}

func overlayOffsetSet(out *OffsetSet, file offsetSetFile) {
	if file.Name != "" {
		out.Name = file.Name
	}
	if file.D2RVersion != "" {
		out.D2RVersion = file.D2RVersion
	}
	if file.Source != "" {
		out.Source = file.Source
	}
	if file.SourceCommit != "" {
		out.SourceCommit = file.SourceCommit
	}
	if file.VerifiedAt != "" {
		out.VerifiedAt = file.VerifiedAt
	}
	if file.ModuleName != "" {
		out.ModuleName = file.ModuleName
	}
	if file.GameData.Set {
		out.GameData = file.GameData.Value
	}
	if file.UnitTable.Set {
		out.UnitTable = file.UnitTable.Value
	}
	if file.UI.Set {
		out.UI = file.UI.Value
	}
	if file.Expansion.Set {
		out.Expansion = file.Expansion.Value
	}
	overlayUnitOffsets(&out.Unit, file.Unit)
	overlayStatOffsets(&out.Stats, file.Stats)
}

func overlayUnitOffsets(out *UnitOffsets, file unitOffsetsFile) {
	if file.UnitType.Set {
		out.UnitType = file.UnitType.Value
	}
	if file.UnitID.Set {
		out.UnitID = file.UnitID.Value
	}
	if file.UnitData.Set {
		out.UnitData = file.UnitData.Value
	}
	if file.Path.Set {
		out.Path = file.Path.Value
	}
	if file.StatsListEx.Set {
		out.StatsListEx = file.StatsListEx.Value
	}
	if file.Inventory.Set {
		out.Inventory = file.Inventory.Value
	}
	if file.NextUnit.Set {
		out.NextUnit = file.NextUnit.Value
	}
	if file.SkillsList.Set {
		out.SkillsList = file.SkillsList.Value
	}
	if file.MainPlayerNormal.Set {
		out.MainPlayerNormal = file.MainPlayerNormal.Value
	}
	if file.MainPlayerExpansion.Set {
		out.MainPlayerExpansion = file.MainPlayerExpansion.Value
	}
	if file.ExpansionCharFlag.Set {
		out.ExpansionCharFlag = file.ExpansionCharFlag.Value
	}
	if file.PositionX.Set {
		out.PositionX = file.PositionX.Value
	}
	if file.PositionY.Set {
		out.PositionY = file.PositionY.Value
	}
	if file.PathRoom1.Set {
		out.PathRoom1 = file.PathRoom1.Value
	}
	if file.Room2.Set {
		out.Room2 = file.Room2.Value
	}
	if file.Level.Set {
		out.Level = file.Level.Value
	}
	if file.Area.Set {
		out.Area = file.Area.Value
	}
	if file.StatsListBase.Set {
		out.StatsListBase = file.StatsListBase.Value
	}
	if file.StatsListActive.Set {
		out.StatsListActive = file.StatsListActive.Value
	}
}

func overlayStatOffsets(out *StatOffsets, file statOffsetsFile) {
	if file.ListPtr.Set {
		out.ListPtr = file.ListPtr.Value
	}
	if file.Count.Set {
		out.Count = file.Count.Value
	}
	if file.EntryStride.Set {
		out.EntryStride = file.EntryStride.Value
	}
	if file.Layer.Set {
		out.Layer = file.Layer.Value
	}
	if file.ID.Set {
		out.ID = file.ID.Value
	}
	if file.Value.Set {
		out.Value = file.Value.Value
	}
}

func validateOffsetSet(o OffsetSet) error {
	if o.Name == "" {
		return fmt.Errorf("name is required")
	}
	if o.Source == "" {
		return fmt.Errorf("source is required")
	}
	if o.SourceCommit == "" {
		return fmt.Errorf("source_commit is required")
	}
	if o.UnitTable == 0 {
		return fmt.Errorf("unit_table is required")
	}
	if o.UI == 0 {
		return fmt.Errorf("ui is required")
	}
	if o.UI < 0xA {
		return fmt.Errorf("ui must be >= 0xA for in-game gate")
	}
	if o.Unit.StatsListEx == 0 {
		return fmt.Errorf("unit.stats_list_ex is required")
	}
	if o.Unit.NextUnit == 0 {
		return fmt.Errorf("unit.next_unit is required")
	}
	if o.Unit.Path == 0 {
		return fmt.Errorf("unit.path is required")
	}
	if o.Unit.Inventory == 0 {
		return fmt.Errorf("unit.inventory is required")
	}
	if o.Stats.EntryStride == 0 {
		return fmt.Errorf("stats.entry_stride must be > 0")
	}
	return nil
}

// SaveOffsetSetFile writes an [OffsetSet] as YAML for local overrides or scan caching.
func SaveOffsetSetFile(path string, off OffsetSet) error {
	data, err := marshalOffsetSetYAML(off)
	if err != nil {
		return fmt.Errorf("marshal offsets: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write offsets %q: %w", path, err)
	}
	return nil
}

func marshalOffsetSetYAML(off OffsetSet) ([]byte, error) {
	file := offsetSetFile{
		Name:         off.Name,
		D2RVersion:   off.D2RVersion,
		Source:       off.Source,
		SourceCommit: off.SourceCommit,
		VerifiedAt:   off.VerifiedAt,
		ModuleName:   off.ModuleName,
		GameData:     hexUintptr{Value: off.GameData, Set: off.GameData != 0},
		UnitTable:    hexUintptr{Value: off.UnitTable, Set: off.UnitTable != 0},
		UI:           hexUintptr{Value: off.UI, Set: off.UI != 0},
		Expansion:    hexUintptr{Value: off.Expansion, Set: off.Expansion != 0},
		Unit: unitOffsetsFile{
			UnitType:            hexUintptr{Value: off.Unit.UnitType, Set: true},
			UnitID:              hexUintptr{Value: off.Unit.UnitID, Set: true},
			UnitData:            hexUintptr{Value: off.Unit.UnitData, Set: true},
			Path:                hexUintptr{Value: off.Unit.Path, Set: true},
			StatsListEx:         hexUintptr{Value: off.Unit.StatsListEx, Set: true},
			Inventory:           hexUintptr{Value: off.Unit.Inventory, Set: true},
			NextUnit:            hexUintptr{Value: off.Unit.NextUnit, Set: true},
			SkillsList:          hexUintptr{Value: off.Unit.SkillsList, Set: true},
			MainPlayerNormal:    hexUintptr{Value: off.Unit.MainPlayerNormal, Set: true},
			MainPlayerExpansion: hexUintptr{Value: off.Unit.MainPlayerExpansion, Set: true},
			ExpansionCharFlag:   hexUintptr{Value: off.Unit.ExpansionCharFlag, Set: true},
			PositionX:           hexUintptr{Value: off.Unit.PositionX, Set: true},
			PositionY:           hexUintptr{Value: off.Unit.PositionY, Set: true},
			PathRoom1:           hexUintptr{Value: off.Unit.PathRoom1, Set: true},
			Room2:               hexUintptr{Value: off.Unit.Room2, Set: true},
			Level:               hexUintptr{Value: off.Unit.Level, Set: true},
			Area:                hexUintptr{Value: off.Unit.Area, Set: true},
			StatsListBase:       hexUintptr{Value: off.Unit.StatsListBase, Set: true},
			StatsListActive:     hexUintptr{Value: off.Unit.StatsListActive, Set: true},
		},
		Stats: statOffsetsFile{
			ListPtr:     hexUintptr{Value: off.Stats.ListPtr, Set: true},
			Count:       hexUintptr{Value: off.Stats.Count, Set: true},
			EntryStride: hexUintptr{Value: off.Stats.EntryStride, Set: true},
			Layer:       hexUintptr{Value: off.Stats.Layer, Set: true},
			ID:          hexUintptr{Value: off.Stats.ID, Set: true},
			Value:       hexUintptr{Value: off.Stats.Value, Set: true},
		},
	}
	return yaml.Marshal(file)
}

// ResolveOffsetSet returns the active offset set from defaults and an optional YAML override file.
func ResolveOffsetSet(offsetsFile string) (OffsetSet, error) {
	if offsetsFile == "" {
		return DefaultOffsetSet(), nil
	}
	return LoadOffsetSetFile(offsetsFile)
}
