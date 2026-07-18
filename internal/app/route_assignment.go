package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
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
	manifest = RouteAssignmentManifest{SchemaVersion: RouteAssignmentSchemaVersion, Revision: 1, Assignments: map[string]map[tasks.RunID]string{}}
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
	return clone
}
