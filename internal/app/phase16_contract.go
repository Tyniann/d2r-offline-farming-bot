package app

import (
	"fmt"
	"sort"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// Phase16D2SPrefixLength ist die kleinste feste Präfixlänge, die im D2R-v105-Layout Magic, Version, Klasse und den ersten vollständigen Namensslot umfasst.
	Phase16D2SPrefixLength = 315
	// Phase16D2SMagic ist die Little-Endian-Signatur eines D2S-Spielstands.
	Phase16D2SMagic uint32 = 0xAA55AA55
	// Phase16D2SVersion105 ist die lokal charakterisierte und für Phase 16 freigegebene Save-Version.
	Phase16D2SVersion105 uint32 = 105
	// Phase16D2SClassOffset ist der Klassenbyte-Offset im freigegebenen v105-Layout.
	Phase16D2SClassOffset = 0x18
	// Phase16D2SNameOffset ist der Beginn des ersten v105-Namensslots.
	Phase16D2SNameOffset = 0x12B
	// Phase16D2SNameLength begrenzt den ausgewerteten nullterminierten Charakternamen.
	Phase16D2SNameLength = 16
)

// Phase16D2SClassMapping bindet eine belegte D2S-ID an die kanonische World-Klasse.
type Phase16D2SClassMapping struct {
	// ID ist das unveränderte Klassenbyte aus dem v105-Präfix.
	ID byte
	// Class ist die einzige kanonische Projektion der ID.
	Class world.CharacterClass
}

// Phase16D2SClasses liefert die vollständige belegte D2R-v105-Klassenzuordnung.
func Phase16D2SClasses() []Phase16D2SClassMapping {
	return []Phase16D2SClassMapping{
		{ID: 0, Class: world.CharacterClassAmazon},
		{ID: 1, Class: world.CharacterClassSorceress},
		{ID: 2, Class: world.CharacterClassNecromancer},
		{ID: 3, Class: world.CharacterClassPaladin},
		{ID: 4, Class: world.CharacterClassBarbarian},
		{ID: 5, Class: world.CharacterClassDruid},
		{ID: 6, Class: world.CharacterClassAssassin},
		{ID: 7, Class: world.CharacterClassWarlock},
	}
}

// Phase16D2SAllowedVersions liefert ausschließlich lokal charakterisierte Save-Versionen.
func Phase16D2SAllowedVersions() []uint32 {
	return []uint32{Phase16D2SVersion105}
}

// Phase16ContractOwner benennt den einzigen Owner einer Charakter-Setup-Verantwortung.
type Phase16ContractOwner struct {
	// Responsibility ist der stabile maschinenlesbare Verantwortungsbereich.
	Responsibility string
	// Owner ist die einzige schreibende oder auswertende Autorität.
	Owner string
	// Boundary beschreibt die harte Grenze zu anderen Komponenten.
	Boundary string
}

// Phase16ContractOwners liefert die verbindlichen Datei-, Persistenz-, Safety- und Projektionsgrenzen.
func Phase16ContractOwners() []Phase16ContractOwner {
	return []Phase16ContractOwner{
		{Responsibility: "d2s_read_and_header_evaluation", Owner: "internal/app", Boundary: "bounded read-only v105 prefix without external parser"},
		{Responsibility: "known_character_class_mapping", Owner: "internal/app and internal/world", Boundary: "one mapping onto world.CharacterClass"},
		{Responsibility: "profile_approval_and_default", Owner: "config.ProfileConfig.Setup", Boundary: "developer-owned metadata on the existing combat profile"},
		{Responsibility: "selected_character_profile", Owner: "OperatorSettingsStore", Boundary: "character-scoped class and profile pair"},
		{Responsibility: "pickit_assignment", Owner: "PickitAssignmentStore", Boundary: "character and run scoped ordered profile chain"},
		{Responsibility: "selection_image", Owner: "Go Core PNG writer", Boundary: "existing config root and 210x60 crop"},
		{Responsibility: "safety_and_input", Owner: "Go Core", Boundary: "existing supervisor and guarded input controller"},
		{Responsibility: "user_facing_copy", Owner: "React reason projection", Boundary: "central exhaustive mapping without raw codes"},
	}
}

