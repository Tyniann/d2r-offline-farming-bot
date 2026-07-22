// Code generated from internal/api/schema/openapi.json; DO NOT EDIT.

export interface StatusDTO {
  schema_version: number;
  core_version: string;
  state: string;
  generation: number;
  lifecycle_phase: string;
  pending_intent?: string;
  active_run_id?: string;
  run_id?: string;
  game_id?: string;
  step?: string;
  d2r: D2RDTO;
  input: InputDTO;
  world: WorldDTO;
  selection: SelectionStatusDTO;
  queue: QueueStatusDTO;
  last_result?: SessionResultDTO;
  last_error?: ErrorDTO;
}

export interface SessionResultDTO {
  disposition: string;
  reason?: string;
}

export interface SelectionStatusDTO {
  character?: string;
  difficulty?: string;
}

export interface QueueStatusDTO {
  entries: Array<string>;
  default_entries: Array<string>;
  index: number;
  cycle: number;
  retry: number;
  started_runs: number;
  consecutive_failures: number;
  total_restarts: number;
  budgets: QueueBudgetsDTO;
}

export interface QueueBudgetsDTO {
  max_runs: number;
  max_duration_ms: number;
  max_consecutive_failures: number;
  max_total_restarts: number;
}

export interface D2RDTO {
  state: string;
  pid?: number;
  window_bound: boolean;
  client_width?: number;
  client_height?: number;
}

export interface InputDTO {
  enabled: boolean;
  paused: boolean;
  stopped: boolean;
}

export interface WorldDTO {
  valid: boolean;
  phase: string;
  area_id?: number;
  area_name?: string;
}

export interface LiveEvent {
  sequence: number;
  timestamp: string;
  event: string;
  session_id?: string;
  game_id?: string;
  run_id?: string;
  run?: string;
  act?: string;
  step?: string;
  area_id?: number;
  area?: string;
  reason?: string;
  workflow_id?: string;
  state?: string;
  segment?: number;
  progress?: number;
  details?: Record<string, unknown>;
}

export interface CatalogDTO {
  schema_version: number;
  revision: number;
  default_difficulty: string;
  characters: Array<CharacterCatalogEntry>;
  difficulties: Array<DifficultyCatalogEntry>;
  profiles: Array<ProfileCatalogEntry>;
  runs: Array<RunCatalogEntry>;
}

export interface CharacterCatalogEntry {
  name: string;
  slug: string;
  expected_class?: string;
  selectable: boolean;
  reasons?: Array<string>;
}

export interface DifficultyCatalogEntry {
  id: string;
  display_name: string;
}

export interface ProfileCatalogEntry {
  id: string;
  character_class: string;
}

export interface RunCatalogEntry {
  run_id: string;
  display_name: string;
  status: string;
  reasons?: Array<string>;
}

export interface CommandRequest {
  command_id: string;
  expected_generation: number;
  payload?: Record<string, unknown>;
}

export interface CommandResponse {
  schema_version: number;
  command_id: string;
  generation: number;
  state: string;
}

export interface SelectionPreviewRequest {
  character: string;
  difficulty: string;
  catalog_revision: number;
}

export interface SelectionPreviewDTO {
  schema_version: number;
  character: string;
  old_difficulty?: string;
  new_difficulty: string;
  affected_routes: Array<string>;
  invalidation_reason?: string;
  requires_confirmation: boolean;
  confirmation_token: string;
  catalog_revision: number;
  lifecycle_revision: number;
}

export interface QueueValidationRequest {
  entries: Array<string>;
  character: string;
  difficulty: string;
  catalog_revision: number;
}

export interface QueueValidationDTO {
  schema_version: number;
  entries: Array<string>;
  character: string;
  difficulty: string;
  catalog_revision: number;
  budgets: QueueBudgetsDTO;
}

export interface SessionStartPayload {
  entries: Array<string>;
  character: string;
  difficulty: string;
  catalog_revision: number;
}

export interface RouteLibraryDTO {
  schema_version: number;
  revision: number;
  character: string;
  routes: Array<RouteEntryDTO>;
}

export interface RouteEntryDTO {
  route_id: string;
  display_name: string;
  run_id: string;
  character: string;
  difficulty: string;
  lifecycle_status: string;
  management_status: string;
  assigned: boolean;
  reason?: string;
}

export interface RecordingOptionDTO {
  run_id: string;
  display_name: string;
  instructions_de: string;
  start_waypoint: string;
  allowed_start_area_id: number;
  allowed_route_area_ids: Array<number>;
  terminal_area_id: number;
  terminal_max_distance_tiles: number;
  available: boolean;
  reason?: string;
}

