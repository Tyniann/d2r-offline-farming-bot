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
