package api

import "github.com/Tyniann/d2r-offline-farming-bot/internal/app"

// OperatorSettingsDTO projiziert den vollständigen Core-eigenen Schema-2-Stand.
type OperatorSettingsDTO struct {
	SchemaVersion int                                     `json:"schema_version"`
	Revision      uint64                                  `json:"revision"`
	LastCharacter string                                  `json:"last_character,omitempty"`
	Characters    map[string]OperatorCharacterSettingsDTO `json:"characters"`
	Budgets       OperatorBudgetSettingsDTO               `json:"budgets"`
	Input         OperatorInputSettingsDTO                `json:"input"`
	History       OperatorHistorySettingsDTO              `json:"history"`
}

// OperatorCharacterSettingsDTO enthält read-only Setupwerte sowie charakterbezogene Queue und Difficulty.
type OperatorCharacterSettingsDTO struct {
	CharacterClass string   `json:"character_class,omitempty"`
	CombatProfile  string   `json:"combat_profile,omitempty"`
	LastDifficulty string   `json:"last_difficulty"`
	Queue          []string `json:"queue"`
}

// OperatorBudgetSettingsDTO enthält globale endliche Queuebudgets.
type OperatorBudgetSettingsDTO struct {
	MaxRuns                int `json:"max_runs"`
	MaxDurationMs          int `json:"max_duration_ms"`
	MaxConsecutiveFailures int `json:"max_consecutive_failures"`
	MaxTotalRestarts       int `json:"max_total_restarts"`
}

// OperatorInputSettingsDTO enthält Input-Opt-in und Gameplay-Hotkeys.
type OperatorInputSettingsDTO struct {
	Enabled               bool   `json:"enabled"`
	PauseHotkey           string `json:"pause_hotkey"`
	StopAfterRunHotkey    string `json:"stop_after_run_hotkey"`
	RecordingFinishHotkey string `json:"recording_finish_hotkey"`
	EmergencyStopHotkey   string `json:"emergency_stop_hotkey"`
}

// OperatorHistorySettingsDTO enthält die Retentionseinstellungen.
type OperatorHistorySettingsDTO struct {
	RetentionEnabled bool `json:"retention_enabled"`
	RetentionDays    int  `json:"retention_days"`
}

// OperatorSettingsMutationRequest bindet eine Ersetzung an Revision und Supervisorgeneration.
type OperatorSettingsMutationRequest struct {
	ExpectedRevision   uint64              `json:"expected_revision"`
	ExpectedGeneration uint64              `json:"expected_generation"`
	Settings           OperatorSettingsDTO `json:"settings"`
}

// OperatorSettingsResetRequest bindet einen Reset an Revision und Supervisorgeneration.
type OperatorSettingsResetRequest struct {
	ExpectedRevision   uint64 `json:"expected_revision"`
	ExpectedGeneration uint64 `json:"expected_generation"`
}

// OperatorSettingsChangeDTO projiziert Vorschau oder persistiertes Ergebnis.
type OperatorSettingsChangeDTO struct {
	SchemaVersion   int                 `json:"schema_version"`
	Generation      uint64              `json:"generation"`
	Settings        OperatorSettingsDTO `json:"settings"`
	ChangedFields   []string            `json:"changed_fields"`
	RestartRequired bool                `json:"restart_required"`
	ReasonCode      string              `json:"reason_code,omitempty"`
}

func operatorSettingsDTO(settings app.OperatorSettings) OperatorSettingsDTO {
	characters := make(map[string]OperatorCharacterSettingsDTO, len(settings.Characters))
	for character, value := range settings.Characters {
		characters[character] = OperatorCharacterSettingsDTO{
			CharacterClass: value.CharacterClass, CombatProfile: value.CombatProfile,
			LastDifficulty: value.LastDifficulty, Queue: append([]string(nil), value.Queue...),
		}
	}
	return OperatorSettingsDTO{
		SchemaVersion: settings.SchemaVersion, Revision: settings.Revision, LastCharacter: settings.LastCharacter, Characters: characters,
		Budgets: OperatorBudgetSettingsDTO(settings.Budgets), Input: OperatorInputSettingsDTO(settings.Input), History: OperatorHistorySettingsDTO(settings.History),
	}
}

func operatorSettingsFromDTO(settings OperatorSettingsDTO) app.OperatorSettings {
	characters := make(map[string]app.OperatorCharacterSettings, len(settings.Characters))
	for character, value := range settings.Characters {
		characters[character] = app.OperatorCharacterSettings{
			CharacterClass: value.CharacterClass, CombatProfile: value.CombatProfile,
			LastDifficulty: value.LastDifficulty, Queue: append([]string(nil), value.Queue...),
		}
	}
	return app.OperatorSettings{
		SchemaVersion: settings.SchemaVersion, Revision: settings.Revision, LastCharacter: settings.LastCharacter, Characters: characters,
		Budgets: app.OperatorBudgetSettings(settings.Budgets), Input: app.OperatorInputSettings(settings.Input), History: app.OperatorHistorySettings(settings.History),
	}
}

func operatorSettingsChangeDTO(change app.OperatorSettingsChange, generation uint64) OperatorSettingsChangeDTO {
	reason := ""
	if change.RestartRequired {
		reason = string(app.Phase15ReasonConfigRestartRequired)
	}
	return OperatorSettingsChangeDTO{
		SchemaVersion: schemaVersion, Generation: generation, Settings: operatorSettingsDTO(change.Settings),
		ChangedFields: append([]string(nil), change.ChangedFields...), RestartRequired: change.RestartRequired, ReasonCode: reason,
	}
}