export interface RouteCandidateDTO {
  candidate_id: string;
  run_id: string;
  character: string;
  difficulty: string;
  state: string;
  measured_boss_distance: number;
  route_sha256: string;
  reason?: string;
}

export interface SystemRouteStatusDTO {
  act: string;
  ready: boolean;
  reason?: string;
}

export interface HotkeyHelpDTO {
  recording_finish: string;
  stop_after_run: string;
  emergency_stop: string;
  pause: string;
}

export interface RouteWorkflowDTO {
  workflow_id: string;
  generation: number;
  state: string;
  run_id: string;
  character: string;
  act?: string;
  area_id?: number;
  segment?: number;
  progress?: number;
  reason?: string;
}

export interface RouteMutationPreviewDTO {
  operation: string;
  route_id: string;
  candidate_id?: string;
  replaced_route_id?: string;
  catalog_revision: number;
  lifecycle_revision: number;
  assignment_revision: number;
  confirmation_token: string;
}

export interface RouteWorkflowRequest {
  expected_generation: number;
  operation: string;
  run_id?: string;
  candidate_id?: string;
  act?: string;
}

export interface RouteRecordingStartRequest {
  expected_generation: number;
  run_id: string;
}

export interface RouteWorkflowFinishRequest {
  expected_generation: number;
}

export interface RouteMutationPreviewRequest {
  operation: string;
  route_id?: string;
  candidate_id?: string;
}

export interface RouteMutationConfirmRequest {
  confirmation_token: string;
  confirm_route_id?: string;
}

export interface PickitCatalogDTO {
  schema_version: number;
  catalog_version: string;
  bases: Array<PickitBaseDTO>;
  identities: Array<PickitIdentityDTO>;
  actions: Array<string>;
  qualities: Array<string>;
  speed_categories: Array<string>;
}

export interface PickitBaseDTO {
  txt_file_no: number;
  code: string;
  name: string;
  type: string;
  base_tier: string;
}

export interface PickitIdentityDTO {
  kind: string;
  raw_id: number;
  key: string;
  display_name: string;
  base_code: string;
  set_key?: string;
  set_name?: string;
  spawnable: boolean;
}

export interface PickitRuleDTO {
  id: string;
  action: string;
  expression: string;
}

export interface PickitProfileDTO {
  schema_version: number;
  revision: number;
  id: string;
  name: string;
  rules: Array<PickitRuleDTO>;
}

export interface PickitProfilesDTO {
  profiles: Array<PickitProfileDTO>;
  assignment_revision: number;
}

export interface PickitValidationRequest {
  profile: PickitProfileDTO;
}

export interface PickitValidationDTO {
  valid: boolean;
  profile: PickitProfileDTO;
}

export interface PickitPreviewItemDTO {
  code: string;
  name?: string;
  type?: string;
  quality: string;
  identity_kind?: string;
  identity_key?: string;
  identity_available?: boolean;
  identity_valid?: boolean;
  identified?: boolean;
  ethereal?: boolean;
}

export interface PickitPreviewRequest {
  profile: PickitProfileDTO;
  item: PickitPreviewItemDTO;
}

export interface PickitTraceDTO {
  rule_index: number;
  profile_id: string;
  rule_id: string;
  action: string;
  expression: string;
  matched: boolean;
  profile_revision: number;
  assignment_revision: number;
}

export interface PickitPreviewDTO {
  matched: boolean;
  rule_index: number;
  profile_id: string;
  rule_id: string;
  action: string;
  profile_revision: number;
  assignment_revision: number;
  trace: Array<PickitTraceDTO>;
}

export interface PickitCreateRequest {
  profile: PickitProfileDTO;
}

export interface PickitUpdateRequest {
  expected_revision: number;
  profile: PickitProfileDTO;
}

export interface PickitDuplicateRequest {
  target_id: string;
  target_name: string;
}

export interface PickitDeleteRequest {
  expected_revision: number;
}

export interface PickitAssignmentsDTO {
  schema_version: number;
  revision: number;
  assignments: Record<string, unknown>;
}

export interface PickitAssignmentUpdateRequest {
  character: string;
  run_id: string;
  profile_ids: Array<string>;
  expected_revision: number;
}

export interface PickitImportRequest {
  text: string;
  action: string;
}

export interface PickitImportDTO {
  rules: Array<PickitRuleDTO>;
  warnings: Array<string>;
}

export interface PickitExportDTO {
  profile_id: string;
  revision: number;
  text: string;
  warning: string;
}

