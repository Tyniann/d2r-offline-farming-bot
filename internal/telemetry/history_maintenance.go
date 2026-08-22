package telemetry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// HistoryMaintenanceDiagnostic ist eine pfadfreie Diagnose einer Retention- oder Löschoperation.
type HistoryMaintenanceDiagnostic struct {
	FileID string
	Code   HistoryReasonCode
}

// HistoryRetentionResult beschreibt genau einen automatischen Retentionlauf.
type HistoryRetentionResult struct {
	Ran          bool
	DeletedFiles int
	DeletedBytes int64
	Diagnostics  []HistoryMaintenanceDiagnostic
}

// HistoryDeletePreview bindet eine Komplettlöschung an Indexgeneration und Dateimetadaten.
type HistoryDeletePreview struct {
	Token          string
	Generation     uint64
	CandidateFiles int
	CandidateBytes int64
	ProtectedFiles int
	Categories     map[string]int
}

// HistoryDeleteConfirmation ist die zweite, exakt an eine Vorschau gebundene Bestätigung.
type HistoryDeleteConfirmation struct {
	Token          string
	Generation     uint64
	CandidateFiles int
	CandidateBytes int64
}

// HistoryDeleteResult beschreibt eine bestätigte Komplettlöschung ohne lokale Pfade.
type HistoryDeleteResult struct {
	DeletedFiles   int
	DeletedBytes   int64
	ProtectedFiles int
	Diagnostics    []HistoryMaintenanceDiagnostic
}

type historyDeleteFile struct {
	name       string
	size       int64
	modifiedAt time.Time
	category   string
}

type historyDeleteRecord struct {
	preview HistoryDeletePreview
	files   []historyDeleteFile
}

// HistoryMaintenanceService besitzt ausschließlich einen kanonischen Telemetrie-Root.
type HistoryMaintenanceService struct {
	mu       sync.Mutex
	root     string
	reader   *HistoryReader
	index    *HistoryIndex
	preview  *historyDeleteRecord
	lastAuto time.Time
	remove   func(string) error
	now      func() time.Time
}

// NewHistoryMaintenanceService erstellt den Retention-/Löschowner für einen existierenden direkten Root.
func NewHistoryMaintenanceService(root string, index *HistoryIndex) (*HistoryMaintenanceService, error) {
	if strings.TrimSpace(root) == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%s: history root must be absolute", HistoryReasonRetentionBlocked)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("%s: resolve history root: %w", HistoryReasonRetentionBlocked, err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: resolve canonical history root: %w", HistoryReasonRetentionBlocked, err)
		}
		canonical = filepath.Clean(absolute)
	}
	info, err := os.Lstat(canonical)
	if err != nil && !os.IsNotExist(err) || err == nil && !info.IsDir() {
		return nil, fmt.Errorf("%s: history root is not a directory", HistoryReasonRetentionBlocked)
	}
	if err == nil {
		reparse, reparseErr := historyPathIsReparse(canonical)
		if reparseErr != nil || reparse {
			return nil, fmt.Errorf("%s: history root is a reparse point", HistoryReasonRetentionBlocked)
		}
	}
	reader, err := NewHistoryReader(canonical)
	if err != nil {
		return nil, err
	}
	return &HistoryMaintenanceService{root: canonical, reader: reader, index: index, remove: os.Remove, now: time.Now}, nil
}

// Automatic löscht höchstens täglich ausschließlich vollständige alte terminale Session-Bundles.
func (s *HistoryMaintenanceService) Automatic(retentionDays int, idle bool, activeNames []string) (HistoryRetentionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if !idle {
		return HistoryRetentionResult{Diagnostics: []HistoryMaintenanceDiagnostic{maintenanceDiagnostic("", HistoryReasonRetentionBlocked)}}, nil
	}
	if retentionDays <= 0 {
		return HistoryRetentionResult{}, fmt.Errorf("%s: retention days must be positive", HistoryReasonRetentionBlocked)
	}
	if !s.lastAuto.IsZero() && now.Sub(s.lastAuto) < 24*time.Hour {
		return HistoryRetentionResult{}, nil
	}
	s.lastAuto = now
	files, diagnostics, err := s.scan()
	if err != nil {
		return HistoryRetentionResult{}, err
	}
	active := historyNameSet(activeNames)
	cutoff := now.AddDate(0, 0, -retentionDays)
	candidates, blocked := s.automaticCandidates(files, cutoff, active)
	diagnostics = append(diagnostics, blocked...)
	result := HistoryRetentionResult{Ran: true, Diagnostics: diagnostics}
	for _, candidate := range candidates {
		if active[candidate.name] {
			result.Diagnostics = append(result.Diagnostics, maintenanceDiagnostic(candidate.name, HistoryReasonDeleteActiveProtected))
			continue
		}
		if err := s.revalidate(candidate); err != nil {
			result.Diagnostics = append(result.Diagnostics, maintenanceDiagnostic(candidate.name, HistoryReasonRetentionPartial))
			continue
		}
		if err := s.remove(filepath.Join(s.root, candidate.name)); err != nil {
			result.Diagnostics = append(result.Diagnostics, maintenanceDiagnostic(candidate.name, HistoryReasonRetentionPartial))
			continue
		}
		result.DeletedFiles++
		result.DeletedBytes += candidate.size
	}
	if result.DeletedFiles > 0 && s.index != nil {
		if err := s.index.Refresh(); err != nil {
			return result, fmt.Errorf("refresh history after retention: %w", err)
		}
	}
	return result, nil
}

