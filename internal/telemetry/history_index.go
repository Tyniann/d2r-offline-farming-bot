package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// HistoryRun ist eine vollständig korrelierte, immutable Run-Sicht.
type HistoryRun struct {
	RunID                  string
	SessionID              string
	GameID                 string
	Mode                   HistoryMode
	Character              string
	Difficulty             string
	GameVersion            string
	Run                    string
	DefinitionID           string
	RouteID                string
	RouteLayoutFingerprint string
	QueueIndex             int
	QueueCycle             int
	StartedAt              time.Time
	ObservedAt             time.Time
	EndedAt                *time.Time
	Outcome                HistoryOutcome
	Reason                 string
	Events                 []Event
	RunFile                string
	SessionFile            string
}

// HistorySnapshot ist eine defensive, stabil sortierte Sicht auf einen Indexstand.
type HistorySnapshot struct {
	Generation   uint64
	UpdatedAt    time.Time
	Runs         []HistoryRun
	Diagnostics  []HistoryFileDiagnostic
	IgnoredFiles int
}

// HistoryIndex hält ausschließlich rebuildbare In-Memory-Projektionen aus JSONL.
type HistoryIndex struct {
	mu          sync.RWMutex
	reader      *HistoryReader
	files       map[string]HistoryFile
	runs        []HistoryRun
	diagnostics []HistoryFileDiagnostic
	ignored     int
	generation  uint64
	updatedAt   time.Time
	signature   string
}

// NewHistoryIndex erstellt einen leeren Index ohne Hintergrunddienst oder Watcher.
func NewHistoryIndex(directory string) (*HistoryIndex, error) {
	reader, err := NewHistoryReader(directory)
	if err != nil {
		return nil, err
	}
	return &HistoryIndex{reader: reader, files: make(map[string]HistoryFile)}, nil
}

// Refresh scannt direkte JSONL-Dateien und ersetzt den Index atomar.
func (i *HistoryIndex) Refresh() error {
	if i == nil || i.reader == nil {
		return fmt.Errorf("%s: history index is unavailable", HistoryReasonUnavailable)
	}
	entries, err := os.ReadDir(i.reader.directory)
	if err != nil {
		if os.IsNotExist(err) {
			entries = nil
		} else {
			return fmt.Errorf("read history directory: %w", err)
		}
	}
	i.mu.RLock()
	previous := make(map[string]HistoryFile, len(i.files))
	for name, file := range i.files {
		previous[name] = cloneHistoryFile(file)
	}
	i.mu.RUnlock()

	files := make(map[string]HistoryFile)
	diagnostics := make([]HistoryFileDiagnostic, 0)
	ignored := 0
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".jsonl" {
			continue
		}
		source, readErr := i.reader.readSource(name)
		if readErr != nil {
			if readErr == errHistoryFileChanging {
				if stable, ok := previous[name]; ok {
					files[name] = stable
					if stable.Ignored {
						ignored++
					}
				}
				continue
			}
			code := historyErrorCode(readErr)
			message, _ := HistoryReasonMessage(code)
			diagnostics = append(diagnostics, HistoryFileDiagnostic{File: name, Code: code, Message: message})
			continue
		}
		if stable, ok := previous[name]; ok && stable.Fingerprint == source.fingerprint {
			stable.Size, stable.ModifiedAt = source.size, source.modifiedAt
			files[name] = stable
			if stable.Ignored {
				ignored++
			}
			continue
		}
		file, readErr := parseHistorySource(name, source)
		if readErr != nil {
			code := historyErrorCode(readErr)
			message, _ := HistoryReasonMessage(code)
			diagnostics = append(diagnostics, HistoryFileDiagnostic{File: name, Code: code, Message: message})
			continue
		}
		if file.Ignored {
			ignored++
		}
		files[name] = file
	}
	runs, correlationDiagnostics := correlateHistoryFiles(files)
	diagnostics = append(diagnostics, correlationDiagnostics...)
	sort.Slice(diagnostics, func(a, b int) bool {
		if diagnostics[a].File == diagnostics[b].File {
			return diagnostics[a].Code < diagnostics[b].Code
		}
		return diagnostics[a].File < diagnostics[b].File
	})
	signature := historyIndexSignature(files, diagnostics, ignored)
	now := time.Now().UTC()
	i.mu.Lock()
	defer i.mu.Unlock()
	i.files, i.runs, i.diagnostics, i.ignored = files, runs, diagnostics, ignored
	if signature != i.signature {
		i.generation++
		i.signature = signature
		i.updatedAt = now
	}
	return nil
}