export interface HistoryFilterDTO {
  from_utc?: string;
  to_utc?: string;
  runs: Array<string>;
  characters: Array<string>;
  difficulties: Array<string>;
  outcomes: Array<string>;
  reasons: Array<string>;
  pickit_profiles: Array<string>;
  sort?: "keep_per_hour" | "success_rate" | "average_duration";
}

export interface HistoryDiagnosticDTO {
  file: string;
  code: string;
  message: string;
}

export interface HistoryMetaDTO {
  schema_version: number;
  generated_at: string;
  timezone: string;
  index_generation: number;
  filter: HistoryFilterDTO;
  diagnostics: Array<HistoryDiagnosticDTO>;
  ignored_files: number;
}

export interface HistoryDurationDTO {
  count: number;
  total_ms: number;
  average_ms: number;
  median_ms: number;
  minimum_ms: number;
  maximum_ms: number;
}

export interface HistoryStagesDTO {
  travel_ms: number;
  combat_ms: number;
  loot_ms: number;
  return_town_ms: number;
  other_ms: number;
}

export interface HistoryFunnelDTO {
  seen: number;
  matched: number;
  picked_up: number;
  stashed: number;
  sold: number;
  keep_return: number;
  pickup_lost: number;
  post_pickup_lost: number;
}

export interface HistoryFailureDTO {
  step: string;
  reason: string;
  reason_message: string;
  count: number;
  lost_duration_ms: number;
}

export interface HistorySummaryDTO {
  runs: number;
  terminal_runs: number;
  successful: number;
  failed: number;
  aborted: number;
  incomplete: number;
  running: number;
  success_rate?: number;
  boss_kills: number;
  durations: HistoryDurationDTO;
  stages: HistoryStagesDTO;
  funnel: HistoryFunnelDTO;
  keep_per_run?: number;
  keep_per_kill?: number;
  keep_per_hour?: number;
  top_failure?: HistoryFailureDTO;
}

export interface HistoryComparisonDTO {
  id: string;
  character: string;
  difficulty: string;
  definition_id: string;
  run: string;
  route_id: string;
  terminal_runs: number;
  successful: number;
  failed: number;
  aborted: number;
  success_rate?: number;
  boss_kills: number;
  low_sample: boolean;
  durations: HistoryDurationDTO;
  stages: HistoryStagesDTO;
  funnel: HistoryFunnelDTO;
  keep_per_run?: number;
  keep_per_kill?: number;
  keep_per_hour?: number;
  top_failure?: HistoryFailureDTO;
}

export interface HistoryItemDTO {
  item_key: string;
  item_name: string;
  base_code?: string;
  quality?: string;
  seen: number;
  matched: number;
  picked_up: number;
  stashed: number;
  sold: number;
  pickup_lost: number;
  post_pickup_lost: number;
  yield_per_run?: number;
  yield_per_kill?: number;
  yield_per_hour?: number;
}

export interface HistoryRunDTO {
  run_id: string;
  started_at: string;
  observed_at: string;
  character: string;
  difficulty: string;
  run: string;
  definition_id: string;
  route_id: string;
  outcome: string;
  reason?: string;
  reason_message?: string;
  last_step?: string;
  duration_ms: number;
  boss_kills: number;
  funnel: HistoryFunnelDTO;
}

export interface HistoryRunItemDTO {
  unit_id: number;
  item_key?: string;
  item_name?: string;
  base_code?: string;
  quality?: string;
  identity_kind?: string;
  identity_key?: string;
  pickit_profile_id?: string;
  pickit_rule_id?: string;
  pickit_action?: string;
  pickit_profile_revision?: number;
  pickit_assignment_revision?: number;
  seen: boolean;
  matched: boolean;
  picked_up: boolean;
  stashed: boolean;
  sold: boolean;
  pickup_lost: boolean;
  post_pickup_lost: boolean;
}

export interface HistoryRunDetailDTO {
  run_id: string;
  started_at: string;
  observed_at: string;
  character: string;
  difficulty: string;
  run: string;
  definition_id: string;
  route_id: string;
  outcome: string;
  reason?: string;
  reason_message?: string;
  last_step?: string;
  duration_ms: number;
  boss_kills: number;
  funnel: HistoryFunnelDTO;
  ended_at?: string;
  route_layout_fingerprint?: string;
  stages: HistoryStagesDTO;
  items: Array<HistoryRunItemDTO>;
  raw_events?: Array<Record<string, unknown>>;
}

export interface HistorySummaryResponse {
  meta: HistoryMetaDTO;
  summary: HistorySummaryDTO;
}

