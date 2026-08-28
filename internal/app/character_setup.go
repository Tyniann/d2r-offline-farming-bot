package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

// CharacterSetupState beschreibt den vollständigen, Core-berechneten Einrichtungsstand.
type CharacterSetupState string

const (
	CharacterSetupBlocked     CharacterSetupState = "blocked"
	CharacterSetupNeedsSetup  CharacterSetupState = "needs_setup"
	CharacterSetupNeedsAnchor CharacterSetupState = "needs_anchor"
	CharacterSetupReady       CharacterSetupState = "ready"
)

// CharacterAnchorState beschreibt ausschließlich den namensgebundenen Bildbeleg.
type CharacterAnchorState string

const (
	CharacterAnchorMissing CharacterAnchorState = "missing"
	CharacterAnchorReady   CharacterAnchorState = "ready"
	CharacterAnchorInvalid CharacterAnchorState = "invalid"
)

// CharacterSetupProfile ist ein freigegebenes Kampfprofil ohne interne Verhaltensparameter.
type CharacterSetupProfile struct {
	ID                 string
	DisplayName        string
	IsDefault          bool
	IsSelected         bool
	StandardAttack     string
	RequiredSkills     []CharacterSetupRequiredSkill
	OptionalSkillPairs []CharacterSetupOptionalSkillPair
	RequiresMercenary  bool
	BindingsReady      bool
	BindingReasons     []string
	SupportedRuns      []string
	// DefaultBeltLayout is derived from combat_profiles.*.resources belt columns.
	DefaultBeltLayout OperatorBeltLayout
	// BeltLayout is the effective operator override or DefaultBeltLayout.
	BeltLayout OperatorBeltLayout
	// DefaultHealingRestock is `town.thresholds.healing` from the base config.
	DefaultHealingRestock int
	// DefaultManaRestock is `town.thresholds.mana` from the base config.
	DefaultManaRestock int
}

// CharacterSetupRequiredSkill is one ordered, labeled profile skill for read-only Setup/API.
type CharacterSetupRequiredSkill struct {
	Skill   string
	SkillID uint16
	Slot    string
}

// CharacterSetupOptionalSkillPair is one Core-defined all-or-nothing binding pair.
type CharacterSetupOptionalSkillPair struct {
	Skills []CharacterSetupRequiredSkill
}

// CharacterSetupPickitDefault projiziert eine feste Run-Kette mit lesbaren Profilnamen.
type CharacterSetupPickitDefault struct {
	RunID        tasks.RunID
	ProfileNames []string
	State        string
}

// CharacterSetupPreview ist der pfadfreie, vollständig vom Core berechnete Setupstand.
type CharacterSetupPreview struct {
	CatalogRevision          uint64
	OperatorSettingsRevision uint64
	PickitAssignmentRevision uint64
	CharacterName            string
	CharacterSlug            string
	CharacterClass           string
	Supported                bool
	Profiles                 []CharacterSetupProfile
	SelectedProfileID        string
	DefaultProfileID         string
	PickitDefaults           []CharacterSetupPickitDefault
	AnchorState              CharacterAnchorState
	SetupState               CharacterSetupState
	Reasons                  []string
}

// CharacterSetupConfirmRequest bindet einen Setupwrite an alle drei fachlichen Revisionen.
type CharacterSetupConfirmRequest struct {
	Character                string
	ProfileID                string
	ExpectedCatalogRevision  uint64
	ExpectedSettingsRevision uint64
	ExpectedPickitRevision   uint64
}

// CharacterSetupCaptureRequest bindet die Bilderfassung an Charakter und Katalogrevision.
type CharacterSetupCaptureRequest struct {
	Character               string
	ExpectedCatalogRevision uint64
}

// CharacterSetupError enthält einen stabilen maschinenlesbaren Fehlercode.
type CharacterSetupError struct {
	Code    string
	Partial bool
	Err     error
}

