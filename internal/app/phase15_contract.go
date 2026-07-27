package app

const (
	// Phase15DataRootDirectoryName ist der kanonische installierte LocalAppData-Unterordner.
	Phase15DataRootDirectoryName = "D2ROfflineFarmingBot"
	// Phase15DefaultHistoryRetentionDays ist die sichere automatische Aufbewahrungsfrist.
	Phase15DefaultHistoryRetentionDays = 60
	// Phase15MaximumConfigBackups begrenzt erfolgreiche Config- und Migrationsbackups.
	Phase15MaximumConfigBackups = 10
)

// Phase15ContractOwner benennt den einzigen Owner einer Desktop-Produktverantwortung.
type Phase15ContractOwner struct {
	Responsibility string
	Owner          string
	Boundary       string
}

// Phase15ContractOwners liefert die verbindlichen Fach-, Desktop- und Projektionsgrenzen.
func Phase15ContractOwners() []Phase15ContractOwner {
	return []Phase15ContractOwner{
		{Responsibility: "d2r_input_safety_and_workflow_locks", Owner: "internal/app and Go Core", Boundary: "Supervisor, Availability, Preview/Confirm and control token"},
		{Responsibility: "operator_settings_and_history_mutation", Owner: "Go Core", Boundary: "versioned stores, expected revision and active generation"},
		{Responsibility: "window_tray_autostart_notifications", Owner: "Electron Main", Boundary: "narrow desktop IPC without gameplay semantics"},
		{Responsibility: "desktop_core_process_relationship", Owner: "Electron Main", Boundary: "one child, bounded handshake, shutdown and crash policy"},
		{Responsibility: "renderer_projection_and_intents", Owner: "React", Boundary: "generated API client and narrow desktop bridge"},
		{Responsibility: "history_metrics_and_buckets", Owner: "internal/telemetry", Boundary: "AnalyzeHistory is the only calculation authority"},
		{Responsibility: "diagnostic_content_and_redaction", Owner: "Go Core", Boundary: "allowlisted content; Electron selects only the destination"},
		{Responsibility: "release_update_check", Owner: "Electron Main", Boundary: "one allowlisted HTTPS request without credentials"},
	}
}

// Phase15DataRootEntry beschreibt einen erlaubten direkten Bestandteil des installierten Datenroots.
type Phase15DataRootEntry struct {
	RelativePath string
	Owner        string
}

// Phase15DataRootLayout liefert das minimale persistente Desktop-Layout.
func Phase15DataRootLayout() []Phase15DataRootEntry {
	return []Phase15DataRootEntry{
		{RelativePath: "configs", Owner: "Go Core"},
		{RelativePath: "logs/telemetry", Owner: "Go Core"},
		{RelativePath: "backups", Owner: "Go Core"},
		{RelativePath: "diagnostics", Owner: "Go Core"},
		{RelativePath: "desktop-settings.json", Owner: "Electron Main"},
	}
}

// Phase15DesktopActiveStates liefert die Zustände, in denen Schließen nur ins Tray führen darf.
func Phase15DesktopActiveStates() []SupervisorState {
	return []SupervisorState{
		SupervisorStateStartingGame,
		SupervisorStateStartingRun,
		SupervisorStateRunningRun,
		SupervisorStatePausedBetweenRuns,
		SupervisorStateExitingGame,
		SupervisorStateCancelling,
	}
}

// Phase15NonGoals liefert die ausdrücklich verbotenen Architekturpfade.
func Phase15NonGoals() []string {
	return []string{
		"renderer_node_integration",
		"renderer_filesystem_process_or_d2r_access",
		"electron_owned_queue_config_or_statistics",
		"second_ui_overlay_browser_product_or_remote_content",
		"sqlite_or_persistent_history_cache",
		"electron_updater_or_automatic_download",
		"ffi_or_new_gameplay_path",
		"router_state_or_component_framework",
		"portable_msi_store_or_non_windows_artifact",
		"d2r_or_farming_autostart",
	}
}

// Phase15ReasonCode ist ein stabiler maschinenlesbarer Desktop-Produktfehler.
type Phase15ReasonCode string

