package app

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"gopkg.in/yaml.v3"
)

const (
	// OperatorSettingsSchemaVersion ist die einzige unterstützte persistente Operatorversion.
	OperatorSettingsSchemaVersion = 3
	operatorSettingsBackupLimit   = 10
	operatorSettingsFilename      = "operator-settings.local.yaml"
	operatorInventoryRows         = 4
	operatorInventoryCols         = 10
)

// OperatorSettings enthält ausschließlich Core-eigene persistente GUI-Fachwerte.
type OperatorSettings struct {
	SchemaVersion int                                  `yaml:"schema_version" json:"schema_version"`
	Revision      uint64                               `yaml:"revision" json:"revision"`
	LastCharacter string                               `yaml:"last_character,omitempty" json:"last_character,omitempty"`
	Characters    map[string]OperatorCharacterSettings `yaml:"characters" json:"characters"`
	Budgets       OperatorBudgetSettings               `yaml:"budgets" json:"budgets"`
	Input         OperatorInputSettings                `yaml:"input" json:"input"`
	History       OperatorHistorySettings              `yaml:"history" json:"history"`
}

// OperatorCharacterSettings bindet Setup-Paar, Queue und letzte Difficulty an genau einen Charakter.
type OperatorCharacterSettings struct {
	CharacterClass  string                             `yaml:"character_class,omitempty" json:"character_class,omitempty"`
	CombatProfile   string                             `yaml:"combat_profile,omitempty" json:"combat_profile,omitempty"`
	LastDifficulty  string                             `yaml:"last_difficulty" json:"last_difficulty"`
	Queue           []string                           `yaml:"queue" json:"queue"`
	ProfileBindings map[string]OperatorProfileBindings `yaml:"profile_bindings,omitempty" json:"profile_bindings,omitempty"`
	InventoryLock   *OperatorInventoryLock             `yaml:"inventory_lock,omitempty" json:"inventory_lock,omitempty"`
}

// OperatorProfileBindings stores skill F-keys, belt keys and potion columns for one combat profile.
type OperatorProfileBindings struct {
	Skills     map[string]string    `yaml:"skills,omitempty" json:"skills,omitempty"` // canonical skill key -> f1..f8
	Belt       OperatorBeltBindings `yaml:"belt,omitempty" json:"belt,omitempty"`
	BeltLayout OperatorBeltLayout   `yaml:"belt_layout,omitempty" json:"belt_layout,omitempty"`
}

// OperatorBeltBindings stores optional belt slot keys for one combat profile.
type OperatorBeltBindings struct {
	Slot1 string `yaml:"slot_1,omitempty" json:"slot_1,omitempty"`
	Slot2 string `yaml:"slot_2,omitempty" json:"slot_2,omitempty"`
	Slot3 string `yaml:"slot_3,omitempty" json:"slot_3,omitempty"`
	Slot4 string `yaml:"slot_4,omitempty" json:"slot_4,omitempty"`
}

// OperatorInventoryLock is presence-sensitive; nil pointer = unconfigured.
type OperatorInventoryLock struct {
	Grid [][]int `yaml:"grid" json:"grid"` // exactly 4x10 of 0/1 when present
}

// OperatorBudgetSettings enthält die endlichen globalen Queue-Hardlimits.
type OperatorBudgetSettings struct {
	MaxRuns                int `yaml:"max_runs" json:"max_runs"`
	MaxDurationMs          int `yaml:"max_duration_ms" json:"max_duration_ms"`
	MaxConsecutiveFailures int `yaml:"max_consecutive_failures" json:"max_consecutive_failures"`
	MaxTotalRestarts       int `yaml:"max_total_restarts" json:"max_total_restarts"`
}

// OperatorInputSettings enthält Opt-in und die vier paarweise verschiedenen Gameplay-Hotkeys.
type OperatorInputSettings struct {
	Enabled               bool   `yaml:"enabled" json:"enabled"`
	PauseHotkey           string `yaml:"pause_hotkey" json:"pause_hotkey"`
	StopAfterRunHotkey    string `yaml:"stop_after_run_hotkey" json:"stop_after_run_hotkey"`
	RecordingFinishHotkey string `yaml:"recording_finish_hotkey" json:"recording_finish_hotkey"`
	EmergencyStopHotkey   string `yaml:"emergency_stop_hotkey" json:"emergency_stop_hotkey"`
}

// OperatorHistorySettings konfiguriert ausschließlich die spätere Core-Retention.
type OperatorHistorySettings struct {
	RetentionEnabled bool `yaml:"retention_enabled" json:"retention_enabled"`
	RetentionDays    int  `yaml:"retention_days" json:"retention_days"`
}

