package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	objectInspectDefaultTimeout = 30 * time.Second
	objectInspectSchemaVersion  = 2
	// objectInspectQuantityStatID is itemstatcost.txt quantity (*ID 70). Gate 23.0
	// only reports the raw live stat; the productive StatQuantity constant belongs
	// to 23.1 after this report.
	objectInspectQuantityStatID uint16 = 70
)

var objectInspectLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type objectInspectEvidenceReader interface {
	CollectObjectInspectEvidence() ([]memory.ObjectInspectEvidence, error)
	CollectItemStatListEvidence() ([]memory.ItemStatListEvidence, error)
}

type objectInspectArtifact struct {
	SchemaVersion int                    `json:"schema_version"`
	CapturedAt    time.Time              `json:"captured_at"`
	Label         string                 `json:"label"`
	GameVersion   string                 `json:"game_version"`
	AreaID        uint32                 `json:"area_id"`
	AreaName      string                 `json:"area_name"`
	PlayerX       uint32                 `json:"player_x"`
	PlayerY       uint32                 `json:"player_y"`
	Hover         world.HoverInfo        `json:"hover"`
	ObjectCount   int                    `json:"object_count"`
	Objects       []objectInspectObject  `json:"objects"`
	KeyStacks     []objectInspectKeyItem `json:"key_stacks"`
	CatalogSource string                 `json:"catalog_source"`
	Notes         []string               `json:"notes"`
}

type objectInspectObject struct {
	TxtFileNo     uint32 `json:"txt_file_no"`
	UnitID        uint32 `json:"unit_id"`
	PosX          uint32 `json:"pos_x"`
	PosY          uint32 `json:"pos_y"`
	PositionKnown bool   `json:"position_known"`
	Mode          uint32 `json:"mode"`
	ModeKnown     bool   `json:"mode_known"`
	Hovered       bool   `json:"hovered"`
	CatalogName   string `json:"catalog_name,omitempty"`
	CatalogClass  string `json:"catalog_class,omitempty"`
	RuntimeKind   string `json:"runtime_kind"`
	DistanceTiles int    `json:"distance_tiles"`
}

type objectInspectKeyItem struct {
	TxtFileNo           uint32             `json:"txt_file_no"`
	UnitID              uint32             `json:"unit_id"`
	Code                string             `json:"code"`
	Name                string             `json:"name"`
	Location            world.ItemLocation `json:"location"`
	GridX               int                `json:"grid_x"`
	GridY               int                `json:"grid_y"`
	Identified          bool               `json:"identified"`
	Stats               []world.ItemStat   `json:"stats"`
	StatsListExPresent  bool               `json:"stats_list_ex_present"`
	StatsActive         []world.ItemStat   `json:"stats_active"`
	StatsActiveReadable bool               `json:"stats_active_readable"`
	StatsActiveError    string             `json:"stats_active_error,omitempty"`
	StatsBase           []world.ItemStat   `json:"stats_base"`
	StatsBaseReadable   bool               `json:"stats_base_readable"`
	StatsBaseError      string             `json:"stats_base_error,omitempty"`
	QuantityStat        *int32             `json:"quantity_stat,omitempty"`
	QuantityKnown       bool               `json:"quantity_known"`
	QuantitySource      string             `json:"quantity_source,omitempty"`
	QuantityStatID      uint16             `json:"quantity_stat_id"`
}

type objectInspectCatalogEntry struct {
	Class string
	Name  string
}

func validateObjectInspectLabel(label string) error {
	if !objectInspectLabelPattern.MatchString(label) {
		return fmt.Errorf("--object-inspect label must match %s", objectInspectLabelPattern.String())
	}
	return nil
}

