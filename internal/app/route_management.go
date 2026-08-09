package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"gopkg.in/yaml.v3"
)

// RouteMutationPreview is an immutable, revision-bound management decision.
type RouteMutationPreview struct {
	Token              string
	Operation          RouteMutationOperation
	CandidateID        string
	CandidateSHA256    string
	RouteID            string
	PreviousRouteID    string
	Character          string
	RunID              tasks.RunID
	RouteRole          pathing.RouteRole
	CatalogRevision    uint64
	LifecycleRevision  uint64
	AssignmentRevision uint64
}

// RouteMutationConfirm supplies the opaque preview token and destructive ID acknowledgement.
type RouteMutationConfirm struct {
	Token          string
	ConfirmRouteID string
}

// RouteManagementHooks inject deterministic failures only at named transaction checkpoints.
type RouteManagementHooks struct{ AfterCheckpoint func(string) error }

// RouteManagementService coordinates candidate, lifecycle, assignment, and recovery authorities.
type RouteManagementService struct {
	mu          sync.Mutex
	cfg         *config.Config
	candidates  *CandidateStore
	lifecycle   *RouteLifecycleStore
	assignments *RouteAssignmentStore
	journalPath string
	previews    map[string]RouteMutationPreview
	hooks       RouteManagementHooks
}

// NewRouteManagementService creates a serialized management authority and recovers known journals.
func NewRouteManagementService(cfg *config.Config, hooks RouteManagementHooks) (*RouteManagementService, error) {
	candidates, err := NewCandidateStore(cfg)
	if err != nil {
		return nil, err
	}
	lifecycle, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		return nil, err
	}
	assignments, err := NewRouteAssignmentStore(cfg)
	if err != nil {
		return nil, err
	}
	service := &RouteManagementService{cfg: cfg, candidates: candidates, lifecycle: lifecycle, assignments: assignments, journalPath: cfg.ResolvePath(cfg.Routes.RecoveryFile), previews: map[string]RouteMutationPreview{}, hooks: hooks}
	if err := service.recover(); err != nil {
		return nil, err
	}
	return service, nil
}

// PreviewCandidate prepares publish or replace without mutating persistent state.
func (s *RouteManagementService) PreviewCandidate(candidateID string) (RouteMutationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, route, err := s.candidates.Load(candidateID)
	if err != nil {
		return RouteMutationPreview{}, err
	}
	if candidate.State != RouteCandidateTestPassed {
		return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonCandidateInvalid)
	}
	manifest, catalog, err := s.lifecycle.Snapshot()
	if err != nil {
		return RouteMutationPreview{}, err
	}
	assignments, err := s.assignments.Snapshot()
	if err != nil {
		return RouteMutationPreview{}, err
	}
	character := strings.ToLower(candidate.Character)
	previous := assignedRoute(assignments, character, candidate.RunID, candidate.RouteRole)
	if candidate.RouteRole == "" {
		if candidate.SourceCatalogRevision != catalog.Revision || candidate.SourceAssignmentRevision != assignments.Revision {
			return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonCandidateChanged)
		}
	} else if previous != candidate.SourceAssignedRouteID || route.Binding.RouteRole != candidate.RouteRole || !routeSetCompatibleWithCatalog(route, candidate.RunID, candidate.RouteRole, character, assignments, catalog) {
		return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonCandidateChanged)
	}
	operation := RouteMutationPublish
	if previous != "" {
		operation = RouteMutationReplace
	}
	routeID, err := generatedRoleRouteID(candidate.RunID, candidate.RouteRole, character)
	if err != nil {
		return RouteMutationPreview{}, err
	}
	preview, err := s.newPreview(RouteMutationPreview{Operation: operation, CandidateID: candidateID, CandidateSHA256: candidate.ImmutableRouteSHA256, RouteID: routeID, PreviousRouteID: previous, Character: character, RunID: candidate.RunID, RouteRole: candidate.RouteRole, CatalogRevision: catalog.Revision, LifecycleRevision: manifest.Revision, AssignmentRevision: assignments.Revision})
	return preview, err
}

