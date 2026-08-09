package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"gopkg.in/yaml.v3"
)

// RouteAssignmentStore serializes the sole character/run assignment authority.
type RouteAssignmentStore struct {
	mu         *sync.Mutex
	path       string
	configPath string
	character  string
	legacy     map[string]string
}

var routeAssignmentFileLocks sync.Map

// NewRouteAssignmentStore creates a store and captures one-shot legacy migration input.
func NewRouteAssignmentStore(cfg *config.Config) (*RouteAssignmentStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("route assignment requires config")
	}
	path := cfg.ResolvePath(cfg.Routes.AssignmentsFile)
	lock, _ := routeAssignmentFileLocks.LoadOrStore(filepath.Clean(path), &sync.Mutex{})
	return &RouteAssignmentStore{mu: lock.(*sync.Mutex), path: path, configPath: cfg.LoadedFrom, character: strings.ToLower(strings.TrimSpace(cfg.Session.Character)), legacy: cfg.Runs.LegacyRouteIDs()}, nil
}

// Snapshot loads the manifest or atomically performs the one-shot legacy migration.
func (s *RouteAssignmentStore) Snapshot() (RouteAssignmentManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := s.loadLocked()
	if err == nil {
		return cloneRouteAssignmentManifest(manifest), nil
	}
	if !os.IsNotExist(err) {
		return RouteAssignmentManifest{}, err
	}
	manifest = RouteAssignmentManifest{SchemaVersion: RouteAssignmentSchemaVersion, Revision: 1, Assignments: map[string]map[tasks.RunID]string{}, RouteSets: map[string]map[tasks.RunID]map[pathing.RouteRole]string{}}
	if s.character != "" {
		for rawRunID, routeID := range s.legacy {
			runID := tasks.RunID(rawRunID)
			if _, ok := tasks.DefaultRunRegistry().Definition(runID); !ok || routeID == "" {
				continue
			}
			if manifest.Assignments[s.character] == nil {
				manifest.Assignments[s.character] = map[tasks.RunID]string{}
			}
			manifest.Assignments[s.character][runID] = routeID
		}
	}
	if err := s.writeLocked(manifest); err != nil {
		return RouteAssignmentManifest{}, err
	}
	if len(s.legacy) > 0 {
		if err := removeLegacyRouteIDs(s.configPath); err != nil {
			_ = os.Remove(s.path)
			return RouteAssignmentManifest{}, fmt.Errorf("remove migrated route_id fields: %w", err)
		}
	}
	return cloneRouteAssignmentManifest(manifest), nil
}

// Resolve returns the assigned route and manifest revision for one pair.
func (s *RouteAssignmentStore) Resolve(character string, runID tasks.RunID) (string, uint64, error) {
	manifest, err := s.Snapshot()
	if err != nil {
		return "", 0, err
	}
	routeID := manifest.Assignments[strings.ToLower(strings.TrimSpace(character))][runID]
	if routeID == "" {
		return "", manifest.Revision, fmt.Errorf("%s", RouteReasonAssignmentMissing)
	}
	return routeID, manifest.Revision, nil
}

// ResolveRouteSet returns a defensive copy of every currently assigned role.
// Partial sets are preserved for dashboard diagnosis and are not considered an error.
func (s *RouteAssignmentStore) ResolveRouteSet(character string, runID tasks.RunID) (map[pathing.RouteRole]string, uint64, error) {
	manifest, err := s.Snapshot()
	if err != nil {
		return nil, 0, err
	}
	roles := manifest.RouteSets[strings.ToLower(strings.TrimSpace(character))][runID]
	result := make(map[pathing.RouteRole]string, len(roles))
	for role, routeID := range roles {
		result[role] = routeID
	}
	return result, manifest.Revision, nil
}

// CommitRouteSetRole atomically replaces or removes exactly one declared role
// while retaining all other assignments and route-set roles. An empty routeID
// removes the role and is used only by archive/recovery management.
func (s *RouteAssignmentStore) CommitRouteSetRole(expected uint64, character string, runID tasks.RunID, role pathing.RouteRole, routeID string) (RouteAssignmentManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := s.loadLocked()
	if err != nil {
		return RouteAssignmentManifest{}, err
	}
	if manifest.Revision != expected {
		return RouteAssignmentManifest{}, fmt.Errorf("%s", RouteReasonAssignmentConflict)
	}
	character = strings.ToLower(strings.TrimSpace(character))
	definition, ok := tasks.DefaultRunRegistry().Definition(runID)
	if character == "" || !ok || definition.RouteSet == nil {
		return RouteAssignmentManifest{}, fmt.Errorf("route set role requires character and declared run")
	}
	if _, declared := definition.RouteSet.Recordings[role]; !declared {
		return RouteAssignmentManifest{}, fmt.Errorf("route set role %q is not declared for %s", role, runID)
	}
	if manifest.RouteSets[character] == nil {
		manifest.RouteSets[character] = map[tasks.RunID]map[pathing.RouteRole]string{}
	}
	if manifest.RouteSets[character][runID] == nil {
		manifest.RouteSets[character][runID] = map[pathing.RouteRole]string{}
	}
	if strings.TrimSpace(routeID) == "" {
		delete(manifest.RouteSets[character][runID], role)
		if len(manifest.RouteSets[character][runID]) == 0 {
			delete(manifest.RouteSets[character], runID)
		}
		if len(manifest.RouteSets[character]) == 0 {
			delete(manifest.RouteSets, character)
		}
	} else {
		manifest.RouteSets[character][runID][role] = routeID
	}
	manifest.Revision++
	if err := s.writeLocked(manifest); err != nil {
		return RouteAssignmentManifest{}, err
	}
	return cloneRouteAssignmentManifest(manifest), nil
}