// RunObjectInspect writes one read-only Gate-23.0 object report for the current
// area. It never selects a run and never sends keyboard or mouse input.
func (rt *Runtime) RunObjectInspect(label string) error {
	if err := validateObjectInspectLabel(label); err != nil {
		return err
	}
	if rt.ObjectInspect == nil {
		return fmt.Errorf("object inspect: object reader unavailable")
	}
	timeout := time.Duration(rt.Options.ObjectInspectTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = objectInspectDefaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()

	ticker := time.NewTicker(time.Duration(max(1, rt.Config.Runtime.PollIntervalMs)) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	rt.Log.Info("object inspect started", "label", label, "timeout", timeout.String(), "input", "disabled")

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("object inspect timeout after %s waiting for an in-game snapshot", timeout)
			}
			return nil
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("object inspect poll: %w", err)
			}
			if !state.attached || !rt.lastSnapshot.Valid || rt.lastSnapshot.Phase != memory.GamePhaseInGame {
				continue
			}
			current := rt.World.Current()
			if !current.Valid || current.Phase != world.GamePhaseInGame {
				continue
			}
			objects, collectErr := rt.ObjectInspect.CollectObjectInspectEvidence()
			if collectErr != nil {
				return fmt.Errorf("object inspect collect: %w", collectErr)
			}
			statLists, statErr := rt.ObjectInspect.CollectItemStatListEvidence()
			if statErr != nil {
				return fmt.Errorf("object inspect item stats: %w", statErr)
			}
			catalog, catalogSource := loadObjectInspectCatalog(rt.objectInspectCatalogPath())
			artifact := buildObjectInspectArtifact(label, rt.Config.Memory.GameVersion, current, objects, statLists, catalog, catalogSource)
			path, err := saveObjectInspectArtifact(rt.Config.ResolvePath(filepath.Join("diagnostics", "objects")), artifact)
			if err != nil {
				return err
			}
			rt.Log.Info("object inspect written",
				"path", path, "label", label, "area_id", artifact.AreaID,
				"area", artifact.AreaName, "objects", artifact.ObjectCount,
				"key_stacks", len(artifact.KeyStacks), "catalog_source", artifact.CatalogSource,
			)
			for _, object := range artifact.Objects {
				rt.Log.Info("object inspect object",
					"txt_file_no", object.TxtFileNo, "unit_id", object.UnitID,
					"x", object.PosX, "y", object.PosY, "mode", object.Mode,
					"mode_known", object.ModeKnown, "catalog_name", object.CatalogName,
					"catalog_class", object.CatalogClass, "runtime_kind", object.RuntimeKind,
					"distance_tiles", object.DistanceTiles, "hovered", object.Hovered,
				)
			}
			for _, key := range artifact.KeyStacks {
				rt.Log.Info("object inspect key stack",
					"unit_id", key.UnitID, "location", key.Location,
					"grid", fmt.Sprintf("%d,%d", key.GridX, key.GridY),
					"identified", key.Identified,
					"stats_list_ex_present", key.StatsListExPresent,
					"stats_active_readable", key.StatsActiveReadable, "stats_active_count", len(key.StatsActive),
					"stats_base_readable", key.StatsBaseReadable, "stats_base_count", len(key.StatsBase),
					"quantity_known", key.QuantityKnown, "quantity_stat", valueOrNil(key.QuantityStat),
					"quantity_source", key.QuantitySource,
					"stat_count", len(key.Stats),
				)
			}
			return nil
		}
	}
}

func (rt *Runtime) objectInspectCatalogPath() string {
	if rt.Config != nil && strings.TrimSpace(rt.Config.DataRoot) != "" {
		return filepath.Join(rt.Config.DataRoot, ".tmp", "d2r-excel", "objects.txt")
	}
	return filepath.Join(".tmp", "d2r-excel", "objects.txt")
}

func buildObjectInspectArtifact(label, gameVersion string, state world.State, objects []memory.ObjectInspectEvidence, statLists []memory.ItemStatListEvidence, catalog map[uint32]objectInspectCatalogEntry, catalogSource string) objectInspectArtifact {
	rows := make([]objectInspectObject, 0, len(objects))
	for _, object := range objects {
		name := world.LookupObjectName(object.TxtFileNo)
		class := ""
		if entry, ok := catalog[object.TxtFileNo]; ok {
			if name == "" {
				name = entry.Name
			}
			class = entry.Class
		}
		distance := 0
		if object.PositionKnown {
			dx := int(object.PosX) - int(state.Player.Position.X)
			dy := int(object.PosY) - int(state.Player.Position.Y)
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			distance = dx + dy
		}
		rows = append(rows, objectInspectObject{
			TxtFileNo: object.TxtFileNo, UnitID: object.UnitID,
			PosX: object.PosX, PosY: object.PosY, PositionKnown: object.PositionKnown,
			Mode: object.Mode, ModeKnown: object.ModeKnown, Hovered: object.Hovered,
			CatalogName: name, CatalogClass: class,
			RuntimeKind:   world.LookupObjectKind(object.TxtFileNo).String(),
			DistanceTiles: distance,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].DistanceTiles != rows[j].DistanceTiles {
			return rows[i].DistanceTiles < rows[j].DistanceTiles
		}
		if rows[i].TxtFileNo != rows[j].TxtFileNo {
			return rows[i].TxtFileNo < rows[j].TxtFileNo
		}
		return rows[i].UnitID < rows[j].UnitID
	})
	return objectInspectArtifact{
		SchemaVersion: objectInspectSchemaVersion,
		CapturedAt:    time.Now().UTC(),
		Label:         label,
		GameVersion:   gameVersion,
		AreaID:        uint32(state.Area.ID),
		AreaName:      state.Area.Name,
		PlayerX:       state.Player.Position.X,
		PlayerY:       state.Player.Position.Y,
		Hover:         state.Hover,
		ObjectCount:   len(rows),
		Objects:       rows,
		KeyStacks:     collectObjectInspectKeyStacks(state.Items, statLists),
		CatalogSource: catalogSource,
		Notes: []string{
			"Read-only Gate 23.0 capture. This mode never sends keyboard or mouse input.",
			"The object walk is not filtered by the productive runtime allowlist.",
			"No Supertruhe or rack IDs are authorized for product use by this report.",
			"Mode uses UnitAny+0x0C. Closed, opening, open, and locked values come from live deltas.",
			"stats is the productive Active-preferred item list. stats_active and stats_base are read separately.",
			"quantity_stat is itemstatcost ID 70 on layer 0, Active first then Base. This is not a productive stack counter.",
		},
	}
}

