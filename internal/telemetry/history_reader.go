package telemetry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	// HistoryMaximumFileBytes begrenzt eine einzelne Historienquelle.
	HistoryMaximumFileBytes int64 = 32 << 20
	// HistoryMaximumLineBytes begrenzt ein einzelnes JSONL-Ereignis.
	HistoryMaximumLineBytes = 1 << 20
	// HistoryMaximumEventsPerFile begrenzt die Arbeit pro Historienquelle.
	HistoryMaximumEventsPerFile = 100_000
)

var errHistoryFileChanging = errors.New("history file is currently changing")

// HistoryFileDiagnostic beschreibt eine isolierte, nicht statistikfähige Datei.
type HistoryFileDiagnostic struct {
	File    string            `json:"file"`
	Code    HistoryReasonCode `json:"code"`
	Message string            `json:"message"`
}

// HistoryFile ist das immutable Ergebnis einer vollständig validierten JSONL-Datei.
type HistoryFile struct {
	Name        string
	Stream      HistoryStream
	SessionID   string
	RunID       string
	Events      []Event
	Fingerprint string
	Size        int64
	ModifiedAt  time.Time
	Ignored     bool
}

// HistoryReader liest nur reguläre JSONL-Dateien innerhalb eines festen Verzeichnisses.
type HistoryReader struct {
	directory string
}

// NewHistoryReader erstellt einen begrenzten, read-only Schema-4-Reader.
func NewHistoryReader(directory string) (*HistoryReader, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("%s: telemetry directory is required", HistoryReasonUnavailable)
	}
	return &HistoryReader{directory: filepath.Clean(directory)}, nil
}

// Read liest und validiert eine einzelne direkte Verzeichnisdatei vollständig.
func (r *HistoryReader) Read(name string) (HistoryFile, error) {
	source, err := r.readSource(name)
	if err != nil {
		return HistoryFile{}, err
	}
	return parseHistorySource(name, source)
}

type historySource struct {
	data        []byte
	fingerprint string
	size        int64
	modifiedAt  time.Time
}

func (r *HistoryReader) readSource(name string) (historySource, error) {
	info, err := r.stat(name)
	if err != nil {
		return historySource{}, err
	}
	data, err := os.ReadFile(filepath.Join(r.directory, name))
	if err != nil {
		return historySource{}, fmt.Errorf("read history file %q: %w", name, err)
	}
	// Recorder writes and flushes one newline-terminated record atomically from
	// its perspective. A missing final newline can only be an empty/new or
	// concurrently observed write; it is retried instead of diagnosed as corrupt.
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return historySource{}, errHistoryFileChanging
	}
	if int64(len(data)) > HistoryMaximumFileBytes {
		return historySource{}, historyReadError(HistoryReasonFileTooLarge, "history file exceeds %d bytes", HistoryMaximumFileBytes)
	}
	hash := sha256.Sum256(data)
	return historySource{data: data, fingerprint: hex.EncodeToString(hash[:]), size: int64(len(data)), modifiedAt: info.ModTime().UTC()}, nil
}

func parseHistorySource(name string, source historySource) (HistoryFile, error) {
	data := source.data
	lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
	if len(lines) > HistoryMaximumEventsPerFile {
		return HistoryFile{}, historyReadError(HistoryReasonFileTooLarge, "history file exceeds %d events", HistoryMaximumEventsPerFile)
	}
	if len(lines[0]) > HistoryMaximumLineBytes {
		return HistoryFile{}, historyReadError(HistoryReasonLineTooLarge, "invalid history line 1")
	}
	var epoch struct {
		SchemaVersion int `json:"schema_version"`
	}
	if len(lines[0]) == 0 || json.Unmarshal(lines[0], &epoch) != nil {
		return HistoryFile{}, historyReadError(HistoryReasonFileInvalid, "first history event is invalid")
	}
	result := HistoryFile{Name: name, Fingerprint: source.fingerprint, Size: source.size, ModifiedAt: source.modifiedAt}
	if epoch.SchemaVersion < HistorySchemaVersion {
		result.Ignored = true
		return result, nil
	}
	if epoch.SchemaVersion != HistorySchemaVersion {
		return HistoryFile{}, historyReadError(HistoryReasonSchemaUnsupported, "unsupported schema %d", epoch.SchemaVersion)
	}
	result.Events = make([]Event, 0, len(lines))
	var runContext Event
	for index, line := range lines {
		if len(line) == 0 || len(line) > HistoryMaximumLineBytes {
			code := HistoryReasonFileInvalid
			if len(line) > HistoryMaximumLineBytes {
				code = HistoryReasonLineTooLarge
			}
			return HistoryFile{}, historyReadError(code, "invalid history line %d", index+1)
		}
		var event Event
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			return HistoryFile{}, historyReadError(HistoryReasonFileInvalid, "decode history line %d: %v", index+1, err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return HistoryFile{}, historyReadError(HistoryReasonFileInvalid, "decode history line %d: %v", index+1, err)
		}
		if index > 0 && runContext.Stream == HistoryStreamRun {
			if err := hydrateCompactRunEvent(&event, runContext); err != nil {
				return HistoryFile{}, fmt.Errorf("line %d: %w", index+1, err)
			}
		}
		if err := validateHistoryEvent(event); err != nil {
			return HistoryFile{}, fmt.Errorf("line %d: %w", index+1, err)
		}
		if index == 0 {
			runContext = cloneHistoryEvent(event)
		}
		result.Events = append(result.Events, cloneHistoryEvent(event))
	}
	if err := validateHistoryFile(&result); err != nil {
		return HistoryFile{}, err
	}
	return result, nil
}

