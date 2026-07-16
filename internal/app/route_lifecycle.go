package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"gopkg.in/yaml.v3"
)

const routeLifecycleSchemaVersion = 1

// RouteLifecycleStatus is the persistent/static availability state of a Farming route.
type RouteLifecycleStatus string

const (
	// RouteLifecycleValid means the route also passed the supplied live layout proof.
	RouteLifecycleValid RouteLifecycleStatus = "valid"
	// RouteLifecycleRuntimeValidationRequired permits a run but gates playback on its live fingerprint.
	RouteLifecycleRuntimeValidationRequired RouteLifecycleStatus = "runtime_validation_required"
	// RouteLifecycleStale blocks a route invalidated by difficulty or layout change.
	RouteLifecycleStale RouteLifecycleStatus = "stale"
	// RouteLifecycleUnavailable blocks a missing, malformed, duplicate, changed, or misbound route.
	RouteLifecycleUnavailable RouteLifecycleStatus = "unavailable"
)

// RouteLifecycleManifest persists only lifecycle metadata; route files remain authoritative content.
type RouteLifecycleManifest struct {
	SchemaVersion     int                                `yaml:"schema_version"`
	Revision          uint64                             `yaml:"revision"`
	BootstrapExpected *RouteLifecycleContext             `yaml:"bootstrap_expected,omitempty"`
	Characters        map[string]RouteLifecycleCharacter `yaml:"characters"`
}

// RouteLifecycleContext identifies one character/difficulty selection.
type RouteLifecycleContext struct {
	Character  string `yaml:"character" json:"character"`
	Difficulty string `yaml:"difficulty" json:"difficulty"`
}

// RouteLifecycleCharacter stores confirmation and per-route invalidation metadata.
type RouteLifecycleCharacter struct {
	LastConfirmedDifficulty string                         `yaml:"last_confirmed_difficulty,omitempty"`
	ConfirmedAt             *time.Time                     `yaml:"confirmed_at,omitempty"`
	Routes                  map[string]RouteLifecycleRoute `yaml:"routes"`
}

// RouteLifecycleRoute correlates one route file without duplicating its route contract.
type RouteLifecycleRoute struct {
	RecordedAt              time.Time  `yaml:"recorded_at"`
	ObservedFileFingerprint string     `yaml:"observed_file_fingerprint"`
	InvalidatedAt           *time.Time `yaml:"invalidated_at,omitempty"`
	InvalidationReason      string     `yaml:"invalidation_reason,omitempty"`
}

// FarmingRouteCatalogEntry is one deterministic recursive catalog result.
type FarmingRouteCatalogEntry struct {
	ID              string
	Path            string
	Character       string
	Difficulty      string
	Route           pathing.Route
	FileFingerprint string
	Status          RouteLifecycleStatus
	Reason          string
}

// FarmingRouteCatalog is an immutable snapshot of all Farming route sets below one root.
type FarmingRouteCatalog struct {
	Revision uint64
	Entries  []FarmingRouteCatalogEntry
}

// RouteLifecyclePreview binds an invalidation decision to one immutable manifest revision.
type RouteLifecyclePreview struct {
	Revision       uint64
	Character      string
	OldDifficulty  string
	NewDifficulty  string
	AffectedRoutes []string
	Reason         string
}

// RouteLifecycleStore serializes bootstrap, preview, confirmation, and invalidation commits.
type RouteLifecycleStore struct {
	mu       *sync.Mutex
	root     string
	path     string
	expected RouteLifecycleContext
}

var routeLifecycleFileLocks sync.Map

// NewRouteLifecycleStore creates the sole persistent lifecycle authority for a config.
func NewRouteLifecycleStore(cfg *config.Config) (*RouteLifecycleStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("route lifecycle requires config")
	}
	expected := RouteLifecycleContext{Character: strings.TrimSpace(cfg.Session.Character), Difficulty: strings.ToLower(strings.TrimSpace(cfg.Session.Difficulty))}
	path := cfg.ResolvePath(cfg.Routes.LifecycleFile)
	lock, _ := routeLifecycleFileLocks.LoadOrStore(filepath.Clean(path), &sync.Mutex{})
	return &RouteLifecycleStore{mu: lock.(*sync.Mutex), root: cfg.ResolvePath(cfg.Routes.FarmingRoot), path: path, expected: expected}, nil
}

// Snapshot loads or bootstraps the manifest and returns a deterministic catalog.
func (s *RouteLifecycleStore) Snapshot() (RouteLifecycleManifest, FarmingRouteCatalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, entries, created, err := s.loadOrBootstrapLocked()
	if err != nil {
		return RouteLifecycleManifest{}, FarmingRouteCatalog{}, err
	}
	if created {
		if err := s.writeLocked(manifest); err != nil {
			return RouteLifecycleManifest{}, FarmingRouteCatalog{}, err
		}
	}
	return cloneRouteLifecycleManifest(manifest), catalogFrom(manifest, entries), nil
}