// OperatorSettingsChange beschreibt eine validierte Vorschau ohne Dateisystemmutation.
type OperatorSettingsChange struct {
	Settings        OperatorSettings
	ChangedFields   []string
	RestartRequired bool
}

// OperatorSettingsError bindet Settingsfehler an einen stabilen Phase-15-Code.
type OperatorSettingsError struct {
	Code Phase15ReasonCode
	Err  error
}

// OperatorSettingsValidationError is a user-actionable Core validation failure.
type OperatorSettingsValidationError struct {
	Message string
}

// Error returns the user-facing validation message.
func (e *OperatorSettingsValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Error liefert Reason-Code und technische Ursache.
func (e *OperatorSettingsError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

// Unwrap erhält die technische Ursache.
func (e *OperatorSettingsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// OperatorSettingsStore besitzt Schema, Revision, Backups und den effektiven Core-Stand.
type OperatorSettingsStore struct {
	mu                *sync.Mutex
	path              string
	backupRoot        string
	defaults          OperatorSettings
	profiles          config.ProfilesConfig
	characterDefaults OperatorCharacterSettings
	read              func(string) ([]byte, error)
	write             func(string, []byte, string) error
	now               func() time.Time
	effective         *OperatorSettings
}

var operatorSettingsLocks sync.Map

// OpenOperatorSettings öffnet oder initialisiert den installierten Schema-3-Store und liefert seinen validierten Snapshot.
func OpenOperatorSettings(cfg *config.Config) (*OperatorSettingsStore, OperatorSettings, error) {
	if cfg == nil || cfg.DataRoot == "" {
		return nil, OperatorSettings{}, fmt.Errorf("installed operator settings require an explicit data root")
	}
	catalog, err := ResolveCharacterCatalog(cfg)
	if err != nil {
		return nil, OperatorSettings{}, fmt.Errorf("resolve operator settings characters: %w", err)
	}
	names := make([]string, 0, len(catalog.Characters))
	for _, character := range catalog.Characters {
		names = append(names, character.Name)
	}
	store, err := NewOperatorSettingsStore(cfg.DataRoot, cfg, names)
	if err != nil {
		return nil, OperatorSettings{}, err
	}
	settings, err := store.Snapshot()
	if err != nil {
		return nil, OperatorSettings{}, err
	}
	return store, settings, nil
}

// NewOperatorSettingsStore erstellt einen Store für einen absoluten Datenroot und dessen sichere Config-Defaults.
func NewOperatorSettingsStore(dataRoot string, cfg *config.Config, characterNames []string) (*OperatorSettingsStore, error) {
	if cfg == nil || strings.TrimSpace(dataRoot) == "" || !filepath.IsAbs(dataRoot) {
		return nil, fmt.Errorf("operator settings require config and an absolute data root")
	}
	cleanRoot := filepath.Clean(dataRoot)
	path := filepath.Join(cleanRoot, "configs", operatorSettingsFilename)
	characters := make(map[string]string, len(characterNames)+1)
	for _, name := range append(append([]string(nil), characterNames...), cfg.Session.Character) {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			characters[strings.ToLower(trimmed)] = trimmed
		}
	}
	defaults := operatorSettingsDefaults(cfg, characters)
	profiles := make(config.ProfilesConfig, len(cfg.Profiles))
	for id, profile := range cfg.Profiles {
		profiles[id] = profile
	}
	lock, _ := operatorSettingsLocks.LoadOrStore(path, &sync.Mutex{})
	return &OperatorSettingsStore{
		mu: lock.(*sync.Mutex), path: path, backupRoot: filepath.Join(cleanRoot, "backups"),
		defaults: defaults, profiles: profiles,
		characterDefaults: OperatorCharacterSettings{LastDifficulty: cfg.Session.Difficulty, Queue: append([]string(nil), cfg.Session.Queue...)},
		read:              os.ReadFile, write: writeAtomicYAML, now: time.Now,
	}, nil
}

// Snapshot lädt oder initialisiert den vollständig validierten effektiven Stand.
func (s *OperatorSettingsStore) Snapshot() (OperatorSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := s.loadLocked()
	if errors.Is(err, fs.ErrNotExist) {
		if writeErr := s.write(s.path, mustMarshalOperatorSettings(s.defaults), "operator-settings"); writeErr != nil {
			return OperatorSettings{}, fmt.Errorf("initialize operator settings: %w", writeErr)
		}
		settings, err = s.loadLocked()
	}
	if err != nil {
		return OperatorSettings{}, err
	}
	s.effective = pointerToOperatorSettings(settings)
	return cloneOperatorSettings(settings), nil
}

// Preview validiert eine revisionierte Ersetzung ohne Datei, Backup oder effektiven Stand zu ändern.
func (s *OperatorSettingsStore) Preview(expectedRevision uint64, replacement OperatorSettings) (OperatorSettingsChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.currentLocked()
	if err != nil {
		return OperatorSettingsChange{}, err
	}
	return s.previewLocked(current, expectedRevision, replacement, false)
}

// PreviewReset validiert die sicheren Defaults und ihre Neustartwirkung ohne Mutation.
func (s *OperatorSettingsStore) PreviewReset(expectedRevision uint64) (OperatorSettingsChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.currentLocked()
	if err != nil {
		return OperatorSettingsChange{}, err
	}
	replacement := s.resetReplacement(current, expectedRevision)
	return s.previewLocked(current, expectedRevision, replacement, false)
}

// Update ersetzt den Gesamtvertrag atomar, liest ihn erneut und behält bei Fehlern den alten effektiven Stand.
func (s *OperatorSettingsStore) Update(expectedRevision uint64, replacement OperatorSettings) (OperatorSettingsChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.currentLocked()
	if err != nil {
		return OperatorSettingsChange{}, err
	}
	change, err := s.previewLocked(current, expectedRevision, replacement, false)
	if err != nil || len(change.ChangedFields) == 0 {
		return change, err
	}
	return s.commitLocked(current, change)
}

// Reset stellt die sicheren Defaults mit einer neuen Revision wieder her und bewahrt jedes Setup-Paar.
func (s *OperatorSettingsStore) Reset(expectedRevision uint64) (OperatorSettingsChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.currentLocked()
	if err != nil {
		return OperatorSettingsChange{}, err
	}
	replacement := s.resetReplacement(current, expectedRevision)
	change, err := s.previewLocked(current, expectedRevision, replacement, false)
	if err != nil || len(change.ChangedFields) == 0 {
		return change, err
	}
	return s.commitLocked(current, change)
}

// ConfirmSelection persists the last Core-confirmed character and difficulty
// without replacing unrelated queues, budgets, input, or history settings.
func (s *OperatorSettingsStore) ConfirmSelection(character, difficulty string) (OperatorSettingsChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.currentLocked()
	if err != nil {
		return OperatorSettingsChange{}, err
	}
	key := strings.ToLower(strings.TrimSpace(character))
	value, ok := current.Characters[key]
	if !ok {
		return OperatorSettingsChange{}, fmt.Errorf("operator settings character %q is unknown", character)
	}
	replacement := cloneOperatorSettings(current)
	replacement.LastCharacter = strings.TrimSpace(character)
	value.LastDifficulty = strings.ToLower(strings.TrimSpace(difficulty))
	replacement.Characters[key] = value
	change, err := s.previewLocked(current, current.Revision, replacement, false)
	if err != nil || len(change.ChangedFields) == 0 {
		return change, err
	}
	return s.commitLocked(current, change)
}

func (s *OperatorSettingsStore) currentLocked() (OperatorSettings, error) {
	current, err := s.loadLocked()
	if err != nil {
		return OperatorSettings{}, err
	}
	if s.effective == nil {
		s.effective = pointerToOperatorSettings(current)
	}
	return current, nil
}

// AssignCharacterProfile setzt Klasse und freigegebenes Profil gemeinsam und legt einen neu gefundenen Charakter mit sicheren Defaults an.
func (s *OperatorSettingsStore) AssignCharacterProfile(character, characterClass, combatProfile string, expectedRevision uint64) (OperatorSettingsChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.currentLocked()
	if err != nil {
		return OperatorSettingsChange{}, err
	}
	displayName := strings.TrimSpace(character)
	key := strings.ToLower(displayName)
	if displayName == "" || !offlineCharacterNamePattern.MatchString(displayName) {
		return OperatorSettingsChange{}, fmt.Errorf("operator settings character %q is invalid", character)
	}
	characterClass = strings.TrimSpace(characterClass)
	combatProfile = strings.TrimSpace(combatProfile)
	replacement := cloneOperatorSettings(current)
	value, ok := replacement.Characters[key]
	if !ok {
		value = s.characterDefaults
		value.Queue = append([]string(nil), s.characterDefaults.Queue...)
	}
	value.CharacterClass = characterClass
	value.CombatProfile = combatProfile
	replacement.Characters[key] = value
	return s.previewAndCommitLocked(current, expectedRevision, replacement, true)
}

func (s *OperatorSettingsStore) previewAndCommitLocked(current OperatorSettings, expectedRevision uint64, replacement OperatorSettings, allowSetupMutation bool) (OperatorSettingsChange, error) {
	change, err := s.previewLocked(current, expectedRevision, replacement, allowSetupMutation)
	if err != nil || len(change.ChangedFields) == 0 {
		return change, err
	}
	return s.commitLocked(current, change)
}

func (s *OperatorSettingsStore) previewLocked(current OperatorSettings, expectedRevision uint64, replacement OperatorSettings, allowSetupMutation bool) (OperatorSettingsChange, error) {
	if current.Revision != expectedRevision || replacement.Revision != expectedRevision {
		return OperatorSettingsChange{}, &OperatorSettingsError{Code: Phase15ReasonConfigRevisionConflict, Err: fmt.Errorf("expected revision %d, current revision %d", expectedRevision, current.Revision)}
	}
	if !allowSetupMutation && !sameOperatorSetup(current, replacement) {
		return OperatorSettingsChange{}, fmt.Errorf("operator_settings_setup_read_only")
	}
	replacement.SchemaVersion = OperatorSettingsSchemaVersion
	replacement.Revision = current.Revision + 1
	if err := validateOperatorSettings(replacement, s.profiles); err != nil {
		return OperatorSettingsChange{}, err
	}
	changed := operatorSettingsChangedFields(current, replacement)
	if len(changed) == 0 {
		replacement = current
	}
	restart := containsOperatorField(changed, "input")
	return OperatorSettingsChange{Settings: cloneOperatorSettings(replacement), ChangedFields: changed, RestartRequired: restart}, nil
}

func (s *OperatorSettingsStore) commitLocked(current OperatorSettings, change OperatorSettingsChange) (OperatorSettingsChange, error) {
	oldData := mustMarshalOperatorSettings(current)
	if err := s.backupLocked(current, oldData); err != nil {
		return OperatorSettingsChange{}, fmt.Errorf("backup operator settings: %w", err)
	}
	newData := mustMarshalOperatorSettings(change.Settings)
	if err := s.write(s.path, newData, "operator-settings"); err != nil {
		_ = s.pruneBackupsLocked()
		return OperatorSettingsChange{}, fmt.Errorf("write operator settings: %w", err)
	}
	persisted, err := s.loadLocked()
	if err != nil || !reflect.DeepEqual(persisted, change.Settings) {
		// Der alte In-Memory-Stand bleibt autoritativ. Zusätzlich wird der
		// letzte valide Dateistand best-effort atomar zurückgeschrieben.
		_ = writeAtomicYAML(s.path, oldData, "operator-settings-rollback")
		_ = s.pruneBackupsLocked()
		if err == nil {
			err = fmt.Errorf("operator settings re-read differs from written value")
		}
		return OperatorSettingsChange{}, fmt.Errorf("re-read operator settings: %w", err)
	}
	s.effective = pointerToOperatorSettings(persisted)
	if err := s.pruneBackupsLocked(); err != nil {
		return OperatorSettingsChange{}, fmt.Errorf("prune operator settings backups: %w", err)
	}
	change.Settings = cloneOperatorSettings(persisted)
	return change, nil
}

func (s *OperatorSettingsStore) loadLocked() (OperatorSettings, error) {
	data, err := s.read(s.path)
	if err != nil {
		return OperatorSettings{}, err
	}
	var settings OperatorSettings
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if decodeErr := decoder.Decode(&settings); decodeErr != nil {
		return OperatorSettings{}, fmt.Errorf("decode operator settings: %w", decodeErr)
	}
	if trailingErr := decoder.Decode(&struct{}{}); trailingErr != io.EOF {
		return OperatorSettings{}, fmt.Errorf("operator settings must contain one YAML document")
	}
	if settings.SchemaVersion != OperatorSettingsSchemaVersion {
		return OperatorSettings{}, &OperatorSettingsError{Code: Phase15ReasonConfigSchemaUnsupported, Err: fmt.Errorf("unsupported operator settings schema %d", settings.SchemaVersion)}
	}
	if validationErr := validateOperatorSettings(settings, s.profiles); validationErr != nil {
		return OperatorSettings{}, validationErr
	}
	return settings, nil
}

func (s *OperatorSettingsStore) backupLocked(current OperatorSettings, data []byte) error {
	if err := os.MkdirAll(s.backupRoot, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("operator-settings-r%020d-%s.yaml", current.Revision, s.now().UTC().Format("20060102T150405.000000000Z"))
	path := filepath.Join(s.backupRoot, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (s *OperatorSettingsStore) pruneBackupsLocked() error {
	entries, err := os.ReadDir(s.backupRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "operator-settings-r") && strings.HasSuffix(entry.Name(), ".yaml") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for len(names) > operatorSettingsBackupLimit {
		if removeErr := os.Remove(filepath.Join(s.backupRoot, names[0])); removeErr != nil {
			return removeErr
		}
		names = names[1:]
	}
	return nil
}

func operatorSettingsDefaults(cfg *config.Config, characters map[string]string) OperatorSettings {
	settings := OperatorSettings{
		SchemaVersion: OperatorSettingsSchemaVersion, Revision: 1,
		LastCharacter: strings.TrimSpace(cfg.Session.Character),
		Characters:    make(map[string]OperatorCharacterSettings, len(characters)),
		Budgets: OperatorBudgetSettings{
			MaxRuns: cfg.Session.MaxRuns, MaxDurationMs: cfg.Session.MaxDurationMs,
			MaxConsecutiveFailures: cfg.Session.MaxConsecutiveFailures, MaxTotalRestarts: cfg.Session.MaxTotalRestarts,
		},
		Input: OperatorInputSettings{
			Enabled: cfg.Input.Enabled, PauseHotkey: cfg.Input.PauseHotkey,
			StopAfterRunHotkey: cfg.Input.StopAfterRunHotkey, RecordingFinishHotkey: cfg.Input.RecordingFinishHotkey,
			EmergencyStopHotkey: cfg.Input.StopHotkey,
		},
		History: OperatorHistorySettings{RetentionEnabled: true, RetentionDays: 60},
	}
	for slug := range characters {
		settings.Characters[slug] = OperatorCharacterSettings{LastDifficulty: cfg.Session.Difficulty, Queue: append([]string(nil), cfg.Session.Queue...)}
	}
	return settings
}

func validateOperatorSettings(settings OperatorSettings, profiles config.ProfilesConfig) error {
	if settings.SchemaVersion != OperatorSettingsSchemaVersion {
		return &OperatorSettingsError{Code: Phase15ReasonConfigSchemaUnsupported, Err: fmt.Errorf("unsupported operator settings schema %d", settings.SchemaVersion)}
	}
	if settings.Revision == 0 {
		return fmt.Errorf("operator settings revision must be positive")
	}
	if settings.Characters == nil {
		return fmt.Errorf("operator settings characters must not be null")
	}
	if settings.LastCharacter != "" {
		if settings.LastCharacter != strings.TrimSpace(settings.LastCharacter) {
			return fmt.Errorf("operator settings last_character must be trimmed")
		}
		if _, ok := settings.Characters[strings.ToLower(settings.LastCharacter)]; !ok {
			return fmt.Errorf("operator settings last_character %q is unknown", settings.LastCharacter)
		}
	}
	for rawCharacter, value := range settings.Characters {
		character := strings.ToLower(strings.TrimSpace(rawCharacter))
		if character == "" || character != rawCharacter || !offlineCharacterNamePattern.MatchString(character) {
			return fmt.Errorf("operator settings character keys must be canonical lowercase names")
		}
		if (value.CharacterClass == "") != (value.CombatProfile == "") {
			return fmt.Errorf("operator settings character %q must set character_class and combat_profile together", rawCharacter)
		}
		if value.CharacterClass != "" {
			if value.CharacterClass != strings.TrimSpace(value.CharacterClass) || value.CombatProfile != strings.TrimSpace(value.CombatProfile) {
				return fmt.Errorf("operator settings character %q setup values must be trimmed", rawCharacter)
			}
			profile, ok := profiles[value.CombatProfile]
			if !ok {
				return fmt.Errorf("operator settings character %q references unknown combat profile %q", rawCharacter, value.CombatProfile)
			}
			if !profile.Setup.Enabled {
				return fmt.Errorf("operator settings character %q references a profile not enabled for setup", rawCharacter)
			}
			if profile.CharacterClass != value.CharacterClass {
				return fmt.Errorf("operator settings character %q class and combat profile are incompatible", rawCharacter)
			}
		}
		switch value.LastDifficulty {
		case "normal", "nightmare", "hell":
		default:
			return fmt.Errorf("operator settings character %q has invalid difficulty", rawCharacter)
		}
		if _, err := validateUniqueQueueRunIDs(value.Queue); err != nil {
			return fmt.Errorf("operator settings character %q queue: %w", rawCharacter, err)
		}
		for _, runID := range value.Queue {
			if !tasks.IsKnownRun(runID) {
				return fmt.Errorf("operator settings character %q queue contains unknown run %q", rawCharacter, runID)
			}
		}
		if err := validateOperatorProfileBindings(rawCharacter, value.ProfileBindings, profiles, settings.Input); err != nil {
			return err
		}
		if err := validateOperatorInventoryLock(rawCharacter, value.InventoryLock); err != nil {
			return err
		}
	}
	if settings.Budgets.MaxRuns <= 0 || settings.Budgets.MaxRuns > 10000 || settings.Budgets.MaxDurationMs <= 0 || settings.Budgets.MaxDurationMs > int((24*time.Hour)/time.Millisecond) {
		return fmt.Errorf("operator settings run and duration budgets are outside finite limits")
	}
	if settings.Budgets.MaxConsecutiveFailures < 0 || settings.Budgets.MaxConsecutiveFailures > 100 || settings.Budgets.MaxTotalRestarts < 0 || settings.Budgets.MaxTotalRestarts > 100 {
		return fmt.Errorf("operator settings failure and restart budgets are outside finite limits")
	}
	hotkeys := []string{settings.Input.PauseHotkey, settings.Input.StopAfterRunHotkey, settings.Input.RecordingFinishHotkey, settings.Input.EmergencyStopHotkey}
	seenHotkeys := make(map[string]struct{}, len(hotkeys))
	for _, hotkey := range hotkeys {
		canonical := strings.ToLower(strings.TrimSpace(hotkey))
		if canonical == "" || canonical != hotkey {
			return fmt.Errorf("operator settings hotkeys must be canonical lowercase values")
		}
		if _, duplicate := seenHotkeys[canonical]; duplicate {
			return fmt.Errorf("operator settings hotkeys must differ")
		}
		seenHotkeys[canonical] = struct{}{}
		if err := input.ValidateKeyStrings(canonical); err != nil {
			return fmt.Errorf("operator settings hotkey %q: %w", canonical, err)
		}
	}
	if settings.History.RetentionDays <= 0 || settings.History.RetentionDays > 3650 {
		return fmt.Errorf("operator settings history retention_days must be between 1 and 3650")
	}
	return nil
}

func (s *OperatorSettingsStore) resetReplacement(current OperatorSettings, expectedRevision uint64) OperatorSettings {
	replacement := cloneOperatorSettings(s.defaults)
	replacement.Revision = expectedRevision
	for character, currentValue := range current.Characters {
		value, ok := replacement.Characters[character]
		if !ok {
			value = s.characterDefaults
			value.Queue = append([]string(nil), s.characterDefaults.Queue...)
		}
		value.CharacterClass = currentValue.CharacterClass
		value.CombatProfile = currentValue.CombatProfile
		value.ProfileBindings = cloneOperatorProfileBindings(currentValue.ProfileBindings)
		value.InventoryLock = cloneOperatorInventoryLock(currentValue.InventoryLock)
		replacement.Characters[character] = value
	}
	return replacement
}

// ApplyOperatorSettingsToConfig setzt einen validierten persistenten Stand vor dem Aufbau mutabler Runtime-Komponenten.
func ApplyOperatorSettingsToConfig(cfg *config.Config, settings OperatorSettings) {
	if cfg == nil {
		return
	}
	displayCharacter := strings.TrimSpace(settings.LastCharacter)
	if displayCharacter == "" {
		displayCharacter = strings.TrimSpace(cfg.Session.Character)
	}
	character := strings.ToLower(displayCharacter)
	if character != "" {
		if value, ok := settings.Characters[character]; ok {
			cfg.Session.Character = displayCharacter
			cfg.Session.Queue = append([]string(nil), value.Queue...)
			if len(value.Queue) > 0 {
				cfg.Session.Run = value.Queue[0]
			}
			cfg.Session.Difficulty = value.LastDifficulty
		}
	}
	cfg.Session.MaxRuns = settings.Budgets.MaxRuns
	cfg.Session.MaxDurationMs = settings.Budgets.MaxDurationMs
	cfg.Session.MaxConsecutiveFailures = settings.Budgets.MaxConsecutiveFailures
	cfg.Session.MaxTotalRestarts = settings.Budgets.MaxTotalRestarts
	cfg.Input.Enabled = settings.Input.Enabled
	cfg.Input.PauseHotkey = settings.Input.PauseHotkey
	cfg.Input.StopAfterRunHotkey = settings.Input.StopAfterRunHotkey
	cfg.Input.RecordingFinishHotkey = settings.Input.RecordingFinishHotkey
	cfg.Input.StopHotkey = settings.Input.EmergencyStopHotkey
}

func operatorSettingsChangedFields(current, next OperatorSettings) []string {
	fields := make([]string, 0, 4)
	if current.LastCharacter != next.LastCharacter {
		fields = append(fields, "last_character")
	}
	if !reflect.DeepEqual(current.Characters, next.Characters) {
		fields = append(fields, "characters")
	}
	if current.Budgets != next.Budgets {
		fields = append(fields, "budgets")
	}
	if current.Input != next.Input {
		fields = append(fields, "input")
	}
	if current.History != next.History {
		fields = append(fields, "history")
	}
	return fields
}

func sameOperatorSetup(left, right OperatorSettings) bool {
	if len(left.Characters) != len(right.Characters) {
		return false
	}
	for character, leftValue := range left.Characters {
		rightValue, ok := right.Characters[character]
		if !ok || leftValue.CharacterClass != rightValue.CharacterClass || leftValue.CombatProfile != rightValue.CombatProfile {
			return false
		}
	}
	return true
}

func containsOperatorField(fields []string, expected string) bool {
	for _, field := range fields {
		if field == expected {
			return true
		}
	}
	return false
}

func cloneOperatorSettings(settings OperatorSettings) OperatorSettings {
	clone := settings
	clone.Characters = make(map[string]OperatorCharacterSettings, len(settings.Characters))
	for character, value := range settings.Characters {
		value.Queue = append([]string(nil), value.Queue...)
		value.ProfileBindings = cloneOperatorProfileBindings(value.ProfileBindings)
		value.InventoryLock = cloneOperatorInventoryLock(value.InventoryLock)
		clone.Characters[character] = value
	}
	return clone
}

func cloneOperatorProfileBindings(bindings map[string]OperatorProfileBindings) map[string]OperatorProfileBindings {
	if bindings == nil {
		return nil
	}
	clone := make(map[string]OperatorProfileBindings, len(bindings))
	for profileID, value := range bindings {
		cloned := OperatorProfileBindings{Belt: value.Belt, BeltLayout: value.BeltLayout}
		if value.Skills != nil {
			cloned.Skills = make(map[string]string, len(value.Skills))
			for skill, key := range value.Skills {
				cloned.Skills[skill] = key
			}
		}
		clone[profileID] = cloned
	}
	return clone
}

func cloneOperatorInventoryLock(lock *OperatorInventoryLock) *OperatorInventoryLock {
	if lock == nil {
		return nil
	}
	clone := &OperatorInventoryLock{Grid: make([][]int, len(lock.Grid))}
	for row, values := range lock.Grid {
		clone.Grid[row] = append([]int(nil), values...)
	}
	return clone
}

func validateOperatorProfileBindings(character string, bindings map[string]OperatorProfileBindings, profiles config.ProfilesConfig, inputSettings OperatorInputSettings) error {
	if bindings == nil {
		return nil
	}
	botHotkeys := operatorBotHotkeySet(inputSettings)
	for rawProfileID, value := range bindings {
		profileID := strings.TrimSpace(rawProfileID)
		if profileID == "" || profileID != rawProfileID {
			return fmt.Errorf("operator settings character %q has invalid profile_bindings key", character)
		}
		profileCfg, ok := profiles[profileID]
		if !ok {
			return fmt.Errorf("operator settings character %q profile_bindings references unknown combat profile %q", character, profileID)
		}
		if err := validateOptionalSkillPairBindings(value, profileCfg); err != nil {
			return fmt.Errorf("operator settings character %q profile %q: %w", character, profileID, err)
		}
		if err := validateOperatorBeltLayout(value.BeltLayout); err != nil {
			return fmt.Errorf("operator settings character %q profile %q: %w", character, profileID, err)
		}
		if beltLayoutConfigured(value.BeltLayout) {
			mercEnabled, _ := profileCfg.Resources.Mercenary.Resolve()
			if (profileCfg.RequiresMercenary || mercEnabled) && !beltLayoutHasKind(value.BeltLayout, beltPotionHealing) {
				return fmt.Errorf("operator settings character %q profile %q belt_layout needs at least one healing column for mercenary potions", character, profileID)
			}
		}
		usedKeys := make(map[string]string, len(value.Skills)+4)
		for skillKey, rawKey := range value.Skills {
			canonicalSkill := strings.ToLower(strings.TrimSpace(skillKey))
			if canonicalSkill == "" || canonicalSkill != skillKey {
				return fmt.Errorf("operator settings character %q profile %q skill keys must be canonical lowercase", character, profileID)
			}
			if _, err := memory.ParseSkillTestName(canonicalSkill); err != nil {
				return fmt.Errorf("operator settings character %q profile %q skill %q: %w", character, profileID, skillKey, err)
			}
			key := strings.ToLower(strings.TrimSpace(rawKey))
			if key == "" && rawKey == "" && profileOptionalSkill(profileCfg, skillKey) {
				continue
			}
			if key == "" || key != rawKey {
				return fmt.Errorf("operator settings character %q profile %q skill %q key must be canonical lowercase", character, profileID, skillKey)
			}
			if !isOperatorSkillFKey(key) {
				return fmt.Errorf("operator settings character %q profile %q skill %q must use f1-f8", character, profileID, skillKey)
			}
			if _, reserved := botHotkeys[key]; reserved {
				return fmt.Errorf("operator settings character %q profile %q skill %q collides with a bot hotkey", character, profileID, skillKey)
			}
			if prior, duplicate := usedKeys[key]; duplicate {
				return fmt.Errorf("operator settings character %q profile %q reuses key %q for %s and %s", character, profileID, key, prior, skillKey)
			}
			usedKeys[key] = "skill:" + skillKey
		}
		for _, slot := range []struct {
			name string
			key  string
		}{
			{name: "slot_1", key: value.Belt.Slot1},
			{name: "slot_2", key: value.Belt.Slot2},
			{name: "slot_3", key: value.Belt.Slot3},
			{name: "slot_4", key: value.Belt.Slot4},
		} {
			if strings.TrimSpace(slot.key) == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(slot.key))
			if key != slot.key {
				return fmt.Errorf("operator settings character %q profile %q belt %s must be canonical lowercase", character, profileID, slot.name)
			}
			if !isOperatorBeltKey(key) {
				return fmt.Errorf("operator settings character %q profile %q belt %s uses an unsafe key", character, profileID, slot.name)
			}
			if _, reserved := botHotkeys[key]; reserved {
				return fmt.Errorf("operator settings character %q profile %q belt %s collides with a bot hotkey", character, profileID, slot.name)
			}
			if prior, duplicate := usedKeys[key]; duplicate {
				return fmt.Errorf("operator settings character %q profile %q reuses key %q for %s and belt %s", character, profileID, key, prior, slot.name)
			}
			usedKeys[key] = "belt:" + slot.name
		}
	}
	return nil
}

func profileOptionalSkill(profileCfg config.ProfileConfig, skill string) bool {
	for _, pair := range profileCfg.OptionalSkillPairs {
		for _, entry := range pair.Skills {
			if entry.Skill == skill {
				return true
			}
		}
	}
	return false
}

func validateOptionalSkillPairBindings(bindings OperatorProfileBindings, profileCfg config.ProfileConfig) error {
	for _, pair := range profileCfg.OptionalSkillPairs {
		if len(pair.Skills) != 2 {
			return fmt.Errorf("optionaler Skill-Paar-Vertrag ist ungültig")
		}
		first := pair.Skills[0].Skill
		second := pair.Skills[1].Skill
		firstSet := strings.TrimSpace(bindings.Skills[first]) != ""
		secondSet := strings.TrimSpace(bindings.Skills[second]) != ""
		if firstSet != secondSet {
			message := fmt.Sprintf("%s und %s müssen gemeinsam belegt oder beide leer sein.", optionalSkillDisplayName(first), optionalSkillDisplayName(second))
			if optionalSkillPairIsCTA(first, second) {
				message = hammerdinCTAPairRequiredMessage
			}
			return &OperatorSettingsValidationError{Message: message}
		}
	}
	return nil
}

func optionalSkillPairIsCTA(first, second string) bool {
	skills := map[string]bool{first: true, second: true}
	return skills["battle_command"] && skills["battle_orders"]
}

func optionalSkillDisplayName(skill string) string {
	if entry, ok := memory.LookupSkillByKey(skill); ok && entry.SourceName != "" {
		return entry.SourceName
	}
	return skill
}

func validateOperatorInventoryLock(character string, lock *OperatorInventoryLock) error {
	if lock == nil {
		return nil
	}
	if len(lock.Grid) != operatorInventoryRows {
		return fmt.Errorf("operator settings character %q inventory_lock.grid must be %dx%d", character, operatorInventoryRows, operatorInventoryCols)
	}
	for row, values := range lock.Grid {
		if len(values) != operatorInventoryCols {
			return fmt.Errorf("operator settings character %q inventory_lock.grid must be %dx%d", character, operatorInventoryRows, operatorInventoryCols)
		}
		for col, cell := range values {
			if cell != 0 && cell != 1 {
				return fmt.Errorf("operator settings character %q inventory_lock.grid[%d][%d] must be 0 or 1", character, row, col)
			}
		}
	}
	return nil
}

func operatorBotHotkeySet(settings OperatorInputSettings) map[string]struct{} {
	return map[string]struct{}{
		strings.ToLower(strings.TrimSpace(settings.PauseHotkey)):           {},
		strings.ToLower(strings.TrimSpace(settings.StopAfterRunHotkey)):    {},
		strings.ToLower(strings.TrimSpace(settings.RecordingFinishHotkey)): {},
		strings.ToLower(strings.TrimSpace(settings.EmergencyStopHotkey)):   {},
	}
}

func isOperatorSkillFKey(key string) bool {
	switch key {
	case "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8":
		return true
	default:
		return false
	}
}

func isOperatorBeltKey(key string) bool {
	if len(key) != 1 {
		return false
	}
	switch key[0] {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
		'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'`', '.', '-', ']':
		return true
	default:
		return false
	}
}

func pointerToOperatorSettings(settings OperatorSettings) *OperatorSettings {
	clone := cloneOperatorSettings(settings)
	return &clone
}

func mustMarshalOperatorSettings(settings OperatorSettings) []byte {
	data, err := yaml.Marshal(settings)
	if err != nil {
		panic(fmt.Sprintf("marshal validated operator settings: %v", err))
	}
	return data
}