func hydrateCompactRunEvent(event *Event, context Event) error {
	if event.SchemaVersion != 0 && event.SchemaVersion != context.SchemaVersion {
		return historyReadError(HistoryReasonSchemaUnsupported, "mixed schema %d", event.SchemaVersion)
	}
	if event.Stream != "" && event.Stream != context.Stream {
		return historyReadError(HistoryReasonContextMissing, "compact run stream changed")
	}
	event.SchemaVersion = context.SchemaVersion
	event.Stream = context.Stream
	for _, field := range []struct {
		name   string
		target *string
		value  string
	}{
		{name: "run_id", target: &event.RunID, value: context.RunID},
		{name: "session_id", target: &event.SessionID, value: context.SessionID},
		{name: "game_id", target: &event.GameID, value: context.GameID},
		{name: "character", target: &event.Character, value: context.Character},
		{name: "difficulty", target: &event.Difficulty, value: context.Difficulty},
		{name: "game_version", target: &event.GameVersion, value: context.GameVersion},
		{name: "run", target: &event.Run, value: context.Run},
		{name: "definition_id", target: &event.DefinitionID, value: context.DefinitionID},
		{name: "phase", target: &event.Phase, value: context.Phase},
		{name: "route_id", target: &event.RouteID, value: context.RouteID},
		{name: "route_layout_fingerprint", target: &event.RouteLayoutFingerprint, value: context.RouteLayoutFingerprint},
		{name: "setup_route_id", target: &event.SetupRouteID, value: context.SetupRouteID},
		{name: "setup_route_layout_fingerprint", target: &event.SetupRouteLayoutFingerprint, value: context.SetupRouteLayoutFingerprint},
	} {
		if err := applyImmutableString(field.name, field.target, field.value); err != nil {
			return historyReadError(HistoryReasonContextMissing, "%v", err)
		}
	}
	if event.Mode != "" && event.Mode != context.Mode {
		return historyReadError(HistoryReasonContextMissing, "compact run mode changed")
	}
	event.Mode = context.Mode
	if event.QueueIndex != nil || event.QueueCycle != nil || event.RunStartedAt != nil || event.PickitAssignmentRevision != 0 || len(event.PickitProfiles) != 0 {
		return historyReadError(HistoryReasonContextMissing, "compact run repeats immutable snapshot")
	}
	event.QueueIndex = cloneOptionalInt(context.QueueIndex)
	event.QueueCycle = cloneOptionalInt(context.QueueCycle)
	event.RunStartedAt = cloneOptionalTime(context.RunStartedAt)
	event.PickitAssignmentRevision = context.PickitAssignmentRevision
	event.PickitProfiles = append([]PickitProfileContext(nil), context.PickitProfiles...)
	return nil
}

func cloneOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneOptionalTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

// stat performs the direct-file and size gates before source bytes are loaded.
func (r *HistoryReader) stat(name string) (os.FileInfo, error) {
	if r == nil || filepath.Base(name) != name || filepath.Ext(name) != ".jsonl" {
		return nil, historyReadError(HistoryReasonFileInvalid, "invalid history filename")
	}
	info, err := os.Lstat(filepath.Join(r.directory, name))
	if err != nil {
		return nil, fmt.Errorf("stat history file %q: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, historyReadError(HistoryReasonFileInvalid, "history source is not a regular file")
	}
	if info.Size() > HistoryMaximumFileBytes {
		return nil, historyReadError(HistoryReasonFileTooLarge, "history file exceeds %d bytes", HistoryMaximumFileBytes)
	}
	return info, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values")
	}
	return err
}