// Preview computes the exact route impact without writing the manifest.
func (s *RouteLifecycleStore) Preview(character, difficulty string) (RouteLifecyclePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, entries, created, err := s.loadOrBootstrapLocked()
	if err != nil {
		return RouteLifecyclePreview{}, err
	}
	if created {
		if err := s.writeLocked(manifest); err != nil {
			return RouteLifecyclePreview{}, err
		}
	}
	return buildRouteLifecyclePreview(manifest, entries, character, difficulty)
}

// Confirm commits lifecycle changes only after a verified in-game selection.
func (s *RouteLifecycleStore) Confirm(preview RouteLifecyclePreview, at time.Time) (RouteLifecycleManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, entries, created, err := s.loadOrBootstrapLocked()
	if err != nil {
		return RouteLifecycleManifest{}, err
	}
	if created {
		if err := s.writeLocked(manifest); err != nil {
			return RouteLifecycleManifest{}, err
		}
	}
	if manifest.Revision != preview.Revision {
		return RouteLifecycleManifest{}, fmt.Errorf("route lifecycle revision changed: got %d want %d", manifest.Revision, preview.Revision)
	}
	current, err := buildRouteLifecyclePreview(manifest, entries, preview.Character, preview.NewDifficulty)
	if err != nil {
		return RouteLifecycleManifest{}, err
	}
	if current.OldDifficulty != preview.OldDifficulty || current.Reason != preview.Reason || !sameStrings(current.AffectedRoutes, preview.AffectedRoutes) {
		return RouteLifecycleManifest{}, fmt.Errorf("route lifecycle preview changed")
	}
	slug := strings.ToLower(preview.Character)
	character := manifest.Characters[slug]
	if character.Routes == nil {
		character.Routes = map[string]RouteLifecycleRoute{}
	}
	if preview.Reason == "difficulty_changed" {
		for id, route := range character.Routes {
			stamp := at.UTC()
			route.InvalidatedAt = &stamp
			route.InvalidationReason = "difficulty_changed"
			character.Routes[id] = route
		}
	}
	stamp := at.UTC()
	character.LastConfirmedDifficulty = preview.NewDifficulty
	character.ConfirmedAt = &stamp
	manifest.Characters[slug] = character
	if manifest.BootstrapExpected != nil && strings.EqualFold(manifest.BootstrapExpected.Character, preview.Character) {
		manifest.BootstrapExpected = nil
	}
	manifest.Revision++
	if err := s.writeLocked(manifest); err != nil {
		return RouteLifecycleManifest{}, err
	}
	return cloneRouteLifecycleManifest(manifest), nil
}

// InvalidateLayout atomically stales all Farming routes of exactly one character.
func (s *RouteLifecycleStore) InvalidateLayout(character string, expectedRevision uint64, at time.Time) (RouteLifecycleManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, _, created, err := s.loadOrBootstrapLocked()
	if err != nil {
		return RouteLifecycleManifest{}, err
	}
	if created {
		if err := s.writeLocked(manifest); err != nil {
			return RouteLifecycleManifest{}, err
		}
	}
	if manifest.Revision != expectedRevision {
		return RouteLifecycleManifest{}, fmt.Errorf("route lifecycle revision changed: got %d want %d", manifest.Revision, expectedRevision)
	}
	slug := strings.ToLower(strings.TrimSpace(character))
	entry, ok := manifest.Characters[slug]
	if !ok {
		return RouteLifecycleManifest{}, fmt.Errorf("route lifecycle character %q not found", character)
	}
	stamp := at.UTC()
	for id, route := range entry.Routes {
		route.InvalidatedAt = &stamp
		route.InvalidationReason = "layout_mismatch_detected"
		entry.Routes[id] = route
	}
	manifest.Characters[slug] = entry
	manifest.Revision++
	if err := s.writeLocked(manifest); err != nil {
		return RouteLifecycleManifest{}, err
	}
	return cloneRouteLifecycleManifest(manifest), nil
}