// Phase16CharacterReasonCode ist ein stabiler maschinenlesbarer Setup-Grund.
type Phase16CharacterReasonCode string

const (
	// Phase16ReasonCharacterSaveMissing bezeichnet einen konfigurierten Charakter ohne regulären Save.
	Phase16ReasonCharacterSaveMissing Phase16CharacterReasonCode = "character_save_missing"
	// Phase16ReasonCharacterSaveUnreadable bezeichnet einen nicht sicher lesbaren Save.
	Phase16ReasonCharacterSaveUnreadable Phase16CharacterReasonCode = "character_save_unreadable"
	// Phase16ReasonCharacterSaveHeaderInvalid bezeichnet einen strukturell ungültigen begrenzten Header.
	Phase16ReasonCharacterSaveHeaderInvalid Phase16CharacterReasonCode = "character_save_header_invalid"
	// Phase16ReasonCharacterSaveVersionUnsupported bezeichnet eine Save-Version außerhalb der Allowlist.
	Phase16ReasonCharacterSaveVersionUnsupported Phase16CharacterReasonCode = "character_save_version_unsupported"
	// Phase16ReasonCharacterSaveNameMismatch bezeichnet abweichende Datei- und Headernamen.
	Phase16ReasonCharacterSaveNameMismatch Phase16CharacterReasonCode = "character_save_name_mismatch"
	// Phase16ReasonCharacterSaveNameConflict bezeichnet case-insensitiv kollidierende Save-Namen.
	Phase16ReasonCharacterSaveNameConflict Phase16CharacterReasonCode = "character_save_name_conflict"
	// Phase16ReasonCharacterClassUnknown bezeichnet eine Klassen-ID außerhalb des bekannten Bereichs.
	Phase16ReasonCharacterClassUnknown Phase16CharacterReasonCode = "character_class_unknown"
	// Phase16ReasonCharacterClassUnsupported bezeichnet eine bekannte Klasse ohne freigegebenes Setup-Profil.
	Phase16ReasonCharacterClassUnsupported Phase16CharacterReasonCode = "character_class_unsupported"
	// Phase16ReasonCharacterProfileMissing bezeichnet eine unterstützte Klasse ohne persistiertes Profil.
	Phase16ReasonCharacterProfileMissing Phase16CharacterReasonCode = "character_profile_missing"
	// Phase16ReasonCharacterProfileIncompatible bezeichnet ein nicht zur Save-Klasse passendes persistiertes Profil.
	Phase16ReasonCharacterProfileIncompatible Phase16CharacterReasonCode = "character_profile_incompatible"
	// Phase16ReasonCharacterProfileRunIncompatible bezeichnet ein nicht zum gewählten Run passendes Charakterprofil.
	Phase16ReasonCharacterProfileRunIncompatible Phase16CharacterReasonCode = "character_profile_run_incompatible"
	// Phase16ReasonCharacterAnchorMissing bezeichnet einen fehlenden oder formal ungültigen Auswahlbeleg.
	Phase16ReasonCharacterAnchorMissing Phase16CharacterReasonCode = "character_anchor_missing"
	// Phase16ReasonCharacterAnchorExists bezeichnet einen bereits gültigen, nicht überschreibbaren Auswahlbeleg.
	Phase16ReasonCharacterAnchorExists Phase16CharacterReasonCode = "character_anchor_exists"
)