const (
	Phase15ReasonDesktopInstanceRunning       Phase15ReasonCode = "desktop_instance_running"
	Phase15ReasonCoreStartFailed              Phase15ReasonCode = "core_start_failed"
	Phase15ReasonCoreHandshakeTimeout         Phase15ReasonCode = "core_handshake_timeout"
	Phase15ReasonCoreHandshakeInvalid         Phase15ReasonCode = "core_handshake_invalid"
	Phase15ReasonCoreExited                   Phase15ReasonCode = "core_exited"
	Phase15ReasonCoreRecoveryRequired         Phase15ReasonCode = "core_recovery_required"
	Phase15ReasonCoreShutdownFailed           Phase15ReasonCode = "core_shutdown_failed"
	Phase15ReasonDataRootUnavailable          Phase15ReasonCode = "data_root_unavailable"
	Phase15ReasonDataRootLocked               Phase15ReasonCode = "data_root_locked"
	Phase15ReasonDataImportInvalid            Phase15ReasonCode = "data_import_invalid"
	Phase15ReasonDataImportConflict           Phase15ReasonCode = "data_import_conflict"
	Phase15ReasonDataImportFailed             Phase15ReasonCode = "data_import_failed"
	Phase15ReasonConfigSchemaUnsupported      Phase15ReasonCode = "config_schema_unsupported"
	Phase15ReasonConfigRevisionConflict       Phase15ReasonCode = "config_revision_conflict"
	Phase15ReasonConfigRestartRequired        Phase15ReasonCode = "config_restart_required"
	Phase15ReasonD2RVersionNotDetected        Phase15ReasonCode = "d2r_version_not_detected"
	Phase15ReasonD2RVersionUnreadable         Phase15ReasonCode = "d2r_version_unreadable"
	Phase15ReasonD2RVersionUnsupported        Phase15ReasonCode = "d2r_version_unsupported"
	Phase15ReasonOffsetVersionMismatch        Phase15ReasonCode = "offset_version_mismatch"
	Phase15ReasonPrivilegeMismatch            Phase15ReasonCode = "privilege_mismatch"
	Phase15ReasonOnboardingInputDisabled      Phase15ReasonCode = "onboarding_input_disabled"
	Phase15ReasonOnboardingPrerequisite       Phase15ReasonCode = "onboarding_route_prerequisite_missing"
	Phase15ReasonOnboardingRouteInterrupted   Phase15ReasonCode = "onboarding_route_interrupted"
	Phase15ReasonHistoryTimezoneInvalid       Phase15ReasonCode = "history_timezone_invalid"
	Phase15ReasonHistoryRetentionBlocked      Phase15ReasonCode = "history_retention_blocked"
	Phase15ReasonHistoryRetentionPartial      Phase15ReasonCode = "history_retention_partial"
	Phase15ReasonHistoryDeletePreviewStale    Phase15ReasonCode = "history_delete_preview_stale"
	Phase15ReasonHistoryDeleteActiveProtected Phase15ReasonCode = "history_delete_active_protected"
	Phase15ReasonHistoryDeleteFailed          Phase15ReasonCode = "history_delete_failed"
	Phase15ReasonUpdateCheckUnavailable       Phase15ReasonCode = "update_check_unavailable"
	Phase15ReasonUpdateResponseInvalid        Phase15ReasonCode = "update_response_invalid"
	Phase15ReasonExternalLinkRejected         Phase15ReasonCode = "external_link_rejected"
	Phase15ReasonDiagnosticBundleFailed       Phase15ReasonCode = "diagnostic_bundle_failed"
	Phase15ReasonDiagnosticContentRejected    Phase15ReasonCode = "diagnostic_content_rejected"
)

// Phase15ReasonGroup hält die vollständige geordnete Reason-Code-Menge eines Bereichs.
type Phase15ReasonGroup struct {
	Area  string
	Codes []Phase15ReasonCode
}

// Phase15ReasonGroups liefert die reservierten Codes ohne UI-erfundene Synonyme.
func Phase15ReasonGroups() []Phase15ReasonGroup {
	return []Phase15ReasonGroup{
		{Area: "desktop_core", Codes: []Phase15ReasonCode{Phase15ReasonDesktopInstanceRunning, Phase15ReasonCoreStartFailed, Phase15ReasonCoreHandshakeTimeout, Phase15ReasonCoreHandshakeInvalid, Phase15ReasonCoreExited, Phase15ReasonCoreRecoveryRequired, Phase15ReasonCoreShutdownFailed}},
		{Area: "data_migration", Codes: []Phase15ReasonCode{Phase15ReasonDataRootUnavailable, Phase15ReasonDataRootLocked, Phase15ReasonDataImportInvalid, Phase15ReasonDataImportConflict, Phase15ReasonDataImportFailed, Phase15ReasonConfigSchemaUnsupported, Phase15ReasonConfigRevisionConflict, Phase15ReasonConfigRestartRequired}},
		{Area: "compatibility", Codes: []Phase15ReasonCode{Phase15ReasonD2RVersionNotDetected, Phase15ReasonD2RVersionUnreadable, Phase15ReasonD2RVersionUnsupported, Phase15ReasonOffsetVersionMismatch, Phase15ReasonPrivilegeMismatch}},
		{Area: "onboarding", Codes: []Phase15ReasonCode{Phase15ReasonOnboardingInputDisabled, Phase15ReasonOnboardingPrerequisite, Phase15ReasonOnboardingRouteInterrupted}},
		{Area: "history", Codes: []Phase15ReasonCode{Phase15ReasonHistoryTimezoneInvalid, Phase15ReasonHistoryRetentionBlocked, Phase15ReasonHistoryRetentionPartial, Phase15ReasonHistoryDeletePreviewStale, Phase15ReasonHistoryDeleteActiveProtected, Phase15ReasonHistoryDeleteFailed}},
		{Area: "desktop_services", Codes: []Phase15ReasonCode{Phase15ReasonUpdateCheckUnavailable, Phase15ReasonUpdateResponseInvalid, Phase15ReasonExternalLinkRejected, Phase15ReasonDiagnosticBundleFailed, Phase15ReasonDiagnosticContentRejected}},
	}
}
