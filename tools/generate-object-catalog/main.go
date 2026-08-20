// Command generate-object-catalog regenerates versioned object IDs from local D2R objects.txt.
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

type objectRow struct {
	ID          uint32
	Class       string
	Name        string
	Description string
}

type selectedObjects struct {
	TownPortal      objectRow
	PermanentPortal objectRow
	WirtsBody       objectRow
	GoodChest       objectRow
	Stash           objectRow
	Waypoints       []objectRow
	SuperChests     []objectRow
	Racks           []objectRow
}

func main() {
	src := flag.String("src", ".tmp/d2r-excel", "directory containing objects.txt")
	version := flag.String("version", "", "D2R version that produced objects.txt")
	memoryOut := flag.String("memory-out", filepath.Join("internal", "memory", "object_ids_data.go"), "generated memory allowlist path")
	worldOut := flag.String("world-out", filepath.Join("internal", "world", "object_ids_data.go"), "generated world catalog path")
	flag.Parse()

	if strings.TrimSpace(*version) == "" {
		fatalf("-version is required")
	}
	rows, err := readObjectRows(filepath.Join(*src, "objects.txt"))
	if err != nil {
		fatalf("read objects: %v", err)
	}
	selected, err := selectObjects(rows)
	if err != nil {
		fatalf("select objects: %v", err)
	}
	outputs := []struct {
		path string
		data []byte
	}{
		{*memoryOut, renderMemory(*version, selected)},
		{*worldOut, renderWorld(*version, selected)},
	}
	for _, output := range outputs {
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
	fmt.Fprintf(os.Stderr, "generate object catalog: "+format+"\n", args...)
	os.Exit(1)
}

func readObjectRows(path string) ([]objectRow, error) {
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
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%q has no data rows", path)
	}
	header := headerIndex(records[0])
	for _, required := range []string{"class", "name", "*description", "*id"} {
		if _, ok := header[required]; !ok {
			return nil, fmt.Errorf("%q missing column %q", path, required)
		}
	}

	rows := make([]objectRow, 0, len(records)-1)
	seen := make(map[uint32]bool)
	for line, record := range records[1:] {
		rawID := value(record, header, "*id")
		if rawID == "" {
			// objects.txt contains section marker rows such as "Expansion".
			continue
		}
		id, err := strconv.ParseUint(rawID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%q line %d invalid *ID %q", path, line+2, rawID)
		}
		objectID := uint32(id)
		if seen[objectID] {
			return nil, fmt.Errorf("%q line %d duplicate *ID %d", path, line+2, objectID)
		}
		seen[objectID] = true
		rows = append(rows, objectRow{
			ID:          objectID,
			Class:       value(record, header, "class"),
			Name:        value(record, header, "name"),
			Description: value(record, header, "*description"),
		})
	}
	return rows, nil
}