export interface HistoryComparisonsResponse {
  meta: HistoryMetaDTO;
  comparisons: Array<HistoryComparisonDTO>;
}

export interface HistoryItemsResponse {
  meta: HistoryMetaDTO;
  items: Array<HistoryItemDTO>;
  next_cursor?: string;
}

export interface HistoryRunsResponse {
  meta: HistoryMetaDTO;
  runs: Array<HistoryRunDTO>;
  next_cursor?: string;
}

export interface HistoryRunDetailResponse {
  meta: HistoryMetaDTO;
  run: HistoryRunDetailDTO;
}

export interface HistoryReportDTO {
  meta: HistoryMetaDTO;
  summary: HistorySummaryDTO;
  comparisons: Array<HistoryComparisonDTO>;
  items: Array<HistoryItemDTO>;
  runs: Array<HistoryRunDTO>;
}

export interface ErrorDTO {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  request_id: string;
}

export const API_VERSION = "v1" as const;

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) { const error = await response.json().catch(() => null) as { message?: string } | null; throw new Error(error?.message ?? `API-Abfrage fehlgeschlagen (${response.status})`); }
  return response.json() as Promise<T>;
}

async function sendJSON<T>(path: string, method: string, body: unknown, token = "", signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json", "Content-Type": "application/json" };
  if (token) headers["X-D2RBot-Control-Token"] = token;
  const response = await fetch(path, { method, body: JSON.stringify(body), signal, headers });
  if (!response.ok) { const error = await response.json().catch(() => null) as { message?: string } | null; throw new Error(error?.message ?? `API-Mutation fehlgeschlagen (${response.status})`); }
  return response.status === 204 ? (undefined as T) : response.json() as Promise<T>;
}

export function getStatus(signal?: AbortSignal): Promise<StatusDTO> {
  return getJSON<StatusDTO>("/api/v1/status", signal);
}

export function getCatalog(signal?: AbortSignal): Promise<CatalogDTO> {
  return getJSON<CatalogDTO>("/api/v1/catalog", signal);
}

export interface HistoryQuery {
  from?: string;
  to?: string;
  run?: string[];
  character?: string[];
  difficulty?: string[];
  outcome?: string[];
  reason?: string[];
  pickit_profile?: string[];
  sort?: "keep_per_hour" | "success_rate" | "average_duration";
  limit?: number;
  cursor?: string;
}

function historyQuery(query: HistoryQuery = {}): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === "" || Array.isArray(value) && value.length === 0) continue;
    if (Array.isArray(value)) value.forEach((entry) => params.append(key, entry));
    else params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? "?" + encoded : "";
}

