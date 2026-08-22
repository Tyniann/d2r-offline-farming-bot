package api

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
)

// RunAvailabilities resolves the read-only run catalog for one character and difficulty.
func (b *LiveBackend) RunAvailabilities(character, difficulty string) (RunAvailabilitiesDTO, error) {
	if strings.TrimSpace(character) == "" {
		return RunAvailabilitiesDTO{}, &commandError{code: "request_invalid", params: map[string]any{"field": "character"}}
	}
	catalog, err := b.reloadCharacterCatalog()
	if err != nil {
		return RunAvailabilitiesDTO{}, err
	}
	entry, ok := findCharacterCatalogEntry(catalog.Characters, character)
	if !ok {
		return RunAvailabilitiesDTO{}, &commandError{code: app.CharacterReasonSaveMissing}
	}
	if strings.TrimSpace(difficulty) == "" {
		difficulty = b.cfg.Session.Difficulty
	}
	runs, err := b.resolveRunsForEntry(entry, difficulty)
	if err != nil {
		return RunAvailabilitiesDTO{}, err
	}
	return RunAvailabilitiesDTO{Character: entry.Name, Difficulty: difficulty, Runs: runs}, nil
}

func (b *LiveBackend) resolveRunsForEntry(entry app.CharacterCatalogEntry, difficulty string) ([]RunCatalogEntry, error) {
	ctx := app.BuildCharacterRunAvailabilityContext(b.cfg, entry, difficulty)
	report, err := app.ResolveRunAvailabilities(b.cfg, ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve run availabilities: %w", err)
	}
	return runCatalogEntries(report, b.cfg), nil
}

func (b *LiveBackend) farmQueueValidationContext(character, difficulty string, revision uint64) app.FarmQueueValidationContext {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ctx := app.FarmQueueValidationContext{Character: character, Difficulty: difficulty, CatalogRevision: revision}
	if entry, ok := b.bootstrap.character(character); ok {
		built := app.BuildCharacterRunAvailabilityContext(b.cfg, entry, difficulty)
		ctx.CharacterClass = built.CharacterClass
		ctx.CombatProfile = built.CombatProfile
	}
	return ctx
}

func findCharacterCatalogEntry(characters []app.CharacterCatalogEntry, name string) (app.CharacterCatalogEntry, bool) {
	for _, candidate := range characters {
		if strings.EqualFold(candidate.Name, strings.TrimSpace(name)) || strings.EqualFold(candidate.Slug, strings.TrimSpace(name)) {
			return candidate, true
		}
	}
	return app.CharacterCatalogEntry{}, false
}
