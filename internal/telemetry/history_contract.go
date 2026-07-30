package telemetry

// HistorySchemaVersion beginnt die auswertbare Phase-14-Telemetrieepoche.
const HistorySchemaVersion = 3

const (
	// HistoryDefaultPageLimit begrenzt Listen ohne explizites Limit.
	HistoryDefaultPageLimit = 50
	// HistoryMaximumPageLimit ist die harte Obergrenze einer Historienseite.
	HistoryMaximumPageLimit = 200
	// HistoryLowSampleBossKills markiert Vergleiche mit kleiner Datenbasis.
	HistoryLowSampleBossKills = 10
)

// HistoryStream unterscheidet die weiterhin getrennt geschriebenen JSONL-Ströme.
type HistoryStream string

const (
	// HistoryStreamSession enthält Session-, Game- und Run-Grenzen.
	HistoryStreamSession HistoryStream = "session"
	// HistoryStreamRun enthält Stages, Boss- und Itemereignisse eines Runs.
	HistoryStreamRun HistoryStream = "run"
)

// HistoryMode klassifiziert den Ursprung eines Schema-3-Runs ohne Dateinamen-Heuristik.
type HistoryMode string

const (
	// HistoryModeProductiveFarming ist die einzige statistikfähige Population.
	HistoryModeProductiveFarming HistoryMode = "productive_farming"
	// HistoryModeDiagnostic kennzeichnet isolierte Test- und Diagnoseabläufe.
	HistoryModeDiagnostic HistoryMode = "diagnostic"
)

// HistoryStage ist eine disjunkte, nutzerorientierte Zeitkategorie.
type HistoryStage string

const (
	// HistoryStageTravel umfasst Town-Ausgang, Waypoint und Farming-Route.
	HistoryStageTravel HistoryStage = "travel"
	// HistoryStageCombat umfasst Boss-Suche, Encounter-Aktionen und Kampf.
	HistoryStageCombat HistoryStage = "combat"
	// HistoryStageLoot umfasst Repositionierung, Drop-Wartezeit und Pickup.
	HistoryStageLoot HistoryStage = "loot"
	// HistoryStageReturnTown umfasst Portal, Herkunftsnormalisierung, Stash und Town-Handoff.
	HistoryStageReturnTown HistoryStage = "return_town"
)

// HistoryOutcome ist das normalisierte Ergebnis eines Historien-Runs.
type HistoryOutcome string

const (
	// HistoryOutcomeSuccess entspricht genau einem `run_completed`.
	HistoryOutcomeSuccess HistoryOutcome = "success"
	// HistoryOutcomeFailed entspricht genau einem `run_failed`.
	HistoryOutcomeFailed HistoryOutcome = "failed"
	// HistoryOutcomeAborted entspricht genau einem `run_aborted`.
	HistoryOutcomeAborted HistoryOutcome = "aborted"
	// HistoryOutcomeIncomplete bezeichnet eine Datei ohne terminales Run-Ereignis.
	HistoryOutcomeIncomplete HistoryOutcome = "incomplete"
	// HistoryOutcomeRunning ist nur bei bestätigter aktiver Core-Run-ID zulässig.
	HistoryOutcomeRunning HistoryOutcome = "running"
)

// HistorySort benennt serverseitige, deterministisch gebrochene Sortierungen.
type HistorySort string

const (
	// HistorySortKeepPerHour ist die Standardsortierung der Boss-/Routenvergleiche.
	HistorySortKeepPerHour HistorySort = "keep_per_hour"
	// HistorySortStartedAt sortiert Runlisten primär nach UTC-Startzeit.
	HistorySortStartedAt HistorySort = "started_at"
	// HistorySortItemName sortiert Itemtabellen primär nach Anzeigename.
	HistorySortItemName HistorySort = "item_name"
	// HistorySortSuccessRate sortiert Vergleiche primär nach Erfolgsquote.
	HistorySortSuccessRate HistorySort = "success_rate"
	// HistorySortAverageDuration sortiert Vergleiche primär nach kürzester Durchschnittsdauer.
	HistorySortAverageDuration HistorySort = "average_duration"
)

// HistoryReasonCode ist ein stabiler maschinenlesbarer Historienfehler.
type HistoryReasonCode string

