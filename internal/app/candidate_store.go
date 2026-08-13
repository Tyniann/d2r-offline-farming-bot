package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

const candidateMetadataFilename = "candidate.yaml"

// CandidateStore owns immutable route candidates outside the Farming root.
type CandidateStore struct {
	mu   sync.Mutex
	root string
}

// NewCandidateStore creates the sole candidate persistence authority.
func NewCandidateStore(cfg *config.Config) (*CandidateStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("candidate store requires config")
	}
	root := filepath.Clean(cfg.ResolvePath(cfg.Routes.CandidateRoot))
	farming := filepath.Clean(cfg.ResolvePath(cfg.Routes.FarmingRoot))
	if root == farming || pathWithin(root, farming) || pathWithin(farming, root) {
		return nil, fmt.Errorf("candidate root must be disjoint from farming root")
	}
	return &CandidateStore{root: root}, nil
}

// Freeze atomically persists route content before any safety-return input and
// returns metadata bound to the immutable content hash.
func (s *CandidateStore) Freeze(route pathing.Route, candidate RouteCandidate) (RouteCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(candidate.CandidateID) == "" {
		id, err := newCandidateID()
		if err != nil {
			return RouteCandidate{}, err
		}
		candidate.CandidateID = id
	}
	if route.Binding.RouteRole != candidate.RouteRole {
		return RouteCandidate{}, fmt.Errorf("candidate route role mismatch")
	}
	directory := filepath.Join(s.root, candidate.CandidateID)
	if !pathWithin(directory, s.root) {
		return RouteCandidate{}, fmt.Errorf("candidate path escapes candidate root")
	}
	if _, err := os.Stat(directory); err == nil {
		return RouteCandidate{}, fmt.Errorf("candidate %q already exists", candidate.CandidateID)
	} else if !os.IsNotExist(err) {
		return RouteCandidate{}, err
	}
	routePath := filepath.Join(directory, "route.yaml")
	if err := pathing.SaveRoute(routePath, route); err != nil {
		return RouteCandidate{}, fmt.Errorf("freeze candidate route: %w", err)
	}
	hash, err := fileSHA256(routePath)
	if err != nil {
		return RouteCandidate{}, fmt.Errorf("hash candidate route: %w", err)
	}
	candidate.SchemaVersion = RouteCandidateSchemaVersion
	candidate.ImmutableRouteFile = "route.yaml"
	candidate.ImmutableRouteSHA256 = hash
	if candidate.State == "" {
		candidate.State = RouteCandidateRecorded
	}
	if candidate.CreatedAt.IsZero() {
		candidate.CreatedAt = time.Now().UTC()
	}
	if err := candidate.Validate(); err != nil {
		_ = os.RemoveAll(directory)
		return RouteCandidate{}, err
	}
	if err := s.writeMetadata(directory, candidate); err != nil {
		_ = os.RemoveAll(directory)
		return RouteCandidate{}, err
	}
	return candidate, nil
}

// Load verifies metadata and immutable content before returning a candidate.
func (s *CandidateStore) Load(candidateID string) (RouteCandidate, pathing.Route, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked(candidateID)
}

// UpdateState changes diagnostic workflow metadata only after rechecking content integrity.
func (s *CandidateStore) UpdateState(candidateID string, state RouteCandidateState, reason RouteReason, testedAt *time.Time) (RouteCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, _, err := s.loadLocked(candidateID)
	if err != nil {
		return RouteCandidate{}, err
	}
	candidate.State = state
	candidate.FailureReason = reason
	candidate.TestedAt = testedAt
	if err := candidate.Validate(); err != nil {
		return RouteCandidate{}, err
	}
	if err := s.writeMetadata(filepath.Join(s.root, candidateID), candidate); err != nil {
		return RouteCandidate{}, err
	}
	return candidate, nil
}

// Delete removes exactly one integrity-checked candidate when its state and
// immutable content still match the values bound by a management preview.
func (s *CandidateStore) Delete(candidateID string, expectedState RouteCandidateState, expectedSHA256 string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, _, err := s.loadLocked(candidateID)
	if err != nil {
		return err
	}
	if candidate.State != expectedState || candidate.ImmutableRouteSHA256 != expectedSHA256 {
		return fmt.Errorf("%s", RouteReasonCandidateChanged)
	}
	directory := filepath.Join(s.root, candidateID)
	if !pathWithin(directory, s.root) {
		return fmt.Errorf("candidate path escapes candidate root")
	}
	quarantine := directory + ".delete"
	if err := os.Rename(directory, quarantine); err != nil {
		return fmt.Errorf("quarantine candidate: %w", err)
	}
	if err := os.RemoveAll(quarantine); err != nil {
		_ = os.Rename(quarantine, directory)
		return fmt.Errorf("delete candidate: %w", err)
	}
	return nil
}

// List returns integrity-checked candidates in stable newest-first order.
func (s *CandidateStore) List() ([]RouteCandidate, error) {
	entries, err := os.ReadDir(s.root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]RouteCandidate, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate, _, loadErr := s.Load(entry.Name())
		if loadErr != nil {
			return nil, loadErr
		}
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result, nil
}

func (s *CandidateStore) loadLocked(candidateID string) (RouteCandidate, pathing.Route, error) {
	if strings.ContainsAny(candidateID, `/\\`) || strings.TrimSpace(candidateID) == "" {
		return RouteCandidate{}, pathing.Route{}, fmt.Errorf("invalid candidate ID")
	}
	directory := filepath.Join(s.root, candidateID)
	data, err := os.ReadFile(filepath.Join(directory, candidateMetadataFilename))
	if err != nil {
		return RouteCandidate{}, pathing.Route{}, err
	}
	var candidate RouteCandidate
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if decodeErr := decoder.Decode(&candidate); decodeErr != nil {
		return RouteCandidate{}, pathing.Route{}, decodeErr
	}
	if validationErr := candidate.Validate(); validationErr != nil {
		return RouteCandidate{}, pathing.Route{}, validationErr
	}
	if candidate.CandidateID != candidateID || candidate.ImmutableRouteFile != "route.yaml" {
		return RouteCandidate{}, pathing.Route{}, fmt.Errorf("candidate metadata path mismatch")
	}
	routePath := filepath.Join(directory, candidate.ImmutableRouteFile)
	hash, err := fileSHA256(routePath)
	if err != nil || hash != candidate.ImmutableRouteSHA256 {
		return RouteCandidate{}, pathing.Route{}, fmt.Errorf("%s", RouteReasonCandidateChanged)
	}
	route, err := pathing.LoadRoute(routePath)
	if err != nil {
		return RouteCandidate{}, pathing.Route{}, err
	}
	if route.Binding.RouteRole != candidate.RouteRole {
		return RouteCandidate{}, pathing.Route{}, fmt.Errorf("candidate route role mismatch")
	}
	return candidate, route, nil
}

func (s *CandidateStore) writeMetadata(directory string, candidate RouteCandidate) error {
	data, err := yaml.Marshal(candidate)
	if err != nil {
		return err
	}
	return writeAtomicYAML(filepath.Join(directory, candidateMetadataFilename), data, "candidate")
}

func newCandidateID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate candidate ID: %w", err)
	}
	return "candidate-" + hex.EncodeToString(random[:]), nil
}

func pathWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
