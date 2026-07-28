package app

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

var phase16CharacterAnchorSize = image.Pt(210, 60)

const (
	// CharacterReasonSaveMissing marks a configured character without a regular save filename.
	CharacterReasonSaveMissing = string(Phase16ReasonCharacterSaveMissing)
	// CharacterReasonSaveUnreadable bezeichnet einen nicht sicher lesbaren Save.
	CharacterReasonSaveUnreadable = string(Phase16ReasonCharacterSaveUnreadable)
	// CharacterReasonSaveHeaderInvalid bezeichnet einen strukturell ungültigen begrenzten Header.
	CharacterReasonSaveHeaderInvalid = string(Phase16ReasonCharacterSaveHeaderInvalid)
	// CharacterReasonSaveVersionUnsupported bezeichnet eine Save-Version außerhalb der Allowlist.
	CharacterReasonSaveVersionUnsupported = string(Phase16ReasonCharacterSaveVersionUnsupported)
	// CharacterReasonSaveNameMismatch bezeichnet abweichende Datei- und Headernamen.
	CharacterReasonSaveNameMismatch = string(Phase16ReasonCharacterSaveNameMismatch)
	// CharacterReasonSaveNameConflict bezeichnet case-insensitiv kollidierende Save-Namen.
	CharacterReasonSaveNameConflict = string(Phase16ReasonCharacterSaveNameConflict)
	// CharacterReasonClassUnknown bezeichnet eine unbekannte D2S-Klassen-ID.
	CharacterReasonClassUnknown = string(Phase16ReasonCharacterClassUnknown)
	// CharacterReasonClassUnsupported bezeichnet eine bekannte Klasse ohne freigegebenes Setup-Profil.
	CharacterReasonClassUnsupported = string(Phase16ReasonCharacterClassUnsupported)
	// CharacterReasonProfileMissing bezeichnet eine unterstützte Klasse ohne persistiertes Charakterprofil.
	CharacterReasonProfileMissing = string(Phase16ReasonCharacterProfileMissing)
	// CharacterReasonProfileIncompatible bezeichnet ein zur Save-Klasse unpassendes persistiertes Profil.
	CharacterReasonProfileIncompatible = string(Phase16ReasonCharacterProfileIncompatible)
	// CharacterReasonAnchorMissing marks a character without a valid versioned selection anchor.
	CharacterReasonAnchorMissing = string(Phase16ReasonCharacterAnchorMissing)
)

// CharacterCatalogEntry is one immutable read-only offline character projection.
type CharacterCatalogEntry struct {
	Name          string
	Slug          string
	ExpectedClass string
	CombatProfile string
	Selectable    bool
	Reasons       []string
	AnchorPath    string
}

// CharacterCatalog contains the deterministic visible save projection and its process-local revision.
type CharacterCatalog struct {
	// Revision ändert sich pro Prozess nur bei einer fachlich neuen Projektion.
	Revision uint64
	// Characters enthält defensive, deterministisch sortierte Einträge.
	Characters []CharacterCatalogEntry
}

// CharacterCatalogError bindet einen globalen Reloadfehler an einen stabilen Reason-Code.
type CharacterCatalogError struct {
	// Reason ist der stabile globale Reload-Grund.
	Reason Phase16CharacterReasonCode
	// Err ist die technische Ursache ohne Dateiinhalte.
	Err error
}

// Error liefert Reason-Code und technische Ursache.
func (e *CharacterCatalogError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Reason, e.Err)
}