const (
	HistoryReasonFileInvalid           HistoryReasonCode = "history_file_invalid"
	HistoryReasonFileTooLarge          HistoryReasonCode = "history_file_too_large"
	HistoryReasonLineTooLarge          HistoryReasonCode = "history_line_too_large"
	HistoryReasonSchemaUnsupported     HistoryReasonCode = "history_schema_unsupported"
	HistoryReasonEventInvalid          HistoryReasonCode = "history_event_invalid"
	HistoryReasonContextMissing        HistoryReasonCode = "history_context_missing"
	HistoryReasonRunIDMismatch         HistoryReasonCode = "history_run_id_mismatch"
	HistoryReasonStreamMissing         HistoryReasonCode = "history_stream_missing"
	HistoryReasonTerminalDuplicate     HistoryReasonCode = "history_terminal_duplicate"
	HistoryReasonTimeInvalid           HistoryReasonCode = "history_time_invalid"
	HistoryReasonBossDuplicate         HistoryReasonCode = "history_boss_duplicate"
	HistoryReasonStageInvalid          HistoryReasonCode = "history_stage_invalid"
	HistoryReasonItemIdentityInvalid   HistoryReasonCode = "history_item_identity_invalid"
	HistoryReasonItemChainInvalid      HistoryReasonCode = "history_item_chain_invalid"
	HistoryReasonFilterInvalid         HistoryReasonCode = "history_filter_invalid"
	HistoryReasonTimezoneInvalid       HistoryReasonCode = "history_timezone_invalid"
	HistoryReasonRetentionBlocked      HistoryReasonCode = "history_retention_blocked"
	HistoryReasonRetentionPartial      HistoryReasonCode = "history_retention_partial"
	HistoryReasonDeletePreviewStale    HistoryReasonCode = "history_delete_preview_stale"
	HistoryReasonDeleteActiveProtected HistoryReasonCode = "history_delete_active_protected"
	HistoryReasonDeleteFailed          HistoryReasonCode = "history_delete_failed"
	HistoryReasonRunNotFound           HistoryReasonCode = "history_run_not_found"
	HistoryReasonCursorInvalid         HistoryReasonCode = "history_cursor_invalid"
	HistoryReasonExportInvalid         HistoryReasonCode = "history_export_invalid"
	HistoryReasonUnavailable           HistoryReasonCode = "history_unavailable"
)

// HistoryReasonMessage liefert den einzigen deutschen Benutzertext für einen Historiencode.
func HistoryReasonMessage(code HistoryReasonCode) (string, bool) {
	message, ok := historyReasonMessages[code]
	return message, ok
}