// PreviewRoute prepares archive, restore, or delete against current revisions.
func (s *RouteManagementService) PreviewRoute(operation RouteMutationOperation, routeID string) (RouteMutationPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if operation != RouteMutationArchive && operation != RouteMutationRestore && operation != RouteMutationDelete {
		return RouteMutationPreview{}, fmt.Errorf("unsupported route operation")
	}
	manifest, catalog, err := s.lifecycle.Snapshot()
	if err != nil {
		return RouteMutationPreview{}, err
	}
	assignments, err := s.assignments.Snapshot()
	if err != nil {
		return RouteMutationPreview{}, err
	}
	entry, ok := catalogEntryByID(catalog, routeID)
	if !ok {
		return RouteMutationPreview{}, fmt.Errorf("route %q not found", routeID)
	}
	character := strings.ToLower(entry.Character)
	role := entry.Route.Binding.RouteRole
	assigned := assignedRoute(assignments, character, entry.RunID, role)
	if operation == RouteMutationArchive && entry.ManagementStatus != RouteManagementActive {
		return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonArchived)
	}
	if operation == RouteMutationRestore && entry.ManagementStatus != RouteManagementArchived {
		return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonRestoreIncompatible)
	}
	if operation == RouteMutationRestore && (!strings.EqualFold(entry.Route.Binding.CharacterName, character) || string(entry.Route.Binding.Difficulty) != strings.ToLower(s.cfg.Session.Difficulty)) {
		return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonRestoreIncompatible)
	}
	if operation == RouteMutationDelete && (entry.ManagementStatus != RouteManagementArchived || routeAssignedAnywhere(assignments, routeID)) {
		return RouteMutationPreview{}, fmt.Errorf("%s", RouteReasonDeleteAssigned)
	}
	preview, err := s.newPreview(RouteMutationPreview{Operation: operation, RouteID: routeID, PreviousRouteID: assigned, Character: character, RunID: entry.RunID, RouteRole: role, CatalogRevision: catalog.Revision, LifecycleRevision: manifest.Revision, AssignmentRevision: assignments.Revision})
	return preview, err
}

// Confirm consumes one preview exactly once and executes its recoverable transaction.
func (s *RouteManagementService) Confirm(confirm RouteMutationConfirm) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, ok := s.previews[confirm.Token]
	if !ok {
		return fmt.Errorf("management confirmation token invalid")
	}
	delete(s.previews, confirm.Token)
	manifest, catalog, err := s.lifecycle.Snapshot()
	if err != nil {
		return err
	}
	assignments, err := s.assignments.Snapshot()
	if err != nil {
		return err
	}
	if manifest.Revision != preview.LifecycleRevision || catalog.Revision != preview.CatalogRevision || assignments.Revision != preview.AssignmentRevision {
		return fmt.Errorf("%s", RouteReasonAssignmentConflict)
	}
	if preview.Operation == RouteMutationDelete && confirm.ConfirmRouteID != preview.RouteID {
		return fmt.Errorf("%s", RouteReasonDeleteConfirmationMismatch)
	}
	switch preview.Operation {
	case RouteMutationPublish, RouteMutationReplace:
		return s.confirmCandidate(preview, assignments)
	case RouteMutationArchive:
		return s.confirmArchive(preview, assignments)
	case RouteMutationRestore:
		return s.confirmRestore(preview, assignments)
	case RouteMutationDelete:
		return s.confirmDelete(preview)
	default:
		return fmt.Errorf("unsupported management operation")
	}
}

// PreviewForToken returns the immutable pending preview without consuming it.
// API adapters use this only to revalidate live character/difficulty context.
func (s *RouteManagementService) PreviewForToken(token string) (RouteMutationPreview, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, ok := s.previews[token]
	return preview, ok
}