// Unwrap erhält die technische Ursache.
func (e *CharacterCatalogError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ResolveCharacterCatalog liest den begrenzten Header direkter regulärer lokaler Offline-Saves.
func ResolveCharacterCatalog(cfg *config.Config) (CharacterCatalog, error) {
	if cfg == nil {
		return CharacterCatalog{}, fmt.Errorf("character catalog requires config")
	}
	root, err := savedGamesDirectory()
	if err != nil {
		return CharacterCatalog{}, fmt.Errorf("resolve Windows Saved Games directory: %w", err)
	}
	return resolveCharacterCatalogAt(cfg, filepath.Join(root, "Diablo II Resurrected"))
}

func resolveCharacterCatalogAt(cfg *config.Config, saveDirectory string) (CharacterCatalog, error) {
	saves, err := readCharacterSaves(saveDirectory)
	if err != nil && !os.IsNotExist(err) && !savedGamesPathMissing(err) {
		return CharacterCatalog{}, err
	}
	if err != nil {
		saves = nil
	}
	configured := strings.TrimSpace(cfg.Session.Character)
	if configured != "" && !containsCharacterSave(saves, configured) {
		saves = append(saves, characterSaveCatalogItem{name: configured, reason: Phase16ReasonCharacterSaveMissing})
	}
	sort.Slice(saves, func(i, j int) bool { return strings.ToLower(saves[i].name) < strings.ToLower(saves[j].name) })
	loadedFrom, err := filepath.Abs(cfg.LoadedFrom)
	if err != nil {
		return CharacterCatalog{}, fmt.Errorf("resolve character catalog config path: %w", err)
	}
	configDirectory := filepath.Dir(loadedFrom)
	globalAnchorsValid := validPNGSize(filepath.Join(configDirectory, "ui", "character-play.png"), image.Pt(203, 47)) &&
		validPNGSize(filepath.Join(configDirectory, "ui", "difficulty-dialog.png"), image.Pt(180, 175))
	entries := make([]CharacterCatalogEntry, 0, len(saves))
	for _, save := range saves {
		slug := characterSlug(save.name)
		entry := CharacterCatalogEntry{Name: save.name, Slug: slug, AnchorPath: filepath.Join(configDirectory, "ui", "characters", slug+"-selected.png")}
		if save.reason != "" {
			entry.Reasons = []string{string(save.reason)}
			entries = append(entries, entry)
			continue
		}
		entry.ExpectedClass = save.header.Class.String()
		if !hasEnabledSetupProfile(cfg.Profiles, entry.ExpectedClass) {
			entry.Reasons = []string{CharacterReasonClassUnsupported}
			entries = append(entries, entry)
			continue
		}

		// Der reine Save-Resolver kennt bewusst keine OperatorSettings. Erst der
		// Store projiziert das autoritative Profil; ohne diese Projektion bleibt
		// ein unterstützter Save fail-closed und erbt nichts aus einem Run.
		entry.Reasons = append(entry.Reasons, CharacterReasonProfileMissing)
		characterAnchorValid := validPNGSize(entry.AnchorPath, phase16CharacterAnchorSize)
		if !globalAnchorsValid || !characterAnchorValid {
			entry.Reasons = append(entry.Reasons, CharacterReasonAnchorMissing)
		}
		entry.Reasons = uniqueSorted(entry.Reasons)
		entries = append(entries, entry)
	}
	return CharacterCatalog{Revision: 1, Characters: entries}, nil
}

type characterSaveCatalogItem struct {
	name   string
	header characterSaveHeader
	reason Phase16CharacterReasonCode
}

func readCharacterSaves(directory string) ([]characterSaveCatalogItem, error) {
	return readCharacterSavesWith(directory, os.ReadDir, os.Lstat, readCharacterSaveHeader)
}

func readCharacterSavesWith(
	directory string,
	readDirectory func(string) ([]os.DirEntry, error),
	lstat func(string) (os.FileInfo, error),
	readHeader func(string, string) (characterSaveHeader, error),
) ([]characterSaveCatalogItem, error) {
	entries, err := readDirectory(directory)
	if err != nil {
		return nil, fmt.Errorf("read offline save directory: %w", err)
	}
	saves := make([]characterSaveCatalogItem, 0, len(entries))
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".d2s") {
			continue
		}
		filenameName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		name, validErr := validateOfflineCharacter(filenameName)
		if validErr != nil {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, infoErr := lstat(path)
		if infoErr != nil {
			if duplicateCharacterSave(seen, name) {
				return nil, &CharacterCatalogError{Reason: Phase16ReasonCharacterSaveNameConflict, Err: fmt.Errorf("duplicate save name %q", name)}
			}
			saves = append(saves, characterSaveCatalogItem{name: name, reason: Phase16ReasonCharacterSaveUnreadable})
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 || fileInfoIsReparsePoint(info) {
			continue
		}
		if duplicateCharacterSave(seen, name) {
			return nil, &CharacterCatalogError{Reason: Phase16ReasonCharacterSaveNameConflict, Err: fmt.Errorf("duplicate save name %q", name)}
		}
		header, readErr := readHeader(path, name)
		if readErr != nil {
			saves = append(saves, characterSaveCatalogItem{name: name, reason: characterSaveErrorReason(readErr)})
			continue
		}
		saves = append(saves, characterSaveCatalogItem{name: header.Name, header: header})
	}
	return saves, nil
}

func characterSlug(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func duplicateCharacterSave(seen map[string]struct{}, name string) bool {
	key := strings.ToLower(name)
	if _, duplicate := seen[key]; duplicate {
		return true
	}
	seen[key] = struct{}{}
	return false
}

func containsCharacterSave(saves []characterSaveCatalogItem, target string) bool {
	for _, save := range saves {
		if strings.EqualFold(save.name, target) {
			return true
		}
	}
	return false
}

func validPNGSize(path string, expected image.Point) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	metadata, err := png.DecodeConfig(file)
	return err == nil && metadata.Width == expected.X && metadata.Height == expected.Y
}

// CharacterCatalogStore hält genau die letzte erfolgreich veröffentlichte Katalogprojektion.
type CharacterCatalogStore struct {
	mu       sync.RWMutex
	resolve  func() (CharacterCatalog, error)
	cfg      *config.Config
	settings *OperatorSettingsStore
	current  CharacterCatalog
}

// NewCharacterCatalogStore baut die erste Katalogprojektion mit Revision 1 auf.
func NewCharacterCatalogStore(cfg *config.Config) (*CharacterCatalogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("character catalog store requires config")
	}
	store, err := newCharacterCatalogStore(func() (CharacterCatalog, error) {
		return ResolveCharacterCatalog(cfg)
	})
	if err != nil {
		return nil, err
	}
	store.cfg = cfg
	return store, nil
}