type historyReadErrorValue struct {
	code HistoryReasonCode
	err  error
}

func (e *historyReadErrorValue) Error() string { return fmt.Sprintf("%s: %v", e.code, e.err) }
func (e *historyReadErrorValue) Unwrap() error { return e.err }

func historyReadError(code HistoryReasonCode, format string, args ...any) error {
	return &historyReadErrorValue{code: code, err: fmt.Errorf(format, args...)}
}

func historyErrorCode(err error) HistoryReasonCode {
	var target *historyReadErrorValue
	if errors.As(err, &target) {
		return target.code
	}
	return HistoryReasonFileInvalid
}

// HistoryErrorCode extrahiert den stabilen Reason-Code eines Historienfehlers.
func HistoryErrorCode(err error) HistoryReasonCode {
	return historyErrorCode(err)
}

func validateHistoryEvent(event Event) error {
	if event.SchemaVersion != HistorySchemaVersion {
		return historyReadError(HistoryReasonSchemaUnsupported, "mixed schema %d", event.SchemaVersion)
	}
	if event.Stream != HistoryStreamSession && event.Stream != HistoryStreamRun {
		return historyReadError(HistoryReasonEventInvalid, "unknown stream %q", event.Stream)
	}
	if event.Timestamp.IsZero() || event.RunID != "" && safePart(event.RunID) != event.RunID {
		return historyReadError(HistoryReasonContextMissing, "event timestamp or run ID is invalid")
	}
	if !knownHistoryEvent(event.Event) {
		return historyReadError(HistoryReasonEventInvalid, "unknown event %q", event.Event)
	}
	if !historyEventAllowedInStream(event.Stream, event.Event) {
		return historyReadError(HistoryReasonEventInvalid, "event %q is invalid in %q stream", event.Event, event.Stream)
	}
	if event.Mode != HistoryModeProductiveFarming && event.Mode != HistoryModeDiagnostic {
		return historyReadError(HistoryReasonContextMissing, "unknown mode %q", event.Mode)
	}
	return nil
}