func (s *RouteManagementService) confirmCandidate(preview RouteMutationPreview, assignments RouteAssignmentManifest) (retErr error) {
	candidate, route, err := s.candidates.Load(preview.CandidateID)
	if err != nil || candidate.ImmutableRouteSHA256 != preview.CandidateSHA256 || candidate.RouteRole != preview.RouteRole || route.Binding.RouteRole != preview.RouteRole {
		return fmt.Errorf("%s", RouteReasonCandidateChanged)
	}
	route.ID, route.Name = preview.RouteID, managedRouteDisplayName(preview.RouteID)
	targetDir, err := farmingRouteDirectory(s.cfg, candidate.Character, candidate.Difficulty)
	if err != nil {
		return err
	}
	target := filepath.Join(targetDir, preview.RouteID+".yaml")
	journal := RouteRecoveryJournal{SchemaVersion: RouteRecoveryJournalSchemaVersion, Operation: preview.Operation, RouteID: preview.RouteID, CandidateID: preview.CandidateID, PreviousRouteID: preview.PreviousRouteID, Character: preview.Character, RunID: preview.RunID, RouteRole: preview.RouteRole, Checkpoint: "before_route_publish", StartedAt: time.Now().UTC()}
	if journalErr := s.writeJournal(journal); journalErr != nil {
		return journalErr
	}
	if checkpointErr := s.checkpoint(journal); checkpointErr != nil {
		_ = s.clearJournal()
		return checkpointErr
	}
	defer func() {
		if retErr != nil {
			_ = s.rollbackCandidate(preview, target)
		}
	}()
	if saveErr := pathing.SaveRoute(target, route); saveErr != nil {
		return saveErr
	}
	journal.Checkpoint = "after_route_publish"
	if checkpointErr := s.checkpoint(journal); checkpointErr != nil {
		return checkpointErr
	}
	lifecycle, err := s.lifecycle.RecordRoute(target)
	if err != nil {
		return err
	}
	if preview.PreviousRouteID != "" {
		lifecycle, err = s.lifecycle.SetManagement(preview.PreviousRouteID, RouteManagementArchived, preview.RunID, lifecycle.Revision)
		if err != nil {
			return err
		}
		journal.Checkpoint = "after_old_archive_prepare"
		if err := s.checkpoint(journal); err != nil {
			return err
		}
	}
	if _, err := commitAssignedRoute(s.assignments, assignments, preview.Character, preview.RunID, preview.RouteRole, preview.RouteID); err != nil {
		return err
	}
	journal.Checkpoint = "after_assignment_commit"
	if err := s.checkpoint(journal); err != nil {
		return err
	}
	return s.clearJournal()
}

func (s *RouteManagementService) confirmArchive(preview RouteMutationPreview, assignments RouteAssignmentManifest) (retErr error) {
	journal := RouteRecoveryJournal{SchemaVersion: 1, Operation: RouteMutationArchive, RouteID: preview.RouteID, PreviousRouteID: preview.PreviousRouteID, Character: preview.Character, RunID: preview.RunID, RouteRole: preview.RouteRole, Checkpoint: "before_assignment_remove", StartedAt: time.Now().UTC()}
	if err := s.writeJournal(journal); err != nil {
		return err
	}
	if err := s.checkpoint(journal); err != nil {
		_ = s.clearJournal()
		return err
	}
	defer func() {
		if retErr != nil {
			_ = s.restoreManagement(preview.RouteID, preview.RunID, RouteManagementActive)
			if preview.PreviousRouteID == preview.RouteID {
				current, snapshotErr := s.assignments.Snapshot()
				if snapshotErr == nil && assignedRoute(current, preview.Character, preview.RunID, preview.RouteRole) == "" {
					_, _ = commitAssignedRoute(s.assignments, current, preview.Character, preview.RunID, preview.RouteRole, preview.RouteID)
				}
			}
		}
	}()
	if _, err := s.lifecycle.SetManagement(preview.RouteID, RouteManagementArchived, preview.RunID, preview.LifecycleRevision); err != nil {
		return err
	}
	if preview.PreviousRouteID == preview.RouteID {
		if _, err := commitAssignedRoute(s.assignments, assignments, preview.Character, preview.RunID, preview.RouteRole, ""); err != nil {
			return err
		}
	}
	journal.Checkpoint = "after_assignment_remove"
	if err := s.checkpoint(journal); err != nil {
		return err
	}
	return s.clearJournal()
}