func selectObjects(rows []objectRow) (selectedObjects, error) {
	var selected selectedObjects
	portalCount := 0
	permanentPortalCount := 0
	wirtsBodyCount := 0
	chestCount := 0
	stashCount := 0
	superChestCounts := map[string]int{}
	rackCounts := map[string]int{}
	for _, row := range rows {
		switch row.Class {
		case "TownPortal":
			selected.TownPortal = row
			portalCount++
		case "PortalPermanent":
			selected.PermanentPortal = row
			permanentPortalCount++
		case "Wirt":
			selected.WirtsBody = row
			wirtsBodyCount++
		case "PlaceUniqueChest":
			selected.GoodChest = row
			chestCount++
		case "Bank":
			selected.Stash = row
			stashCount++
		case "JungleChest", "JungleChest2":
			selected.SuperChests = append(selected.SuperChests, row)
			superChestCounts[row.Class]++
		case "ArmorStand1", "WeaponRack2":
			selected.Racks = append(selected.Racks, row)
			rackCounts[row.Class]++
		}
		if row.Name == "Waypoint" {
			selected.Waypoints = append(selected.Waypoints, row)
		}
	}
	if portalCount != 1 {
		return selectedObjects{}, fmt.Errorf("Class=TownPortal count = %d, want 1", portalCount)
	}
	if permanentPortalCount != 1 {
		return selectedObjects{}, fmt.Errorf("Class=PortalPermanent count = %d, want 1", permanentPortalCount)
	}
	if wirtsBodyCount != 1 {
		return selectedObjects{}, fmt.Errorf("Class=Wirt count = %d, want 1", wirtsBodyCount)
	}
	if chestCount != 1 {
		return selectedObjects{}, fmt.Errorf("Class=PlaceUniqueChest count = %d, want 1", chestCount)
	}
	if stashCount != 1 {
		return selectedObjects{}, fmt.Errorf("Class=Bank count = %d, want 1", stashCount)
	}
	if len(selected.Waypoints) == 0 {
		return selectedObjects{}, fmt.Errorf("Name=Waypoint count = 0")
	}
	for _, class := range []string{"JungleChest", "JungleChest2"} {
		if superChestCounts[class] != 1 {
			return selectedObjects{}, fmt.Errorf("Class=%s count = %d, want 1", class, superChestCounts[class])
		}
	}
	for _, class := range []string{"ArmorStand1", "WeaponRack2"} {
		if rackCounts[class] != 1 {
			return selectedObjects{}, fmt.Errorf("Class=%s count = %d, want 1", class, rackCounts[class])
		}
	}
	sort.Slice(selected.Waypoints, func(i, j int) bool { return selected.Waypoints[i].ID < selected.Waypoints[j].ID })
	sort.Slice(selected.SuperChests, func(i, j int) bool { return selected.SuperChests[i].ID < selected.SuperChests[j].ID })
	sort.Slice(selected.Racks, func(i, j int) bool { return selected.Racks[i].ID < selected.Racks[j].ID })
	return selected, nil
}

func renderMemory(version string, selected selectedObjects) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local data/global/excel/objects.txt; DO NOT EDIT.\n", version)
	b.WriteString("package memory\n\n")
	b.WriteString("var runtimeObjectIDs = map[uint32]struct{}{\n")
	fmt.Fprintf(&b, "\t%d: {}, // %s\n", selected.TownPortal.ID, selected.TownPortal.Class)
	fmt.Fprintf(&b, "\t%d: {}, // %s\n", selected.PermanentPortal.ID, selected.PermanentPortal.Class)
	fmt.Fprintf(&b, "\t%d: {}, // %s\n", selected.WirtsBody.ID, selected.WirtsBody.Class)
	fmt.Fprintf(&b, "\t%d: {}, // %s\n", selected.GoodChest.ID, selected.GoodChest.Class)
	fmt.Fprintf(&b, "\t%d: {}, // %s\n", selected.Stash.ID, selected.Stash.Class)
	for _, chest := range selected.SuperChests {
		fmt.Fprintf(&b, "\t%d: {}, // %s\n", chest.ID, chest.Class)
	}
	for _, rack := range selected.Racks {
		fmt.Fprintf(&b, "\t%d: {}, // %s\n", rack.ID, rack.Class)
	}
	for _, waypoint := range selected.Waypoints {
		fmt.Fprintf(&b, "\t%d: {}, // %s\n", waypoint.ID, waypoint.Class)
	}
	b.WriteString("}\n")
	return b.Bytes()
}