// RecordRoute imports one newly published route and rehabilitates only that route.
func (s *RouteLifecycleStore) RecordRoute(path string) (RouteLifecycleManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, _, created, err := s.loadOrBootstrapLocked()
	if err != nil {
		return RouteLifecycleManifest{}, err
	}
	if created {
		if err := s.writeLocked(manifest); err != nil {
			return RouteLifecycleManifest{}, err
		}
	}
	route, err := pathing.LoadRoute(path)
	if err != nil {
		return RouteLifecycleManifest{}, err
	}
	fingerprint, err := fileSHA256(path)
	if err != nil {
		return RouteLifecycleManifest{}, err
	}
	slug := strings.ToLower(route.Binding.CharacterName)
	character := manifest.Characters[slug]
	if character.Routes == nil {
		character.Routes = map[string]RouteLifecycleRoute{}
	}
	character.Routes[route.ID] = RouteLifecycleRoute{RecordedAt: route.Recording.RecordedAt.UTC(), ObservedFileFingerprint: fingerprint}
	manifest.Characters[slug] = character
	manifest.Revision++
	if err := s.writeLocked(manifest); err != nil {
		return RouteLifecycleManifest{}, err
	}
	return cloneRouteLifecycleManifest(manifest), nil
}

func (s *RouteLifecycleStore) loadOrBootstrapLocked() (RouteLifecycleManifest, []FarmingRouteCatalogEntry, bool, error) {
	entries, err := scanFarmingRoutes(s.root)
	if err != nil {
		return RouteLifecycleManifest{}, nil, false, err
	}
	data, err := os.ReadFile(s.path)
	if err == nil {
		var manifest RouteLifecycleManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return RouteLifecycleManifest{}, nil, false, fmt.Errorf("decode route lifecycle %q: %w", s.path, err)
		}
		if err := validateRouteLifecycleManifest(manifest); err != nil {
			return RouteLifecycleManifest{}, nil, false, err
		}
		return manifest, entries, false, nil
	}
	if !os.IsNotExist(err) {
		return RouteLifecycleManifest{}, nil, false, fmt.Errorf("read route lifecycle %q: %w", s.path, err)
	}
	manifest := RouteLifecycleManifest{SchemaVersion: routeLifecycleSchemaVersion, Revision: 1, Characters: map[string]RouteLifecycleCharacter{}}
	if s.expected.Character != "" && s.expected.Difficulty != "" {
		expected := s.expected
		manifest.BootstrapExpected = &expected
	}
	for _, entry := range entries {
		if entry.Status == RouteLifecycleUnavailable {
			continue
		}
		slug := strings.ToLower(entry.Character)
		character := manifest.Characters[slug]
		if character.Routes == nil {
			character.Routes = map[string]RouteLifecycleRoute{}
		}
		character.Routes[entry.ID] = RouteLifecycleRoute{RecordedAt: entry.Route.Recording.RecordedAt.UTC(), ObservedFileFingerprint: entry.FileFingerprint}
		manifest.Characters[slug] = character
	}
	return manifest, entries, true, nil
}

func (s *RouteLifecycleStore) writeLocked(manifest RouteLifecycleManifest) error {
	if err := validateRouteLifecycleManifest(manifest); err != nil {
		return err
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode route lifecycle: %w", err)
	}
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create route lifecycle directory %q: %w", directory, err)
	}
	tmp, err := os.CreateTemp(directory, ".route-lifecycle-*.tmp")
	if err != nil {
		return fmt.Errorf("create route lifecycle temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write route lifecycle temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush route lifecycle temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close route lifecycle temp file: %w", err)
	}
	if err := replaceFile(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace route lifecycle %q: %w", s.path, err)
	}
	return nil
}