func (s *RouteManagementService) confirmRestore(preview RouteMutationPreview, assignments RouteAssignmentManifest) (retErr error) {
	journal := RouteRecoveryJournal{SchemaVersion: 1, Operation: RouteMutationRestore, RouteID: preview.RouteID, PreviousRouteID: preview.PreviousRouteID, Character: preview.Character, RunID: preview.RunID, RouteRole: preview.RouteRole, Checkpoint: "after_current_archive_prepare", StartedAt: time.Now().UTC()}
	if err := s.writeJournal(journal); err != nil {
		return err
	}
	if err := s.checkpoint(journal); err != nil {
		_ = s.clearJournal()
		return err
	}
	defer func() {
		if retErr != nil {
			_ = s.restoreManagement(preview.RouteID, preview.RunID, RouteManagementArchived)
			if preview.PreviousRouteID == "" {
				return
			}
			_ = s.restoreManagement(preview.PreviousRouteID, preview.RunID, RouteManagementActive)
			current, snapshotErr := s.assignments.Snapshot()
			if snapshotErr == nil && assignedRoute(current, preview.Character, preview.RunID, preview.RouteRole) != preview.PreviousRouteID {
				_, _ = commitAssignedRoute(s.assignments, current, preview.Character, preview.RunID, preview.RouteRole, preview.PreviousRouteID)
			}
		}
	}()
	lifecycleRevision := preview.LifecycleRevision
	var err error
	if preview.PreviousRouteID != "" && preview.PreviousRouteID != preview.RouteID {
		manifest, archiveErr := s.lifecycle.SetManagement(preview.PreviousRouteID, RouteManagementArchived, preview.RunID, lifecycleRevision)
		if archiveErr != nil {
			return archiveErr
		}
		lifecycleRevision = manifest.Revision
	}
	if _, err = s.lifecycle.SetManagement(preview.RouteID, RouteManagementActive, preview.RunID, lifecycleRevision); err != nil {
		return err
	}
	if _, err := commitAssignedRoute(s.assignments, assignments, preview.Character, preview.RunID, preview.RouteRole, preview.RouteID); err != nil {
		return err
	}
	journal.Checkpoint = "after_assignment_commit"
	if err := s.checkpoint(journal); err != nil {
		return err
	}
	return s.clearJournal()
}

func (s *RouteManagementService) confirmDelete(preview RouteMutationPreview) (retErr error) {
	_, catalog, err := s.lifecycle.Snapshot()
	if err != nil {
		return err
	}
	entry, ok := catalogEntryByID(catalog, preview.RouteID)
	if !ok {
		return fmt.Errorf("route not found")
	}
	quarantine := entry.Path + ".quarantine"
	journal := RouteRecoveryJournal{SchemaVersion: 1, Operation: RouteMutationDelete, RouteID: preview.RouteID, Character: preview.Character, RunID: preview.RunID, RoutePath: entry.Path, Checkpoint: "after_quarantine_rename", StartedAt: time.Now().UTC()}
	if err := s.writeJournal(journal); err != nil {
		return err
	}
	if err := os.Rename(entry.Path, quarantine); err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			_ = os.Rename(quarantine, entry.Path)
			if _, loadErr := pathing.LoadRoute(entry.Path); loadErr == nil {
				_, _ = s.lifecycle.RecordRoute(entry.Path)
				_ = s.restoreManagement(preview.RouteID, preview.RunID, RouteManagementArchived)
			}
		}
	}()
	if err := s.checkpoint(journal); err != nil {
		return err
	}
	if _, err := s.lifecycle.RemoveRoute(preview.RouteID, preview.LifecycleRevision); err != nil {
		return err
	}
	journal.Checkpoint = "after_manifest_commit"
	if err := s.checkpoint(journal); err != nil {
		return err
	}
	if err := os.Remove(quarantine); err != nil {
		return err
	}
	return s.clearJournal()
}

func (s *RouteManagementService) rollbackCandidate(preview RouteMutationPreview, target string) error {
	assignments, assignmentErr := s.assignments.Snapshot()
	if assignmentErr == nil && assignedRoute(assignments, preview.Character, preview.RunID, preview.RouteRole) == preview.RouteID {
		_, _ = commitAssignedRoute(s.assignments, assignments, preview.Character, preview.RunID, preview.RouteRole, preview.PreviousRouteID)
	}
	_ = os.Remove(target)
	manifest, _, err := s.lifecycle.Snapshot()
	if err == nil {
		if _, removeErr := s.lifecycle.RemoveRoute(preview.RouteID, manifest.Revision); removeErr != nil && !strings.Contains(removeErr.Error(), "not found") {
			err = removeErr
		}
	}
	if preview.PreviousRouteID != "" {
		_ = s.restoreManagement(preview.PreviousRouteID, preview.RunID, RouteManagementActive)
	}
	_ = s.clearJournal()
	return err
}