func validateHistoryFile(file *HistoryFile) error {
	first := file.Events[0]
	file.Stream, file.SessionID = first.Stream, first.SessionID
	previous := first.Timestamp
	terminals := make(map[string]EventName)
	bossKills := make(map[string]bool)
	items := make(map[uint32]historyItemContext)
	var activeStep *Event
	for index, event := range file.Events {
		if index > 0 && event.Timestamp.Before(previous) {
			return historyReadError(HistoryReasonTimeInvalid, "timestamps move backwards")
		}
		previous = event.Timestamp
		_, offset := event.Timestamp.Zone()
		if offset != 0 {
			return historyReadError(HistoryReasonTimeInvalid, "timestamp is not UTC")
		}
		if event.Stage != "" && !validHistoryStage(event.Stage) {
			return historyReadError(HistoryReasonStageInvalid, "unknown stage %q", event.Stage)
		}
		if event.Stream != first.Stream || event.Mode != first.Mode || event.SessionID != first.SessionID || event.Character != first.Character || event.Difficulty != first.Difficulty || event.GameVersion != first.GameVersion {
			return historyReadError(HistoryReasonContextMissing, "immutable file context changed")
		}
		if event.Stream == HistoryStreamRun {
			if index == 0 {
				file.RunID = event.RunID
			}
			if event.RunID == "" || event.RunID != file.RunID || event.GameID != first.GameID || event.Run != first.Run || event.DefinitionID != first.DefinitionID || event.RouteID != first.RouteID || event.RouteLayoutFingerprint != first.RouteLayoutFingerprint || event.SetupRouteID != first.SetupRouteID || event.SetupRouteLayoutFingerprint != first.SetupRouteLayoutFingerprint {
				return historyReadError(HistoryReasonRunIDMismatch, "immutable run context changed")
			}
			if !equalOptionalInt(event.QueueIndex, first.QueueIndex) || !equalOptionalInt(event.QueueCycle, first.QueueCycle) || !equalOptionalTime(event.RunStartedAt, first.RunStartedAt) || event.PickitAssignmentRevision != first.PickitAssignmentRevision || !slices.Equal(event.PickitProfiles, first.PickitProfiles) {
				return historyReadError(HistoryReasonContextMissing, "immutable run snapshot changed")
			}
			if first.Mode == HistoryModeProductiveFarming && (event.SessionID == "" || event.GameID == "" || event.Character == "" || event.Difficulty == "" || event.GameVersion == "" || event.DefinitionID == "" || event.RouteID == "" || event.RunStartedAt == nil || event.QueueIndex == nil || event.QueueCycle == nil) {
				return historyReadError(HistoryReasonContextMissing, "productive run context is incomplete")
			}
			if event.Event == BossKillConfirmed {
				if event.Stage != HistoryStageCombat || event.UnitID == 0 || event.BossID == "" {
					return historyReadError(HistoryReasonEventInvalid, "boss kill context is incomplete")
				}
				if bossKills[event.RunID] {
					return historyReadError(HistoryReasonBossDuplicate, "duplicate boss kill")
				}
				bossKills[event.RunID] = true
			}
			if event.Event == RunStepStarted || event.Event == RunStepCompleted || event.Event == RunStepFailed {
				if event.Step == "" || !validHistoryStage(event.Stage) {
					return historyReadError(HistoryReasonStageInvalid, "run step has no stable stage")
				}
				switch event.Event {
				case RunStepStarted:
					if activeStep != nil {
						return historyReadError(HistoryReasonStageInvalid, "run steps overlap")
					}
					started := cloneHistoryEvent(event)
					activeStep = &started
				case RunStepCompleted, RunStepFailed:
					if activeStep == nil || activeStep.Step != event.Step || activeStep.Stage != event.Stage {
						return historyReadError(HistoryReasonStageInvalid, "run step terminal has no matching start")
					}
					activeStep = nil
				}
			}
			if isHistoryItemEvent(event.Event) {
				if err := validateHistoryItemEvent(event, items); err != nil {
					return err
				}
			}
		} else if event.RunID != "" && event.Event != RunStarted && !isRunTerminal(event.Event) {
			return historyReadError(HistoryReasonEventInvalid, "session event %q carries run ID", event.Event)
		}
		if isRunTerminal(event.Event) {
			if terminals[event.RunID] != "" {
				return historyReadError(HistoryReasonTerminalDuplicate, "duplicate terminal for run %q", event.RunID)
			}
			terminals[event.RunID] = event.Event
		}
	}
	if first.Stream == HistoryStreamRun && strings.TrimSuffix(file.Name, ".jsonl") != file.RunID {
		return historyReadError(HistoryReasonRunIDMismatch, "run filename does not match run ID")
	}
	if first.Stream == HistoryStreamSession && (first.SessionID == "" || strings.TrimSuffix(file.Name, ".jsonl") != first.SessionID) {
		return historyReadError(HistoryReasonContextMissing, "session filename does not match session ID")
	}
	return nil
}

type historyItemContext struct {
	itemKey, profileID, ruleID, action  string
	profileRevision, assignmentRevision uint64
}

func validateHistoryItemEvent(event Event, items map[uint32]historyItemContext) error {
	if event.UnitID == 0 {
		return historyReadError(HistoryReasonItemChainInvalid, "item event has no unit ID")
	}
	wantStage := HistoryStageLoot
	if event.Event == StashAttempt || event.Event == StashSuccess || event.Event == SellSuccess {
		wantStage = HistoryStageReturnTown
	}
	if event.Stage != wantStage {
		return historyReadError(HistoryReasonStageInvalid, "item event %q has stage %q", event.Event, event.Stage)
	}
	if event.ItemKey != "" && !strings.HasPrefix(event.ItemKey, "base:") && !strings.HasPrefix(event.ItemKey, "set:") && !strings.HasPrefix(event.ItemKey, "unique:") {
		return historyReadError(HistoryReasonItemIdentityInvalid, "unsupported item key %q", event.ItemKey)
	}
	if event.ItemIdentityKey != "" {
		expected := event.ItemIdentityKind + ":" + event.ItemIdentityKey
		if (event.ItemIdentityKind != "set" && event.ItemIdentityKind != "unique") || event.ItemKey != expected {
			return historyReadError(HistoryReasonItemIdentityInvalid, "exact item identity is inconsistent")
		}
	}
	context := items[event.UnitID]
	for _, pair := range [][2]string{{context.itemKey, event.ItemKey}, {context.profileID, event.PickitProfileID}, {context.ruleID, event.PickitRuleID}, {context.action, event.PickitAction}} {
		if pair[0] != "" && pair[1] != "" && pair[0] != pair[1] {
			return historyReadError(HistoryReasonItemChainInvalid, "item context changed for unit %d", event.UnitID)
		}
	}
	if context.profileRevision != 0 && event.PickitProfileRevision != 0 && context.profileRevision != event.PickitProfileRevision || context.assignmentRevision != 0 && event.PickitAssignmentRevision != 0 && context.assignmentRevision != event.PickitAssignmentRevision {
		return historyReadError(HistoryReasonItemChainInvalid, "item policy revision changed for unit %d", event.UnitID)
	}
	if context.itemKey == "" {
		context.itemKey = event.ItemKey
	}
	if context.profileID == "" {
		context.profileID = event.PickitProfileID
	}
	if context.ruleID == "" {
		context.ruleID = event.PickitRuleID
	}
	if context.action == "" {
		context.action = event.PickitAction
	}
	if context.profileRevision == 0 {
		context.profileRevision = event.PickitProfileRevision
	}
	if context.assignmentRevision == 0 {
		context.assignmentRevision = event.PickitAssignmentRevision
	}
	items[event.UnitID] = context
	return nil
}

