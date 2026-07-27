package app

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

const (
	// CharacterReasonSaveMissing marks a configured character without a regular save filename.
	CharacterReasonSaveMissing = "character_save_missing"
	// CharacterReasonUnconfigured marks a visible save without a configured session/profile context.
	CharacterReasonUnconfigured = "character_unconfigured"
	// CharacterReasonAnchorMissing marks a character without a valid versioned selection anchor.
	CharacterReasonAnchorMissing = "character_anchor_missing"
)

// CharacterCatalogEntry is one immutable read-only offline character projection.
type CharacterCatalogEntry struct {
	Name          string
	Slug          string
	ExpectedClass string
	Selectable    bool
	Reasons       []string
	AnchorPath    string
}

// CharacterCatalog contains the deterministic visible save-name projection.
type CharacterCatalog struct {
	Characters []CharacterCatalogEntry
}

// ResolveCharacterCatalog lists save filenames without opening or parsing any save.
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
	names, err := readCharacterSaveNames(saveDirectory)
	if err != nil && !os.IsNotExist(err) && !savedGamesPathMissing(err) {
		return CharacterCatalog{}, err
	}
	configured := strings.TrimSpace(cfg.Session.Character)
	if configured != "" && !containsFold(names, configured) {
		names = append(names, configured)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	loadedFrom, err := filepath.Abs(cfg.LoadedFrom)
	if err != nil {
		return CharacterCatalog{}, fmt.Errorf("resolve character catalog config path: %w", err)
	}
	configDirectory := filepath.Dir(loadedFrom)
	globalAnchorsValid := validPNGSize(filepath.Join(configDirectory, "ui", "character-play.png"), image.Pt(203, 47)) &&
		validPNGSize(filepath.Join(configDirectory, "ui", "difficulty-dialog.png"), image.Pt(180, 175))
	class := configuredCharacterClass(cfg)
	entries := make([]CharacterCatalogEntry, 0, len(names))
	for _, name := range names {
		slug := characterSlug(name)
		entry := CharacterCatalogEntry{Name: name, Slug: slug, AnchorPath: filepath.Join(configDirectory, "ui", "characters", slug+"-selected.png")}
		characterAnchorValid := validPNGSize(entry.AnchorPath, image.Pt(210, 60))
		// Ein namensgebundener Auswahlanker ist die explizite Konfiguration des
		// Charakters. `session.character` bleibt nur die Startvorauswahl; andernfalls
		// könnte ein frischer Root mit absichtlich leerer Vorauswahl nie über das
		// Onboarding eingerichtet werden.
		if class == "" || !characterAnchorValid {
			entry.Reasons = append(entry.Reasons, CharacterReasonUnconfigured)
		} else {
			entry.ExpectedClass = class
		}
		if !saveExistsRegular(saveDirectory, name) {
			entry.Reasons = append(entry.Reasons, CharacterReasonSaveMissing)
		}
		if !globalAnchorsValid || !characterAnchorValid {
			entry.Reasons = append(entry.Reasons, CharacterReasonAnchorMissing)
		}
		entry.Reasons = uniqueSorted(entry.Reasons)
		entry.Selectable = len(entry.Reasons) == 0
		entries = append(entries, entry)
	}
	return CharacterCatalog{Characters: entries}, nil
}

func readCharacterSaveNames(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read offline save directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".d2s") {
			continue
		}
		info, infoErr := os.Lstat(filepath.Join(directory, entry.Name()))
		if infoErr != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 || fileInfoIsReparsePoint(info) {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if _, validErr := validateOfflineCharacter(name); validErr != nil {
			continue
		}
		key := strings.ToLower(name)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names, nil
}

func characterSlug(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func configuredCharacterClass(cfg *config.Config) string {
	run, ok := cfg.Runs.Run(cfg.Session.Run)
	if !ok {
		return ""
	}
	profile, ok := cfg.Profiles[run.Combat.Profile]
	if !ok {
		return ""
	}
	return profile.CharacterClass
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

func saveExistsRegular(directory, name string) bool {
	info, err := os.Lstat(filepath.Join(directory, name+".d2s"))
	return err == nil && info.Mode().IsRegular() && info.Mode()&fs.ModeSymlink == 0 && !fileInfoIsReparsePoint(info)
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
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