func renderWorld(version string, selected selectedObjects) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated from D2R %s local data/global/excel/objects.txt; DO NOT EDIT.\n", version)
	b.WriteString("package world\n\n")
	b.WriteString("const (\n")
	fmt.Fprintf(&b, "\t// TownPortalID is the player-cast portal object for this generated D2R version.\n\tTownPortalID uint32 = %d\n", selected.TownPortal.ID)
	fmt.Fprintf(&b, "\t// PermanentPortalID is the area-bound red portal object from objects.txt Class=PortalPermanent.\n\tPermanentPortalID uint32 = %d\n", selected.PermanentPortal.ID)
	fmt.Fprintf(&b, "\t// WirtsBodyID is the Tristram quest object from objects.txt Class=Wirt.\n\tWirtsBodyID uint32 = %d\n", selected.WirtsBody.ID)
	fmt.Fprintf(&b, "\t// GoodChestID is the unique chest placement object for this generated D2R version.\n\tGoodChestID uint32 = %d\n", selected.GoodChest.ID)
	fmt.Fprintf(&b, "\t// PersonalStashID is the character stash object for this generated D2R version.\n\tPersonalStashID uint32 = %d\n", selected.Stash.ID)
	for _, chest := range selected.SuperChests {
		fmt.Fprintf(&b, "\t// %sID is objects.txt Class=%s, live Lower-Kurast Supertruhe.\n\t%sID uint32 = %d\n", chest.Class, chest.Class, chest.Class, chest.ID)
	}
	for _, rack := range selected.Racks {
		fmt.Fprintf(&b, "\t// %sID is objects.txt Class=%s, live Lower-Kurast Hüttengestell.\n\t%sID uint32 = %d\n", rack.Class, rack.Class, rack.Class, rack.ID)
	}
	b.WriteString(")\n\n")
	b.WriteString("var waypointIDs = []uint32{\n")
	for _, waypoint := range selected.Waypoints {
		fmt.Fprintf(&b, "\t%d, // %s\n", waypoint.ID, waypoint.Class)
	}
	b.WriteString("}\n\n")
	b.WriteString("var superChestIDs = []uint32{\n")
	for _, chest := range selected.SuperChests {
		fmt.Fprintf(&b, "\t%d, // %s\n", chest.ID, chest.Class)
	}
	b.WriteString("}\n\n")
	b.WriteString("var rackIDs = []uint32{\n")
	for _, rack := range selected.Racks {
		fmt.Fprintf(&b, "\t%d, // %s\n", rack.ID, rack.Class)
	}
	b.WriteString("}\n\n")
	b.WriteString("// IsWaypointID reports whether id is a waypoint object in the generated catalog.\n")
	b.WriteString("func IsWaypointID(id uint32) bool {\n\tfor _, waypointID := range waypointIDs {\n\t\tif id == waypointID {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\n")
	b.WriteString("// AllWaypointIDs returns a copy of generated waypoint object IDs.\n")
	b.WriteString("func AllWaypointIDs() []uint32 { return append([]uint32(nil), waypointIDs...) }\n\n")
	b.WriteString("// IsSuperChestID reports whether id is a live Lower-Kurast Supertruhe class.\n")
	b.WriteString("func IsSuperChestID(id uint32) bool {\n\tfor _, chestID := range superChestIDs {\n\t\tif id == chestID {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\n")
	b.WriteString("// AllSuperChestIDs returns a copy of generated Supertruhe object IDs.\n")
	b.WriteString("func AllSuperChestIDs() []uint32 { return append([]uint32(nil), superChestIDs...) }\n\n")
	b.WriteString("// IsRackID reports whether id is a live Lower-Kurast Hüttengestell class.\n")
	b.WriteString("func IsRackID(id uint32) bool {\n\tfor _, rackID := range rackIDs {\n\t\tif id == rackID {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\n")
	b.WriteString("// AllRackIDs returns a copy of generated Hüttengestell object IDs.\n")
	b.WriteString("func AllRackIDs() []uint32 { return append([]uint32(nil), rackIDs...) }\n\n")
	b.WriteString("var objectNames = map[uint32]string{\n")
	fmt.Fprintf(&b, "\tTownPortalID: \"Town Portal\",\n\tPermanentPortalID: \"Permanent Portal\",\n\tWirtsBodyID: \"Wirt's Body\",\n\tGoodChestID: \"Good Chest\",\n\tPersonalStashID: \"Personal Stash\",\n")
	for _, chest := range selected.SuperChests {
		fmt.Fprintf(&b, "\t%d: \"Super Chest\",\n", chest.ID)
	}
	for _, rack := range selected.Racks {
		name := "Rack"
		switch rack.Class {
		case "ArmorStand1":
			name = "Armor Stand"
		case "WeaponRack2":
			name = "Weapon Rack"
		}
		fmt.Fprintf(&b, "\t%d: %q,\n", rack.ID, name)
	}
	for _, waypoint := range selected.Waypoints {
		fmt.Fprintf(&b, "\t%d: \"Waypoint\",\n", waypoint.ID)
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
