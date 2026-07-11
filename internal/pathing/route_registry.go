package pathing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	// ErrRouteNotFound indicates that a route ID is absent or blocked.
	ErrRouteNotFound = errors.New("route not found")
	// ErrRouteDuplicateID indicates that an ID is published by multiple files.
	ErrRouteDuplicateID = errors.New("route duplicate id")
)

// RouteRegistryStatus describes whether a discovered file is usable.
type RouteRegistryStatus string

const (
	// RouteRegistryValid marks a validated and uniquely addressable route.
	RouteRegistryValid RouteRegistryStatus = "valid"
	// RouteRegistryInvalid marks a file that failed decoding or validation.
	RouteRegistryInvalid RouteRegistryStatus = "invalid"
	// RouteRegistryDuplicate marks every file sharing a route ID.
	RouteRegistryDuplicate RouteRegistryStatus = "duplicate_id"
)

// RouteRegistryEntry is the GUI-neutral management view of one route file.
type RouteRegistryEntry struct {
	Path       string
	ID         string
	Name       string
	Tags       []string
	Binding    RouteBinding
	RecordedAt string
	Status     RouteRegistryStatus
	Reason     string
}

// RouteRegistry indexes validated route files by stable ID.
type RouteRegistry struct {
	directory string
	entries   []RouteRegistryEntry
	routes    map[string]Route
	blocked   map[string]error
}

// LoadRouteRegistry scans a directory and blocks invalid or duplicate route files.
func LoadRouteRegistry(directory string) (*RouteRegistry, error) {
	items, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return &RouteRegistry{directory: directory, routes: map[string]Route{}, blocked: map[string]error{}}, nil
		}
		return nil, fmt.Errorf("read route directory %q: %w", directory, err)
	}
	registry := &RouteRegistry{directory: directory, routes: make(map[string]Route), blocked: make(map[string]error)}
	idEntries := make(map[string][]int)
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(item.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(directory, item.Name())
		route, loadErr := LoadRoute(path)
		entry := RouteRegistryEntry{Path: path, Status: RouteRegistryInvalid}
		if loadErr != nil {
			entry.Reason = loadErr.Error()
			registry.entries = append(registry.entries, entry)
			continue
		}
		entry.ID, entry.Name, entry.Tags, entry.Binding = route.ID, route.Name, append([]string(nil), route.Tags...), route.Binding
		entry.RecordedAt = route.Recording.RecordedAt.Format("2006-01-02T15:04:05Z")
		entry.Status = RouteRegistryValid
		registry.entries = append(registry.entries, entry)
		index := len(registry.entries) - 1
		idEntries[route.ID] = append(idEntries[route.ID], index)
		registry.routes[route.ID] = route
	}
	for id, indexes := range idEntries {
		if len(indexes) < 2 {
			continue
		}
		delete(registry.routes, id)
		registry.blocked[id] = fmt.Errorf("%w: %q", ErrRouteDuplicateID, id)
		for _, index := range indexes {
			registry.entries[index].Status = RouteRegistryDuplicate
			registry.entries[index].Reason = fmt.Sprintf("%v: %q", ErrRouteDuplicateID, id)
		}
	}
	sort.Slice(registry.entries, func(i, j int) bool { return registry.entries[i].Path < registry.entries[j].Path })
	return registry, nil
}

// Directory returns the scanned route directory.
func (r *RouteRegistry) Directory() string { return r.directory }

// Entries returns a defensive copy of all discovered route metadata.
func (r *RouteRegistry) Entries() []RouteRegistryEntry {
	entries := make([]RouteRegistryEntry, len(r.entries))
	copy(entries, r.entries)
	for i := range entries {
		entries[i].Tags = append([]string(nil), entries[i].Tags...)
	}
	return entries
}

// Get returns one uniquely validated route by stable ID.
func (r *RouteRegistry) Get(id string) (Route, error) {
	if err, blocked := r.blocked[id]; blocked {
		return Route{}, err
	}
	route, ok := r.routes[id]
	if !ok {
		return Route{}, fmt.Errorf("%w: %q", ErrRouteNotFound, id)
	}
	route.Tags = append([]string(nil), route.Tags...)
	route.Segments = append([]RouteSegment(nil), route.Segments...)
	for i := range route.Segments {
		route.Segments[i].Points = append([]RoutePoint(nil), route.Segments[i].Points...)
	}
	return route, nil
}
