package api

import "github.com/Tyniann/d2r-offline-farming-bot/internal/app"

// OperatorSettingsDTO projiziert den vollständigen Core-eigenen Schema-3-Stand.
type OperatorSettingsDTO struct {
	SchemaVersion int                                     `json:"schema_version"`
	Revision      uint64                                  `json:"revision"`
	LastCharacter string                                  `json:"last_character,omitempty"`
	Characters    map[string]OperatorCharacterSettingsDTO `json:"characters"`
	Budgets       OperatorBudgetSettingsDTO               `json:"budgets"`
	Input         OperatorInputSettingsDTO                `json:"input"`
	History       OperatorHistorySettingsDTO              `json:"history"`
}

// OperatorCharacterSettingsDTO enthält Setupwerte, Queue, Difficulty sowie optionale Bindings und Inventar.
type OperatorCharacterSettingsDTO struct {
	CharacterClass  string                                `json:"character_class,omitempty"`
	CombatProfile   string                                `json:"combat_profile,omitempty"`
	LastDifficulty  string                                `json:"last_difficulty"`
	Queue           []string                              `json:"queue"`
	ProfileBindings map[string]OperatorProfileBindingsDTO `json:"profile_bindings,omitempty"`
	InventoryLock   *OperatorInventoryLockDTO             `json:"inventory_lock"`
}

// OperatorProfileBindingsDTO speichert Skill-F-Tasten, Gürteltasten und Trankspalten eines Kampfprofils.
type OperatorProfileBindingsDTO struct {
	Skills     map[string]string       `json:"skills,omitempty"`
	Belt       OperatorBeltBindingsDTO `json:"belt,omitempty"`
	BeltLayout OperatorBeltLayoutDTO   `json:"belt_layout,omitempty"`
}

// OperatorBeltBindingsDTO speichert optionale Gürteltasten.
type OperatorBeltBindingsDTO struct {
	Slot1 string `json:"slot_1,omitempty"`
	Slot2 string `json:"slot_2,omitempty"`
	Slot3 string `json:"slot_3,omitempty"`
	Slot4 string `json:"slot_4,omitempty"`
}

// OperatorBeltLayoutDTO speichert die Tranktypen der Gürtelspalten 1–4.
type OperatorBeltLayoutDTO struct {
	Slot1 string `json:"slot_1,omitempty"`
	Slot2 string `json:"slot_2,omitempty"`
	Slot3 string `json:"slot_3,omitempty"`
	Slot4 string `json:"slot_4,omitempty"`
}

// OperatorInventoryLockDTO ist presence-sensitiv; null = unkonfiguriert.
type OperatorInventoryLockDTO struct {
	Grid [][]int `json:"grid"`
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
		characters[character] = operatorCharacterSettingsDTO(value)
	}
	return OperatorSettingsDTO{
		SchemaVersion: settings.SchemaVersion, Revision: settings.Revision, LastCharacter: settings.LastCharacter, Characters: characters,
		Budgets: OperatorBudgetSettingsDTO(settings.Budgets), Input: OperatorInputSettingsDTO(settings.Input), History: OperatorHistorySettingsDTO(settings.History),
	}
}

func operatorCharacterSettingsDTO(value app.OperatorCharacterSettings) OperatorCharacterSettingsDTO {
	dto := OperatorCharacterSettingsDTO{
		CharacterClass: value.CharacterClass, CombatProfile: value.CombatProfile,
		LastDifficulty: value.LastDifficulty, Queue: append([]string(nil), value.Queue...),
	}
	if value.ProfileBindings != nil {
		dto.ProfileBindings = make(map[string]OperatorProfileBindingsDTO, len(value.ProfileBindings))
		for profileID, bindings := range value.ProfileBindings {
			cloned := OperatorProfileBindingsDTO{
				Belt:       OperatorBeltBindingsDTO(bindings.Belt),
				BeltLayout: OperatorBeltLayoutDTO(bindings.BeltLayout),
			}
			if bindings.Skills != nil {
				cloned.Skills = make(map[string]string, len(bindings.Skills))
				for skill, key := range bindings.Skills {
					cloned.Skills[skill] = key
				}
			}
			dto.ProfileBindings[profileID] = cloned
		}
	}
	if value.InventoryLock != nil {
		dto.InventoryLock = &OperatorInventoryLockDTO{Grid: cloneIntGrid(value.InventoryLock.Grid)}
	}
	return dto
}

func operatorSettingsFromDTO(settings OperatorSettingsDTO) app.OperatorSettings {
	characters := make(map[string]app.OperatorCharacterSettings, len(settings.Characters))
	for character, value := range settings.Characters {
		characters[character] = operatorCharacterSettingsFromDTO(value)
	}
	return app.OperatorSettings{
		SchemaVersion: settings.SchemaVersion, Revision: settings.Revision, LastCharacter: settings.LastCharacter, Characters: characters,
		Budgets: app.OperatorBudgetSettings(settings.Budgets), Input: app.OperatorInputSettings(settings.Input), History: app.OperatorHistorySettings(settings.History),
	}
}

func operatorCharacterSettingsFromDTO(value OperatorCharacterSettingsDTO) app.OperatorCharacterSettings {
	settings := app.OperatorCharacterSettings{
		CharacterClass: value.CharacterClass, CombatProfile: value.CombatProfile,
		LastDifficulty: value.LastDifficulty, Queue: append([]string(nil), value.Queue...),
	}
	if value.ProfileBindings != nil {
		settings.ProfileBindings = make(map[string]app.OperatorProfileBindings, len(value.ProfileBindings))
		for profileID, bindings := range value.ProfileBindings {
			cloned := app.OperatorProfileBindings{
				Belt:       app.OperatorBeltBindings(bindings.Belt),
				BeltLayout: app.OperatorBeltLayout(bindings.BeltLayout),
			}
			if bindings.Skills != nil {
				cloned.Skills = make(map[string]string, len(bindings.Skills))
				for skill, key := range bindings.Skills {
					cloned.Skills[skill] = key
				}
			}
			settings.ProfileBindings[profileID] = cloned
		}
	}
	if value.InventoryLock != nil {
		settings.InventoryLock = &app.OperatorInventoryLock{Grid: cloneIntGrid(value.InventoryLock.Grid)}
	}
	return settings
}

func cloneIntGrid(grid [][]int) [][]int {
	if grid == nil {
		return nil
	}
	clone := make([][]int, len(grid))
	for row, values := range grid {
		clone[row] = append([]int(nil), values...)
	}
	return clone
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
