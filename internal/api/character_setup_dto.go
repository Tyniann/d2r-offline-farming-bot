package api

import "github.com/Tyniann/d2r-offline-farming-bot/internal/app"

// CharacterReloadDTO bestätigt einen synchronen, pfadfreien Katalogreload.
type CharacterReloadDTO struct {
	SchemaVersion int        `json:"schema_version"`
	Catalog       CatalogDTO `json:"catalog"`
}

// CharacterSetupPreviewRequest fordert den frischen Setupstand eines Namens an.
type CharacterSetupPreviewRequest struct {
	Character string `json:"character"`
}

// CharacterSetupProfileDTO beschreibt ein freigegebenes Profil.
type CharacterSetupProfileDTO struct {
	ID             string                            `json:"id"`
	DisplayName    string                            `json:"display_name"`
	IsDefault      bool                              `json:"is_default"`
	IsSelected     bool                              `json:"is_selected"`
	StandardAttack string                            `json:"standard_attack,omitempty"`
	RequiredSkills []CharacterSetupRequiredSkillDTO  `json:"required_skills,omitempty"`
	SupportedRuns  []string                          `json:"supported_runs,omitempty"`
}

// CharacterSetupRequiredSkillDTO is one ordered required skill for read-only Setup UI.
type CharacterSetupRequiredSkillDTO struct {
	Skill       string `json:"skill"`
	SkillID     uint16 `json:"skill_id"`
	DisplayName string `json:"display_name"`
}

// CharacterSetupPickitDefaultDTO beschreibt eine feste lesbare Run-Kette.
type CharacterSetupPickitDefaultDTO struct {
	RunID          string   `json:"run_id"`
	RunDisplayName string   `json:"run_display_name"`
	ProfileNames   []string `json:"profile_names"`
	State          string   `json:"state"`
}

// CharacterSetupPreviewDTO ist die vollständige Core-Projektion ohne Dateipfade.
type CharacterSetupPreviewDTO struct {
	SchemaVersion            int                              `json:"schema_version"`
	CatalogRevision          uint64                           `json:"catalog_revision"`
	OperatorSettingsRevision uint64                           `json:"operator_settings_revision"`
	PickitAssignmentRevision uint64                           `json:"pickit_assignment_revision"`
	Character                CharacterSetupCharacterDTO       `json:"character"`
	Supported                bool                             `json:"supported"`
	Profiles                 []CharacterSetupProfileDTO       `json:"profiles"`
	SelectedProfileID        string                           `json:"selected_profile_id,omitempty"`
	DefaultProfileID         string                           `json:"default_profile_id,omitempty"`
	PickitDefaults           []CharacterSetupPickitDefaultDTO `json:"pickit_defaults"`
	AnchorState              string                           `json:"anchor_state"`
	SetupState               string                           `json:"setup_state"`
	Reasons                  []string                         `json:"reasons"`
}

// CharacterSetupCharacterDTO enthält Name, Slug, Klasse und deutschen Klassennamen.
type CharacterSetupCharacterDTO struct {
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	CharacterClass   string `json:"character_class"`
	ClassDisplayName string `json:"class_display_name"`
}

// CharacterSetupConfirmRequest bestätigt Profil und fehlende Defaults revisionsgebunden.
type CharacterSetupConfirmRequest struct {
	CommandID                        string `json:"command_id"`
	Character                        string `json:"character"`
	ProfileID                        string `json:"profile_id,omitempty"`
	ExpectedCatalogRevision          uint64 `json:"expected_catalog_revision"`
	ExpectedOperatorSettingsRevision uint64 `json:"expected_operator_settings_revision"`
	ExpectedPickitAssignmentRevision uint64 `json:"expected_pickit_assignment_revision"`
	ExpectedGeneration               uint64 `json:"expected_generation"`
}

// CharacterSelectionCaptureRequest bestätigt die aktuell markierte Auswahl revisionsgebunden.
type CharacterSelectionCaptureRequest struct {
	CommandID               string `json:"command_id"`
	Character               string `json:"character"`
	ExpectedCatalogRevision uint64 `json:"expected_catalog_revision"`
	ExpectedGeneration      uint64 `json:"expected_generation"`
}

func characterSetupPreviewDTO(value app.CharacterSetupPreview) CharacterSetupPreviewDTO {
	profiles := make([]CharacterSetupProfileDTO, len(value.Profiles))
	for index, profile := range value.Profiles {
		skills := make([]CharacterSetupRequiredSkillDTO, len(profile.RequiredSkills))
		for skillIndex, skill := range profile.RequiredSkills {
			skills[skillIndex] = CharacterSetupRequiredSkillDTO{
				Skill: skill.Skill, SkillID: skill.SkillID, DisplayName: skill.DisplayName,
			}
		}
		profiles[index] = CharacterSetupProfileDTO{
			ID: profile.ID, DisplayName: profile.DisplayName, IsDefault: profile.IsDefault, IsSelected: profile.IsSelected,
			StandardAttack: profile.StandardAttack, RequiredSkills: skills, SupportedRuns: append([]string(nil), profile.SupportedRuns...),
		}
	}
	defaults := make([]CharacterSetupPickitDefaultDTO, len(value.PickitDefaults))
	for index, item := range value.PickitDefaults {
		defaults[index] = CharacterSetupPickitDefaultDTO{
			RunID: string(item.RunID), RunDisplayName: item.RunDisplayName,
			ProfileNames: append([]string(nil), item.ProfileNames...), State: item.State,
		}
	}
	return CharacterSetupPreviewDTO{
		SchemaVersion: schemaVersion, CatalogRevision: value.CatalogRevision,
		OperatorSettingsRevision: value.OperatorSettingsRevision, PickitAssignmentRevision: value.PickitAssignmentRevision,
		Character: CharacterSetupCharacterDTO{Name: value.CharacterName, Slug: value.CharacterSlug, CharacterClass: value.CharacterClass, ClassDisplayName: value.ClassDisplayName},
		Supported: value.Supported, Profiles: profiles, SelectedProfileID: value.SelectedProfileID, DefaultProfileID: value.DefaultProfileID,
		PickitDefaults: defaults, AnchorState: string(value.AnchorState), SetupState: string(value.SetupState), Reasons: append([]string(nil), value.Reasons...),
	}
}