// PreviewDeleteAll erzeugt eine zufällige Einmalbestätigung für alle nicht aktiven direkten JSONL-Dateien.
func (s *HistoryMaintenanceService) PreviewDeleteAll(activeNames []string) (HistoryDeletePreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		if err := s.index.Refresh(); err != nil {
			return HistoryDeletePreview{}, fmt.Errorf("refresh history before delete preview: %w", err)
		}
	}
	files, _, err := s.scan()
	if err != nil {
		return HistoryDeletePreview{}, err
	}
	active := historyNameSet(activeNames)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return HistoryDeletePreview{}, fmt.Errorf("create history confirmation token: %w", err)
	}
	preview := HistoryDeletePreview{Token: hex.EncodeToString(tokenBytes), Categories: make(map[string]int)}
	if s.index != nil {
		preview.Generation = s.index.Snapshot("").Generation
	}
	var candidates []historyDeleteFile
	for _, file := range files {
		if active[file.name] {
			preview.ProtectedFiles++
			continue
		}
		candidates = append(candidates, file)
		preview.CandidateFiles++
		preview.CandidateBytes += file.size
		preview.Categories[file.category]++
	}
	s.preview = &historyDeleteRecord{preview: preview, files: candidates}
	return cloneHistoryDeletePreview(preview), nil
}

// ConfirmDeleteAll löscht eine Vorschau einmalig nach Generation-, Metadaten- und Active-Set-Recheck.
func (s *HistoryMaintenanceService) ConfirmDeleteAll(confirmation HistoryDeleteConfirmation, activeNames []string) (HistoryDeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.preview
	s.preview = nil
	if record == nil || confirmation.Token == "" || confirmation.Token != record.preview.Token ||
		confirmation.Generation != record.preview.Generation || confirmation.CandidateFiles != record.preview.CandidateFiles ||
		confirmation.CandidateBytes != record.preview.CandidateBytes {
		return HistoryDeleteResult{}, historyReadError(HistoryReasonDeletePreviewStale, "history delete preview does not match")
	}
	if s.index != nil {
		if err := s.index.Refresh(); err != nil {
			return HistoryDeleteResult{}, fmt.Errorf("refresh history before delete: %w", err)
		}
		if s.index.Snapshot("").Generation != confirmation.Generation {
			return HistoryDeleteResult{}, historyReadError(HistoryReasonDeletePreviewStale, "history generation changed")
		}
	}
	for _, file := range record.files {
		if err := s.revalidate(file); err != nil {
			return HistoryDeleteResult{}, historyReadError(HistoryReasonDeletePreviewStale, "history file metadata changed")
		}
	}
	active := historyNameSet(activeNames)
	result := HistoryDeleteResult{}
	for _, file := range record.files {
		if active[file.name] {
			result.ProtectedFiles++
			result.Diagnostics = append(result.Diagnostics, maintenanceDiagnostic(file.name, HistoryReasonDeleteActiveProtected))
			continue
		}
		if err := s.revalidate(file); err != nil {
			result.Diagnostics = append(result.Diagnostics, maintenanceDiagnostic(file.name, HistoryReasonDeleteFailed))
			continue
		}
		if err := s.remove(filepath.Join(s.root, file.name)); err != nil {
			result.Diagnostics = append(result.Diagnostics, maintenanceDiagnostic(file.name, HistoryReasonDeleteFailed))
			continue
		}
		result.DeletedFiles++
		result.DeletedBytes += file.size
	}
	if (result.DeletedFiles > 0 || len(result.Diagnostics) > 0) && s.index != nil {
		if err := s.index.Refresh(); err != nil {
			return result, fmt.Errorf("refresh history after delete: %w", err)
		}
	}
	if len(result.Diagnostics) > result.ProtectedFiles {
		return result, historyReadError(HistoryReasonDeleteFailed, "one or more history files remained")
	}
	return result, nil
}

