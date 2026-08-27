import type { TFunction } from "i18next";
import type { ApiError, ProblemDTO } from "../api/generated";

export type AppTranslator = TFunction<"translation">;

export function presentDifficultyName(id: string, t: AppTranslator): string {
  const keys = { normal: "systemCatalog.difficulties.normal", nightmare: "systemCatalog.difficulties.nightmare", hell: "systemCatalog.difficulties.hell" } as const;
  const key = keys[id as keyof typeof keys];
  return key ? t(key) : id;
}

export function presentRunName(id: string, t: AppTranslator): string {
  const keys = { countess: "systemCatalog.runs.countess", summoner: "systemCatalog.runs.summoner", mephisto: "systemCatalog.runs.mephisto", nihlathak: "systemCatalog.runs.nihlathak", "lower-kurast": "systemCatalog.runs.lower-kurast", cows: "systemCatalog.runs.cows", "cow-level": "systemCatalog.runs.cow-level" } as const;
  const key = keys[id as keyof typeof keys];
  return key ? t(key) : id;
}

export function presentClassName(id: string, t: AppTranslator): string {
  const keys = { amazon: "systemCatalog.classes.amazon", assassin: "systemCatalog.classes.assassin", barbarian: "systemCatalog.classes.barbarian", druid: "systemCatalog.classes.druid", necromancer: "systemCatalog.classes.necromancer", paladin: "systemCatalog.classes.paladin", sorceress: "systemCatalog.classes.sorceress" } as const;
  const key = keys[id as keyof typeof keys];
  return key ? t(key) : id;
}

export function presentProfileName(id: string, fallback: string, t: AppTranslator): string {
  const keys = { necro_bone_spear: "systemCatalog.profiles.necro_bone_spear", paladin_hammerdin: "systemCatalog.profiles.paladin_hammerdin" } as const;
  const key = keys[id as keyof typeof keys];
  return key ? t(key) : fallback || id;
}

export function presentUnknownCode(code: string, t: AppTranslator): string {
  return t("errors.unknownCode", { code });
}

const runStageKeys = {
  town_preparation: "dashboard.active.stages.town_preparation",
  waypoint_black_marsh: "dashboard.active.stages.waypoint_black_marsh",
  waypoint_durance_of_hate_level_2: "dashboard.active.stages.waypoint_durance_of_hate_level_2",
  waypoint_arcane_sanctuary: "dashboard.active.stages.waypoint_arcane_sanctuary",
  waypoint_halls_of_pain: "dashboard.active.stages.waypoint_halls_of_pain",
  waypoint_lower_kurast: "dashboard.active.stages.waypoint_lower_kurast",
  waypoint_stony_field: "dashboard.active.stages.waypoint_stony_field",
  travel_tower: "dashboard.active.stages.travel_tower",
  travel_mephisto: "dashboard.active.stages.travel_mephisto",
  travel_summoner: "dashboard.active.stages.travel_summoner",
  travel_nihlathak: "dashboard.active.stages.travel_nihlathak",
  travel_huts: "dashboard.active.stages.travel_huts",
  travel_tristram: "dashboard.active.stages.travel_tristram",
  cellar_floor: "dashboard.active.stages.cellar_floor",
  boss_combat: "dashboard.active.stages.boss_combat",
  loot: "dashboard.active.stages.loot",
  return_town: "dashboard.active.stages.return_town",
  return_village: "dashboard.active.stages.return_village",
  stash: "dashboard.active.stages.stash",
  complete: "dashboard.active.stages.complete",
  superchests: "dashboard.active.stages.superchests",
  wirts_body: "dashboard.active.stages.wirts_body",
  wirts_leg: "dashboard.active.stages.wirts_leg",
  buy_tome: "dashboard.active.stages.buy_tome",
  cow_portal: "dashboard.active.stages.cow_portal",
  cow_sweep: "dashboard.active.stages.cow_sweep",
} as const;

export function presentRunStage(code: string, params: Record<string, unknown> | undefined, t: AppTranslator): string {
  const key = runStageKeys[code as keyof typeof runStageKeys];
  return key ? t(key, params ?? {}) : presentUnknownCode(code, t);
}

const recordingInstructionKeys = {
  record_countess: "routes.instructions.record_countess",
  record_mephisto: "routes.instructions.record_mephisto",
  record_summoner: "routes.instructions.record_summoner",
  record_nihlathak: "routes.instructions.record_nihlathak",
  record_cow_leg: "routes.instructions.record_cow_leg",
  record_cow_sweep: "routes.instructions.record_cow_sweep",
  record_lower_kurast: "routes.instructions.record_lower_kurast",
} as const;