// Commit replaces assignments only when the optimistic revision still matches.
func (s *RouteAssignmentStore) Commit(expected uint64, assignments map[string]map[tasks.RunID]string) (RouteAssignmentManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := s.loadLocked()
	if err != nil {
		return RouteAssignmentManifest{}, err
	}
	if manifest.Revision != expected {
		return RouteAssignmentManifest{}, fmt.Errorf("%s", RouteReasonAssignmentConflict)
	}
	manifest.Assignments = assignments
	manifest.Revision++
	if err := s.writeLocked(manifest); err != nil {
		return RouteAssignmentManifest{}, err
	}
	return cloneRouteAssignmentManifest(manifest), nil
}

func (s *RouteAssignmentStore) loadLocked() (RouteAssignmentManifest, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return RouteAssignmentManifest{}, err
	}
	var manifest RouteAssignmentManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return RouteAssignmentManifest{}, fmt.Errorf("decode route assignments: %w", err)
	}
	if manifest.SchemaVersion == 1 {
		manifest.SchemaVersion = RouteAssignmentSchemaVersion
		if manifest.Assignments == nil {
			manifest.Assignments = map[string]map[tasks.RunID]string{}
		}
		manifest.RouteSets = map[string]map[tasks.RunID]map[pathing.RouteRole]string{}
		if err := manifest.Validate(); err != nil {
			return RouteAssignmentManifest{}, err
		}
		if err := s.writeLocked(manifest); err != nil {
			return RouteAssignmentManifest{}, fmt.Errorf("migrate route assignments v1 to v2: %w", err)
		}
		return manifest, nil
	}
	if err := manifest.Validate(); err != nil {
		return RouteAssignmentManifest{}, err
	}
	return manifest, nil
}

func (s *RouteAssignmentStore) writeLocked(manifest RouteAssignmentManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode route assignments: %w", err)
	}
	return writeAtomicYAML(s.path, data, "route-assignments")
}

func writeAtomicYAML(path string, data []byte, prefix string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(directory, "."+prefix+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceFile(tmpPath, path)
}

func removeLegacyRouteIDs(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document yaml.Node
	if decodeErr := yaml.Unmarshal(data, &document); decodeErr != nil {
		return decodeErr
	}
	removeYAMLKeyRecursive(&document, "route_id", true)
	encoded, err := yaml.Marshal(&document)
	if err != nil {
		return err
	}
	return writeAtomicYAML(path, encoded, "config-migration")
}

func removeYAMLKeyRecursive(node *yaml.Node, key string, definitionsOnly bool) {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); {
			if node.Content[i].Value == key && !definitionsOnly {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				continue
			}
			childDefinitionsOnly := definitionsOnly
			if node.Content[i].Value == "definitions" {
				childDefinitionsOnly = false
			}
			removeYAMLKeyRecursive(node.Content[i+1], key, childDefinitionsOnly)
			i += 2
		}
	} else {
		for _, child := range node.Content {
			removeYAMLKeyRecursive(child, key, definitionsOnly)
		}
	}
}

func cloneRouteAssignmentManifest(manifest RouteAssignmentManifest) RouteAssignmentManifest {
	clone := manifest
	clone.Assignments = make(map[string]map[tasks.RunID]string, len(manifest.Assignments))
	for character, runs := range manifest.Assignments {
		clone.Assignments[character] = make(map[tasks.RunID]string, len(runs))
		for runID, routeID := range runs {
			clone.Assignments[character][runID] = routeID
		}
	}
	clone.RouteSets = make(map[string]map[tasks.RunID]map[pathing.RouteRole]string, len(manifest.RouteSets))
	for character, runs := range manifest.RouteSets {
		clone.RouteSets[character] = make(map[tasks.RunID]map[pathing.RouteRole]string, len(runs))
		for runID, roles := range runs {
			clone.RouteSets[character][runID] = make(map[pathing.RouteRole]string, len(roles))
			for role, routeID := range roles {
				clone.RouteSets[character][runID][role] = routeID
			}
		}
	}
	return clone
}