func scanFarmingRoutes(root string) ([]FarmingRouteCatalogEntry, error) {
	entries := []FarmingRouteCatalogEntry{}
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.Type()&os.ModeSymlink != 0 {
			if item.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if item.IsDir() || (strings.ToLower(filepath.Ext(item.Name())) != ".yaml" && strings.ToLower(filepath.Ext(item.Name())) != ".yml") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		entry := FarmingRouteCatalogEntry{Path: path, Status: RouteLifecycleUnavailable}
		if len(parts) != 3 {
			entry.Reason = "route_context_path_invalid"
			entries = append(entries, entry)
			return nil
		}
		entry.Character, entry.Difficulty = parts[0], parts[1]
		route, err := pathing.LoadRoute(path)
		if err != nil {
			entry.Reason = err.Error()
			entries = append(entries, entry)
			return nil
		}
		entry.ID, entry.Route = route.ID, route
		if !strings.EqualFold(route.Binding.CharacterName, entry.Character) || string(route.Binding.Difficulty) != strings.ToLower(entry.Difficulty) {
			entry.Reason = "route_context_binding_mismatch"
			entries = append(entries, entry)
			return nil
		}
		fingerprint, err := fileSHA256(path)
		if err != nil {
			entry.Reason = err.Error()
			entries = append(entries, entry)
			return nil
		}
		entry.FileFingerprint = fingerprint
		entry.Status = RouteLifecycleRuntimeValidationRequired
		entries = append(entries, entry)
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan farming route root %q: %w", root, err)
	}
	ids := map[string][]int{}
	for i, entry := range entries {
		if entry.ID != "" {
			ids[entry.ID] = append(ids[entry.ID], i)
		}
	}
	for id, indexes := range ids {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			entries[index].Status = RouteLifecycleUnavailable
			entries[index].Reason = "route_duplicate_id:" + id
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func catalogFrom(manifest RouteLifecycleManifest, scanned []FarmingRouteCatalogEntry) FarmingRouteCatalog {
	entries := make([]FarmingRouteCatalogEntry, len(scanned))
	copy(entries, scanned)
	for i := range entries {
		entry := &entries[i]
		if entry.Status == RouteLifecycleUnavailable {
			continue
		}
		character, ok := manifest.Characters[strings.ToLower(entry.Character)]
		if !ok {
			entry.Status, entry.Reason = RouteLifecycleUnavailable, "route_lifecycle_missing"
			continue
		}
		lifecycle, ok := character.Routes[entry.ID]
		if !ok {
			entry.Status, entry.Reason = RouteLifecycleUnavailable, "route_lifecycle_missing"
			continue
		}
		if lifecycle.ObservedFileFingerprint != entry.FileFingerprint {
			entry.Status, entry.Reason = RouteLifecycleUnavailable, "route_file_changed"
			continue
		}
		if lifecycle.InvalidatedAt != nil && !entry.Route.Recording.RecordedAt.After(*lifecycle.InvalidatedAt) {
			entry.Status, entry.Reason = RouteLifecycleStale, lifecycle.InvalidationReason
		}
	}
	return FarmingRouteCatalog{Revision: manifest.Revision, Entries: entries}
}

func buildRouteLifecyclePreview(manifest RouteLifecycleManifest, entries []FarmingRouteCatalogEntry, character, difficulty string) (RouteLifecyclePreview, error) {
	validatedCharacter, err := validateOfflineCharacter(character)
	if err != nil {
		return RouteLifecyclePreview{}, err
	}
	validatedDifficulty, err := parseOfflineDifficulty(difficulty)
	if err != nil {
		return RouteLifecyclePreview{}, err
	}
	preview := RouteLifecyclePreview{Revision: manifest.Revision, Character: validatedCharacter, NewDifficulty: string(validatedDifficulty)}
	entry := manifest.Characters[strings.ToLower(validatedCharacter)]
	preview.OldDifficulty = entry.LastConfirmedDifficulty
	if preview.OldDifficulty == "" && manifest.BootstrapExpected != nil && strings.EqualFold(manifest.BootstrapExpected.Character, validatedCharacter) {
		preview.OldDifficulty = manifest.BootstrapExpected.Difficulty
		if preview.NewDifficulty != preview.OldDifficulty {
			preview.Reason = "difficulty_changed"
		}
	} else if preview.OldDifficulty != "" && preview.OldDifficulty != preview.NewDifficulty {
		preview.Reason = "difficulty_changed"
	}
	if preview.Reason != "" {
		for _, route := range entries {
			if route.ID != "" && strings.EqualFold(route.Character, validatedCharacter) {
				preview.AffectedRoutes = append(preview.AffectedRoutes, route.ID)
			}
		}
		sort.Strings(preview.AffectedRoutes)
	}
	return preview, nil
}

func validateRouteLifecycleManifest(manifest RouteLifecycleManifest) error {
	if manifest.SchemaVersion != routeLifecycleSchemaVersion {
		return fmt.Errorf("route lifecycle schema_version must be %d", routeLifecycleSchemaVersion)
	}
	if manifest.Revision == 0 {
		return fmt.Errorf("route lifecycle revision must be positive")
	}
	if manifest.Characters == nil {
		return fmt.Errorf("route lifecycle characters are required")
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("fingerprint route file %q: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cloneRouteLifecycleManifest(manifest RouteLifecycleManifest) RouteLifecycleManifest {
	clone := manifest
	if manifest.BootstrapExpected != nil {
		expected := *manifest.BootstrapExpected
		clone.BootstrapExpected = &expected
	}
	clone.Characters = make(map[string]RouteLifecycleCharacter, len(manifest.Characters))
	for slug, character := range manifest.Characters {
		copyCharacter := character
		copyCharacter.Routes = make(map[string]RouteLifecycleRoute, len(character.Routes))
		for id, route := range character.Routes {
			copyCharacter.Routes[id] = route
		}
		clone.Characters[slug] = copyCharacter
	}
	return clone
}