// Rebuild verwirft ausschließlich den flüchtigen Cache und rekonstruiert aus JSONL.
func (i *HistoryIndex) Rebuild() error {
	if i == nil {
		return fmt.Errorf("%s: history index is unavailable", HistoryReasonUnavailable)
	}
	i.mu.Lock()
	i.files = make(map[string]HistoryFile)
	i.runs = nil
	i.diagnostics = nil
	i.ignored = 0
	i.signature = ""
	i.mu.Unlock()
	return i.Refresh()
}

// Snapshot liefert defensive Daten; nur die bestätigte aktive Core-Run-ID wird als laufend markiert.
func (i *HistoryIndex) Snapshot(activeRunID string) HistorySnapshot {
	if i == nil {
		return HistorySnapshot{}
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	snapshot := HistorySnapshot{Generation: i.generation, UpdatedAt: i.updatedAt, IgnoredFiles: i.ignored}
	snapshot.Diagnostics = append([]HistoryFileDiagnostic(nil), i.diagnostics...)
	snapshot.Runs = make([]HistoryRun, len(i.runs))
	for index, run := range i.runs {
		snapshot.Runs[index] = cloneHistoryRun(run)
		if run.Outcome == HistoryOutcomeIncomplete && run.RunID == activeRunID {
			snapshot.Runs[index].Outcome = HistoryOutcomeRunning
		}
	}
	return snapshot
}

type sessionRunBoundary struct {
	file     string
	started  Event
	terminal *Event
}

func correlateHistoryFiles(files map[string]HistoryFile) ([]HistoryRun, []HistoryFileDiagnostic) {
	boundaries := make(map[string]sessionRunBoundary)
	runFiles := make(map[string]HistoryFile)
	diagnostics := make([]HistoryFileDiagnostic, 0)
	invalidSessionFiles := make(map[string]bool)
	for _, file := range sortedHistoryFiles(files) {
		switch file.Stream {
		case HistoryStreamSession:
			for _, event := range file.Events {
				if event.Event == RunStarted {
					if previous, duplicate := boundaries[event.RunID]; duplicate {
						invalidSessionFiles[file.Name], invalidSessionFiles[previous.file] = true, true
						continue
					}
					boundaries[event.RunID] = sessionRunBoundary{file: file.Name, started: cloneHistoryEvent(event)}
				} else if isRunTerminal(event.Event) {
					boundary, ok := boundaries[event.RunID]
					if !ok || boundary.file != file.Name || boundary.terminal != nil {
						invalidSessionFiles[file.Name] = true
						continue
					}
					terminal := cloneHistoryEvent(event)
					boundary.terminal = &terminal
					boundaries[event.RunID] = boundary
				}
			}
		case HistoryStreamRun:
			runFiles[file.RunID] = file
		}
	}
	for file := range invalidSessionFiles {
		diagnostics = append(diagnostics, historyDiagnostic(file, HistoryReasonTerminalDuplicate))
	}
	runs := make([]HistoryRun, 0, len(runFiles))
	for runID, runFile := range runFiles {
		first := runFile.Events[0]
		if first.Mode != HistoryModeProductiveFarming {
			continue
		}
		boundary, ok := boundaries[runID]
		if !ok || invalidSessionFiles[boundary.file] {
			diagnostics = append(diagnostics, historyDiagnostic(runFile.Name, HistoryReasonStreamMissing))
			continue
		}
		started := boundary.started
		if started.SessionID != first.SessionID || started.GameID != first.GameID || started.RunID != first.RunID || started.Run != first.Run || started.Mode != first.Mode || started.Character != first.Character || started.Difficulty != first.Difficulty || started.GameVersion != first.GameVersion || started.QueueIndex == nil || first.QueueIndex == nil || *started.QueueIndex != *first.QueueIndex || started.QueueCycle == nil || first.QueueCycle == nil || *started.QueueCycle != *first.QueueCycle {
			diagnostics = append(diagnostics, historyDiagnostic(runFile.Name, HistoryReasonRunIDMismatch))
			continue
		}
		outcome, reason := HistoryOutcomeIncomplete, ""
		var endedAt *time.Time
		if boundary.terminal != nil {
			terminal := boundary.terminal
			if terminal.GameID != started.GameID || terminal.Run != started.Run || terminal.Timestamp.Before(started.Timestamp) {
				diagnostics = append(diagnostics, historyDiagnostic(runFile.Name, HistoryReasonRunIDMismatch))
				continue
			}
			switch terminal.Event {
			case RunCompleted:
				outcome = HistoryOutcomeSuccess
			case RunFailed:
				outcome = HistoryOutcomeFailed
			case RunAborted:
				outcome = HistoryOutcomeAborted
			}
			reason = terminal.Reason
			ended := terminal.Timestamp.UTC()
			endedAt = &ended
		}
		for _, event := range runFile.Events {
			if event.Timestamp.Before(started.Timestamp) || endedAt != nil && event.Timestamp.After(*endedAt) {
				diagnostics = append(diagnostics, historyDiagnostic(runFile.Name, HistoryReasonTimeInvalid))
				endedAt = nil
				outcome = ""
				break
			}
		}
		if outcome == "" {
			continue
		}
		observed := started.Timestamp.UTC()
		events := make([]Event, 0, len(runFile.Events)+2)
		events = append(events, cloneHistoryEvent(started))
		for _, event := range runFile.Events {
			events = append(events, cloneHistoryEvent(event))
			if event.Timestamp.After(observed) {
				observed = event.Timestamp.UTC()
			}
		}
		if boundary.terminal != nil {
			events = append(events, cloneHistoryEvent(*boundary.terminal))
			if boundary.terminal.Timestamp.After(observed) {
				observed = boundary.terminal.Timestamp.UTC()
			}
		}
		sort.SliceStable(events, func(a, b int) bool { return events[a].Timestamp.Before(events[b].Timestamp) })
		runs = append(runs, HistoryRun{
			RunID: runID, SessionID: first.SessionID, GameID: first.GameID, Mode: first.Mode,
			Character: first.Character, Difficulty: first.Difficulty, GameVersion: first.GameVersion,
			Run: first.Run, DefinitionID: first.DefinitionID, RouteID: first.RouteID, RouteLayoutFingerprint: first.RouteLayoutFingerprint,
			QueueIndex: *first.QueueIndex, QueueCycle: *first.QueueCycle, StartedAt: started.Timestamp.UTC(), ObservedAt: observed,
			EndedAt: endedAt, Outcome: outcome, Reason: reason, Events: events, RunFile: runFile.Name, SessionFile: boundary.file,
		})
	}
	sort.Slice(runs, func(a, b int) bool {
		if runs[a].StartedAt.Equal(runs[b].StartedAt) {
			return runs[a].RunID < runs[b].RunID
		}
		return runs[a].StartedAt.Before(runs[b].StartedAt)
	})
	return runs, diagnostics
}

func historyDiagnostic(file string, code HistoryReasonCode) HistoryFileDiagnostic {
	message, _ := HistoryReasonMessage(code)
	return HistoryFileDiagnostic{File: file, Code: code, Message: message}
}

func historyIndexSignature(files map[string]HistoryFile, diagnostics []HistoryFileDiagnostic, ignored int) string {
	var builder strings.Builder
	for _, file := range sortedHistoryFiles(files) {
		fmt.Fprintf(&builder, "%s:%s;", file.Name, file.Fingerprint)
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(&builder, "%s:%s;", diagnostic.File, diagnostic.Code)
	}
	fmt.Fprintf(&builder, "ignored:%d", ignored)
	return builder.String()
}

func cloneHistoryFile(file HistoryFile) HistoryFile {
	source := file.Events
	file.Events = make([]Event, len(source))
	for index, event := range source {
		file.Events[index] = cloneHistoryEvent(event)
	}
	return file
}

func cloneHistoryRun(run HistoryRun) HistoryRun {
	source := run.Events
	run.Events = make([]Event, len(source))
	for index, event := range source {
		run.Events[index] = cloneHistoryEvent(event)
	}
	if run.EndedAt != nil {
		ended := *run.EndedAt
		run.EndedAt = &ended
	}
	return run
}