func collectObjectInspectKeyStacks(items []world.Item, statLists []memory.ItemStatListEvidence) []objectInspectKeyItem {
	byUnit := make(map[uint32]memory.ItemStatListEvidence, len(statLists))
	for _, list := range statLists {
		byUnit[list.UnitID] = list
	}
	out := make([]objectInspectKeyItem, 0)
	for _, item := range items {
		if item.Code != "key" {
			continue
		}
		row := objectInspectKeyItem{
			TxtFileNo: item.TxtFileNo, UnitID: item.UnitID, Code: item.Code, Name: item.Name,
			Location: item.Location, GridX: item.GridX, GridY: item.GridY,
			Identified:     item.Identified,
			Stats:          append([]world.ItemStat(nil), item.Stats...),
			QuantityStatID: objectInspectQuantityStatID,
		}
		if list, ok := byUnit[item.UnitID]; ok {
			row.StatsListExPresent = list.StatsListExPresent
			row.StatsActive = rawStatsToItemStats(list.Active)
			row.StatsActiveReadable = list.ActiveReadable
			row.StatsActiveError = list.ActiveError
			row.StatsBase = rawStatsToItemStats(list.Base)
			row.StatsBaseReadable = list.BaseReadable
			row.StatsBaseError = list.BaseError
		}
		if quantity, known := objectInspectQuantity(row.StatsActive); known {
			value := quantity
			row.QuantityStat = &value
			row.QuantityKnown = true
			row.QuantitySource = "active"
		} else if quantity, known := objectInspectQuantity(row.StatsBase); known {
			value := quantity
			row.QuantityStat = &value
			row.QuantityKnown = true
			row.QuantitySource = "base"
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Location != out[j].Location {
			return out[i].Location < out[j].Location
		}
		if out[i].GridY != out[j].GridY {
			return out[i].GridY < out[j].GridY
		}
		return out[i].GridX < out[j].GridX
	})
	return out
}

func objectInspectQuantity(stats []world.ItemStat) (int32, bool) {
	for _, stat := range stats {
		if stat.Layer == 0 && stat.ID == objectInspectQuantityStatID {
			return stat.Value, true
		}
	}
	return 0, false
}

func rawStatsToItemStats(stats []memory.RawStat) []world.ItemStat {
	if stats == nil {
		return nil
	}
	out := make([]world.ItemStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, world.ItemStat{ID: stat.ID, Layer: stat.Layer, Value: stat.Value})
	}
	return out
}

func loadObjectInspectCatalog(path string) (map[uint32]objectInspectCatalogEntry, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "lookup_only"
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		return nil, "lookup_only"
	}
	header := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		header[strings.ToLower(strings.TrimSpace(name))] = i
	}
	classIdx, classOK := header["class"]
	nameIdx, nameOK := header["name"]
	idIdx, idOK := header["*id"]
	if !classOK || !nameOK || !idOK {
		return nil, "lookup_only"
	}
	out := make(map[uint32]objectInspectCatalogEntry)
	for _, record := range records[1:] {
		if idIdx >= len(record) {
			continue
		}
		rawID := strings.TrimSpace(record[idIdx])
		if rawID == "" {
			continue
		}
		id, err := strconv.ParseUint(rawID, 10, 32)
		if err != nil {
			continue
		}
		entry := objectInspectCatalogEntry{}
		if classIdx < len(record) {
			entry.Class = strings.TrimSpace(record[classIdx])
		}
		if nameIdx < len(record) {
			entry.Name = strings.TrimSpace(record[nameIdx])
		}
		out[uint32(id)] = entry
	}
	if len(out) == 0 {
		return nil, "lookup_only"
	}
	return out, path
}

func saveObjectInspectArtifact(directory string, artifact objectInspectArtifact) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create object inspect directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s.json", artifact.CapturedAt.Format("20060102T150405.000000000Z"), artifact.Label)
	path := filepath.Join(directory, name)
	tmp, err := os.CreateTemp(directory, ".object-inspect-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary object inspect artifact: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(artifact); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("encode object inspect artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("flush object inspect artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close object inspect artifact: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", fmt.Errorf("publish object inspect artifact: %w", err)
	}
	return path, nil
}

func valueOrNil(value *int32) any {
	if value == nil {
		return nil
	}
	return *value
}
