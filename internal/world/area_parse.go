package world

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseAreaSpec resolves a CLI area spec into an AreaID. Accepted forms are a
// numeric ID (`"6"`) or a catalog name in any casing with spaces, dashes, or
// underscores (`"black_marsh"`, `"Black Marsh"`). Unknown names or IDs outside
// the catalog return an error listing the offending spec.
func ParseAreaSpec(spec string) (AreaID, error) {
	raw := strings.TrimSpace(spec)
	if raw == "" {
		return 0, fmt.Errorf("area spec is empty")
	}

	if n, err := strconv.ParseUint(raw, 10, 32); err == nil {
		id := AreaID(n)
		if id == 0 || !LookupArea(id).IsKnown() {
			return 0, fmt.Errorf("unknown area id %d", n)
		}
		return id, nil
	}

	want := normalizeAreaName(raw)
	for id, entry := range areaCatalog {
		if entry.name != "" && normalizeAreaName(entry.name) == want {
			return id, nil
		}
	}
	return 0, fmt.Errorf("unknown area name %q", spec)
}

// normalizeAreaName lowercases and unifies separators for name matching.
func normalizeAreaName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Join(strings.Fields(s), " ")
}