func isHistoryItemEvent(name EventName) bool {
	switch name {
	case DropSeen, PickitMatch, PickupAttempt, PickupSuccess, PickupFailed, StashAttempt, StashSuccess, SellSuccess:
		return true
	default:
		return false
	}
}

func equalOptionalInt(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalOptionalTime(left, right *time.Time) bool {
	return left == nil && right == nil || left != nil && right != nil && left.Equal(*right)
}

func validHistoryStage(stage HistoryStage) bool {
	return stage == HistoryStageTravel || stage == HistoryStageCombat || stage == HistoryStageLoot || stage == HistoryStageReturnTown
}

func historyEventAllowedInStream(stream HistoryStream, name EventName) bool {
	if stream == HistoryStreamRun {
		switch name {
		case SessionStarted, GameStarted, GameExited, GameRestartRequested, SessionCompleted, SessionStopped, SessionFailed:
			return false
		default:
			return true
		}
	}
	switch name {
	case SessionStarted, GameStarted, GameExited, RunStarted, RunCompleted, RunAborted, RunFailed, GameRestartRequested, SessionCompleted, SessionStopped, SessionFailed:
		return true
	default:
		return false
	}
}

func knownHistoryEvent(name EventName) bool {
	_, ok := historyEventNames[name]
	return ok
}

var historyEventNames = func() map[EventName]struct{} {
	names := []EventName{
		DropSeen, PickitMatch, PickupAttempt, PickupSuccess, PickupFailed, InventoryFull, StashAttempt, StashSuccess, StashFull, SellSuccess, BossKillConfirmed,
		RoutePlaybackStarted, RoutePointStarted, RoutePointSkipped, RouteTransitionStarted, RouteSegmentCompleted, RoutePlaybackCompleted, RoutePlaybackFailed, RoutePlaybackStopped,
		SessionStarted, GameStarted, GameExited, RunStarted, RunContext, StuckDetected, RunCompleted, RunAborted, RunFailed, GameRestartRequested, SessionCompleted, SessionStopped, SessionFailed,
		ProfileHookAction, ResourcePotionRequested, ResourceConsumptionConfirmed, ProfileActionFailed, TownAction, TownStepCompleted,
		RunStepStarted, RunStepCompleted, RunStepFailed, RunEncounterActionStarted, RunEncounterActionCompleted,
		RouteThreatDetected, RouteClearStarted, RouteMonsterSnapshotSaturated, RouteClearAction,
		RouteClearProgress, RouteClearCompleted, RouteManaHold, RouteRecoverySuppressed,
		TownMercenaryHealRequested, TownMercenaryHealConfirmed, TownMercenaryReviveRequested, TownMercenaryReviveConfirmed,
		MercenaryDied, CowRecipeProgress, ChestOpened, ChestSkipped, RackOperated, RackSkipped,
		TownPortalEntryUnconfirmed, TownPortalRecoveryStarted, TownPortalRetryClicked, TownPortalRecoveryCompleted,
	}
	out := make(map[EventName]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}()

func cloneHistoryEvent(event Event) Event {
	event.BeltSlots = append([]int(nil), event.BeltSlots...)
	event.PickitProfiles = append([]PickitProfileContext(nil), event.PickitProfiles...)
	return event
}

func sortedHistoryFiles(files map[string]HistoryFile) []HistoryFile {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]HistoryFile, 0, len(names))
	for _, name := range names {
		out = append(out, files[name])
	}
	return out
}