func (e *CharacterSetupError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *CharacterSetupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CharacterSetupCaptureFunc erfasst genau den bereits markierten Auswahlbereich und veröffentlicht ihn am internen Zielpfad.
type CharacterSetupCaptureFunc func(context.Context, string) error

// CharacterSetupDependencies bündelt ausschließlich die bestehenden Autoritäten des schmalen Setup-Ablaufs.
type CharacterSetupDependencies struct {
	Config            *config.Config
	Catalog           *CharacterCatalogStore
	Settings          *OperatorSettingsStore
	PickitAssignments *PickitAssignmentStore
	PickitProfiles    *PickitProfileService
	Capture           CharacterSetupCaptureFunc
}

// CharacterSetupService orchestriert Preview, Confirm und Capture ohne eigene Persistenz.
type CharacterSetupService struct {
	cfg         *config.Config
	catalog     *CharacterCatalogStore
	settings    *OperatorSettingsStore
	assignments *PickitAssignmentStore
	profiles    *PickitProfileService
	capture     CharacterSetupCaptureFunc
}

// SetCapture bindet den vorhandenen Runtime-Capturepfad nach dessen Desktop-Wiring.
func (s *CharacterSetupService) SetCapture(capture CharacterSetupCaptureFunc) {
	s.capture = capture
}

// NewCharacterSetupService erstellt den kleinen Orchestrator über den vorhandenen Stores.
func NewCharacterSetupService(deps CharacterSetupDependencies) (*CharacterSetupService, error) {
	if deps.Config == nil || deps.Catalog == nil || deps.Settings == nil || deps.PickitAssignments == nil || deps.PickitProfiles == nil {
		return nil, fmt.Errorf("character setup requires config, catalog, operator settings and pickit stores")
	}
	if err := ValidateCharacterSetupConfig(deps.Config, deps.PickitProfiles); err != nil {
		return nil, err
	}
	return &CharacterSetupService{
		cfg: deps.Config, catalog: deps.Catalog, settings: deps.Settings,
		assignments: deps.PickitAssignments, profiles: deps.PickitProfiles, capture: deps.Capture,
	}, nil
}

// Preview liest Katalog und Stores frisch und berechnet den vollständigen Setupstand.
func (s *CharacterSetupService) Preview(character string) (CharacterSetupPreview, error) {
	catalog, err := s.catalog.Reload()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	settings, err := s.settings.Snapshot()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	assignments, err := s.assignments.Snapshot()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	entry, ok := findCharacterCatalogEntry(catalog, character)
	if !ok {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase16ReasonCharacterSaveMissing), Err: fmt.Errorf("character %q is not in the current catalog", character)}
	}
	return s.buildPreview(catalog, settings, assignments, entry)
}

// Confirm persistiert zuerst Klasse und Profil und ergänzt danach ausschließlich fehlende Pickit-Defaults.
func (s *CharacterSetupService) Confirm(request CharacterSetupConfirmRequest) (CharacterSetupPreview, error) {
	catalog, err := s.catalog.Reload()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	settings, err := s.settings.Snapshot()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	assignments, err := s.assignments.Snapshot()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	if catalog.Revision != request.ExpectedCatalogRevision || settings.Revision != request.ExpectedSettingsRevision || assignments.Revision != request.ExpectedPickitRevision {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase15ReasonConfigRevisionConflict), Err: fmt.Errorf("character setup revisions changed")}
	}
	entry, ok := findCharacterCatalogEntry(catalog, request.Character)
	if !ok {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase16ReasonCharacterSaveMissing), Err: fmt.Errorf("character is no longer available")}
	}
	preview, err := s.buildPreview(catalog, settings, assignments, entry)
	if err != nil {
		return CharacterSetupPreview{}, err
	}
	if !preview.Supported || preview.CharacterClass == "" {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase16ReasonCharacterClassUnsupported), Err: fmt.Errorf("character class is not enabled for setup")}
	}
	profileID := strings.TrimSpace(request.ProfileID)
	if profileID == "" {
		if len(preview.Profiles) != 1 {
			return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase16ReasonCharacterProfileMissing), Err: fmt.Errorf("an explicit profile is required")}
		}
		profileID = preview.DefaultProfileID
	}
	if !containsSetupProfile(preview.Profiles, profileID) {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase16ReasonCharacterProfileIncompatible), Err: fmt.Errorf("profile %q is unavailable for class %s", profileID, preview.CharacterClass)}
	}
	change, err := s.settings.AssignCharacterProfile(entry.Name, preview.CharacterClass, profileID, request.ExpectedSettingsRevision)
	if err != nil {
		return CharacterSetupPreview{}, setupStoreError(err, false)
	}
	updatedAssignments, err := s.assignments.EnsureMissingDefaults(entry.Name, phase16ConfiguredDefaults(s.cfg), request.ExpectedPickitRevision)
	if err != nil {
		return CharacterSetupPreview{}, setupStoreError(err, change.Settings.Revision != settings.Revision)
	}
	updatedCatalog, err := s.catalog.Reload()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	updatedEntry, ok := findCharacterCatalogEntry(updatedCatalog, entry.Name)
	if !ok {
		return CharacterSetupPreview{}, setupUnavailable(fmt.Errorf("character disappeared after setup"))
	}
	return s.buildPreview(updatedCatalog, change.Settings, updatedAssignments, updatedEntry)
}