func (s *RouteManagementService) restoreManagement(routeID string, runID tasks.RunID, status RouteManagementStatus) error {
	manifest, _, err := s.lifecycle.Snapshot()
	if err != nil {
		return err
	}
	_, err = s.lifecycle.SetManagement(routeID, status, runID, manifest.Revision)
	return err
}

func (s *RouteManagementService) newPreview(preview RouteMutationPreview) (RouteMutationPreview, error) {
	token, err := randomToken(24)
	if err != nil {
		return RouteMutationPreview{}, err
	}
	preview.Token = token
	s.previews[token] = preview
	return preview, nil
}

func (s *RouteManagementService) checkpoint(journal RouteRecoveryJournal) error {
	if err := s.writeJournal(journal); err != nil {
		return err
	}
	if s.hooks.AfterCheckpoint != nil {
		return s.hooks.AfterCheckpoint(journal.Checkpoint)
	}
	return nil
}

func (s *RouteManagementService) writeJournal(journal RouteRecoveryJournal) error {
	data, err := yaml.Marshal(journal)
	if err != nil {
		return err
	}
	return writeAtomicYAML(s.journalPath, data, "route-recovery")
}
func (s *RouteManagementService) clearJournal() error {
	if err := os.Remove(s.journalPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *RouteManagementService) recover() error {
	data, err := os.ReadFile(s.journalPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var journal RouteRecoveryJournal
	if err := yaml.Unmarshal(data, &journal); err != nil {
		return fmt.Errorf("%s: %w", RouteReasonTransactionRecoveryRequired, err)
	}
	if journal.SchemaVersion != RouteRecoveryJournalSchemaVersion {
		return fmt.Errorf("%s", RouteReasonTransactionRecoveryRequired)
	}
	if !knownRecoveryCheckpoint(journal.Operation, journal.Checkpoint) {
		return fmt.Errorf("%s", RouteReasonTransactionRecoveryRequired)
	}
	assignments, assignmentErr := s.assignments.Snapshot()
	if assignmentErr != nil {
		return assignmentErr
	}
	if journal.Operation == RouteMutationDelete {
		quarantine := journal.RoutePath + ".quarantine"
		if journal.Checkpoint == "after_manifest_commit" {
			_ = os.Remove(quarantine)
		} else if _, statErr := os.Stat(quarantine); statErr == nil {
			_ = os.Rename(quarantine, journal.RoutePath)
		}
		return s.clearJournal()
	}
	if journal.Operation == RouteMutationArchive {
		if assignedRoute(assignments, journal.Character, journal.RunID, journal.RouteRole) == "" {
			_, _ = commitAssignedRoute(s.assignments, assignments, journal.Character, journal.RunID, journal.RouteRole, journal.RouteID)
		}
		_ = s.restoreManagement(journal.RouteID, journal.RunID, RouteManagementActive)
		return s.clearJournal()
	}
	if journal.Operation == RouteMutationRestore {
		if assignedRoute(assignments, journal.Character, journal.RunID, journal.RouteRole) == journal.RouteID && journal.Checkpoint == "after_assignment_commit" {
			return s.clearJournal()
		}
		if journal.PreviousRouteID != "" {
			_ = s.restoreManagement(journal.PreviousRouteID, journal.RunID, RouteManagementActive)
		}
		_ = s.restoreManagement(journal.RouteID, journal.RunID, RouteManagementArchived)
		return s.clearJournal()
	}
	if assignedRoute(assignments, journal.Character, journal.RunID, journal.RouteRole) == journal.RouteID && journal.Checkpoint == "after_assignment_commit" {
		return s.clearJournal()
	}
	if journal.RouteID != "" {
		root := s.cfg.ResolvePath(s.cfg.Routes.FarmingRoot)
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr == nil && !entry.IsDir() && filepath.Base(path) == journal.RouteID+".yaml" {
				_ = os.Remove(path)
			}
			return nil
		})
		manifest, _, snapErr := s.lifecycle.Snapshot()
		if snapErr == nil {
			_, _ = s.lifecycle.RemoveRoute(journal.RouteID, manifest.Revision)
		}
	}
	if journal.PreviousRouteID != "" {
		_ = s.restoreManagement(journal.PreviousRouteID, journal.RunID, RouteManagementActive)
	}
	return s.clearJournal()
}

