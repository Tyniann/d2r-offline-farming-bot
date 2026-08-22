package telemetry

// HistorySchemaVersion identifies compact run telemetry with one immutable
// context record followed by event-specific JSONL records.
const HistorySchemaVersion = 4

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

// HistoryMode klassifiziert den Ursprung eines Historien-Runs ohne Dateinamen-Heuristik.
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

// HistoryRunCSVColumns liefert die stabile Spaltenreihenfolge des Run-Exports.
func HistoryRunCSVColumns() []string {
	return []string{"run_id", "started_at_utc", "character", "difficulty", "run", "route_id", "outcome", "reason_code", "duration_ms", "boss_kills", "keep_stashed", "sell_confirmed", "pickup_lost", "post_pickup_lost"}
}

// HistoryItemCSVColumns liefert die stabile Spaltenreihenfolge des Item-Exports.
func HistoryItemCSVColumns() []string {
	return []string{"item_key", "item_name", "seen", "matched", "picked_up", "stashed", "sold", "pickup_lost", "post_pickup_lost", "yield_per_run", "yield_per_kill", "yield_per_hour"}
}