const recordingHintKeys = {
  cow_leg_portal_open: "routes.operatorHints.cow_leg_portal_open",
  cow_leg_do_not_click_wirt: "routes.operatorHints.cow_leg_do_not_click_wirt",
  cow_portal_open: "routes.operatorHints.cow_portal_open",
  cow_level_clear: "routes.operatorHints.cow_level_clear",
} as const;

export function presentRecordingInstruction(code: string, finishKey: string, t: AppTranslator): string {
  const key = recordingInstructionKeys[code as keyof typeof recordingInstructionKeys];
  return key ? t(key, { finish_key: finishKey }) : presentUnknownCode(code, t);
}

export function presentRecordingHint(code: string, t: AppTranslator): string {
  const key = recordingHintKeys[code as keyof typeof recordingHintKeys];
  return key ? t(key) : presentUnknownCode(code, t);
}

const routeReasonKeys = {
  input_disabled: "routes.reasons.input_disabled",
  selection_unconfirmed: "routes.reasons.selection_unconfirmed",
  route_workflow_active: "routes.reasons.route_workflow_active",
  session_active: "routes.reasons.session_active",
  recording_preflight_failed: "routes.reasons.recording_preflight_failed",
  recording_start_area_mismatch: "routes.reasons.recording_start_area_mismatch",
  recording_terminal_area_mismatch: "routes.reasons.recording_terminal_area_mismatch",
  recording_boss_missing: "routes.reasons.recording_boss_missing",
  recording_object_missing: "routes.reasons.recording_object_missing",
  recording_boss_dead: "routes.reasons.recording_boss_dead",
  recording_endpoint_too_far: "routes.reasons.recording_endpoint_too_far",
  pickit_assignment_missing: "routes.reasons.pickit_assignment_missing",
  onboarding_teleport_binding_missing: "routes.reasons.onboarding_teleport_binding_missing",
  onboarding_town_portal_binding_missing: "routes.reasons.onboarding_town_portal_binding_missing",
  onboarding_waypoint_required: "routes.reasons.onboarding_waypoint_required",
  route_test_start_failed: "routes.reasons.route_test_start_failed",
  route_test_playback_failed: "routes.reasons.route_test_playback_failed",
  route_test_terminal_mismatch: "routes.reasons.route_test_terminal_mismatch",
  route_safety_return_failed: "routes.reasons.route_safety_return_failed",
  leg_acquisition_route_missing: "routes.reasons.leg_acquisition_route_missing",
  leg_acquisition_route_stale: "routes.reasons.leg_acquisition_route_stale",
  cow_sweep_route_missing: "routes.reasons.cow_sweep_route_missing",
  cow_sweep_route_stale: "routes.reasons.cow_sweep_route_stale",
  route_set_binding_mismatch: "routes.reasons.route_set_binding_mismatch",
  route_candidate_changed: "routes.reasons.route_candidate_changed",
} as const;

export function presentRouteReason(code: string, t: AppTranslator): string {
  const key = routeReasonKeys[code as keyof typeof routeReasonKeys];
  return t(key ?? "routes.reasons.unavailable");
}

export function presentHistoryReason(code: string, t: AppTranslator): string {
  if (!code) return "";
  let translated = "";
  try {
    translated = t(`historyReasons.${code}` as never, { defaultValue: "" } as never) as unknown as string;
  } catch {
    // Tests reject missing keys globally; history still needs the production
    // fallback for forward-compatible reason codes.
  }
  return translated || t("history.reasonFallback", { code });
}

