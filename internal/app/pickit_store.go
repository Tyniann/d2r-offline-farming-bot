package app

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"gopkg.in/yaml.v3"
)

// PickitProfileService besitzt die atomare YAML-Persistenz globaler Pickit-Profile.
type PickitProfileService struct {
	mu    *sync.Mutex
	root  string
	write func(string, []byte, string) error
}

// PickitAssignmentStore besitzt die einzige atomare Zuordnung pro Charakter und Run.
type PickitAssignmentStore struct {
	mu       *sync.Mutex
	path     string
	profiles *PickitProfileService
	read     func(string) ([]byte, error)
	write    func(string, []byte, string) error
}

// EffectivePickitPolicy ist die geordnete, bereits kompilierte Zuordnung eines Kontexts.
type EffectivePickitPolicy struct {
	Profiles           []string
	ProfileRevisions   map[string]uint64
	AssignmentRevision uint64
	All                *loot.Pickit
}

var pickitStoreLocks sync.Map

var (
	// ErrPickitProfileIDConflict reports that a profile ID already exists.
	ErrPickitProfileIDConflict = errors.New("pickit_profile_id_conflict")
	// ErrPickitProfileRevisionConflict reports a stale profile revision.
	ErrPickitProfileRevisionConflict = errors.New("pickit_profile_revision_conflict")
	// ErrPickitProfileAssigned reports that a referenced profile cannot be deleted.
	ErrPickitProfileAssigned = errors.New("pickit_profile_assigned")
	// ErrPickitAssignmentRevisionConflict reports a stale assignment revision.
	ErrPickitAssignmentRevisionConflict = errors.New("pickit_assignment_revision_conflict")
)

// NewPickitProfileService erstellt einen Profilservice für genau ein Verzeichnis.
func NewPickitProfileService(root string) (*PickitProfileService, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("pickit profile root is required")
	}
	clean := filepath.Clean(root)
	lock, _ := pickitStoreLocks.LoadOrStore("profiles\x00"+clean, &sync.Mutex{})
	return &PickitProfileService{mu: lock.(*sync.Mutex), root: clean, write: writeAtomicYAML}, nil
}

// NewPickitAssignmentStore erstellt einen Store mit Profilreferenzprüfung.
func NewPickitAssignmentStore(path string, profiles *PickitProfileService) (*PickitAssignmentStore, error) {
	if strings.TrimSpace(path) == "" || profiles == nil {
		return nil, fmt.Errorf("pickit assignment path and profile service are required")
	}
	clean := filepath.Clean(path)
	lock, _ := pickitStoreLocks.LoadOrStore("assignments\x00"+clean, &sync.Mutex{})
	return &PickitAssignmentStore{mu: lock.(*sync.Mutex), path: clean, profiles: profiles, read: os.ReadFile, write: writeAtomicYAML}, nil
}

