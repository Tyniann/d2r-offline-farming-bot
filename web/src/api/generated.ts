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
  step?: string;
  area_id?: number;
  area?: string;
  reason?: string;
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

export interface ErrorDTO {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  request_id: string;
}

export const API_VERSION = "v1" as const;

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) throw new Error(`API-Abfrage fehlgeschlagen (${response.status})`);
  return response.json() as Promise<T>;
}

export function getStatus(signal?: AbortSignal): Promise<StatusDTO> {
  return getJSON<StatusDTO>("/api/v1/status", signal);
}

export function getCatalog(signal?: AbortSignal): Promise<CatalogDTO> {
  return getJSON<CatalogDTO>("/api/v1/catalog", signal);
}