func catalogEntryByID(catalog FarmingRouteCatalog, routeID string) (FarmingRouteCatalogEntry, bool) {
	for _, entry := range catalog.Entries {
		if entry.ID == routeID {
			return entry, true
		}
	}
	return FarmingRouteCatalogEntry{}, false
}

func routeAssignedAnywhere(manifest RouteAssignmentManifest, routeID string) bool {
	for _, runs := range manifest.Assignments {
		for _, assigned := range runs {
			if assigned == routeID {
				return true
			}
		}
	}
	for _, runs := range manifest.RouteSets {
		for _, roles := range runs {
			for _, assigned := range roles {
				if assigned == routeID {
					return true
				}
			}
		}
	}
	return false
}

func assignedRoute(manifest RouteAssignmentManifest, character string, runID tasks.RunID, role pathing.RouteRole) string {
	if role != "" {
		return manifest.RouteSets[character][runID][role]
	}
	return manifest.Assignments[character][runID]
}

func commitAssignedRoute(store *RouteAssignmentStore, manifest RouteAssignmentManifest, character string, runID tasks.RunID, role pathing.RouteRole, routeID string) (RouteAssignmentManifest, error) {
	if role != "" {
		return store.CommitRouteSetRole(manifest.Revision, character, runID, role, routeID)
	}
	next := cloneRouteAssignmentManifest(manifest).Assignments
	if next[character] == nil {
		next[character] = map[tasks.RunID]string{}
	}
	if routeID == "" {
		delete(next[character], runID)
	} else {
		next[character][runID] = routeID
	}
	return store.Commit(manifest.Revision, next)
}

func routeSetCompatibleWithCatalog(route pathing.Route, runID tasks.RunID, role pathing.RouteRole, character string, assignments RouteAssignmentManifest, catalog FarmingRouteCatalog) bool {
	for siblingRole, routeID := range assignments.RouteSets[character][runID] {
		if siblingRole == role || routeID == "" {
			continue
		}
		entry, ok := catalogEntryByID(catalog, routeID)
		if !ok || entry.ManagementStatus != RouteManagementActive || !sharedRouteSetIdentity(route.Binding, entry.Route.Binding) {
			return false
		}
	}
	return true
}

func generatedRoleRouteID(runID tasks.RunID, role pathing.RouteRole, character string) (string, error) {
	if role == "" {
		return generatedRouteID(runID, character)
	}
	return generatedRouteID(tasks.RunID(string(runID)+"-"+string(role)), character)
}

func knownRecoveryCheckpoint(operation RouteMutationOperation, checkpoint string) bool {
	allowed := map[RouteMutationOperation]map[string]bool{
		RouteMutationPublish: {"before_route_publish": true, "after_route_publish": true, "after_assignment_commit": true},
		RouteMutationReplace: {"before_route_publish": true, "after_route_publish": true, "after_old_archive_prepare": true, "after_assignment_commit": true},
		RouteMutationArchive: {"before_assignment_remove": true, "after_assignment_remove": true},
		RouteMutationRestore: {"after_current_archive_prepare": true, "after_assignment_commit": true},
		RouteMutationDelete:  {"after_quarantine_rename": true, "after_manifest_commit": true},
	}
	return allowed[operation][checkpoint]
}
func generatedRouteID(runID tasks.RunID, character string) (string, error) {
	suffix, err := randomToken(5)
	if err != nil {
		return "", err
	}
	base := strings.ToLower(string(runID) + "-" + character)
	base = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, base)
	base = strings.Trim(base, "-")
	if len(base) > 45 {
		base = base[:45]
	}
	return base + "-" + suffix, nil
}
func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func managedRouteDisplayName(id string) string {
	words := strings.Fields(strings.ReplaceAll(id, "-", " "))
	for index := range words {
		words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
	}
	return strings.Join(words, " ")
}