// List lädt alle Profile in stabiler ID-Reihenfolge.
func (s *PickitProfileService) List() ([]PickitProfileDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("list pickit profiles: %w", err)
	}
	profiles := make([]PickitProfileDocument, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		profile, loadErr := s.loadPathLocked(filepath.Join(s.root, entry.Name()))
		if loadErr != nil {
			return nil, loadErr
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return profiles, nil
}

// Get lädt ein Profil und validiert Schema, Katalogreferenzen und Ausdruck vollständig.
func (s *PickitProfileService) Get(id string) (PickitProfileDocument, error) {
	if !pickitSlugPattern.MatchString(id) {
		return PickitProfileDocument{}, fmt.Errorf("pickit profile id %q is invalid", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadPathLocked(s.path(id))
}

// Create legt ein neues Profil mit Revision 1 an und überschreibt niemals eine bestehende ID.
func (s *PickitProfileService) Create(profile PickitProfileDocument) (PickitProfileDocument, error) {
	canonical, err := canonicalizePickitProfile(profile)
	if err != nil {
		return PickitProfileDocument{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if canonical.Revision != 1 {
		return PickitProfileDocument{}, fmt.Errorf("new pickit profile revision must be 1")
	}
	path := s.path(canonical.ID)
	if _, err := os.Stat(path); err == nil {
		return PickitProfileDocument{}, ErrPickitProfileIDConflict
	} else if !errors.Is(err, fs.ErrNotExist) {
		return PickitProfileDocument{}, err
	}
	return s.writeProfileLocked(canonical)
}

// Update ersetzt ein Profil nur bei passender Revision; die ID bleibt unveränderlich.
func (s *PickitProfileService) Update(id string, expectedRevision uint64, replacement PickitProfileDocument) (PickitProfileDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadPathLocked(s.path(id))
	if err != nil {
		return PickitProfileDocument{}, err
	}
	if replacement.ID != id {
		return PickitProfileDocument{}, fmt.Errorf("pickit profile id is immutable")
	}
	replacement.Revision = current.Revision
	canonical, err := canonicalizePickitProfile(replacement)
	if err != nil {
		return PickitProfileDocument{}, err
	}
	if samePickitProfileContent(current, canonical) {
		return current, nil
	}
	if current.Revision != expectedRevision {
		return PickitProfileDocument{}, ErrPickitProfileRevisionConflict
	}
	canonical.Revision = current.Revision + 1
	return s.writeProfileLocked(canonical)
}

// Duplicate kopiert Inhalt und Regel-IDs unter eine neue Profil-ID mit Revision 1.
func (s *PickitProfileService) Duplicate(sourceID, targetID, targetName string) (PickitProfileDocument, error) {
	source, err := s.Get(sourceID)
	if err != nil {
		return PickitProfileDocument{}, err
	}
	source.ID = targetID
	source.Name = targetName
	source.Revision = 1
	return s.Create(source)
}

// Delete entfernt ausschließlich ein unzugeordnetes Profil.
func (s *PickitProfileService) Delete(id string, assignments *PickitAssignmentStore) error {
	if !pickitSlugPattern.MatchString(id) {
		return fmt.Errorf("pickit profile id %q is invalid", id)
	}
	if assignments == nil {
		return fmt.Errorf("pickit assignments are required for delete")
	}
	referenced, err := assignments.References(id)
	if err != nil {
		return err
	}
	if referenced {
		return ErrPickitProfileAssigned
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(id)); err != nil {
		return fmt.Errorf("delete pickit profile %q: %w", id, err)
	}
	return nil
}

// Snapshot lädt die aktuelle Assignment-Autorität defensiv.
func (s *PickitAssignmentStore) Snapshot() (PickitAssignmentManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := s.loadLocked()
	if err != nil {
		return PickitAssignmentManifest{}, err
	}
	return clonePickitAssignments(manifest), nil
}

// Initialize schreibt eine noch nicht vorhandene Assignment-Autorität mit Revision 1.
func (s *PickitAssignmentStore) Initialize(assignments map[string]map[tasks.RunID][]string) (PickitAssignmentManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path); err == nil {
		return PickitAssignmentManifest{}, fmt.Errorf("pickit assignments already exist")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return PickitAssignmentManifest{}, err
	}
	manifest := PickitAssignmentManifest{SchemaVersion: PickitAssignmentSchemaVersion, Revision: 1, Assignments: assignments}
	return s.writeManifestLocked(manifest)
}

// Replace ersetzt eine geordnete Zuordnung bei passender globaler Revision.
func (s *PickitAssignmentStore) Replace(character string, runID tasks.RunID, profileIDs []string, expectedRevision uint64) (PickitAssignmentManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := s.loadLocked()
	if err != nil {
		return PickitAssignmentManifest{}, err
	}
	character = strings.TrimSpace(character)
	storedCharacter := character
	for candidate := range manifest.Assignments {
		if strings.EqualFold(candidate, character) {
			storedCharacter = candidate
			break
		}
	}
	if equalStrings(manifest.Assignments[storedCharacter][runID], profileIDs) {
		return clonePickitAssignments(manifest), nil
	}
	if manifest.Revision != expectedRevision {
		return PickitAssignmentManifest{}, ErrPickitAssignmentRevisionConflict
	}
	if manifest.Assignments[storedCharacter] == nil {
		manifest.Assignments[storedCharacter] = map[tasks.RunID][]string{}
	}
	manifest.Assignments[storedCharacter][runID] = append([]string(nil), profileIDs...)
	manifest.Revision++
	return s.writeManifestLocked(manifest)
}

// EnsureMissingDefaults ergänzt mehrere vollständig fehlende Run-Ketten in genau einer Revision und bewahrt jede vorhandene nicht leere Benutzerkette.
func (s *PickitAssignmentStore) EnsureMissingDefaults(character string, defaults map[tasks.RunID][]string, expectedRevision uint64) (PickitAssignmentManifest, error) {
	character = strings.TrimSpace(character)
	if character == "" || !offlineCharacterNamePattern.MatchString(character) {
		return PickitAssignmentManifest{}, fmt.Errorf("pickit assignment character %q is invalid", character)
	}
	for runID, profileIDs := range defaults {
		if _, ok := tasks.DefaultRunRegistry().Definition(runID); !ok {
			return PickitAssignmentManifest{}, fmt.Errorf("pickit default uses unknown run %q", runID)
		}
		if len(profileIDs) == 0 {
			return PickitAssignmentManifest{}, fmt.Errorf("pickit default %s requires at least one profile", runID)
		}
		seen := make(map[string]struct{}, len(profileIDs))
		for _, profileID := range profileIDs {
			if _, duplicate := seen[profileID]; duplicate {
				return PickitAssignmentManifest{}, fmt.Errorf("pickit default %s profile %q is duplicated", runID, profileID)
			}
			seen[profileID] = struct{}{}
			if _, err := s.profiles.Get(profileID); err != nil {
				return PickitAssignmentManifest{}, fmt.Errorf("pickit default %s profile %q is unavailable: %w", runID, profileID, err)
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := s.loadLocked()
	if err != nil {
		return PickitAssignmentManifest{}, err
	}
	storedCharacter := character
	for candidate := range manifest.Assignments {
		if strings.EqualFold(candidate, character) {
			storedCharacter = candidate
			break
		}
	}
	missing := make(map[tasks.RunID][]string)
	for runID, profileIDs := range defaults {
		if len(manifest.Assignments[storedCharacter][runID]) == 0 {
			missing[runID] = append([]string(nil), profileIDs...)
		}
	}
	if len(missing) == 0 {
		return clonePickitAssignments(manifest), nil
	}
	if manifest.Revision != expectedRevision {
		return PickitAssignmentManifest{}, ErrPickitAssignmentRevisionConflict
	}
	if manifest.Assignments[storedCharacter] == nil {
		manifest.Assignments[storedCharacter] = make(map[tasks.RunID][]string, len(missing))
	}
	for runID, profileIDs := range missing {
		manifest.Assignments[storedCharacter][runID] = profileIDs
	}
	manifest.Revision++
	return s.writeManifestLocked(manifest)
}

// References meldet, ob irgendeine Zuordnung die Profil-ID verwendet.
func (s *PickitAssignmentStore) References(profileID string) (bool, error) {
	manifest, err := s.Snapshot()
	if err != nil {
		return false, err
	}
	for _, runs := range manifest.Assignments {
		for _, profiles := range runs {
			for _, candidate := range profiles {
				if candidate == profileID {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// Resolve kompiliert die geordnete Assignment-Kette als einzige Action Policy.
func (s *PickitAssignmentStore) Resolve(character string, runID tasks.RunID) (EffectivePickitPolicy, error) {
	manifest, err := s.Snapshot()
	if err != nil {
		return EffectivePickitPolicy{}, err
	}
	profileIDs := findPickitAssignment(manifest, character, runID)
	if len(profileIDs) == 0 {
		return EffectivePickitPolicy{}, fmt.Errorf("pickit_assignment_missing")
	}
	allSpecs := []loot.PickitRuleSpec{}
	profileRevisions := make(map[string]uint64, len(profileIDs))
	for _, profileID := range profileIDs {
		profile, getErr := s.profiles.Get(profileID)
		if getErr != nil {
			return EffectivePickitPolicy{}, fmt.Errorf("pickit profile %q: %w", profileID, getErr)
		}
		profileRevisions[profile.ID] = profile.Revision
		for _, rule := range profile.Rules {
			spec := loot.PickitRuleSpec{ProfileID: profile.ID, RuleID: rule.ID, Action: rule.Action, Expression: rule.Expression, ProfileRevision: profile.Revision, AssignmentRevision: manifest.Revision}
			allSpecs = append(allSpecs, spec)
		}
	}
	all, err := loot.CompilePickitRules("pickit assignment", allSpecs)
	if err != nil {
		return EffectivePickitPolicy{}, err
	}
	return EffectivePickitPolicy{Profiles: append([]string(nil), profileIDs...), ProfileRevisions: profileRevisions, AssignmentRevision: manifest.Revision, All: all}, nil
}

func (s *PickitProfileService) path(id string) string { return filepath.Join(s.root, id+".yaml") }

func (s *PickitProfileService) loadPathLocked(path string) (PickitProfileDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PickitProfileDocument{}, fmt.Errorf("read pickit profile %q: %w", path, err)
	}
	var profile PickitProfileDocument
	if err := decodeStrictYAML(data, &profile); err != nil {
		return PickitProfileDocument{}, fmt.Errorf("decode pickit profile %q: %w", path, err)
	}
	if err := validateAndCompileProfile(profile); err != nil {
		return PickitProfileDocument{}, fmt.Errorf("invalid pickit profile %q: %w", path, err)
	}
	if filepath.Base(path) != profile.ID+".yaml" {
		return PickitProfileDocument{}, fmt.Errorf("pickit profile filename does not match id %q", profile.ID)
	}
	return clonePickitProfile(profile), nil
}

func (s *PickitProfileService) writeProfileLocked(profile PickitProfileDocument) (PickitProfileDocument, error) {
	canonical, err := canonicalizePickitProfile(profile)
	if err != nil {
		return PickitProfileDocument{}, err
	}
	data, err := yaml.Marshal(canonical)
	if err != nil {
		return PickitProfileDocument{}, fmt.Errorf("encode pickit profile: %w", err)
	}
	path := s.path(canonical.ID)
	if err := s.write(path, data, "pickit-profile"); err != nil {
		return PickitProfileDocument{}, fmt.Errorf("pickit_profile_write_failed: %w", err)
	}
	return s.loadPathLocked(path)
}

func (s *PickitAssignmentStore) loadLocked() (PickitAssignmentManifest, error) {
	data, err := s.read(s.path)
	if err != nil {
		return PickitAssignmentManifest{}, fmt.Errorf("read pickit assignments: %w", err)
	}
	var manifest PickitAssignmentManifest
	if err := decodeStrictYAML(data, &manifest); err != nil {
		return PickitAssignmentManifest{}, fmt.Errorf("decode pickit assignments: %w", err)
	}
	if err := s.validateManifest(manifest); err != nil {
		return PickitAssignmentManifest{}, err
	}
	return manifest, nil
}

func (s *PickitAssignmentStore) writeManifestLocked(manifest PickitAssignmentManifest) (PickitAssignmentManifest, error) {
	if err := s.validateManifest(manifest); err != nil {
		return PickitAssignmentManifest{}, err
	}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return PickitAssignmentManifest{}, fmt.Errorf("encode pickit assignments: %w", err)
	}
	if writeErr := s.write(s.path, data, "pickit-assignments"); writeErr != nil {
		return PickitAssignmentManifest{}, fmt.Errorf("pickit assignment write: %w", writeErr)
	}
	loaded, err := s.loadLocked()
	if err != nil {
		return PickitAssignmentManifest{}, fmt.Errorf("verify pickit assignments: %w", err)
	}
	return clonePickitAssignments(loaded), nil
}

func (s *PickitAssignmentStore) validateManifest(manifest PickitAssignmentManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	for _, runs := range manifest.Assignments {
		for _, profileIDs := range runs {
			for _, profileID := range profileIDs {
				if _, err := s.profiles.Get(profileID); err != nil {
					return fmt.Errorf("pickit assignment profile %q is unavailable: %w", profileID, err)
				}
			}
		}
	}
	return nil
}

func validateAndCompileProfile(profile PickitProfileDocument) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	specs := make([]loot.PickitRuleSpec, 0, len(profile.Rules))
	for _, rule := range profile.Rules {
		specs = append(specs, loot.PickitRuleSpec{ProfileID: profile.ID, RuleID: rule.ID, Action: rule.Action, Expression: rule.Expression})
	}
	_, err := loot.CompilePickitRules(profile.ID, specs)
	return err
}

func canonicalizePickitProfile(profile PickitProfileDocument) (PickitProfileDocument, error) {
	if err := profile.Validate(); err != nil {
		return PickitProfileDocument{}, err
	}
	profile = clonePickitProfile(profile)
	for index := range profile.Rules {
		canonical, err := loot.CanonicalPickitExpression(profile.Rules[index].Expression)
		if err != nil {
			return PickitProfileDocument{}, fmt.Errorf("pickit rule %q: %w", profile.Rules[index].ID, err)
		}
		profile.Rules[index].Expression = canonical
	}
	return profile, validateAndCompileProfile(profile)
}

func decodeStrictYAML(data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(target)
}

func findPickitAssignment(manifest PickitAssignmentManifest, character string, runID tasks.RunID) []string {
	for candidate, runs := range manifest.Assignments {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(character)) {
			return append([]string(nil), runs[runID]...)
		}
	}
	return nil
}

func clonePickitProfile(profile PickitProfileDocument) PickitProfileDocument {
	profile.Rules = append([]PickitProfileRuleDocument(nil), profile.Rules...)
	return profile
}

func clonePickitAssignments(manifest PickitAssignmentManifest) PickitAssignmentManifest {
	clone := manifest
	clone.Assignments = make(map[string]map[tasks.RunID][]string, len(manifest.Assignments))
	for character, runs := range manifest.Assignments {
		clone.Assignments[character] = make(map[tasks.RunID][]string, len(runs))
		for runID, profiles := range runs {
			clone.Assignments[character][runID] = append([]string(nil), profiles...)
		}
	}
	return clone
}

func samePickitProfileContent(left, right PickitProfileDocument) bool {
	if left.SchemaVersion != right.SchemaVersion || left.ID != right.ID || left.Name != right.Name || len(left.Rules) != len(right.Rules) {
		return false
	}
	for index := range left.Rules {
		if left.Rules[index] != right.Rules[index] {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