func (s *HistoryMaintenanceService) scan() ([]historyDeleteFile, []HistoryMaintenanceDiagnostic, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []historyDeleteFile{}, nil, nil
		}
		return nil, nil, fmt.Errorf("read history maintenance root: %w", err)
	}
	var files []historyDeleteFile
	var diagnostics []HistoryMaintenanceDiagnostic
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".jsonl" {
			continue
		}
		path := filepath.Join(s.root, name)
		info, statErr := os.Lstat(path)
		reparse, reparseErr := historyPathIsReparse(path)
		if statErr != nil || reparseErr != nil || reparse || !info.Mode().IsRegular() {
			diagnostics = append(diagnostics, maintenanceDiagnostic(name, HistoryReasonRetentionBlocked))
			continue
		}
		category := "corrupt"
		if parsed, readErr := s.reader.Read(name); readErr == nil {
			switch {
			case parsed.Ignored:
				category = "legacy"
			case parsed.Stream == HistoryStreamSession:
				category = "schema3_session"
			case parsed.Stream == HistoryStreamRun:
				category = "schema3_run"
			}
		}
		files = append(files, historyDeleteFile{name: name, size: info.Size(), modifiedAt: info.ModTime().UTC(), category: category})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, diagnostics, nil
}

func (s *HistoryMaintenanceService) revalidate(file historyDeleteFile) error {
	if filepath.Base(file.name) != file.name || filepath.Ext(file.name) != ".jsonl" {
		return errors.New("invalid direct history file")
	}
	path := filepath.Join(s.root, file.name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.size || !info.ModTime().UTC().Equal(file.modifiedAt) {
		return errors.New("history file metadata changed")
	}
	reparse, err := historyPathIsReparse(path)
	if err != nil || reparse {
		return errors.New("history file is a reparse point")
	}
	return nil
}

func (s *HistoryMaintenanceService) automaticCandidates(files []historyDeleteFile, cutoff time.Time, active map[string]bool) ([]historyDeleteFile, []HistoryMaintenanceDiagnostic) {
	valid := make(map[string]HistoryFile)
	var diagnostics []HistoryMaintenanceDiagnostic
	for _, file := range files {
		parsed, err := s.reader.Read(file.name)
		if err != nil || parsed.Ignored {
			continue
		}
		valid[file.name] = parsed
	}
	runBySession := make(map[string][]HistoryFile)
	for _, file := range valid {
		if file.Stream == HistoryStreamRun {
			runBySession[file.SessionID] = append(runBySession[file.SessionID], file)
		}
	}
	var candidates []historyDeleteFile
	for _, session := range valid {
		if session.Stream != HistoryStreamSession {
			continue
		}
		names := []string{session.Name}
		referenced := make(map[string]bool)
		var sessionTerminal time.Time
		var newestRunTerminal time.Time
		complete := true
		for _, event := range session.Events {
			if event.Event == RunStarted {
				referenced[event.RunID] = true
			}
			if isRunTerminal(event.Event) && event.Timestamp.After(newestRunTerminal) {
				newestRunTerminal = event.Timestamp
			}
			if isSessionTerminal(event.Event) {
				sessionTerminal = event.Timestamp
			}
		}
		if sessionTerminal.IsZero() || newestRunTerminal.IsZero() || !sessionTerminal.Before(cutoff) || !newestRunTerminal.Before(cutoff) {
			complete = false
		}
		runs := runBySession[session.SessionID]
		if len(runs) != len(referenced) {
			complete = false
		}
		for _, run := range runs {
			if !referenced[run.RunID] || active[run.Name] {
				complete = false
			}
			terminals := 0
			for _, event := range run.Events {
				if isRunTerminal(event.Event) {
					terminals++
				}
			}
			if terminals != 1 {
				complete = false
			}
			names = append(names, run.Name)
		}
		if active[session.Name] {
			complete = false
		}
		if !complete {
			diagnostics = append(diagnostics, maintenanceDiagnostic(session.Name, HistoryReasonRetentionBlocked))
			continue
		}
		for _, name := range names {
			for _, file := range files {
				if file.name == name {
					candidates = append(candidates, file)
					break
				}
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })
	return candidates, diagnostics
}

func historyNameSet(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		if filepath.Base(name) == name && filepath.Ext(name) == ".jsonl" {
			out[name] = true
		}
	}
	return out
}

func maintenanceDiagnostic(name string, code HistoryReasonCode) HistoryMaintenanceDiagnostic {
	hash := sha256.Sum256([]byte(name))
	fileID := ""
	if name != "" {
		fileID = "history-" + hex.EncodeToString(hash[:4])
	}
	return HistoryMaintenanceDiagnostic{FileID: fileID, Code: code}
}

func cloneHistoryDeletePreview(value HistoryDeletePreview) HistoryDeletePreview {
	source := value.Categories
	value.Categories = make(map[string]int, len(value.Categories))
	for key, count := range source {
		value.Categories[key] = count
	}
	return value
}