func newCharacterCatalogStore(resolve func() (CharacterCatalog, error)) (*CharacterCatalogStore, error) {
	if resolve == nil {
		return nil, fmt.Errorf("character catalog resolver is required")
	}
	initial, err := resolve()
	if err != nil {
		return nil, err
	}
	initial.Revision = 1
	return &CharacterCatalogStore{resolve: resolve, current: cloneCharacterCatalog(initial)}, nil
}

// Snapshot liefert eine defensive Kopie der letzten erfolgreichen Projektion.
func (s *CharacterCatalogStore) Snapshot() CharacterCatalog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneCharacterCatalog(s.current)
}

// BindOperatorSettings bindet die einzige persistente Profilautorität und veröffentlicht ihre erste Projektion.
func (s *CharacterCatalogStore) BindOperatorSettings(settings *OperatorSettingsStore) (CharacterCatalog, error) {
	if settings == nil {
		return CharacterCatalog{}, fmt.Errorf("character catalog operator settings are required")
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	return s.Reload()
}

// Reload veröffentlicht eine vollständig neu gelesene Projektion nur bei fachlicher Änderung.
func (s *CharacterCatalogStore) Reload() (CharacterCatalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := s.resolve()
	if err != nil {
		return cloneCharacterCatalog(s.current), err
	}
	next, err = s.projectOperatorSettings(next)
	if err != nil {
		return cloneCharacterCatalog(s.current), err
	}
	if reflect.DeepEqual(s.current.Characters, next.Characters) {
		return cloneCharacterCatalog(s.current), nil
	}
	next.Revision = s.current.Revision + 1
	s.current = cloneCharacterCatalog(next)
	return cloneCharacterCatalog(s.current), nil
}

func (s *CharacterCatalogStore) projectOperatorSettings(catalog CharacterCatalog) (CharacterCatalog, error) {
	if s.settings == nil {
		return catalog, nil
	}
	settings, err := s.settings.Snapshot()
	if err != nil {
		return CharacterCatalog{}, fmt.Errorf("load character catalog operator settings: %w", err)
	}
	for index := range catalog.Characters {
		entry := &catalog.Characters[index]
		if entry.ExpectedClass == "" || containsCharacterReason(entry.Reasons, CharacterReasonClassUnsupported) {
			continue
		}
		reasons := make([]string, 0, len(entry.Reasons)+1)
		for _, reason := range entry.Reasons {
			if reason != CharacterReasonProfileMissing && reason != CharacterReasonProfileIncompatible {
				reasons = append(reasons, reason)
			}
		}
		stored := settings.Characters[entry.Slug]
		switch {
		case stored.CharacterClass == "" && stored.CombatProfile == "":
			reasons = append(reasons, CharacterReasonProfileMissing)
		case stored.CharacterClass != entry.ExpectedClass || !isEnabledSetupProfile(s.cfg.Profiles, entry.ExpectedClass, stored.CombatProfile):
			reasons = append(reasons, CharacterReasonProfileIncompatible)
		default:
			entry.CombatProfile = stored.CombatProfile
		}
		entry.Reasons = uniqueSorted(reasons)
		entry.Selectable = len(entry.Reasons) == 0
	}
	return catalog, nil
}

func hasEnabledSetupProfile(profiles config.ProfilesConfig, characterClass string) bool {
	for _, profile := range profiles {
		if profile.Setup.Enabled && profile.CharacterClass == characterClass {
			return true
		}
	}
	return false
}

func isEnabledSetupProfile(profiles config.ProfilesConfig, characterClass, profileID string) bool {
	profile, ok := profiles[profileID]
	return ok && profile.Setup.Enabled && profile.CharacterClass == characterClass
}

func containsCharacterReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func cloneCharacterCatalog(source CharacterCatalog) CharacterCatalog {
	result := CharacterCatalog{Revision: source.Revision, Characters: make([]CharacterCatalogEntry, len(source.Characters))}
	copy(result.Characters, source.Characters)
	for index := range result.Characters {
		result.Characters[index].Reasons = append([]string(nil), source.Characters[index].Reasons...)
	}
	return result
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