export function getHistorySummary(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistorySummaryResponse> { return getJSON<HistorySummaryResponse>("/api/v1/history/summary" + historyQuery(query), signal); }
export function getHistoryComparisons(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistoryComparisonsResponse> { return getJSON<HistoryComparisonsResponse>("/api/v1/history/comparisons" + historyQuery(query), signal); }
export function getHistoryItems(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistoryItemsResponse> { return getJSON<HistoryItemsResponse>("/api/v1/history/items" + historyQuery(query), signal); }
export function getHistoryRuns(query: HistoryQuery = {}, signal?: AbortSignal): Promise<HistoryRunsResponse> { return getJSON<HistoryRunsResponse>("/api/v1/history/runs" + historyQuery(query), signal); }
export function getHistoryRun(runID: string, includeRaw = false, signal?: AbortSignal): Promise<HistoryRunDetailResponse> { return getJSON<HistoryRunDetailResponse>(`/api/v1/history/runs/${encodeURIComponent(runID)}?include_raw=${includeRaw}`, signal); }
export function getHistoryExportURL(format: "json" | "csv", dataset: "" | "runs" | "items" = "", query: HistoryQuery = {}): string {
  const suffix = historyQuery(query);
  const separator = suffix ? "&" : "?";
  return "/api/v1/history/export" + suffix + separator + new URLSearchParams({ format, ...(dataset ? { dataset } : {}) }).toString();
}
export async function downloadHistoryExport(format: "json" | "csv", dataset: "" | "runs" | "items" = "", query: HistoryQuery = {}, signal?: AbortSignal): Promise<{ blob: Blob; filename: string }> {
  const response = await fetch(getHistoryExportURL(format, dataset, query), { signal, headers: { Accept: format === "json" ? "application/json" : "text/csv" } });
  if (!response.ok) { const error = await response.json().catch(() => null) as { message?: string } | null; throw new Error(error?.message ?? `Historienexport fehlgeschlagen (${response.status})`); }
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const filename = disposition.match(/filename="([A-Za-z0-9._-]+)"/)?.[1] ?? `d2r-history.${format}`;
  return { blob: await response.blob(), filename };
}

export function getPickitCatalog(signal?: AbortSignal): Promise<PickitCatalogDTO> { return getJSON<PickitCatalogDTO>("/api/v1/pickit/catalog", signal); }
export function getPickitProfiles(signal?: AbortSignal): Promise<PickitProfilesDTO> { return getJSON<PickitProfilesDTO>("/api/v1/pickit/profiles", signal); }
export function validatePickitProfile(request: PickitValidationRequest, signal?: AbortSignal): Promise<PickitValidationDTO> { return sendJSON<PickitValidationDTO>("/api/v1/pickit/profiles/validate", "POST", request, "", signal); }
export function previewPickit(request: PickitPreviewRequest, signal?: AbortSignal): Promise<PickitPreviewDTO> { return sendJSON<PickitPreviewDTO>("/api/v1/pickit/preview", "POST", request, "", signal); }
export function createPickitProfile(request: PickitCreateRequest, token: string, signal?: AbortSignal): Promise<PickitProfileDTO> { return sendJSON<PickitProfileDTO>("/api/v1/pickit/profiles", "POST", request, token, signal); }
export function updatePickitProfile(id: string, request: PickitUpdateRequest, token: string, signal?: AbortSignal): Promise<PickitProfileDTO> { return sendJSON<PickitProfileDTO>(`/api/v1/pickit/profiles/${encodeURIComponent(id)}`, "PUT", request, token, signal); }
export function duplicatePickitProfile(id: string, request: PickitDuplicateRequest, token: string, signal?: AbortSignal): Promise<PickitProfileDTO> { return sendJSON<PickitProfileDTO>(`/api/v1/pickit/profiles/${encodeURIComponent(id)}/duplicate`, "POST", request, token, signal); }
export function deletePickitProfile(id: string, request: PickitDeleteRequest, token: string, signal?: AbortSignal): Promise<void> { return sendJSON<void>(`/api/v1/pickit/profiles/${encodeURIComponent(id)}`, "DELETE", request, token, signal); }
export function getPickitAssignments(signal?: AbortSignal): Promise<PickitAssignmentsDTO> { return getJSON<PickitAssignmentsDTO>("/api/v1/pickit/assignments", signal); }
export function updatePickitAssignment(request: PickitAssignmentUpdateRequest, token: string, signal?: AbortSignal): Promise<PickitAssignmentsDTO> { return sendJSON<PickitAssignmentsDTO>("/api/v1/pickit/assignments", "PUT", request, token, signal); }
export function importPickit(request: PickitImportRequest, signal?: AbortSignal): Promise<PickitImportDTO> { return sendJSON<PickitImportDTO>("/api/v1/pickit/import", "POST", request, "", signal); }
export function exportPickitProfile(id: string, signal?: AbortSignal): Promise<PickitExportDTO> { return getJSON<PickitExportDTO>(`/api/v1/pickit/profiles/${encodeURIComponent(id)}/export`, signal); }

export function getRouteLibrary(character = "", archived = false, signal?: AbortSignal): Promise<RouteLibraryDTO> {
  const query = new URLSearchParams({ character, include_archived: archived ? "true" : "false" });
  return getJSON<RouteLibraryDTO>("/api/v1/routes?" + query.toString(), signal);
}

export function getRecordingOptions(signal?: AbortSignal): Promise<Array<RecordingOptionDTO>> {
  return getJSON<Array<RecordingOptionDTO>>("/api/v1/route-recording/options", signal);
}

export function getRouteCandidates(signal?: AbortSignal): Promise<Array<RouteCandidateDTO>> {
  return getJSON<Array<RouteCandidateDTO>>("/api/v1/routes/candidates", signal);
}

export function getSystemRouteStatus(signal?: AbortSignal): Promise<Array<SystemRouteStatusDTO>> {
  return getJSON<Array<SystemRouteStatusDTO>>("/api/v1/system-routes/status", signal);
}

export function getHotkeyHelp(signal?: AbortSignal): Promise<HotkeyHelpDTO> {
  return getJSON<HotkeyHelpDTO>("/api/v1/routes/hotkeys", signal);
}

export function getRouteWorkflow(signal?: AbortSignal): Promise<RouteWorkflowDTO> {
  return getJSON<RouteWorkflowDTO>("/api/v1/routes/workflow", signal);
}
