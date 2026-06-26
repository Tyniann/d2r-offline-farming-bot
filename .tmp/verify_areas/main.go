package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	d2goPath := `.tmp/d2go-ref/github.com/hectorgimenez/d2go@v0.0.0-20251023061335-16d248a53591/pkg/data/area/areas.go`
	worldPath := `internal/world/areas_data.go`
	d2goConstPath := `.tmp/d2go-ref/github.com/hectorgimenez/d2go@v0.0.0-20251023061335-16d248a53591/pkg/data/area/area.go`

	d2goNames := parseD2goAreas(d2goPath)
	worldNames := parseWorldCatalog(worldPath)
	mismatches := 0
	for id := uint32(0); id <= 136; id++ {
		dn := d2goNames[id]
		wn := worldNames[id]
		if id == 0 {
			if wn != "" {
				fmt.Printf("ID 0: world should not have catalog name, got %q\n", wn)
				mismatches++
			}
			continue
		}
		if dn != wn {
			fmt.Printf("ID %d: d2go=%q world=%q\n", id, dn, wn)
			mismatches++
		}
	}
	if len(worldNames) != 136 {
		fmt.Printf("world catalog count = %d, want 136\n", len(worldNames))
		mismatches++
	}

	d2goConsts := parseConsts(d2goConstPath, "ID")
	worldConsts := parseConsts(worldPath, "AreaID")
	for name, dval := range d2goConsts {
		wval, ok := worldConsts[name]
		if !ok {
			fmt.Printf("missing constant %s (d2go=%d)\n", name, dval)
			mismatches++
			continue
		}
		if dval != wval {
			fmt.Printf("%s: d2go=%d world=%d\n", name, dval, wval)
			mismatches++
		}
	}
	for name := range worldConsts {
		if _, ok := d2goConsts[name]; !ok {
			fmt.Printf("extra constant %s=%d\n", name, worldConsts[name])
			mismatches++
		}
	}

	if mismatches == 0 {
		fmt.Println("OK: all names and constants match")
	}
	os.Exit(mismatches)
}

func parseD2goAreas(path string) map[uint32]string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	re := regexp.MustCompile(`(\d+):\s+\{Name:\s+"([^"]*)"`)
	m := make(map[uint32]string)
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		id, _ := strconv.ParseUint(match[1], 10, 32)
		m[uint32(id)] = match[2]
	}
	return m
}

func parseWorldCatalog(path string) map[uint32]string {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	re := regexp.MustCompile(`^\s*(\d+):\s+\{name:\s+"([^"]*)"`)
	m := make(map[uint32]string)
	inCatalog := false
	for _, line := range strings.Split(string(content), "\n") {
		if strings.Contains(line, "var areaCatalog = map[AreaID]areaEntry{") {
			inCatalog = true
			continue
		}
		if inCatalog {
			if strings.TrimSpace(line) == "}" {
				break
			}
			if match := re.FindStringSubmatch(line); match != nil {
				id, _ := strconv.ParseUint(match[1], 10, 32)
				m[uint32(id)] = match[2]
			}
		}
	}
	return m
}

func parseConsts(path, typ string) map[string]uint32 {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	re := regexp.MustCompile(`(?m)^\s*([A-Za-z0-9]+)\s+` + typ + ` = (\d+)`)
	m := make(map[string]uint32)
	for _, match := range re.FindAllStringSubmatch(string(content), -1) {
		v, _ := strconv.ParseUint(match[2], 10, 32)
		m[match[1]] = uint32(v)
	}
	return m
}