const errorKeys = {
  request_failed: "errors.requestFailed",
  request_invalid: "errors.requestInvalid",
  payload_too_large: "errors.payloadTooLarge",
  request_unauthorized: "errors.unauthorized",
  control_token_missing: "errors.controlTokenMissing",
  origin_rejected: "errors.originRejected",
  api_version_unsupported: "errors.apiUnsupported",
  feature_unavailable: "errors.featureUnavailable",
  stream_unavailable: "errors.streamUnavailable",
  state_changed: "errors.stateChanged",
  command_conflict: "errors.commandConflict",
  command_invalid: "errors.commandInvalid",
  command_id_conflict: "errors.commandInvalid",
  input_not_ready: "errors.inputNotReady",
  runtime_read_failed: "errors.runtimeReadFailed",
  character_selection_unconfirmed: "errors.selectionUnconfirmed",
  selection_confirmation_invalid: "errors.selectionConfirmationInvalid",
  selection_persistence_failed: "errors.selectionPersistenceFailed",
  character_catalog_unavailable: "errors.characterCatalogUnavailable",
  character_setup_unavailable: "errors.characterSetupUnavailable",
  character_setup_write_failed: "errors.configInvalid",
  character_capture_failed: "errors.characterSetupUnavailable",
  character_save_missing: "errors.characterSaveMissing",
  character_profile_missing: "errors.characterProfileMissing",
  character_profile_incompatible: "errors.characterProfileIncompatible",
  config_unavailable: "errors.configUnavailable",
  config_invalid: "errors.configInvalid",
  config_revision_conflict: "errors.revisionConflict",
  config_schema_unsupported: "errors.schemaUnsupported",
  revision_conflict: "errors.revisionConflict",
  queue_empty: "errors.queueEmpty",
  queue_duplicate_run: "errors.queueDuplicate",
  queue_entry_unavailable: "errors.queueEntryUnavailable",
  queue_context_mismatch: "errors.queueContextMismatch",
  profile_bindings_incomplete: "errors.profileBindingsIncomplete",
  character_inventory_unconfigured: "errors.inventoryUnconfigured",
  game_start_failed: "errors.gameStartFailed",
  game_exit_failed: "errors.gameExitFailed",
  start_town_normalization_failed: "errors.startTownNormalizationFailed",
  unsupported_resolution: "errors.unsupportedResolution",
  run_catalog_refresh_failed: "errors.runCatalogRefreshFailed",
  route_feature_unavailable: "errors.routeUnavailable",
  route_catalog_unavailable: "errors.routeUnavailable",
  route_candidates_unavailable: "errors.routeUnavailable",
  route_preview_stale: "errors.routeConflict",
  route_workflow_conflict: "errors.routeConflict",
  route_manifest_corrupt: "errors.routeManifestCorrupt",
  route_manifest_write_failed: "errors.routeManifestWriteFailed",
  pickit_invalid: "errors.pickitInvalid",
  pickit_not_found: "errors.pickitNotFound",
  id_conflict: "errors.pickitIDConflict",
  profile_assigned: "errors.pickitAssigned",
  pickit_assignment_invalid: "errors.pickitAssignmentInvalid",
  history_unavailable: "errors.historyUnavailable",
  history_run_not_found: "errors.historyRunNotFound",
  history_retention_blocked: "errors.historyRetentionBlocked",
  history_file_invalid: "errors.historyRequestInvalid",
  history_file_too_large: "errors.historyRequestInvalid",
  history_line_too_large: "errors.historyRequestInvalid",
  history_schema_unsupported: "errors.historyRequestInvalid",
  history_event_invalid: "errors.historyRequestInvalid",
  history_context_missing: "errors.historyRequestInvalid",
  history_run_id_mismatch: "errors.historyRequestInvalid",
  history_stream_missing: "errors.historyRequestInvalid",
  history_terminal_duplicate: "errors.historyRequestInvalid",
  history_time_invalid: "errors.historyRequestInvalid",
  history_boss_duplicate: "errors.historyRequestInvalid",
  history_stage_invalid: "errors.historyRequestInvalid",
  history_item_identity_invalid: "errors.historyRequestInvalid",
  history_item_chain_invalid: "errors.historyRequestInvalid",
  history_filter_invalid: "errors.historyRequestInvalid",
  history_timezone_invalid: "errors.historyRequestInvalid",
  history_cursor_invalid: "errors.historyRequestInvalid",
  history_export_invalid: "errors.historyRequestInvalid",
  history_delete_preview_stale: "errors.revisionConflict",
  history_delete_active_protected: "errors.historyRetentionBlocked",
  history_delete_failed: "errors.historyUnavailable",
  history_retention_partial: "errors.historyUnavailable",
  diagnostic_bundle_failed: "errors.diagnosticFailed",
  diagnostic_content_rejected: "errors.diagnosticRejected",
  retry_return_failed: "errors.retryReturnFailed",
} as const;

export function presentProblem(problem: ProblemDTO, t: AppTranslator): string {
  if (problem.code === "retry_return_failed" && typeof problem.params?.original_reason === "string" && typeof problem.params?.recovery_reason === "string") {
    return t("errors.retryReturnFailedDetailed", {
      original: presentHistoryReason(problem.params.original_reason, t),
      recovery: presentHistoryReason(problem.params.recovery_reason, t),
    });
  }
  const key = errorKeys[problem.code as keyof typeof errorKeys];
  return key ? t(key, problem.params ?? {}) : presentUnknownCode(problem.code, t);
}

export function presentApiError(reason: unknown, t: AppTranslator, fallback: string): string {
  return isApiError(reason) ? presentProblem({ code: reason.code, params: reason.params }, t) : fallback;
}

export function apiErrorCode(reason: unknown): string | undefined {
  return isApiError(reason) ? reason.code : undefined;
}

function isApiError(reason: unknown): reason is ApiError {
  if (!reason || typeof reason !== "object") return false;
  const candidate = reason as Partial<ApiError>;
  return typeof candidate.code === "string" && typeof candidate.requestId === "string" && typeof candidate.status === "number";
}