// Capture prüft den aktuellen Setupstand, erfasst ohne Navigation und lädt den Katalog danach neu.
func (s *CharacterSetupService) Capture(ctx context.Context, request CharacterSetupCaptureRequest) (CharacterSetupPreview, error) {
	preview, err := s.Preview(request.Character)
	if err != nil {
		return CharacterSetupPreview{}, err
	}
	if preview.CatalogRevision != request.ExpectedCatalogRevision {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: string(Phase15ReasonConfigRevisionConflict), Err: fmt.Errorf("character catalog revision changed")}
	}
	if preview.SetupState != CharacterSetupNeedsAnchor {
		code := string(Phase16ReasonCharacterProfileMissing)
		if preview.AnchorState == CharacterAnchorReady {
			code = string(Phase16ReasonCharacterAnchorExists)
		}
		return CharacterSetupPreview{}, &CharacterSetupError{Code: code, Err: fmt.Errorf("character is not ready for anchor capture")}
	}
	if s.capture == nil {
		return CharacterSetupPreview{}, setupUnavailable(fmt.Errorf("character capture runtime is unavailable"))
	}
	entry, _ := findCharacterCatalogEntry(s.catalog.Snapshot(), request.Character)
	if captureErr := s.capture(ctx, entry.AnchorPath); captureErr != nil {
		var setupErr *CharacterSetupError
		if errors.As(captureErr, &setupErr) {
			return CharacterSetupPreview{}, setupErr
		}
		return CharacterSetupPreview{}, &CharacterSetupError{Code: "character_capture_failed", Err: captureErr}
	}
	updated, err := s.catalog.Reload()
	if err != nil {
		return CharacterSetupPreview{}, setupUnavailable(err)
	}
	if refreshed, ok := findCharacterCatalogEntry(updated, request.Character); !ok || !validPNGSize(refreshed.AnchorPath, phase16CharacterAnchorSize) {
		return CharacterSetupPreview{}, &CharacterSetupError{Code: "character_capture_failed", Err: fmt.Errorf("published anchor failed re-read")}
	}
	return s.Preview(request.Character)
}

