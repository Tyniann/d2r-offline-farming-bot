package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/version"
)

// BootstrapBackend is the read-only Phase-11.2 projection used before live
// status binding. Mutating commands remain unavailable and cannot start input.
type BootstrapBackend struct {
	status     StatusDTO
	catalog    CatalogDTO
	characters map[string]app.CharacterCatalogEntry
	difficulty string
}

// NewBootstrapBackend resolves the initial read-only catalog without process
// attach, hotkeys, input or session startup.
func NewBootstrapBackend(cfg *config.Config) (*BootstrapBackend, error) {
	if cfg == nil {
		return nil, fmt.Errorf("bootstrap API backend requires config")
	}
	report, err := app.ResolveRunAvailabilities(cfg, app.RunAvailabilityContext{
		Character: cfg.Session.Character, Difficulty: cfg.Session.Difficulty, GameVersion: cfg.Memory.GameVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap run catalog: %w", err)
	}
	runs := runCatalogEntries(report)
	characterCatalog, err := app.ResolveCharacterCatalog(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve character catalog: %w", err)
	}
	characters := make([]CharacterCatalogEntry, 0, len(characterCatalog.Characters))
	characterEntries := make(map[string]app.CharacterCatalogEntry, len(characterCatalog.Characters))
	for _, character := range characterCatalog.Characters {
		characters = append(characters, CharacterCatalogEntry{Name: character.Name, Slug: character.Slug, ExpectedClass: character.ExpectedClass, Selectable: character.Selectable, Reasons: append([]string(nil), character.Reasons...)})
		characterEntries[character.Slug] = character
	}
	profiles := make([]ProfileCatalogEntry, 0, len(cfg.Profiles))
	for id, profile := range cfg.Profiles {
		profiles = append(profiles, ProfileCatalogEntry{ID: id, CharacterClass: profile.CharacterClass})
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	return &BootstrapBackend{
		status:  StatusDTO{CoreVersion: version.Version, AppVersion: version.Version, State: "idle", LifecyclePhase: "idle", D2R: D2RDTO{State: "detached"}, Compatibility: CompatibilityDTO{State: "not_detected", Reason: string(app.Phase15ReasonD2RVersionNotDetected), ExpectedVersion: cfg.Memory.GameVersion}, Input: InputDTO{Enabled: false}, World: WorldDTO{Phase: "unknown"}},
		catalog: CatalogDTO{Revision: 1, DefaultDifficulty: cfg.Session.Difficulty, Characters: characters, Difficulties: []DifficultyCatalogEntry{{ID: "normal", DisplayName: "Normal"}, {ID: "nightmare", DisplayName: "Alptraum"}, {ID: "hell", DisplayName: "Hölle"}}, Profiles: profiles, Runs: runs}, characters: characterEntries, difficulty: cfg.Session.Difficulty,
	}, nil
}

func (b *BootstrapBackend) character(name string) (app.CharacterCatalogEntry, bool) {
	entry, ok := b.characters[strings.ToLower(strings.TrimSpace(name))]
	return entry, ok
}

func runCatalogEntries(report app.RunsInspectReport) []RunCatalogEntry {
	runs := make([]RunCatalogEntry, 0, len(report.Runs))
	for _, run := range report.Runs {
		reasons := make([]string, len(run.Reasons))
		for i, reason := range run.Reasons {
			reasons[i] = string(reason)
		}
		runs = append(runs, RunCatalogEntry{RunID: string(run.RunID), DisplayName: run.DisplayName, Status: string(run.Status), Reasons: reasons})
	}
	return runs
}

// Status returns the immutable startup status.
func (b *BootstrapBackend) Status() StatusDTO {
	return b.status
}

// Catalog returns a defensive copy of the initial run catalog.
func (b *BootstrapBackend) Catalog() CatalogDTO {
	catalog := b.catalog
	catalog.Characters = append([]CharacterCatalogEntry(nil), b.catalog.Characters...)
	for i := range catalog.Characters {
		catalog.Characters[i].Reasons = append([]string(nil), catalog.Characters[i].Reasons...)
	}
	catalog.Difficulties = append([]DifficultyCatalogEntry(nil), b.catalog.Difficulties...)
	catalog.Profiles = append([]ProfileCatalogEntry(nil), b.catalog.Profiles...)
	catalog.Runs = append([]RunCatalogEntry(nil), b.catalog.Runs...)
	for i := range catalog.Runs {
		catalog.Runs[i].Reasons = append([]string(nil), catalog.Runs[i].Reasons...)
	}
	return catalog
}

// Command rejects every mutation until the live supervisor adapter is bound.
func (b *BootstrapBackend) Command(string, CommandRequest) (CommandResponse, error) {
	return CommandResponse{}, fmt.Errorf("UI command backend is not active")
}

// PreviewSelection rejects previews until the live lifecycle backend is bound.
func (b *BootstrapBackend) PreviewSelection(SelectionPreviewRequest) (SelectionPreviewDTO, error) {
	return SelectionPreviewDTO{}, fmt.Errorf("UI preview backend is not active")
}

// ValidateQueue rejects queue preflight until the live backend owns a confirmed selection.
func (b *BootstrapBackend) ValidateQueue(QueueValidationRequest) (QueueValidationDTO, error) {
	return QueueValidationDTO{}, fmt.Errorf("UI queue backend is not active")
}