// Phase16CharacterReasonCodes liefert die vollständige geordnete Reason-Code-Menge.
func Phase16CharacterReasonCodes() []Phase16CharacterReasonCode {
	return []Phase16CharacterReasonCode{
		Phase16ReasonCharacterSaveMissing,
		Phase16ReasonCharacterSaveUnreadable,
		Phase16ReasonCharacterSaveHeaderInvalid,
		Phase16ReasonCharacterSaveVersionUnsupported,
		Phase16ReasonCharacterSaveNameMismatch,
		Phase16ReasonCharacterSaveNameConflict,
		Phase16ReasonCharacterClassUnknown,
		Phase16ReasonCharacterClassUnsupported,
		Phase16ReasonCharacterProfileMissing,
		Phase16ReasonCharacterProfileIncompatible,
		Phase16ReasonCharacterProfileRunIncompatible,
		Phase16ReasonCharacterAnchorMissing,
		Phase16ReasonCharacterAnchorExists,
	}
}

// Phase16DefaultPickitChains liefert unabhängige Kopien der Entwickler-Defaults je Run.
// Die String-Keys stammen aus [config.DefaultCharacterSetupPickitChains], damit
// Config-Default, Validierung, Setup-Preview und Assignment-Erzeugung nicht driften.
func Phase16DefaultPickitChains() map[tasks.RunID][]string {
	configured := config.DefaultCharacterSetupPickitChains()
	chains := make(map[tasks.RunID][]string, len(configured))
	for rawRunID, profiles := range configured {
		chains[tasks.RunID(rawRunID)] = append([]string(nil), profiles...)
	}
	return chains
}

// Phase16DefaultPickitRunIDs returns the default pickit run keys in stable ID order.
func Phase16DefaultPickitRunIDs() []tasks.RunID {
	chains := Phase16DefaultPickitChains()
	ids := make([]tasks.RunID, 0, len(chains))
	for runID := range chains {
		ids = append(ids, runID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// ValidateCharacterSetupConfig prüft die vollständige Entwicklerkonfiguration gegen Run-Registry und Pickit-Profile.
func ValidateCharacterSetupConfig(cfg *config.Config, profiles *PickitProfileService) error {
	if cfg == nil || profiles == nil {
		return fmt.Errorf("character setup config and pickit profile service are required")
	}
	expected := Phase16DefaultPickitChains()
	if len(cfg.CharacterSetup.PickitDefaults) != len(expected) {
		return fmt.Errorf("character_setup.pickit_defaults must contain exactly the configured default runs %v", Phase16DefaultPickitRunIDs())
	}
	registry := tasks.DefaultRunRegistry()
	for runID, expectedProfiles := range expected {
		if _, ok := registry.Definition(runID); !ok {
			return fmt.Errorf("character_setup.pickit_defaults.%s references an unregistered run", runID)
		}
		configured, ok := cfg.CharacterSetup.PickitDefaults[string(runID)]
		if !ok || !equalStrings(configured, expectedProfiles) {
			return fmt.Errorf("character_setup.pickit_defaults.%s must equal %v", runID, expectedProfiles)
		}
		for _, profileID := range configured {
			if _, err := profiles.Get(profileID); err != nil {
				return fmt.Errorf("character_setup.pickit_defaults.%s profile %q: %w", runID, profileID, err)
			}
		}
	}
	for rawRunID := range cfg.CharacterSetup.PickitDefaults {
		if _, ok := registry.Definition(tasks.RunID(rawRunID)); !ok {
			return fmt.Errorf("character_setup.pickit_defaults.%s references an unregistered run", rawRunID)
		}
	}
	return nil
}

// Phase16NonGoals liefert die ausdrücklich ausgeschlossenen Architekturpfade.
func Phase16NonGoals() []string {
	return []string{
		"full_d2s_parser_external_parser_dependency_checksum_or_save_mutation",
		"separate_class_mapping_profile_registry_plugin_or_default_editor",
		"legacy_migration_second_store_database_cross_store_transaction_or_rollback",
		"merge_replace_sort_or_duplicate_existing_pickit_assignments",
		"workflow_builder_command_bus_or_renderer_filesystem_access",
		"settings_page_combat_profile_editor",
		"ocr_character_name_recognition_blind_click_scaling_or_capture_scroll",
		"combat_timing_skill_or_run_value_refactor",
		"filesystem_watcher_background_polling_header_cache_or_index",
		"real_savegame_hexdump_or_save_fixture_in_repository",
	}
}