func (s *CharacterSetupService) buildPreview(catalog CharacterCatalog, settings OperatorSettings, assignments PickitAssignmentManifest, entry CharacterCatalogEntry) (CharacterSetupPreview, error) {
	result := CharacterSetupPreview{
		CatalogRevision: catalog.Revision, OperatorSettingsRevision: settings.Revision, PickitAssignmentRevision: assignments.Revision,
		CharacterName: entry.Name, CharacterSlug: entry.Slug, CharacterClass: entry.ExpectedClass,
		AnchorState: anchorState(entry),
	}
	for _, reason := range entry.Reasons {
		switch reason {
		case CharacterReasonProfileMissing, CharacterReasonAnchorMissing, CharacterReasonClassUnsupported:
		default:
			result.Reasons = append(result.Reasons, reason)
		}
	}
	for id, profile := range s.cfg.Profiles {
		if profile.Setup.Enabled && profile.CharacterClass == entry.ExpectedClass {
			bindings := OperatorProfileBindings{}
			if characterSettings, ok := settings.Characters[strings.ToLower(entry.Name)]; ok && characterSettings.ProfileBindings != nil {
				bindings = characterSettings.ProfileBindings[id]
			}
			setupProfile := CharacterSetupProfile{
				ID:                id,
				DisplayName:       profile.DisplayName,
				IsDefault:         profile.Setup.Default,
				StandardAttack:    strings.TrimSpace(profile.Combat.StandardAttack),
				RequiresMercenary: profile.RequiresMercenary,
				BindingsReady:     ProfileBindingsComplete(bindings, profile),
				SupportedRuns:     DefaultCombatStrategyRegistry().SupportedRuns(id),
			}
			setupProfile.DefaultBeltLayout = EffectiveBeltLayout(OperatorBeltLayout{}, profile.Resources)
			setupProfile.BeltLayout = EffectiveBeltLayout(bindings.BeltLayout, profile.Resources)
			setupProfile.DefaultHealingRestock = s.cfg.Town.Thresholds.Healing
			setupProfile.DefaultManaRestock = s.cfg.Town.Thresholds.Mana
			if !setupProfile.BindingsReady {
				setupProfile.BindingReasons = []string{string(QueueReasonProfileBindingsIncomplete)}
			}
			for _, skill := range profile.RequiredSkills {
				skillID, skillErr := memory.ParseSkillTestName(skill.Skill)
				if skillErr != nil {
					return CharacterSetupPreview{}, fmt.Errorf("character setup profile %q required skill %q: %w", id, skill.Skill, skillErr)
				}
				setupProfile.RequiredSkills = append(setupProfile.RequiredSkills, CharacterSetupRequiredSkill{Skill: skill.Skill, SkillID: skillID, Slot: skill.Slot})
			}
			for _, pair := range profile.OptionalSkillPairs {
				projected := CharacterSetupOptionalSkillPair{}
				for _, skill := range pair.Skills {
					catalog, ok := memory.LookupSkillByKey(skill.Skill)
					if !ok {
						return CharacterSetupPreview{}, fmt.Errorf("character setup profile %q optional skill %q is not in the catalog", id, skill.Skill)
					}
					projected.Skills = append(projected.Skills, CharacterSetupRequiredSkill{Skill: skill.Skill, SkillID: catalog.ID, Slot: skill.Slot})
				}
				setupProfile.OptionalSkillPairs = append(setupProfile.OptionalSkillPairs, projected)
			}
			result.Profiles = append(result.Profiles, setupProfile)
			if profile.Setup.Default {
				result.DefaultProfileID = id
			}
		}
	}
	sort.Slice(result.Profiles, func(i, j int) bool { return result.Profiles[i].ID < result.Profiles[j].ID })
	result.Supported = len(result.Profiles) > 0 && len(result.Reasons) == 0
	stored := settings.Characters[entry.Slug]
	if stored.CharacterClass != "" || stored.CombatProfile != "" {
		if stored.CharacterClass != entry.ExpectedClass || !containsSetupProfile(result.Profiles, stored.CombatProfile) {
			result.Reasons = append(result.Reasons, string(Phase16ReasonCharacterProfileIncompatible))
		} else {
			result.SelectedProfileID = stored.CombatProfile
			for index := range result.Profiles {
				result.Profiles[index].IsSelected = result.Profiles[index].ID == stored.CombatProfile
			}
		}
	}
	defaults := phase16ConfiguredDefaults(s.cfg)
	for _, runID := range Phase16DefaultPickitRunIDs() {
		item := CharacterSetupPickitDefault{RunID: runID, State: "missing"}
		for _, profileID := range defaults[runID] {
			profile, err := s.profiles.Get(profileID)
			if err != nil {
				return CharacterSetupPreview{}, setupUnavailable(err)
			}
			item.ProfileNames = append(item.ProfileNames, profile.Name)
		}
		if len(findPickitAssignment(assignments, entry.Name, runID)) > 0 {
			item.State = "ready"
		}
		result.PickitDefaults = append(result.PickitDefaults, item)
	}
	switch {
	case len(result.Reasons) > 0 || !result.Supported:
		result.SetupState = CharacterSetupBlocked
		if !result.Supported && len(result.Reasons) == 0 {
			result.Reasons = append(result.Reasons, string(Phase16ReasonCharacterClassUnsupported))
		}
	case result.SelectedProfileID == "":
		result.SetupState = CharacterSetupNeedsSetup
		result.Reasons = append(result.Reasons, string(Phase16ReasonCharacterProfileMissing))
	case result.AnchorState != CharacterAnchorReady:
		result.SetupState = CharacterSetupNeedsAnchor
		result.Reasons = append(result.Reasons, string(Phase16ReasonCharacterAnchorMissing))
	default:
		result.SetupState = CharacterSetupReady
	}
	result.Reasons = uniqueSorted(result.Reasons)
	return result, nil
}

func findCharacterCatalogEntry(catalog CharacterCatalog, name string) (CharacterCatalogEntry, bool) {
	for _, entry := range catalog.Characters {
		if strings.EqualFold(entry.Name, strings.TrimSpace(name)) {
			return entry, true
		}
	}
	return CharacterCatalogEntry{}, false
}

func containsSetupProfile(profiles []CharacterSetupProfile, id string) bool {
	for _, profile := range profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func anchorState(entry CharacterCatalogEntry) CharacterAnchorState {
	if validPNGSize(entry.AnchorPath, phase16CharacterAnchorSize) {
		return CharacterAnchorReady
	}
	if _, err := os.Stat(entry.AnchorPath); err == nil {
		return CharacterAnchorInvalid
	}
	return CharacterAnchorMissing
}

func phase16ConfiguredDefaults(cfg *config.Config) map[tasks.RunID][]string {
	result := make(map[tasks.RunID][]string, len(cfg.CharacterSetup.PickitDefaults))
	for runID, profiles := range cfg.CharacterSetup.PickitDefaults {
		result[tasks.RunID(runID)] = append([]string(nil), profiles...)
	}
	return result
}

func setupUnavailable(err error) error {
	return &CharacterSetupError{Code: "character_setup_unavailable", Err: err}
}

func setupStoreError(err error, partial bool) error {
	code := "character_setup_write_failed"
	var settingsErr *OperatorSettingsError
	if errors.Is(err, ErrPickitAssignmentRevisionConflict) || errors.As(err, &settingsErr) && settingsErr.Code == Phase15ReasonConfigRevisionConflict {
		code = string(Phase15ReasonConfigRevisionConflict)
	}
	return &CharacterSetupError{Code: code, Partial: partial, Err: err}
}