var historyReasonMessages = map[HistoryReasonCode]string{
	HistoryReasonFileInvalid:            "Die Telemetriedatei ist beschädigt und wurde vollständig ausgeschlossen.",
	HistoryReasonFileTooLarge:           "Die Telemetriedatei überschreitet die zulässige Größe und wurde ausgeschlossen.",
	HistoryReasonLineTooLarge:           "Eine Telemetriezeile überschreitet die zulässige Größe; die Datei wurde ausgeschlossen.",
	HistoryReasonSchemaUnsupported:      "Die Telemetriedatei verwendet kein unterstütztes Historien-Schema.",
	HistoryReasonEventInvalid:           "Die Telemetriedatei enthält ein unbekanntes oder ungültiges Ereignis.",
	HistoryReasonContextMissing:         "Dem Run fehlt verpflichtender Farming-Kontext.",
	HistoryReasonRunIDMismatch:          "Session- und Run-Telemetrie verwenden unterschiedliche Run-IDs.",
	HistoryReasonStreamMissing:          "Der korrelierte Session- oder Run-Stream fehlt.",
	HistoryReasonTerminalDuplicate:      "Der Run enthält mehr als ein terminales Ergebnis.",
	HistoryReasonTimeInvalid:            "Die Zeitfolge des Runs ist ungültig.",
	HistoryReasonBossDuplicate:          "Der bestätigte Bosskill wurde mehrfach protokolliert.",
	HistoryReasonStageInvalid:           "Die Zeitanteile des Runs sind unvollständig oder überlappen.",
	HistoryReasonItemIdentityInvalid:    "Die Itemidentität ist unvollständig oder widersprüchlich.",
	HistoryReasonItemChainInvalid:       "Die Itemkette ist nicht lückenlos innerhalb desselben Runs korreliert.",
	HistoryReasonFilterInvalid:          "Die gewählten Historienfilter sind ungültig.",
	HistoryReasonTimezoneInvalid:        "Die lokale Zeitzone ist unbekannt oder ungültig.",
	HistoryReasonRetentionBlocked:       "Die automatische Retention hat ein unvollständiges oder geschütztes Bundle übersprungen.",
	HistoryReasonRetentionPartial:       "Die automatische Retention konnte nicht alle vorgesehenen Dateien löschen.",
	HistoryReasonDeletePreviewStale:     "Die Löschvorschau ist nicht mehr aktuell und muss neu erstellt werden.",
	HistoryReasonDeleteActiveProtected:  "Eine aktive Historiendatei wurde geschützt und nicht gelöscht.",
	HistoryReasonDeleteFailed:           "Die bestätigte Historienlöschung konnte nicht vollständig abgeschlossen werden.",
	HistoryReasonRunNotFound:            "Der angeforderte Run ist in der Historie nicht vorhanden.",
	HistoryReasonCursorInvalid:          "Die angeforderte Historienseite ist nicht mehr gültig.",
	HistoryReasonExportInvalid:          "Der angeforderte Historienexport ist ungültig.",
	HistoryReasonUnavailable:            "Die Historienauswertung ist vorübergehend nicht verfügbar.",
	"run_unknown":                       "Der angeforderte Run ist nicht registriert.",
	"run_config_missing":                "Für den Run fehlt die erforderliche Konfiguration.",
	"run_definition_invalid":            "Die Run-Definition ist ungültig.",
	"run_capability_missing":            "Eine für den Run erforderliche Fähigkeit ist nicht verfügbar.",
	"route_missing":                     "Für den Run ist keine Route verfügbar.",
	"route_binding_mismatch":            "Die Route passt nicht zur gewählten Run-Konfiguration.",
	"route_layout_mismatch":             "Die Route passt nicht zum aktuellen Kartenlayout.",
	"route_runtime_validation_required": "Die Route konnte im aktuellen Spiel noch nicht bestätigt werden.",
	"route_stale":                       "Die Route ist veraltet und muss erneut bestätigt werden.",
	"route_lifecycle_unavailable":       "Der Lebenszyklusstatus der Route ist nicht verwendbar.",
	"route_assignment_missing":          "Für Charakter und Run fehlt eine Routenzuweisung.",
	"profile_class_mismatch":            "Das Profil passt nicht zur Charakterklasse.",
	"waypoint_target_unsupported":       "Das Wegpunktziel wird nicht unterstützt.",
	"waypoint_ui_unconfirmed":           "Das Wegpunktfenster konnte nicht bestätigt werden.",
	"waypoint_destination_timeout":      "Das Wegpunktziel wurde nicht rechtzeitig bestätigt.",
	"unexpected_area":                   "Der Run befand sich in einem unerwarteten Gebiet.",
	"boss_not_found":                    "Der Boss wurde nicht gefunden.",
	"boss_pin_lost":                     "Die bestätigte Bossidentität ging verloren.",
	"encounter_action_failed":           "Eine Kampfaktion ist fehlgeschlagen.",
	"boss_kill_unconfirmed":             "Der Bosskill konnte nicht sicher bestätigt werden.",
	"loot_policy_invalid":               "Die Loot-Regel ist ungültig.",
	"item_tier_unknown":                 "Die Basisstufe des Items ist unbekannt.",
	"item_classification_conflict":      "Das Item besitzt widersprüchliche Klassifizierungen.",
	"item_identify_failed":              "Die Identifizierung des Items ist fehlgeschlagen oder unbestätigt.",
	"item_sell_failed":                  "Der Verkauf des Items ist fehlgeschlagen oder unbestätigt.",
	"town_egress_missing":               "Für die Stadt fehlt eine Ausgangsroute.",
	"town_egress_binding_mismatch":      "Die Ausgangsroute passt nicht zur aktuellen Stadt.",
	"hub_transfer_unsupported":          "Der benötigte Stadttransfer wird nicht unterstützt.",
	"town_service_verify_timeout":       "Der Stadtdienst wurde nicht rechtzeitig bestätigt.",
	"route_clear_no_progress":           "Beim Freikämpfen der Route wurde zwölf Sekunden lang kein sicherer Fortschritt bestätigt.",
	"route_threat_out_of_range":         "Ein Gegner blieb auch nach drei wirkungslosen Annäherungsversuchen nicht sicher angreifbar.",
	"boss_combat_unprojectable":         "Der Boss blieb nach der einmaligen Annäherung nicht sicher anzielbar; der Run kehrt kontrolliert nach Akt 1 zurück und startet neu.",
	"retry_return_failed":               "Die kontrollierte Rückkehr nach Akt 1 vor dem Run-Neustart ist fehlgeschlagen.",
	"route_mana_recovery_failed":        "Die Manareserve für die Route konnte nicht rechtzeitig wiederhergestellt werden.",
	"route_recovery_unsafe":             "Eine wirkungslose oder bedrohte Routenkorrektur wurde sicher abgebrochen.",
	"route_threat_state_invalid":        "Der interne Route-Bedrohungszustand war inkonsistent; der Run wurde sicher beendet.",
	"telemetry_failed":                  "Die Run-Telemetrie konnte nicht sicher geschrieben werden.",
	"profile_telemetry_failed":          "Die Profil-Telemetrie konnte nicht sicher geschrieben werden.",
	"operator_stop":                     "Der Run wurde durch den Operator gestoppt.",
	"step_timeout":                      "Der aktuelle Run-Schritt hat sein Zeitlimit überschritten.",
	"hard_stuck":                        "Der Run konnte sich nicht aus einer Blockade erholen.",
}

// HistoryRunCSVColumns liefert die stabile Spaltenreihenfolge des Run-Exports.
func HistoryRunCSVColumns() []string {
	return []string{"run_id", "started_at_utc", "character", "difficulty", "run", "route_id", "outcome", "reason_code", "duration_ms", "boss_kills", "keep_stashed", "sell_confirmed", "pickup_lost", "post_pickup_lost"}
}

// HistoryItemCSVColumns liefert die stabile Spaltenreihenfolge des Item-Exports.
func HistoryItemCSVColumns() []string {
	return []string{"item_key", "item_name", "seen", "matched", "picked_up", "stashed", "sold", "pickup_lost", "post_pickup_lost", "yield_per_run", "yield_per_kill", "yield_per_hour"}
}
